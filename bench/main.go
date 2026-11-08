// Command bench measures stacklit against a grep+read baseline on pinned
// public repos. It makes no network calls except `git fetch` of the pinned
// commits, and calls no LLM. See bench/README.md.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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

// TaskResult pairs both procedures for one task.
type TaskResult struct {
	Task     Task
	Baseline BaselineResult
	Stacklit StacklitResult
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
}

// RepoResult is everything measured for one repo.
type RepoResult struct {
	Spec  RepoSpec
	Gen   GenStats
	Tasks []TaskResult
}

func main() {
	var (
		workdir  = flag.String("workdir", "", "directory for clones (default: a new temp dir, removed on exit)")
		binary   = flag.String("stacklit", "", "path to the stacklit binary (default: build from this checkout)")
		runs     = flag.Int("runs", 5, "timed generate runs per repo")
		only     = flag.String("only", "", "comma separated repo names to run (default: all)")
		topN     = flag.Int("top", 3, "modules the stacklit path inspects per locate task")
		writeMD  = flag.String("write", "", "update the generated block of this markdown file (for example bench/RESULTS.md)")
		jsonOut  = flag.String("json", "", "write raw results to this JSON file")
		maxLines = flag.Int("grep-lines", DefaultBaseline.MaxGrepLines, "baseline: grep output lines the agent sees")
		readTop  = flag.Int("read-files", DefaultBaseline.ReadTopFiles, "baseline: files read in full after grepping")
	)
	flag.Parse()

	if err := run(*workdir, *binary, *runs, *only, *topN, *writeMD, *jsonOut,
		BaselineParams{MaxGrepLines: *maxLines, MaxLineChars: DefaultBaseline.MaxLineChars, ReadTopFiles: *readTop}); err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func run(workdir, binary string, runs int, only string, topN int, writeMD, jsonOut string, bp BaselineParams) error {
	repos, err := loadRepos()
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, n := range strings.Split(only, ",") {
		if n = strings.TrimSpace(n); n != "" {
			keep[n] = true
		}
	}

	if workdir == "" {
		workdir, err = os.MkdirTemp("", "stacklit-bench-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(workdir)
	} else if err := os.MkdirAll(workdir, 0o755); err != nil {
		return err
	}

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
		r, err := benchRepo(workdir, binary, spec, runs, topN, bp)
		if err != nil {
			return fmt.Errorf("%s: %w", spec.Name, err)
		}
		results = append(results, r)
	}

	env := environment(binary)
	md := RenderMarkdown(results, env, bp)
	if writeMD != "" {
		if err := spliceGenerated(writeMD, md); err != nil {
			return err
		}
	} else {
		fmt.Println(md)
	}
	if jsonOut != "" {
		b, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(jsonOut, b, 0o644); err != nil {
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

func benchRepo(workdir, binary string, spec RepoSpec, runs, topN int, bp BaselineParams) (RepoResult, error) {
	dir := filepath.Join(workdir, spec.Name)
	if err := checkout(dir, spec); err != nil {
		return RepoResult{}, err
	}
	// Load sources before generating so stacklit outputs never enter the baseline.
	sources, err := LoadSources(dir, spec)
	if err != nil {
		return RepoResult{}, err
	}

	gen, err := measureGenerate(binary, dir, runs)
	if err != nil {
		return RepoResult{}, err
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
	res := RepoResult{Spec: spec, Gen: gen}
	for _, t := range tasks {
		b, err := RunBaseline(sources, t, iv.ModuleNames(), bp)
		if err != nil {
			return res, err
		}
		s := RunStacklit(iv, gen.MapCost, dir, t, topN)
		res.Tasks = append(res.Tasks, TaskResult{Task: t, Baseline: b, Stacklit: s})
	}
	return res, nil
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
		for _, f := range []string{"stacklit.json", "DEPENDENCIES.md", "stacklit.html"} {
			_ = os.Remove(filepath.Join(dir, f))
		}
		cmd := exec.Command(binary, "generate", "--quiet")
		cmd.Dir = dir
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
