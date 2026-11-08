//go:build unix

// Command scale measures Stacklit performance and output size on large public
// repositories. It measures cost only, never answer quality (see ../ for that).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

type repoSpec struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	URL      string `json:"url"`
	Ref      string `json:"ref"`
}

// runStat is one process execution.
type runStat struct {
	Wall   float64 `json:"wall_s"`
	RSS    int64   `json:"rss_bytes"`
	OK     bool    `json:"ok"`
	DNF    bool    `json:"dnf,omitempty"`
	Stdout int     `json:"stdout_bytes"`
	Stderr string  `json:"-"`
}

type phase struct {
	WallMedian float64   `json:"wall_median_s"`
	RSSMedian  int64     `json:"rss_median_bytes"`
	Runs       []runStat `json:"runs"`
	DNF        bool      `json:"dnf,omitempty"`
	Failed     int       `json:"failed,omitempty"`
}

type queryStats struct {
	Calls    int     `json:"calls"`
	Failed   int     `json:"failed"`
	P50ms    float64 `json:"p50_ms"`
	P95ms    float64 `json:"p95_ms"`
	MaxMS    float64 `json:"max_ms"`
	P50Query float64 `json:"p50_query_ms"`
	P50Slice float64 `json:"p50_slice_ms"`
	P50Imp   float64 `json:"p50_impact_ms"`
	MaxRSS   int64   `json:"max_rss_bytes"`
}

type result struct {
	Name         string  `json:"name"`
	Language     string  `json:"language"`
	Ref          string  `json:"ref"`
	SHA          string  `json:"sha"`
	CloneSeconds float64 `json:"clone_s"`
	Note         string  `json:"note,omitempty"`

	TrackedFiles int            `json:"tracked_files"`
	TextBytes    int64          `json:"tracked_text_bytes"`
	DumpTokens   int64          `json:"full_dump_tokens_bytes_div_4"`
	LangMix      map[string]int `json:"language_files"`
	ScannedFiles int            `json:"scanned_files"`
	TotalLines   int            `json:"total_lines"`

	Cold      *phase `json:"cold_generate,omitempty"`
	Warm      *phase `json:"warm_generate,omitempty"`
	ColdGraph *phase `json:"cold_generate_graph,omitempty"`

	IndexBytes      int64 `json:"stacklit_json_bytes"`
	IndexTokens     int64 `json:"stacklit_json_tokens"`
	DeriveTokens    int64 `json:"derive_tokens"`
	GraphBytes      int64 `json:"graph_json_bytes"`
	Nodes           int   `json:"nodes"`
	Edges           int   `json:"edges"`
	Communities     int   `json:"communities"`
	Lightweight     int   `json:"scan_lightweight"`
	SkippedLarge    int   `json:"scan_skipped_large"`
	SkippedOther    int   `json:"scan_skipped_other"`
	SkippedSymlinks int   `json:"scan_skipped_symlinks"`
	ParseErrors     int   `json:"parse_errors"`
	ParserPanics    int   `json:"parser_panic_warnings"`

	Query *queryStats `json:"query_latency,omitempty"`
}

var (
	runs    = flag.Int("runs", 3, "runs per measurement (median reported)")
	qruns   = flag.Int("qruns", 5, "runs per fixed question")
	only    = flag.String("only", "", "comma separated repo names")
	work    = flag.String("workdir", "", "parent dir for clones (default: mktemp -d)")
	out     = flag.String("json", "", "write raw results JSON here")
	write   = flag.String("write", "", "replace the generated table block in this markdown file")
	timeout = flag.Duration("timeout", 900*time.Second, "cap for one process run")
	specs   = flag.String("repos", "", "repos.json path (default: next to this source file)")
	keep    = flag.Bool("keep", false, "keep clones (debug only)")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "scale:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	rp := *specs
	if rp == "" {
		rp = filepath.Join(root, "bench", "scale", "repos.json")
	}
	var all []repoSpec
	data, err := os.ReadFile(rp)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	want := map[string]bool{}
	for _, n := range strings.Split(*only, ",") {
		if n != "" {
			want[n] = true
		}
	}

	parent := *work
	if parent == "" {
		parent, err = os.MkdirTemp("", "stk-scale-")
		if err != nil {
			return err
		}
	} else if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	defer func() {
		if !*keep {
			os.RemoveAll(parent)
		}
	}()

	bin := filepath.Join(parent, "stk")
	build := exec.Command("go", "build", "-o", bin, "./cmd/stacklit")
	build.Dir = root
	if b, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build: %v\n%s", err, b)
	}
	machine := machineInfo(root)
	fmt.Println(machine)

	var results []*result
	for _, sp := range all {
		if len(want) > 0 && !want[sp.Name] {
			continue
		}
		fmt.Printf("== %s %s\n", sp.Name, sp.Ref)
		r := measure(sp, bin, parent)
		results = append(results, r)
		if *out != "" {
			b, _ := json.MarshalIndent(map[string]any{"machine": machine, "results": results}, "", "  ")
			os.WriteFile(*out, b, 0o644)
		}
	}
	table := renderTable(results)
	fmt.Println(table)
	if *write != "" {
		return replaceBlock(*write, machine, table)
	}
	return nil
}

func repoRoot() (string, error) {
	b, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("run from inside the stacklit checkout: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func machineInfo(root string) string {
	sc := func(k string) string {
		b, _ := exec.Command("sysctl", "-n", k).Output()
		return strings.TrimSpace(string(b))
	}
	gov, _ := exec.Command("go", "version").Output()
	sha, _ := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	var mem int64
	fmt.Sscan(sc("hw.memsize"), &mem)
	return fmt.Sprintf("chip=%s cores=%s ram_gb=%d os=%s/%s %s binary_commit=%s",
		sc("machdep.cpu.brand_string"), sc("hw.ncpu"), mem>>30, runtime.GOOS, runtime.GOARCH,
		strings.TrimSpace(string(gov)), strings.TrimSpace(string(sha)))
}

func measure(sp repoSpec, bin, parent string) *result {
	r := &result{Name: sp.Name, Language: sp.Language, Ref: sp.Ref}
	dir := filepath.Join(parent, sp.Name)
	cache := filepath.Join(parent, sp.Name+"-cache")
	defer func() {
		if !*keep {
			os.RemoveAll(dir)
			os.RemoveAll(cache)
		}
	}()
	env := append(os.Environ(), "STACKLIT_CACHE_DIR="+cache)

	t0 := time.Now()
	cl := exec.Command("git", "-c", "filter.lfs.smudge=", "-c", "filter.lfs.process=", "-c", "filter.lfs.required=false", "clone", "-q", "--depth", "1", "--branch", sp.Ref, sp.URL, dir)
	// Some repos track LFS pointers; fetching real blobs would need git-lfs and distort sizes.
	cl.Env = append(os.Environ(), "GIT_LFS_SKIP_SMUDGE=1")
	if b, err := cl.CombinedOutput(); err != nil {
		r.Note = "clone failed: " + strings.TrimSpace(string(b))
		return r
	}
	r.CloneSeconds = time.Since(t0).Seconds()
	if b, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output(); err == nil {
		r.SHA = strings.TrimSpace(string(b))
	}
	tracked(dir, r)

	stk := func(args ...string) runStat { return exec1(bin, dir, env, args...) }
	resetCache := func() { os.RemoveAll(cache) }

	// Cold: parse cache removed before every run.
	r.Cold = repeat(*runs, func() runStat { resetCache(); return stk("generate") }, func(s runStat) {
		r.ParserPanics = strings.Count(s.Stderr, "parser panic")
	})
	if r.Cold.DNF || r.Cold.Failed == *runs {
		r.Note = "cold generate did not finish; later phases skipped"
		return r
	}
	readIndex(dir, r)
	// Warm: parse cache populated by the previous run.
	r.Warm = repeat(*runs, func() runStat { return stk("generate", "--quiet") }, nil)
	// Cold generate with the graph.
	r.ColdGraph = repeat(*runs, func() runStat { resetCache(); return stk("generate", "--graph", "--quiet") }, nil)
	if r.ColdGraph.DNF || r.ColdGraph.Failed == *runs {
		r.Note = "generate --graph did not finish; query latency skipped"
		return r
	}
	readIndex(dir, r)
	if st, err := os.Stat(filepath.Join(dir, "stacklit.graph.json")); err == nil {
		r.GraphBytes = st.Size()
	}
	graphCounts(dir, r)
	// derive tokens
	dv := exec.Command(bin, "derive")
	dv.Dir = dir
	if b, err := dv.Output(); err == nil {
		r.DeriveTokens = int64(len(b)) / 4
	}
	r.Query = queries(bin, dir, env)
	return r
}

func repeat(n int, f func() runStat, each func(runStat)) *phase {
	p := &phase{}
	var walls []float64
	var rss []int64
	for i := 0; i < n; i++ {
		s := f()
		if each != nil {
			each(s)
		}
		p.Runs = append(p.Runs, s)
		if s.DNF {
			p.DNF = true
			return p
		}
		if !s.OK {
			p.Failed++
			continue
		}
		walls = append(walls, s.Wall)
		rss = append(rss, s.RSS)
	}
	if len(walls) > 0 {
		sort.Float64s(walls)
		sort.Slice(rss, func(i, j int) bool { return rss[i] < rss[j] })
		p.WallMedian = walls[len(walls)/2]
		p.RSSMedian = rss[len(rss)/2]
	}
	return p
}

func exec1(bin, dir string, env []string, args ...string) runStat {
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	start := time.Now()
	err := cmd.Run()
	s := runStat{Wall: time.Since(start).Seconds(), Stdout: so.Len(), Stderr: se.String()}
	if ctx.Err() != nil {
		s.DNF = true
		return s
	}
	s.OK = err == nil
	if cmd.ProcessState != nil {
		if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			s.RSS = int64(ru.Maxrss)
			if runtime.GOOS != "darwin" {
				s.RSS *= 1024
			}
		}
	}
	return s
}

// tracked counts git-tracked files and text bytes for the full-dump estimate.
func tracked(dir string, r *result) {
	b, err := exec.Command("git", "-C", dir, "ls-files", "-z").Output()
	if err != nil {
		return
	}
	buf := make([]byte, 8192)
	for _, rel := range bytes.Split(b, []byte{0}) {
		if len(rel) == 0 {
			continue
		}
		r.TrackedFiles++
		f, err := os.Open(filepath.Join(dir, string(rel)))
		if err != nil {
			continue
		}
		n, _ := io.ReadFull(f, buf)
		f.Close()
		if bytes.IndexByte(buf[:n], 0) >= 0 {
			continue // binary
		}
		if st, err := os.Stat(filepath.Join(dir, string(rel))); err == nil {
			r.TextBytes += st.Size()
		}
	}
	r.DumpTokens = r.TextBytes / 4
}

func readIndex(dir string, r *result) {
	p := filepath.Join(dir, "stacklit.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	r.IndexBytes = int64(len(b))
	r.IndexTokens = int64(len(b)) / 4
	var d struct {
		Tech struct {
			Languages map[string]struct {
				Files int `json:"files"`
			} `json:"languages"`
		} `json:"tech"`
		Structure struct {
			TotalFiles int `json:"total_files"`
			TotalLines int `json:"total_lines"`
		} `json:"structure"`
		Scan struct {
			SkippedLarge    []string `json:"skipped_large"`
			SkippedSymlinks []string `json:"skipped_symlinks"`
			SkippedOther    []string `json:"skipped_other"`
			ParseErrors     []string `json:"parse_errors"`
			Lightweight     []string `json:"lightweight"`
		} `json:"scan"`
	}
	if json.Unmarshal(b, &d) != nil {
		return
	}
	r.LangMix = map[string]int{}
	for k, v := range d.Tech.Languages {
		r.LangMix[k] = v.Files
	}
	r.ScannedFiles, r.TotalLines = d.Structure.TotalFiles, d.Structure.TotalLines
	r.Lightweight = len(d.Scan.Lightweight)
	r.SkippedLarge = len(d.Scan.SkippedLarge)
	r.SkippedOther = len(d.Scan.SkippedOther)
	r.SkippedSymlinks = len(d.Scan.SkippedSymlinks)
	r.ParseErrors = len(d.Scan.ParseErrors)
}

type gnode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
}
type gedge struct {
	F int `json:"f"`
	T int `json:"t"`
}

func loadGraph(dir string) (nodes []gnode, edges []gedge, comms int, err error) {
	f, err := os.Open(filepath.Join(dir, "stacklit.graph.json"))
	if err != nil {
		return nil, nil, 0, err
	}
	defer f.Close()
	var g struct {
		Nodes       []gnode           `json:"nodes"`
		Edges       []gedge           `json:"edges"`
		Communities []json.RawMessage `json:"communities"`
	}
	if err := json.NewDecoder(f).Decode(&g); err != nil {
		return nil, nil, 0, err
	}
	return g.Nodes, g.Edges, len(g.Communities), nil
}

func graphCounts(dir string, r *result) {
	n, e, c, err := loadGraph(dir)
	if err != nil {
		return
	}
	r.Nodes, r.Edges, r.Communities = len(n), len(e), c
}

var questions = []string{
	"how does error handling work",
	"where is configuration loaded",
	"how are requests routed",
	"how does caching work",
	"where is authentication handled",
	"how are tests organized",
	"how does the build or compile step work",
	"where is logging implemented",
}

// pick returns the id of the non-file node at degree rank k (clamped).
func pick(ids []string, k int) string {
	if len(ids) == 0 {
		return ""
	}
	if k >= len(ids) {
		k = len(ids) - 1
	}
	return ids[k]
}

// queries runs 8 query, 6 slice and 6 impact calls, qruns times each.
func queries(bin, dir string, env []string) *queryStats {
	nodes, edges, _, err := loadGraph(dir)
	if err != nil {
		return nil
	}
	deg := make([]int, len(nodes))
	for _, e := range edges {
		if e.F >= 0 && e.F < len(deg) {
			deg[e.F]++
		}
		if e.T >= 0 && e.T < len(deg) {
			deg[e.T]++
		}
	}
	rank := func(wantFile bool) []string {
		idx := []int{}
		for i, n := range nodes {
			if (n.Kind == "file") == wantFile {
				idx = append(idx, i)
			}
		}
		// Degree descending, id ascending: deterministic.
		sort.Slice(idx, func(a, b int) bool {
			if deg[idx[a]] != deg[idx[b]] {
				return deg[idx[a]] > deg[idx[b]]
			}
			return nodes[idx[a]].ID < nodes[idx[b]].ID
		})
		ids := make([]string, len(idx))
		for i, j := range idx {
			ids[i] = nodes[j].ID
		}
		return ids
	}
	syms, files := rank(false), rank(true)
	type call struct {
		kind string
		args []string
	}
	var calls []call
	for _, q := range questions {
		calls = append(calls, call{"query", []string{"query", q}})
	}
	for _, k := range []int{3, 10, 30, 100, 300, 1000} {
		calls = append(calls, call{"slice", []string{"slice", pick(syms, k)}})
	}
	for _, id := range []string{pick(syms, 0), pick(syms, 20), pick(syms, 200), pick(files, 0), pick(files, 20), pick(files, 200)} {
		calls = append(calls, call{"impact", []string{"impact", id}})
	}
	var all []float64
	by := map[string][]float64{}
	qs := &queryStats{}
	for i := 0; i < *qruns; i++ {
		for _, c := range calls {
			s := exec1(bin, dir, env, c.args...)
			qs.Calls++
			if s.RSS > qs.MaxRSS {
				qs.MaxRSS = s.RSS
			}
			if !s.OK || s.Stdout < 30 {
				qs.Failed++ // non-zero exit or an empty answer
				continue
			}
			ms := s.Wall * 1000
			all = append(all, ms)
			by[c.kind] = append(by[c.kind], ms)
		}
	}
	qs.P50ms, qs.P95ms = pct(all, 0.50), pct(all, 0.95)
	if len(all) > 0 {
		qs.MaxMS = pct(all, 1)
	}
	qs.P50Query, qs.P50Slice, qs.P50Imp = pct(by["query"], .5), pct(by["slice"], .5), pct(by["impact"], .5)
	return qs
}

func pct(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	i := int(p*float64(len(s))+0.999999) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(s) {
		i = len(s) - 1
	}
	return s[i]
}

func ph(p *phase) string {
	switch {
	case p == nil:
		return "n/a"
	case p.DNF:
		return "DNF"
	case p.WallMedian == 0:
		return "failed"
	}
	return fmt.Sprintf("%.1fs / %.2f GB", p.WallMedian, float64(p.RSSMedian)/(1<<30))
}

func renderTable(rs []*result) string {
	var sb strings.Builder
	sb.WriteString("| Repo | Ref | Files (tracked / scanned) | Lines | Full dump tokens | Cold generate | Warm generate | Cold generate --graph | stacklit.json (size / tokens) | derive tokens | Graph (size) | Nodes / edges / communities | Lightweight / skipped | Parse errors (panic warnings) | Query p50 / p95 |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rs {
		if r.Note != "" && r.Cold == nil {
			fmt.Fprintf(&sb, "| %s | %s | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | %s |\n", r.Name, r.Ref, r.Note)
			continue
		}
		q := "n/a"
		if r.Query != nil {
			q = fmt.Sprintf("%.0f / %.0f ms", r.Query.P50ms, r.Query.P95ms)
			if r.Query.Failed > 0 {
				q += fmt.Sprintf(" (%d of %d failed)", r.Query.Failed, r.Query.Calls)
			}
		}
		fmt.Fprintf(&sb, "| %s | %s | %d / %d | %d | %d | %s | %s | %s | %s / %d | %d | %s | %d / %d / %d | %d / %d | %d (%d) | %s |\n",
			r.Name, r.Ref, r.TrackedFiles, r.ScannedFiles, r.TotalLines, r.DumpTokens,
			ph(r.Cold), ph(r.Warm), ph(r.ColdGraph),
			human(r.IndexBytes), r.IndexTokens, r.DeriveTokens, human(r.GraphBytes),
			r.Nodes, r.Edges, r.Communities,
			r.Lightweight, r.SkippedLarge+r.SkippedOther+r.SkippedSymlinks,
			r.ParseErrors, r.ParserPanics, q)
	}
	return sb.String()
}

func human(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}

func replaceBlock(path, machine, table string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	const begin, end = "<!-- scale:begin -->", "<!-- scale:end -->"
	s := string(b)
	i, j := strings.Index(s, begin), strings.Index(s, end)
	if i < 0 || j < i {
		return fmt.Errorf("%s: markers %s ... %s not found", path, begin, end)
	}
	block := begin + "\n\nMachine: `" + machine + "`\n\n" + table + "\n" + end
	return os.WriteFile(path, []byte(s[:i]+block+s[j+len(end):]), 0o644)
}
