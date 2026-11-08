// Command agent-ab runs real agent sessions twice per task, once without and
// once with Stacklit configured, and compares the usage Claude Code reports.
// It spends real money, so it refuses to run without --yes-spend.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type repo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	SHA  string `json:"sha"`
}

type task struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	ExpectPaths []string `json:"expect_paths"`
}

// unit is one agent session: a task, a trial number and an arm.
type unit struct {
	Task  task   `json:"task"`
	Trial int    `json:"trial"`
	Arm   string `json:"arm"`
}

// usage is what `claude -p --output-format json` reports for a session.
type usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
}

type claudeResult struct {
	IsError  bool    `json:"is_error"`
	Result   string  `json:"result"`
	NumTurns int     `json:"num_turns"`
	Cost     float64 `json:"total_cost_usd"`
	Usage    usage   `json:"usage"`
}

// record is one finished session.
type record struct {
	Task     string  `json:"task"`
	Trial    int     `json:"trial"`
	Arm      string  `json:"arm"`
	Correct  bool    `json:"correct"`
	Turns    int     `json:"turns"`
	CostUSD  float64 `json:"cost_usd"`
	Usage    usage   `json:"usage"`
	Error    string  `json:"error,omitempty"`
	Duration float64 `json:"seconds"`
}

type config struct {
	repo, reposFile, tasksDir, stacklit, claude, model, out, arms string
	trials, limit, maxTurns                                       int
	seed                                                          int64
	yesSpend, dryRun                                              bool
	maxBudget                                                     float64
	estLow, estMid, estHigh                                       float64
}

func main() {
	var c config
	flag.StringVar(&c.repo, "repo", "gin", "repo name from repos.json")
	flag.StringVar(&c.reposFile, "repos", "bench/repos.json", "pinned repos file")
	flag.StringVar(&c.tasksDir, "tasks", "bench/tasks", "directory of <repo>.json task files")
	flag.StringVar(&c.stacklit, "stacklit", "stacklit", "stacklit binary for the treatment arm")
	flag.StringVar(&c.claude, "claude", "claude", "claude binary")
	flag.StringVar(&c.model, "model", "", "model alias passed to claude (empty: its default)")
	flag.StringVar(&c.out, "out", "agent-ab-results.json", "raw results file")
	flag.StringVar(&c.arms, "arms", "control,stacklit", "arms to run (a smoke test can use just one)")
	flag.IntVar(&c.trials, "trials", 3, "trials per task and arm (3 or more recommended)")
	flag.IntVar(&c.limit, "limit", 0, "use only the first N tasks (0 = all)")
	flag.IntVar(&c.maxTurns, "max-turns", 20, "turn cap per session")
	flag.Int64Var(&c.seed, "seed", 1, "seed for the session order")
	flag.Float64Var(&c.maxBudget, "max-budget-usd", 2, "per-session spend cap passed to claude")
	flag.BoolVar(&c.dryRun, "dry-run", false, "print the plan and the projected cost, run nothing")
	flag.BoolVar(&c.yesSpend, "yes-spend", false, "required: confirm you accept the spend of real agent sessions")
	flag.Float64Var(&c.estLow, "est-low", 0.15, "assumed USD per session, low case (an assumption, not a measurement)")
	flag.Float64Var(&c.estMid, "est-mid", 0.40, "assumed USD per session, middle case")
	flag.Float64Var(&c.estHigh, "est-high", 1.00, "assumed USD per session, high case")
	flag.Parse()
	if err := run(c); err != nil {
		fmt.Fprintln(os.Stderr, "agent-ab:", err)
		os.Exit(1)
	}
}

func run(c config) error {
	tasks, r, err := load(c)
	if err != nil {
		return err
	}
	units := plan(tasks, c.trials, c.seed, strings.Split(c.arms, ","))
	fmt.Print(projection(len(tasks), c, len(units)))
	if c.dryRun {
		return nil
	}
	if !c.yesSpend {
		return errors.New("refusing to start paid agent sessions without --yes-spend (use --dry-run to see the plan)")
	}
	if err := checkIsolation(); err != nil {
		return err
	}
	base, err := os.MkdirTemp("", "agent-ab-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base)
	if err := cloneBase(base, r); err != nil {
		return err
	}
	var recs []record
	for i, u := range units {
		fmt.Printf("[%d/%d] %s trial %d arm %s\n", i+1, len(units), u.Task.ID, u.Trial, u.Arm)
		recs = append(recs, runUnit(c, base, u))
	}
	b, _ := json.MarshalIndent(recs, "", "  ")
	if err := os.WriteFile(c.out, b, 0o644); err != nil {
		return err
	}
	fmt.Print(report(recs))
	return nil
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
	if c.limit > 0 && len(tasks) > c.limit {
		tasks = tasks[:c.limit]
	}
	return tasks, r, nil
}

// plan expands tasks x trials x arms and shuffles with a seed, so arms are
// interleaved (time-of-day or cache effects hit both) and the order repeats.
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

// projection states the cost of the full run before anything is spent. The
// per-session figures are assumptions; replace them with a measured value.
func projection(tasks int, c config, units int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "plan: %d tasks x %d trials x %d arm(s) = %d paid sessions\n", tasks, c.trials, len(strings.Split(c.arms, ",")), units)
	fmt.Fprintf(&b, "projected cost at an ASSUMED $%.2f / $%.2f / $%.2f per session (low/mid/high): $%.2f / $%.2f / $%.2f\n",
		c.estLow, c.estMid, c.estHigh, float64(units)*c.estLow, float64(units)*c.estMid, float64(units)*c.estHigh)
	fmt.Fprintf(&b, "hard cap: %.2f USD per session via --max-budget-usd, so the worst case is $%.2f\n", c.maxBudget, float64(units)*c.maxBudget)
	return b.String()
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

func git(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

func cloneBase(base string, r repo) error {
	dst := filepath.Join(base, "base")
	if out, err := exec.Command("git", "clone", "-q", r.URL, dst).CombinedOutput(); err != nil {
		return fmt.Errorf("clone: %v\n%s", err, out)
	}
	return git(dst, "checkout", "-q", r.SHA)
}

func prompt(t task) string {
	return t.Question + " Answer with the file path or paths only, on one line."
}

func runUnit(c config, base string, u unit) record {
	rec := record{Task: u.Task.ID, Trial: u.Trial, Arm: u.Arm}
	start := time.Now()
	dir, err := os.MkdirTemp(base, "co-")
	if err != nil {
		rec.Error = err.Error()
		return rec
	}
	defer os.RemoveAll(dir)
	if out, err := exec.Command("git", "clone", "-q", "--local", filepath.Join(base, "base"), dir).CombinedOutput(); err != nil {
		rec.Error = fmt.Sprintf("checkout: %v %s", err, out)
		return rec
	}
	env := append(os.Environ(), "STACKLIT_CACHE_DIR="+filepath.Join(base, "cache-"+u.Arm), "STACKLIT_STATS=0")
	tools := "Read,Grep,Glob"
	if u.Arm == "stacklit" {
		for _, args := range [][]string{
			{"generate", "--graph", "--quiet"},
			{"setup", "claude", "--no-mcp", "--no-git-hook", "--session-hook"},
		} {
			cmd := exec.Command(c.stacklit, args...)
			cmd.Dir, cmd.Env = dir, env
			if out, err := cmd.CombinedOutput(); err != nil {
				rec.Error = fmt.Sprintf("stacklit %v: %v %s", args, err, out)
				return rec
			}
		}
		tools += ",Bash(stacklit *)"
	}
	args := []string{"-p", prompt(u.Task), "--output-format", "json", "--no-session-persistence",
		"--setting-sources", "project,local", "--allowedTools", tools,
		"--max-turns", fmt.Sprint(c.maxTurns), "--max-budget-usd", fmt.Sprint(c.maxBudget)}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}
	cmd := exec.Command(c.claude, args...)
	cmd.Dir, cmd.Env = dir, env
	out, err := cmd.Output()
	rec.Duration = time.Since(start).Seconds()
	if err != nil && len(out) == 0 {
		rec.Error = err.Error()
		return rec
	}
	res, perr := parseResult(out)
	if perr != nil {
		rec.Error = perr.Error()
		return rec
	}
	rec.Turns, rec.CostUSD, rec.Usage = res.NumTurns, res.Cost, res.Usage
	rec.Correct = !res.IsError && correct(res.Result, u.Task.ExpectPaths)
	if res.IsError {
		rec.Error = "agent reported an error"
	}
	return rec
}

func parseResult(b []byte) (claudeResult, error) {
	var r claudeResult
	if err := json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("unparseable claude output: %w", err)
	}
	return r, nil
}

// correct is a deterministic heuristic: the answer names an expected path. It
// can miss a right answer phrased differently, equally in both arms.
func correct(answer string, expect []string) bool {
	for _, p := range expect {
		if strings.Contains(answer, p) {
			return true
		}
	}
	return false
}

type armTotals struct {
	n                       int
	correct                 int
	cost                    float64
	in, out, cacheW, cacheR int
	turns                   int
	errs                    int
}

func report(recs []record) string {
	arms := map[string]*armTotals{}
	for _, r := range recs {
		a := arms[r.Arm]
		if a == nil {
			a = &armTotals{}
			arms[r.Arm] = a
		}
		a.n++
		if r.Error != "" {
			a.errs++
			continue
		}
		if r.Correct {
			a.correct++
		}
		a.cost += r.CostUSD
		a.in += r.Usage.InputTokens
		a.out += r.Usage.OutputTokens
		a.cacheW += r.Usage.CacheCreationTokens
		a.cacheR += r.Usage.CacheReadTokens
		a.turns += r.Turns
	}
	names := make([]string, 0, len(arms))
	for n := range arms {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "%-9s %4s %5s %7s %9s %9s %9s %9s %6s\n", "arm", "n", "ok", "errs", "in", "out", "cacheW", "cacheR", "$/run")
	for _, n := range names {
		a := arms[n]
		ok := a.n - a.errs
		if ok == 0 {
			ok = 1
		}
		fmt.Fprintf(&b, "%-9s %4d %5d %7d %9d %9d %9d %9d %6.3f\n", n, a.n, a.correct, a.errs,
			a.in/ok, a.out/ok, a.cacheW/ok, a.cacheR/ok, a.cost/float64(ok))
	}
	b.WriteString("token columns are per-run means as reported by claude; ok = answers naming an expected path\n")
	return b.String()
}
