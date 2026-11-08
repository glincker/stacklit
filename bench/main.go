// Command bench measures stacklit against several baselines on pinned public
// repos. It makes no network calls except `git fetch` of the pinned commits,
// and calls no LLM. See bench/README.md and bench/RESULTS.md.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed repos.json tasks/*.json
var data embed.FS

// RepoSpec pins one benchmark repository.
type RepoSpec struct {
	Name        string   `json:"name"`
	Language    string   `json:"language"`
	URL         string   `json:"url"`
	Ref         string   `json:"ref"`
	SHA         string   `json:"sha"`
	Exts        []string `json:"exts"`
	TestMarkers []string `json:"test_markers"`
	// StacklitIgnore is written to .stacklitrc.json in the checkout, only to
	// keep generation from hanging on files the shell parser cannot handle.
	StacklitIgnore []string `json:"stacklit_ignore,omitempty"`
}

// Task is one navigation question with a hand-verified answer.
type Task struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Question      string   `json:"question"`
	Pattern       string   `json:"pattern"`
	Word          bool     `json:"word"`
	Terms         []string `json:"terms"`
	Target        string   `json:"target,omitempty"`
	ExpectPaths   []string `json:"expect_paths"`
	ExpectModules []string `json:"expect_modules"`
}

// GenStats are the generation measurements for one repo.
type GenStats struct {
	Runs      int
	FirstMs   float64
	MedianMs  float64
	MaxRSSMB  float64
	IndexCost Cost
	MapCost   Cost
	Modules   int
	Files     int
	Lines     int
	// Attempts is how many generate runs it took to get an index with no
	// parse errors; ParseErrs is what the kept index still had.
	Attempts  int
	ParseErrs int
}

// RepoResult is everything measured for one repo.
type RepoResult struct {
	Spec          RepoSpec
	Gen           GenStats
	DumpCost      Cost    // all code files of the repo, tests included
	RepoMapMs     float64 // single-run build time of the repo-map tags
	BM25Ms        float64 // single-run build time of the BM25 index
	Tasks         []TaskResult
	CodeFiles     int
	SearchedFiles int
}

// Opts are the harness knobs; each is a command line flag.
type Opts struct {
	Workdir, Binary, Only, WriteMD, JSONOut string
	Runs, TopN, ReadCap, MaxExtra, Boot     int
	Seed                                    int64
	Baseline                                BaselineParams
	QueryBudgets, MapBudgets                []int
	BM25Listed, BM25Ranked                  int
	ContextWindow                           int
}

func parseInts(s string) ([]int, error) {
	var out []int
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func main() {
	var o Opts
	flag.StringVar(&o.Workdir, "workdir", "", "directory for clones (default: a new temp dir, removed on exit)")
	flag.StringVar(&o.Binary, "stacklit", "", "path to the stacklit binary (default: build from this checkout)")
	flag.IntVar(&o.Runs, "runs", 5, "timed generate runs per repo")
	flag.StringVar(&o.Only, "only", "", "comma separated repo names to run (default: all)")
	flag.IntVar(&o.TopN, "top", 3, "modules the stacklit map+lookup arm inspects per localization task")
	flag.StringVar(&o.WriteMD, "write", "", "update the generated block of this markdown file (for example bench/RESULTS.md)")
	flag.StringVar(&o.JSONOut, "json", "", "write raw results to this JSON file")
	flag.IntVar(&o.Baseline.MaxGrepLines, "grep-lines", DefaultBaseline.MaxGrepLines, "grep arm: output lines the agent sees")
	flag.IntVar(&o.ReadCap, "read-cap", 5, "files an arm may open, in rank order, before a task counts as missed")
	flag.IntVar(&o.MaxExtra, "max-extra", 2, "dependents strict rule: most unexpected modules allowed")
	flag.IntVar(&o.Boot, "boot", 10000, "bootstrap resamples for confidence intervals")
	flag.Int64Var(&o.Seed, "seed", 20260101, "bootstrap seed")
	flag.IntVar(&o.BM25Listed, "bm25-listed", 5, "BM25 arm: paths printed to the agent")
	flag.IntVar(&o.BM25Ranked, "bm25-ranked", 10, "BM25 arm: ranked files kept for found@k")
	flag.IntVar(&o.ContextWindow, "context-window", 200000, "tokens used to say whether a full dump fits")
	qb := flag.String("query-budgets", "1000,2000", "stacklit query arm: --budget values (tokens), one arm each")
	mb := flag.String("repomap-budgets", "1000,4000", "repo-map approximation arm: token budgets, one arm each")
	flag.Parse()
	o.Baseline.MaxLineChars = DefaultBaseline.MaxLineChars
	var err error
	if o.QueryBudgets, err = parseInts(*qb); err == nil {
		o.MapBudgets, err = parseInts(*mb)
	}
	if err == nil {
		err = run(o)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func run(o Opts) error {
	repos, err := loadRepos()
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, n := range strings.Split(o.Only, ",") {
		if n = strings.TrimSpace(n); n != "" {
			keep[n] = true
		}
	}

	workdir := o.Workdir
	if workdir == "" {
		workdir, err = os.MkdirTemp("", "stacklit-bench-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(workdir)
	} else if err := os.MkdirAll(workdir, 0o755); err != nil {
		return err
	}

	binary := o.Binary
	if binary == "" {
		binary = filepath.Join(workdir, "stacklit-bin")
		fmt.Fprintln(os.Stderr, "building stacklit from this checkout")
		cmd := exec.Command("go", "build", "-o", binary, "./cmd/stacklit")
		cmd.Dir = repoRoot()
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go build: %w", err)
		}
	}

	var results []RepoResult
	for _, spec := range repos {
		if len(keep) > 0 && !keep[spec.Name] {
			continue
		}
		fmt.Fprintf(os.Stderr, "== %s (%s @ %s)\n", spec.Name, spec.Ref, spec.SHA[:10])
		r, err := benchRepo(workdir, binary, spec, o)
		if err != nil {
			return fmt.Errorf("%s: %w", spec.Name, err)
		}
		results = append(results, r)
	}

	env := environment(binary)
	md := RenderMarkdown(results, env, o)
	if o.WriteMD != "" {
		if err := spliceGenerated(o.WriteMD, md); err != nil {
			return err
		}
	} else {
		fmt.Println(md)
	}
	if o.JSONOut != "" {
		b, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(o.JSONOut, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func loadRepos() ([]RepoSpec, error) {
	raw, err := data.ReadFile("repos.json")
	if err != nil {
		return nil, err
	}
	var repos []RepoSpec
	return repos, json.Unmarshal(raw, &repos)
}

func loadTasks(name string) ([]Task, error) {
	raw, err := data.ReadFile("tasks/" + name + ".json")
	if err != nil {
		return nil, err
	}
	var tasks []Task
	return tasks, json.Unmarshal(raw, &tasks)
}

// repoRoot finds the stacklit checkout that contains this harness.
func repoRoot() string {
	dir, _ := os.Getwd()
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	return dir
}

func armQuery(b int) string   { return fmt.Sprintf("stacklit query %d", b) }
func armRepoMap(b int) string { return fmt.Sprintf("repo-map approx %d", b) }

const (
	armGrep     = "grep+read"
	armBM25     = "bm25 top-k"
	armDump     = "full dump"
	armStacklit = "stacklit map+lookup"
)

// armOrder lists the arms in report order for the given options.
func armOrder(o Opts) []string {
	arms := []string{armGrep}
	for _, b := range o.MapBudgets {
		arms = append(arms, armRepoMap(b))
	}
	arms = append(arms, armBM25, armDump, armStacklit)
	for _, b := range o.QueryBudgets {
		arms = append(arms, armQuery(b))
	}
	return arms
}

func benchRepo(workdir, binary string, spec RepoSpec, o Opts) (RepoResult, error) {
	dir := filepath.Join(workdir, spec.Name)
	if err := checkout(dir, spec); err != nil {
		return RepoResult{}, err
	}
	if len(spec.StacklitIgnore) > 0 {
		rc, _ := json.Marshal(map[string]any{"ignore": spec.StacklitIgnore})
		if err := os.WriteFile(filepath.Join(dir, ".stacklitrc.json"), rc, 0o644); err != nil {
			return RepoResult{}, err
		}
	}
	// Load sources before generating so stacklit outputs never enter any arm.
	sources, sizes, err := LoadSources(dir, spec)
	if err != nil {
		return RepoResult{}, err
	}

	gen, err := measureGenerate(binary, dir, o.Runs)
	if err != nil {
		return RepoResult{}, err
	}
	gen.Attempts, gen.ParseErrs, err = cleanIndex(binary, dir, maxIndexAttempts)
	if err != nil {
		return RepoResult{}, err
	}
	// Save the symbol graph once so query calls do not rebuild it each time.
	graphCmd := exec.Command(binary, "graph")
	graphCmd.Dir = dir
	if out, err := graphCmd.CombinedOutput(); err != nil {
		return RepoResult{}, fmt.Errorf("graph: %v: %s", err, out)
	}
	iv, err := LoadIndex(filepath.Join(dir, "stacklit.json"))
	if err != nil {
		return RepoResult{}, err
	}
	deriveCmd := exec.Command(binary, "derive")
	deriveCmd.Dir = dir
	mapBytes, err := deriveCmd.Output()
	if err != nil {
		return RepoResult{}, fmt.Errorf("derive: %w", err)
	}
	gen.MapCost = CostOf(string(mapBytes))
	gen.IndexCost = CostOf(string(iv.Raw))
	gen.Modules = len(iv.Modules)
	gen.Files, gen.Lines = totals(iv.Raw)

	tasks, err := loadTasks(spec.Name)
	if err != nil {
		return RepoResult{}, err
	}
	if err := verifyGold(sources, sizes, iv.ModuleNames(), tasks, o.Baseline); err != nil {
		return RepoResult{}, err
	}

	var dump Cost
	for _, c := range sizes {
		dump = dump.Add(c)
	}
	t0 := time.Now()
	rm := BuildRepoMap(sources)
	repoMapMs := float64(time.Since(t0).Microseconds()) / 1000
	t0 = time.Now()
	bm := BuildBM25(sources)
	bm25Ms := float64(time.Since(t0).Microseconds()) / 1000

	res := RepoResult{Spec: spec, Gen: gen, DumpCost: dump, RepoMapMs: repoMapMs, BM25Ms: bm25Ms,
		CodeFiles: len(sizes), SearchedFiles: len(sources)}
	modules := iv.ModuleNames()
	for _, t := range tasks {
		tr := TaskResult{Task: t, Arms: map[string]ArmResult{}}
		g, err := RunBaseline(armGrep, sources, t, modules, o.Baseline, sizes, o.ReadCap, o.MaxExtra)
		if err != nil {
			return res, err
		}
		tr.Arms[armGrep] = g
		for _, b := range o.MapBudgets {
			tr.Arms[armRepoMap(b)] = RunRepoMap(armRepoMap(b), rm, t, b, sizes, o.ReadCap)
		}
		tr.Arms[armBM25] = RunBM25(armBM25, bm, t, o.BM25Listed, o.BM25Ranked, sizes, o.ReadCap)
		tr.Arms[armDump] = ArmResult{Arm: armDump, CostOnly: true, Lookup: dump, ToFound: dump}
		tr.Arms[armStacklit] = RunStacklit(armStacklit, iv, gen.MapCost, t, o.TopN, sizes, o.ReadCap, o.MaxExtra)
		for _, b := range o.QueryBudgets {
			tr.Arms[armQuery(b)] = RunQuery(armQuery(b), binary, dir, t, modules, strconv.Itoa(b), sizes, o.ReadCap, o.MaxExtra)
		}
		res.Tasks = append(res.Tasks, tr)
	}
	return res, nil
}

// maxIndexAttempts bounds the retries in cleanIndex.
const maxIndexAttempts = 10

var parseErrRe = regexp.MustCompile(`\((\d+) errors`)

// cleanIndex regenerates the index until no file was dropped by a parser
// error. The Rust parser panics on a varying subset of files from run to run,
// which would make every stacklit arm non-deterministic; keeping the first
// clean index makes the arms reproducible. It returns the attempts used and
// the parse errors left in the kept index.
func cleanIndex(binary, dir string, maxTries int) (attempts, errs int, err error) {
	for attempts = 1; attempts <= maxTries; attempts++ {
		for _, f := range []string{"stacklit.json", "stacklit.graph.json", "DEPENDENCIES.md", "stacklit.html"} {
			_ = os.Remove(filepath.Join(dir, f))
		}
		cmd := exec.Command(binary, "generate")
		cmd.Dir = dir
		// One CPU makes the parser panics rare enough that a retry succeeds.
		cmd.Env = append(os.Environ(), "STACKLIT_NO_CACHE=1", "GOMAXPROCS=1")
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return attempts, 0, fmt.Errorf("generate: %v: %s", runErr, out)
		}
		errs = 0
		if m := parseErrRe.FindSubmatch(out); m != nil {
			errs, _ = strconv.Atoi(string(m[1]))
		}
		if errs == 0 {
			return attempts, 0, nil
		}
	}
	return maxTries, errs, nil
}

// verifyGold fails the run when a hand-written answer does not hold on the
// pinned checkout: gold files must exist and match the grep pattern, and the
// uncapped grep result for a dependents task must cover every expected module.
func verifyGold(files []SourceFile, sizes RepoSizes, modules []string, tasks []Task, bp BaselineParams) error {
	var bad []string
	for _, t := range tasks {
		pat := t.Pattern
		if t.Word {
			pat = `\b(?:` + pat + `)\b`
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: bad pattern: %v", t.ID, err))
			continue
		}
		if t.Kind == "dependents" {
			got := map[string]bool{}
			for _, f := range files {
				for _, l := range f.Lines {
					if re.MatchString(l) {
						got[moduleOfFile(f.Rel, modules)] = true
						break
					}
				}
			}
			for _, e := range t.ExpectModules {
				if !got[e] {
					bad = append(bad, fmt.Sprintf("%s: expected module %q has no grep hit", t.ID, e))
				}
			}
			continue
		}
		matched := false
		for _, e := range t.ExpectPaths {
			exists := false
			for rel := range sizes {
				if pathMatches(rel, e) {
					exists = true
				}
			}
			if !exists {
				bad = append(bad, fmt.Sprintf("%s: gold path %q does not exist", t.ID, e))
			}
			for _, f := range files {
				if !pathMatches(f.Rel, e) {
					continue
				}
				for _, l := range f.Lines {
					if re.MatchString(l) {
						matched = true
						break
					}
				}
			}
		}
		if !matched {
			bad = append(bad, fmt.Sprintf("%s: pattern %q matches no gold file", t.ID, t.Pattern))
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("gold answers do not hold:\n  %s", strings.Join(bad, "\n  "))
	}
	return nil
}

func totals(raw []byte) (files, lines int) {
	var doc struct {
		Structure struct {
			TotalFiles int `json:"total_files"`
			TotalLines int `json:"total_lines"`
		} `json:"structure"`
	}
	_ = json.Unmarshal(raw, &doc)
	return doc.Structure.TotalFiles, doc.Structure.TotalLines
}

// checkout fetches exactly the pinned commit into dir (idempotent).
func checkout(dir string, spec RepoSpec) error {
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output(); err == nil &&
		strings.TrimSpace(string(out)) == spec.SHA {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	steps := [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", spec.URL},
		{"fetch", "-q", "--depth", "1", "origin", spec.SHA},
		{"checkout", "-q", "FETCH_HEAD"},
	}
	for _, s := range steps {
		cmd := exec.Command("git", append([]string{"-C", dir}, s...)...)
		if out, err := cmd.CombinedOutput(); err != nil && s[0] != "remote" {
			return fmt.Errorf("git %s: %v: %s", s[0], err, strings.TrimSpace(string(out)))
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(out)) != spec.SHA {
		return fmt.Errorf("checked out %q, want %s", strings.TrimSpace(string(out)), spec.SHA)
	}
	return nil
}

// measureGenerate runs `stacklit generate --quiet` several times from a clean
// output state and reports first-run time, median time and peak memory.
func measureGenerate(binary, dir string, runs int) (GenStats, error) {
	if runs < 1 {
		runs = 1
	}
	var times []float64
	var maxRSS int64
	for i := 0; i < runs; i++ {
		for _, f := range []string{"stacklit.json", "stacklit.graph.json", "DEPENDENCIES.md", "stacklit.html"} {
			_ = os.Remove(filepath.Join(dir, f))
		}
		cmd := exec.Command(binary, "generate", "--quiet")
		cmd.Dir = dir
		// Cold-start numbers only: a warm parse cache would hide the real cost.
		cmd.Env = append(os.Environ(), "STACKLIT_NO_CACHE=1")
		start := time.Now()
		if out, err := cmd.CombinedOutput(); err != nil {
			return GenStats{}, fmt.Errorf("generate: %v: %s", err, out)
		}
		times = append(times, float64(time.Since(start).Microseconds())/1000)
		if rss := maxRSSBytes(cmd); rss > maxRSS {
			maxRSS = rss
		}
	}
	first := times[0]
	sort.Float64s(times)
	st := GenStats{Runs: runs, FirstMs: first, MedianMs: times[len(times)/2]}
	if maxRSS > 0 {
		st.MaxRSSMB = float64(maxRSS) / (1024 * 1024)
	}
	return st, nil
}

// Env describes the machine and tool versions of a run.
type Env struct {
	OS, Arch, GoVersion, Stacklit string
	CPUs                          int
	Date                          string
}

func environment(binary string) Env {
	ver := "unknown"
	if out, err := exec.Command(binary, "--version").Output(); err == nil {
		ver = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "-C", repoRoot(), "rev-parse", "--short", "HEAD").Output(); err == nil {
		ver += " (checkout " + strings.TrimSpace(string(out)) + ")"
	}
	return Env{OS: runtime.GOOS, Arch: runtime.GOARCH, GoVersion: runtime.Version(), Stacklit: ver,
		CPUs: runtime.NumCPU(), Date: time.Now().UTC().Format("2006-01-02")}
}
