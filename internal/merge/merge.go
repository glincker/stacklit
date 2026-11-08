// Package merge combines symbol graphs from several repositories into one
// and links repositories that depend on each other through manifest names.
package merge

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/schema"
)

// KindRepo is the node kind for a whole repository.
const KindRepo = "repo"

// EdgeDependsOn links one repo node to another.
const EdgeDependsOn = "depends_on"

// Input is one graph to merge under a repo name.
type Input struct {
	Name  string
	Graph *graph.SymGraph
}

// DefaultName picks a repo name from the graph root or the file path.
func DefaultName(g *graph.SymGraph, file string) string {
	if g != nil && g.Root != "" && g.Root != "." {
		return filepath.Base(g.Root)
	}
	dir := filepath.Base(filepath.Dir(file))
	if dir != "." && dir != string(filepath.Separator) && dir != "" {
		return dir
	}
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}

func repoID(name string) string { return "repo:" + name }

func qualify(repo, id string) string { return repo + ":" + id }

// Merge returns one graph holding every node and edge, with IDs prefixed by the
// repo name so they cannot collide. source_file (File) is kept on every node.
// When multi is non-nil, repos are linked using its manifest-based cross-repo edges.
func Merge(inputs []Input, multi *schema.MultiIndex) (*graph.SymGraph, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("nothing to merge")
	}
	seen := map[string]bool{}
	for _, in := range inputs {
		if in.Name == "" || in.Graph == nil {
			return nil, fmt.Errorf("every input needs a name and a graph")
		}
		if seen[in.Name] {
			return nil, fmt.Errorf("duplicate repo name %q (use name=path.json to rename)", in.Name)
		}
		seen[in.Name] = true
	}

	out := &graph.SymGraph{Version: graph.SchemaVersion, Root: "."}
	repoNode := map[string]int{}
	filesByRepo := map[string]map[string]int{}
	commOffset := 0

	for _, in := range inputs {
		g := in.Graph
		base := len(out.Nodes)
		files := map[string]int{}
		for _, n := range g.Nodes {
			n.Repo = in.Name
			n.ID = qualify(in.Name, n.ID)
			if n.File == "" {
				n.File = strings.SplitN(strings.TrimPrefix(n.ID, in.Name+":"), "::", 2)[0]
			}
			n.Community += commOffset
			if n.Kind == graph.KindFile {
				files[n.File] = len(out.Nodes)
			}
			out.Nodes = append(out.Nodes, n)
		}
		for _, e := range g.Edges {
			e.From += base
			e.To += base
			out.Edges = append(out.Edges, e)
		}
		maxC := -1
		for _, c := range g.Communities {
			if c.ID > maxC {
				maxC = c.ID
			}
			c.ID += commOffset
			c.Label = in.Name + "/" + c.Label
			out.Communities = append(out.Communities, c)
		}
		for _, n := range g.Nodes {
			if n.Community > maxC {
				maxC = n.Community
			}
		}
		commOffset += maxC + 1
		filesByRepo[in.Name] = files
	}

	if multi != nil {
		addLinks(out, inputs, multi, filesByRepo, repoNode, commOffset)
	}
	out.Index()
	return out, nil
}

// addLinks adds repo nodes and edges for each cross-repo edge between merged repos.
func addLinks(out *graph.SymGraph, inputs []Input, multi *schema.MultiIndex, filesByRepo map[string]map[string]int, repoNode map[string]int, comm int) {
	names := map[string]bool{}
	for _, in := range inputs {
		names[in.Name] = true
	}
	repoPath := map[string]string{}
	for _, r := range multi.Repos {
		repoPath[r.Name] = r.Path
	}
	ensure := func(name string) int {
		if i, ok := repoNode[name]; ok {
			return i
		}
		file := repoPath[name]
		if file == "" {
			file = name
		}
		out.Nodes = append(out.Nodes, graph.SymNode{ID: repoID(name), Name: name, Kind: KindRepo, File: file, Community: comm, Repo: name})
		repoNode[name] = len(out.Nodes) - 1
		return repoNode[name]
	}
	linked := false
	for _, ce := range multi.CrossEdges {
		if !names[ce.From] || !names[ce.To] {
			continue
		}
		linked = true
		from, to := ensure(ce.From), ensure(ce.To)
		out.Edges = append(out.Edges, graph.SymEdge{From: from, To: to, Kind: EdgeDependsOn, Conf: graph.ConfExtracted, Score: 1, Src: "manifest"})
		paths := make([]string, 0, len(filesByRepo[ce.From]))
		for f := range filesByRepo[ce.From] {
			paths = append(paths, f)
		}
		sort.Strings(paths)
		for _, mod := range ce.Via {
			for _, f := range paths {
				if mod == "root" || path.Dir(f) == mod || strings.HasPrefix(f, mod+"/") {
					out.Edges = append(out.Edges, graph.SymEdge{From: filesByRepo[ce.From][f], To: to, Kind: graph.EdgeImports, Conf: graph.ConfExtracted, Score: 1, Src: "manifest"})
				}
			}
		}
	}
	if linked {
		out.Communities = append(out.Communities, graph.Community{ID: comm, Label: "repos", Nodes: len(repoNode)})
	}
}
