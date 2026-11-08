// Command agent-ab runs real Claude Code sessions per task and arm, with and
// without Stacklit, and records the usage, tool calls and correctness of each.
// It spends real quota, so it refuses to run without --yes-spend, keeps a
// running ledger and stops at a hard cap or on the first limit or auth error.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type repo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	SHA  string `json:"sha"`
	// Files is the count of tracked files at SHA; the decision rule uses it
	// to find the repos above its size threshold.
	Files int `json:"files,omitempty"`
}

// task is one pinned question. A session is correct when every AllOf path and
// at least one AnyOf path (if any are listed) is named in the answer.
type task struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Question string   `json:"question"`
	AllOf    []string `json:"all_of"`
	AnyOf    []string `json:"any_of"`
	Rubric   string   `json:"rubric"`
	// Check and Verify let a pinned checkout prove the answer with grep
	// (see verify.go); the harness itself does not read them.
	Check  []fileCheck `json:"check,omitempty"`
	Verify *importScan `json:"verify,omitempty"`
}

type unit struct {
	Task  task   `json:"task"`
	Trial int    `json:"trial"`
	Arm   string `json:"arm"`
}

type usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
}

// record is one finished session, written as raw JSON under results/.
type record struct {
	Repo           string         `json:"repo"`
	Task           string         `json:"task"`
	Kind           string         `json:"kind"`
	Trial          int            `json:"trial"`
	Arm            string         `json:"arm"`
	Model          string         `json:"model"`
	Correct        bool           `json:"correct"`
	Recall         float64        `json:"recall"`
	Answer         string         `json:"answer"`
	Turns          int            `json:"turns"`
	CostUSD        float64        `json:"cost_usd"`
	Usage          usage          `json:"usage"`
	ToolCalls      map[string]int `json:"tool_calls"`
	FilesRead      int            `json:"files_read"`
	ReadBytes      int            `json:"read_bytes"`
	StacklitCalls  int            `json:"stacklit_calls"`
	HookEvents     int            `json:"hook_events"`
	CrawlFired     bool           `json:"crawl_guard_fired"`
	CrawlRedirects int            `json:"crawl_redirects"`
	Denials        int            `json:"permission_denials"`
	Subtype        string         `json:"result_subtype"`
	Seconds        float64        `json:"seconds"`
	WeekUtil       float64        `json:"seven_day_utilization"`
	Calls          []string       `json:"call_log"`
	Binary         string         `json:"stacklit_bin,omitempty"`
	Error          string         `json:"error,omitempty"`
	At             string         `json:"at"`
}

type config struct {
	repo, reposFile, tasksDir, stacklit, claude, model, resultsDir, ledger, arms, work, mode, only string
	// grader is "audited" or "prereg" (round 1). smallRepos lists the repos
	// where the small repo arm runs; binSHA identifies the stacklit binary.
	grader, smallRepos, binSHA string
	smallFiles                 int
	trials, limit, maxTurns    int
	seed                       int64
	yesSpend, dryRun, analysis bool
	maxBudget, cap, estMid     float64
}

// errStop means a limit, auth or cap condition: the whole run must halt.
var errStop = errors.New("stop")

func main() {
	var c config
	flag.StringVar(&c.repo, "repo", "theauth-go", "repo name from the repos file, or all")
	flag.StringVar(&c.reposFile, "repos", "bench/agent-ab/repos.json", "pinned repos file")
	flag.StringVar(&c.tasksDir, "tasks", "bench/agent-ab/tasks", "directory of <repo>.json task files")
	flag.StringVar(&c.stacklit, "stacklit", "stacklit", "stacklit binary for the treatment arm(s)")
	flag.StringVar(&c.claude, "claude", "claude", "claude binary")
	flag.StringVar(&c.model, "model", "sonnet", "model alias passed to claude")
	flag.StringVar(&c.resultsDir, "results", "bench/agent-ab/results", "directory for per-session JSON")
	flag.StringVar(&c.ledger, "ledger", "bench/agent-ab/ledger.jsonl", "append-only cost ledger")
	flag.StringVar(&c.arms, "arms", "control,stacklit", "arms to run: control, stacklit, strict, stacklit-e, stacklit-e-small")
	flag.StringVar(&c.grader, "grader", "audited", "answer matcher: audited (accepts ./path and absolute checkout paths, round 2) or prereg (round 1)")
	flag.StringVar(&c.smallRepos, "small-repos", "featuredrop", "repos where the stacklit-e-small arm runs (the others are above the small repo limit)")
	flag.IntVar(&c.smallFiles, "small-repo-files", 600, "STACKLIT_SMALL_REPO_FILES value for the stacklit-e-small arm")
	flag.StringVar(&c.work, "work", "/tmp/abreal/work", "temp dir for clones (deleted afterwards)")
	flag.StringVar(&c.mode, "mode", "run", "run, or memory (all tools disabled, answers from memory)")
	flag.StringVar(&c.only, "only", "", "comma separated task ids to run (default all, or the first --limit)")
	flag.IntVar(&c.trials, "trials", 3, "trials per task and arm")
	flag.IntVar(&c.limit, "limit", 0, "use only the first N tasks (0 = all)")
	flag.IntVar(&c.maxTurns, "max-turns", 25, "turn cap per session")
	flag.Int64Var(&c.seed, "seed", 1, "seed for the session order")
	flag.Float64Var(&c.maxBudget, "max-budget-usd", 1.5, "per-session spend cap passed to claude")
	flag.Float64Var(&c.cap, "cap-usd", 60, "total notional spend cap across the ledger")
	flag.Float64Var(&c.estMid, "est-session-usd", 0.03, "assumed USD per session, for the dry-run projection only (round 1 observed 0.027)")
	flag.BoolVar(&c.dryRun, "dry-run", false, "print the plan, run nothing")
	flag.BoolVar(&c.yesSpend, "yes-spend", false, "required: confirm you accept the spend of real agent sessions")
	flag.BoolVar(&c.analysis, "analyze", false, "analyze existing results into summary.json and tables.md, run nothing")
	flag.Parse()
	if c.analysis {
		names := []string{"levelrail", "theauth-go", "featuredrop"}
		if n, err := repoNames(c.reposFile); err == nil && len(n) > 0 {
			names = n
		}
		if err := analyze(c, names); err != nil {
			fmt.Fprintln(os.Stderr, "agent-ab:", err)
			os.Exit(1)
		}
		return
	}
	runner := run
	if c.repo == "all" {
		runner = runAll
	}
	if err := runner(c); err != nil {
		fmt.Fprintln(os.Stderr, "agent-ab:", err)
		os.Exit(1)
	}
}

// repoNames lists the repos of the repos file, in file order.
func repoNames(file string) ([]string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var rs []repo
	if err := json.Unmarshal(b, &rs); err != nil {
		return nil, err
	}
	var names []string
	for _, r := range rs {
		names = append(names, r.Name)
	}
	return names, nil
}

// armsFor returns the arms to run on one repo. The small repo arm only runs
// where the pre-registration says it applies.
func armsFor(c config, repoName string) []string {
	var arms []string
	for _, a := range strings.Split(c.arms, ",") {
		a = strings.TrimSpace(a)
		if a == "stacklit-e-small" && !inList(c.smallRepos, repoName) {
			continue
		}
		if a != "" {
			arms = append(arms, a)
		}
	}
	return arms
}

func inList(csv, name string) bool {
	for _, x := range strings.Split(csv, ",") {
		if strings.TrimSpace(x) == name {
			return true
		}
	}
	return false
}

// runAll runs (or plans) every repo of the repos file in order and prints the
// projected total. The ledger cap applies across all of them.
func runAll(c config) error {
	names, err := repoNames(c.reposFile)
	if err != nil {
		return err
	}
	total := 0
	for _, n := range names {
		cc := c
		cc.repo = n
		tasks, _, err := load(cc)
		if err != nil {
			return err
		}
		total += len(plan(tasks, cc.trials, cc.seed, armsFor(cc, n)))
		if err := run(cc); err != nil {
			return err
		}
	}
	fmt.Printf("all repos: %d sessions; assumed %.2f USD each = %.2f USD (assumption); total cap %.2f\n", total, c.estMid, c.estMid*float64(total), c.cap)
	return nil
}

// binaryID is a short hash of the stacklit binary, written into every record
// so a result can be tied to the build that produced it.
func binaryID(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:6])
}

func run(c config) error {
	tasks, r, err := load(c)
	if err != nil {
		return err
	}
	arms := armsFor(c, c.repo)
	if c.mode == "memory" {
		arms = []string{"memory"}
		c.trials = 1
	}
	c.binSHA = binaryID(c.stacklit)
	units := plan(tasks, c.trials, c.seed, arms)
	fmt.Printf("plan %s: %d tasks x %d trials x %d arm(s) %v = %d sessions; assumed %.2f USD each = %.2f USD (assumption); per-session cap %.2f, total cap %.2f\n",
		c.repo, len(tasks), c.trials, len(arms), arms, len(units), c.estMid, c.estMid*float64(len(units)), c.maxBudget, c.cap)
	if c.dryRun {
		return nil
	}
	if !c.yesSpend {
		return errors.New("refusing to start paid agent sessions without --yes-spend (use --dry-run to see the plan)")
	}
	if err := checkIsolation(); err != nil {
		return err
	}
	if err := os.MkdirAll(c.work, 0o755); err != nil {
		return err
	}
	base, err := os.MkdirTemp(c.work, "ab-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base)
	if c.mode != "memory" {
		if err := cloneBase(base, r); err != nil {
			return err
		}
	}
	for i, u := range units {
		out := resultPath(c, u)
		if _, err := os.Stat(out); err == nil {
			continue
		}
		total, err := ledgerTotal(c.ledger)
		if err != nil {
			return err
		}
		if total+c.maxBudget > c.cap {
			return fmt.Errorf("%w: ledger total %.2f plus a session cap %.2f would pass the %.2f cap", errStop, total, c.maxBudget, c.cap)
		}
		fmt.Printf("[%d/%d] %s %s trial %d arm %s (ledger %.2f)\n", i+1, len(units), r.Name, u.Task.ID, u.Trial, u.Arm, total)
		rec, stop := runUnit(c, r, base, u)
		if rec.Error != "" {
			fmt.Printf("   error: %s\n", rec.Error)
		}
		if stop != nil {
			return stop
		}
		if rec.Error != "" {
			continue // harness failure: not saved, so a rerun retries it
		}
		if err := writeRecord(out, rec); err != nil {
			return err
		}
		if err := appendLedger(c.ledger, rec, total+rec.CostUSD); err != nil {
			return err
		}
		fmt.Printf("   correct=%v cost=%.3f turns=%d calls=%d stacklit=%d\n", rec.Correct, rec.CostUSD, rec.Turns, sum(rec.ToolCalls), rec.StacklitCalls)
	}
	return nil
}

func sum(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func resultPath(c config, u unit) string {
	return filepath.Join(c.resultsDir, c.repo, u.Arm, fmt.Sprintf("%s-t%d.json", u.Task.ID, u.Trial))
}

func writeRecord(path string, rec record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(rec, "", "  ")
	return os.WriteFile(path, b, 0o644)
}

func ledgerTotal(path string) (float64, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var total float64
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l == "" {
			continue
		}
		var e struct {
			Cost float64 `json:"cost_usd"`
		}
		if json.Unmarshal([]byte(l), &e) == nil {
			total += e.Cost
		}
	}
	return total, nil
}

func appendLedger(path string, rec record, running float64) error {
	e := map[string]any{"at": rec.At, "repo": rec.Repo, "task": rec.Task, "trial": rec.Trial, "arm": rec.Arm,
		"model": rec.Model, "cost_usd": rec.CostUSD, "running_total_usd": running}
	b, _ := json.Marshal(e)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func load(c config) ([]task, repo, error) {
	var repos []repo
	b, err := os.ReadFile(c.reposFile)
	if err != nil {
		return nil, repo{}, err
	}
	if err := json.Unmarshal(b, &repos); err != nil {
		return nil, repo{}, err
	}
	var r repo
	for _, x := range repos {
		if x.Name == c.repo {
			r = x
		}
	}
	if r.SHA == "" {
		return nil, r, fmt.Errorf("repo %q has no pinned sha in %s", c.repo, c.reposFile)
	}
	tb, err := os.ReadFile(filepath.Join(c.tasksDir, c.repo+".json"))
	if err != nil {
		return nil, r, err
	}
	var tasks []task
	if err := json.Unmarshal(tb, &tasks); err != nil {
		return nil, r, err
	}
	if c.only != "" {
		want := map[string]bool{}
		for _, id := range strings.Split(c.only, ",") {
			want[strings.TrimSpace(id)] = true
		}
		var kept []task
		for _, t := range tasks {
			if want[t.ID] {
				kept = append(kept, t)
			}
		}
		tasks = kept
	} else if c.limit > 0 && len(tasks) > c.limit {
		tasks = tasks[:c.limit]
	}
	return tasks, r, nil
}

// plan expands tasks x trials x arms and shuffles with a seed, so arms are
// interleaved and the order repeats.
func plan(tasks []task, trials int, seed int64, arms []string) []unit {
	var us []unit
	for _, t := range tasks {
		for k := 1; k <= trials; k++ {
			for _, a := range arms {
				us = append(us, unit{Task: t, Trial: k, Arm: a})
			}
		}
	}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(us), func(i, j int) { us[i], us[j] = us[j], us[i] })
	return us
}

// checkIsolation refuses to run when the user's own settings already wire
// stacklit in, which would contaminate the control arm.
func checkIsolation() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err == nil && strings.Contains(string(b), "stacklit") {
		return errors.New("~/.claude/settings.json mentions stacklit; remove it so the control arm is clean")
	}
	return nil
}

func sh(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

// cloneBase fetches exactly the pinned commit, shallow.
func cloneBase(base string, r repo) error {
	dst := filepath.Join(base, "base")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, a := range [][]string{{"init", "-q"}, {"remote", "add", "origin", r.URL}, {"fetch", "-q", "--depth", "1", "origin", r.SHA}, {"checkout", "-q", "FETCH_HEAD"}} {
		if err := sh(dst, "git", a...); err != nil {
			return err
		}
	}
	return nil
}

const answerSuffix = " Answer with the file paths only, comma separated, on one line."

// armEnv is the environment that distinguishes the treatment arms. It applies
// to both setup and the session, so the hooks see the same values the rules
// block was written with. Small repo mode is off (0) in every arm but
// stacklit-e-small, so the arms differ in one thing each.
func armEnv(c config, arm string) []string {
	switch arm {
	case "strict":
		return []string{"STACKLIT_STEERING=d"}
	case "stacklit", "stacklit-a":
		return []string{"STACKLIT_STEERING=a", "STACKLIT_SMALL_REPO_FILES=0"}
	case "stacklit-e":
		return []string{"STACKLIT_STEERING=e", "STACKLIT_SMALL_REPO_FILES=0"}
	case "stacklit-e-small":
		return []string{"STACKLIT_STEERING=e", fmt.Sprintf("STACKLIT_SMALL_REPO_FILES=%d", c.smallFiles)}
	}
	return nil
}

func prompt(c config, r repo, t task) string {
	if c.mode == "memory" {
		return "You have no tools and no access to files. From your own knowledge of the open source repository github.com/glincker/" + r.Name + ", answer: " + t.Question + answerSuffix + " If you do not know, give your best guess."
	}
	return t.Question + answerSuffix
}

func runUnit(c config, r repo, base string, u unit) (record, error) {
	rec := record{Repo: r.Name, Task: u.Task.ID, Kind: u.Task.Kind, Trial: u.Trial, Arm: u.Arm, At: time.Now().UTC().Format(time.RFC3339)}
	start := time.Now()
	dir, err := os.MkdirTemp(base, "co-")
	if err != nil {
		rec.Error = err.Error()
		return rec, nil
	}
	defer os.RemoveAll(dir)
	tools := "Read,Grep,Glob"
	var allowed string
	stackBin, _ := filepath.Abs(c.stacklit)
	env := append(os.Environ(), "STACKLIT_CACHE_DIR="+filepath.Join(base, "cache-"+u.Arm), "STACKLIT_STATS=0",
		"PATH="+filepath.Dir(stackBin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if c.mode == "memory" {
		tools = ""
	} else {
		if out, err := exec.Command("git", "clone", "-q", "--depth", "1", "file://"+filepath.Join(base, "base"), dir).CombinedOutput(); err != nil {
			rec.Error = fmt.Sprintf("checkout: %v %s", err, out)
			return rec, nil
		}
		if u.Arm != "control" {
			steps := [][]string{{"generate", "--graph", "--quiet"}}
			setup := []string{"setup", "claude", "--no-mcp", "--no-git-hook", "--session-hook"}
			if u.Arm == "strict" {
				setup = append(setup, "--crawl-guard=redirect")
			}
			env = append(env, armEnv(c, u.Arm)...)
			steps = append(steps, setup)
			for _, args := range steps {
				cmd := exec.Command(stackBin, args...)
				cmd.Dir, cmd.Env = dir, env
				if out, err := cmd.CombinedOutput(); err != nil {
					rec.Error = fmt.Sprintf("stacklit %v: %v %s", args, err, out)
					return rec, nil
				}
			}
			tools += ",Bash"
			allowed = "Bash(stacklit *)"
		}
	}
	args := []string{"-p", prompt(c, r, u.Task), "--output-format", "stream-json", "--verbose", "--no-session-persistence",
		"--setting-sources", "project,local", "--strict-mcp-config", "--disable-slash-commands", "--tools", tools,
		"--max-turns", fmt.Sprint(c.maxTurns), "--max-budget-usd", fmt.Sprint(c.maxBudget), "--model", c.model}
	if allowed != "" {
		args = append(args, "--allowedTools", allowed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.claude, args...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cacheDir := filepath.Join(base, "cache-"+u.Arm)
	runErr := cmd.Run()
	rec.CrawlFired = crawlFired(cacheDir, start)
	rec.Seconds = time.Since(start).Seconds()
	keepTranscript(base, u, stdout.Bytes())
	p, perr := parseStream(stdout.Bytes())
	if perr != nil {
		rec.Error = fmt.Sprintf("parse: %v (run: %v) %s", perr, runErr, truncate(stderr.String(), 300))
		if limitText.MatchString(stderr.String() + stdout.String()) {
			return rec, fmt.Errorf("%w: limit or auth error: %s", errStop, truncate(stderr.String()+stdout.String(), 400))
		}
		return rec, nil
	}
	fill(&rec, p)
	if p.stop != "" {
		return rec, fmt.Errorf("%w: %s", errStop, p.stop)
	}
	rec.Binary = c.binSHA
	rec.Correct, rec.Recall = gradeWith(c.grader, rec.Answer, u.Task)
	return rec, nil
}

// keepTranscript saves the raw stream outside the repo for hand spot checks.
func keepTranscript(base string, u unit, b []byte) {
	dir := os.Getenv("AGENT_AB_TRANSCRIPTS")
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%s-t%d.jsonl", u.Arm, u.Task.ID, u.Trial)), b, 0o600)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

var limitText = regexp.MustCompile(`(?i)rate.?limit|usage limit|limit reached|out of extra usage|not logged in|please run /login|authentication|invalid api key|oauth|unauthorized|credit balance|overloaded_error|\b(401|403|429)\b`)

// pathRe returns a matcher for a path that is not part of a longer path.
func pathRe(p string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_./-])` + regexp.QuoteMeta(p))
}

// gradeWith applies the named matcher. Round 2 uses audited from the start:
// the same boundary-aware match, also accepting ./path and absolute paths
// inside the checkout, which round 1 found it had to add after the fact.
func gradeWith(grader, answer string, t task) (bool, float64) {
	if grader == "prereg" {
		return grade(answer, t)
	}
	return gradeAuditedWithRecall(answer, t)
}

// grade returns whether the answer is correct and the recall of AllOf paths.
func grade(answer string, t task) (bool, float64) {
	hit := 0
	for _, p := range t.AllOf {
		if pathRe(p).MatchString(answer) {
			hit++
		}
	}
	recall := -1.0 // not defined for any-of-only tasks
	if len(t.AllOf) > 0 {
		recall = float64(hit) / float64(len(t.AllOf))
	}
	ok := hit == len(t.AllOf)
	if ok && len(t.AnyOf) > 0 {
		ok = false
		for _, p := range t.AnyOf {
			if pathRe(p).MatchString(answer) {
				ok = true
			}
		}
	}
	return ok, recall
}

// crawlFired reports whether the crawl guard recorded a redirect for a
// session that ran since start: its state file lives in the cache dir.
func crawlFired(cacheDir string, since time.Time) bool {
	files, _ := filepath.Glob(filepath.Join(cacheDir, "*", "crawl-*.json"))
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil || st.ModTime().Before(since) {
			continue
		}
		b, _ := os.ReadFile(f)
		var s struct {
			N        int  `json:"n"`
			Released bool `json:"released"`
		}
		if json.Unmarshal(b, &s) == nil && (s.N > 0 || s.Released) {
			return true
		}
	}
	return false
}
