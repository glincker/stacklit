package engine

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/monorepo"
	"github.com/glincker/stacklit/internal/parser"
	"github.com/glincker/stacklit/internal/schema"
)

type pkgDef struct {
	info     schema.PackageInfo
	manifest monorepo.Manifest
	hasName  bool
}

// buildPackages turns detected workspaces into package boundaries and finds
// the dependency edges that cross them. Edges come from manifest-name imports
// (e.g. "@acme/ui"), relative imports that climb out of a package, and module
// edges that already cross a package line.
func buildPackages(root string, workspaces []string, parsed []*parser.FileInfo, moduleEdges [][2]string, moduleNames []string) ([]schema.PackageInfo, []schema.PackageEdge) {
	if len(workspaces) == 0 {
		return nil, nil
	}

	defs := make([]*pkgDef, 0, len(workspaces))
	seenPath := map[string]bool{}
	usedName := map[string]bool{}
	for _, ws := range workspaces {
		p := path.Clean(filepath.ToSlash(ws))
		if p == "." || p == "" || seenPath[p] {
			continue
		}
		seenPath[p] = true
		d := &pkgDef{info: schema.PackageInfo{Name: p, Path: p}}
		if m, ok := monorepo.ReadManifest(filepath.Join(root, filepath.FromSlash(p))); ok && !usedName[m.Name] {
			d.manifest, d.hasName = m, true
			d.info.Name, d.info.Kind = m.Name, m.Kind
		}
		usedName[d.info.Name] = true
		defs = append(defs, d)
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].info.Path < defs[j].info.Path })

	// Longest-prefix owner lookup.
	owner := func(p string) *pkgDef {
		var best *pkgDef
		for _, d := range defs {
			if p == d.info.Path || strings.HasPrefix(p, d.info.Path+"/") {
				if best == nil || len(d.info.Path) > len(best.info.Path) {
					best = d
				}
			}
		}
		return best
	}

	weights := map[[2]string]int{}
	add := func(from, to *pkgDef, n int) {
		if from == nil || to == nil || from == to {
			return
		}
		weights[[2]string{from.info.Name, to.info.Name}] += n
	}

	for _, f := range parsed {
		from := owner(f.Path)
		if from == nil {
			continue
		}
		from.info.Files++
		from.info.Lines += f.LineCount
		dir := path.Dir(f.Path)
		for _, imp := range f.Imports {
			if strings.HasPrefix(imp, "./") || strings.HasPrefix(imp, "../") {
				add(from, owner(path.Clean(path.Join(dir, imp))), 1)
				continue
			}
			for _, d := range defs {
				if d != from && d.hasName && d.manifest.Matches(imp) {
					add(from, d, 1)
					break
				}
			}
		}
	}

	for _, name := range moduleNames {
		if d := owner(name); d != nil {
			d.info.Modules = append(d.info.Modules, name)
		}
	}
	for _, e := range moduleEdges {
		from, to := owner(e[0]), owner(e[1])
		if from != nil && to != nil && from != to {
			k := [2]string{from.info.Name, to.info.Name}
			if weights[k] == 0 {
				weights[k] = 1
			}
		}
	}

	byName := map[string]*pkgDef{}
	for _, d := range defs {
		byName[d.info.Name] = d
	}
	edges := make([]schema.PackageEdge, 0, len(weights))
	for k, w := range weights {
		edges = append(edges, schema.PackageEdge{From: k[0], To: k[1], Weight: w})
		byName[k[0]].info.DependsOn = append(byName[k[0]].info.DependsOn, k[1])
		byName[k[1]].info.DependedBy = append(byName[k[1]].info.DependedBy, k[0])
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	pkgs := make([]schema.PackageInfo, 0, len(defs))
	for _, d := range defs {
		sort.Strings(d.info.Modules)
		sort.Strings(d.info.DependsOn)
		sort.Strings(d.info.DependedBy)
		pkgs = append(pkgs, d.info)
	}
	return pkgs, edges
}
