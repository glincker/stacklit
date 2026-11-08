package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glincker/stacklit/internal/fsutil"
	"github.com/glincker/stacklit/internal/monorepo"
	"github.com/glincker/stacklit/internal/parser"
	"github.com/glincker/stacklit/internal/schema"
)

// DefaultMultiOutput is where RunMulti writes when no output is given.
const DefaultMultiOutput = "stacklit-multi.json"

// maxViaModules caps how many importing modules a cross-repo edge lists.
const maxViaModules = 5

// MultiOptions configures a RunMulti call.
type MultiOptions struct {
	ReposFile  string
	Output     string // defaults to DefaultMultiOutput in the current directory
	Quiet      bool
	Timestamps bool // stamp generated_at; off by default so the file never churns
}

// MultiResult holds the output of a RunMulti call.
type MultiResult struct {
	OutputPath string
	RepoCount  int
	Failed     int
	Duration   time.Duration
	Index      *schema.MultiIndex
}

// ParseReposFile reads one repo path per line. Blank lines and # comments are
// skipped, a leading ~ expands to the home directory, and relative paths
// resolve against the repos file's own directory so the list works from any cwd.
func ParseReposFile(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading repos file: %w", err)
	}
	base := filepath.Dir(file)
	home, _ := os.UserHomeDir()
	seen := map[string]bool{}
	var repos []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if home != "" && (line == "~" || strings.HasPrefix(line, "~/")) {
			line = filepath.Join(home, strings.TrimPrefix(line, "~"))
		}
		if !filepath.IsAbs(line) {
			line = filepath.Join(base, filepath.FromSlash(line))
		}
		line, err = filepath.Abs(line)
		if err != nil {
			return nil, err
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		repos = append(repos, line)
	}
	if len(repos) == 0 {
		return nil, errors.New("repos file lists no repositories")
	}
	return repos, nil
}

type repoScan struct {
	name     string
	path     string
	display  string // repo path relative to the repos file, slash-separated
	result   *Result
	provides []string
}

// RunMulti scans every repo listed in a repos file without writing into any
// of them, then writes one combined index with per-repo package boundaries
// and cross-repo dependency edges.
func RunMulti(opts MultiOptions) (*MultiResult, error) {
	start := time.Now()
	repos, err := ParseReposFile(opts.ReposFile)
	if err != nil {
		return nil, err
	}

	reposDir, _ := filepath.Abs(filepath.Dir(opts.ReposFile))
	var scans []repoScan
	var failed []schema.RepoFailure
	usedNames := map[string]int{}
	for _, repo := range repos {
		if !opts.Quiet {
			fmt.Printf("[stacklit] scanning %s...\n", repo)
		}
		res, err := Run(Options{Root: repo, Quiet: true, NoWrite: true})
		if err != nil {
			fmt.Fprintf(os.Stderr, "[stacklit] warning: failed to scan %s: %v\n", repo, err)
			failed = append(failed, schema.RepoFailure{Path: filepath.ToSlash(repo), Error: err.Error()})
			continue
		}
		name := res.Index.Project.Name
		if n := usedNames[name]; n > 0 {
			// Two repos with the same folder name must stay distinguishable.
			name = fmt.Sprintf("%s-%d", name, n+1)
		}
		usedNames[res.Index.Project.Name]++
		display := filepath.ToSlash(repo)
		if rel, relErr := filepath.Rel(reposDir, repo); relErr == nil {
			display = filepath.ToSlash(rel)
		}
		scans = append(scans, repoScan{name: name, path: repo, display: display, result: res, provides: res.Provides})
	}
	if len(scans) == 0 {
		return nil, fmt.Errorf("none of the %d repos could be scanned", len(repos))
	}

	multi := buildMultiIndex(scans, failed, opts.Timestamps)

	output := opts.Output
	if output == "" {
		output = DefaultMultiOutput
	}
	data, err := json.MarshalIndent(multi, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling multi-index: %w", err)
	}
	if err := fsutil.WriteFileIfChanged(output, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", output, err)
	}

	dur := time.Since(start)
	if !opts.Quiet {
		fmt.Printf("[stacklit] multi-repo scan done in %s: %d repos, %d modules, %d files, %d cross-repo edges\n",
			dur.Round(time.Millisecond), len(scans), multi.TotalModules, multi.TotalFiles, len(multi.CrossEdges))
	}
	return &MultiResult{OutputPath: output, RepoCount: len(scans), Failed: len(failed), Duration: dur, Index: multi}, nil
}

// buildMultiIndex assembles the combined index in a stable repo order.
func buildMultiIndex(scans []repoScan, failed []schema.RepoFailure, stamp bool) *schema.MultiIndex {
	sort.SliceStable(scans, func(i, j int) bool { return scans[i].name < scans[j].name })

	multi := &schema.MultiIndex{
		Schema:        "https://stacklit.dev/schema/v1-multi.json",
		Version:       "1",
		SchemaVersion: schema.CurrentSchemaVersion,
		Type:          "polyrepo",
		Failed:        failed,
	}
	if stamp {
		multi.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}

	for _, sc := range scans {
		idx := sc.result.Index
		multi.TotalFiles += idx.Structure.TotalFiles
		multi.TotalLines += idx.Structure.TotalLines
		multi.TotalModules += len(idx.Modules)
		multi.Repos = append(multi.Repos, schema.RepoSummary{
			Name:            sc.name,
			Path:            sc.display,
			PrimaryLanguage: idx.Tech.PrimaryLanguage,
			TotalFiles:      idx.Structure.TotalFiles,
			TotalLines:      idx.Structure.TotalLines,
			MerkleHash:      idx.MerkleHash,
			Provides:        sc.provides,
			Modules:         idx.Modules,
			Packages:        idx.Packages,
			PackageEdges:    idx.PackageEdges,
			Frameworks:      idx.Tech.Frameworks,
			Entrypoints:     idx.Structure.Entrypoints,
		})
	}
	multi.CrossEdges = crossRepoEdges(scans)
	return multi
}

// crossRepoEdges links repo A to repo B when a module in A imports a name that
// B publishes through a manifest. Matching on manifest names (not on paths)
// keeps it exact: no guessing from folder names.
func crossRepoEdges(scans []repoScan) []schema.CrossRepoEdge {
	var providers []struct {
		repo string
		m    monorepo.Manifest
	}
	for _, sc := range scans {
		for _, name := range sc.provides {
			kind, n := splitProvide(name)
			providers = append(providers, struct {
				repo string
				m    monorepo.Manifest
			}{sc.name, monorepo.Manifest{Name: n, Kind: kind}})
		}
	}

	type key struct{ from, to string }
	weights := map[key]int{}
	via := map[key]map[string]bool{}
	for _, sc := range scans {
		for mod, imports := range sc.result.Imports {
			for _, imp := range imports {
				for _, p := range providers {
					if p.repo == sc.name || !p.m.Matches(imp) {
						continue
					}
					k := key{sc.name, p.repo}
					weights[k]++
					if via[k] == nil {
						via[k] = map[string]bool{}
					}
					via[k][mod] = true
					break
				}
			}
		}
	}

	edges := make([]schema.CrossRepoEdge, 0, len(weights))
	for k, w := range weights {
		mods := make([]string, 0, len(via[k]))
		for m := range via[k] {
			mods = append(mods, m)
		}
		sort.Strings(mods)
		if len(mods) > maxViaModules {
			mods = mods[:maxViaModules]
		}
		edges = append(edges, schema.CrossRepoEdge{From: k.from, To: k.to, Weight: w, Via: mods})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	return edges
}

// provideSep separates kind and name in Result.Provides entries ("npm:@acme/ui").
const provideSep = ":"

func splitProvide(s string) (kind, name string) {
	i := strings.Index(s, provideSep)
	if i < 0 {
		return "", s
	}
	return s[:i], s[i+1:]
}

// repoProvides lists the manifest identities a repo publishes: its root
// manifest plus one per workspace package.
func repoProvides(root string, mono *monorepo.Result) []string {
	seen := map[string]bool{}
	var out []string
	add := func(dir string) {
		if m, ok := monorepo.ReadManifest(dir); ok {
			v := m.Kind + provideSep + m.Name
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	add(root)
	if mono != nil {
		for _, ws := range mono.Workspaces {
			add(filepath.Join(root, filepath.FromSlash(ws)))
		}
	}
	sort.Strings(out)
	return out
}

// importsByModule groups every file's imports under the module it belongs to.
func importsByModule(parsed []*parser.FileInfo, modules map[string]schema.ModuleInfo) map[string][]string {
	owner := func(file string) string {
		dir := path.Dir(file)
		for dir != "." && dir != "/" && dir != "" {
			if _, ok := modules[dir]; ok {
				return dir
			}
			dir = path.Dir(dir)
		}
		if _, ok := modules["root"]; ok {
			return "root"
		}
		return ""
	}
	sets := map[string]map[string]bool{}
	for _, f := range parsed {
		m := owner(f.Path)
		if m == "" {
			continue
		}
		if sets[m] == nil {
			sets[m] = map[string]bool{}
		}
		for _, imp := range f.Imports {
			sets[m][imp] = true
		}
	}
	out := make(map[string][]string, len(sets))
	for m, set := range sets {
		list := make([]string, 0, len(set))
		for imp := range set {
			list = append(list, imp)
		}
		sort.Strings(list)
		out[m] = list
	}
	return out
}
