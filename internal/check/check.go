// Package check evaluates architecture rules from .stacklitrc.json against the
// module graph: forbidden edges, cycles, fan-in and fan-out limits, orphans.
package check

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/schema"
)

// Rule names used in violations.
const (
	RuleForbidden = "forbidden-edge"
	RuleCycle     = "cycle"
	RuleFanIn     = "max-fan-in"
	RuleFanOut    = "max-fan-out"
	RuleOrphan    = "orphan"
)

// Violation is one broken rule.
type Violation struct {
	Rule    string   `json:"rule"`
	Message string   `json:"message"`
	Modules []string `json:"modules"`
}

// Result is the full check outcome.
type Result struct {
	OK         bool        `json:"ok"`
	Violations []Violation `json:"violations"`
}

// match reports whether module matches a pattern: exact, path.Match glob, or "dir/**".
func match(pattern, module string) bool {
	if pattern == module || pattern == "**" {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		return module == base || strings.HasPrefix(module, base+"/")
	}
	ok, err := path.Match(pattern, module)
	return err == nil && ok
}

// edges returns the sorted, de-duplicated module edges that reference known modules.
func edges(idx *schema.Index) [][2]string {
	seen := map[[2]string]bool{}
	var out [][2]string
	add := func(a, b string) {
		k := [2]string{a, b}
		if a == b || seen[k] {
			return
		}
		if _, ok := idx.Modules[a]; !ok {
			return
		}
		if _, ok := idx.Modules[b]; !ok {
			return
		}
		seen[k] = true
		out = append(out, k)
	}
	for _, e := range idx.Dependencies.Edges {
		add(e[0], e[1])
	}
	for n, m := range idx.Modules {
		for _, d := range m.DependsOn {
			add(n, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// Run evaluates cfg against idx. A nil cfg yields no violations.
func Run(idx *schema.Index, cfg *config.CheckConfig) *Result {
	res := &Result{OK: true, Violations: []Violation{}}
	if cfg == nil {
		return res
	}
	es := edges(idx)

	for _, e := range es {
		for _, r := range cfg.Forbidden {
			if match(r.From, e[0]) && match(r.To, e[1]) {
				msg := e[0] + " must not depend on " + e[1]
				if r.Reason != "" {
					msg += " (" + r.Reason + ")"
				}
				res.Violations = append(res.Violations, Violation{Rule: RuleForbidden, Message: msg, Modules: []string{e[0], e[1]}})
				break
			}
		}
	}

	if cfg.Cycles {
		for _, c := range cycles(idx, es) {
			res.Violations = append(res.Violations, Violation{
				Rule: RuleCycle, Message: "import cycle: " + strings.Join(append(append([]string{}, c...), c[0]), " -> "), Modules: c,
			})
		}
	}

	fanIn, fanOut := map[string]int{}, map[string]int{}
	for _, e := range es {
		fanOut[e[0]]++
		fanIn[e[1]]++
	}
	names := make([]string, 0, len(idx.Modules))
	for n := range idx.Modules {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if cfg.MaxFanIn > 0 && fanIn[n] > cfg.MaxFanIn {
			res.Violations = append(res.Violations, Violation{Rule: RuleFanIn, Modules: []string{n},
				Message: n + " has fan-in " + strconv.Itoa(fanIn[n]) + ", limit " + strconv.Itoa(cfg.MaxFanIn)})
		}
		if cfg.MaxFanOut > 0 && fanOut[n] > cfg.MaxFanOut {
			res.Violations = append(res.Violations, Violation{Rule: RuleFanOut, Modules: []string{n},
				Message: n + " has fan-out " + strconv.Itoa(fanOut[n]) + ", limit " + strconv.Itoa(cfg.MaxFanOut)})
		}
		if cfg.Orphans && fanIn[n] == 0 && fanOut[n] == 0 {
			res.Violations = append(res.Violations, Violation{Rule: RuleOrphan, Modules: []string{n},
				Message: n + " has no dependencies and nothing depends on it"})
		}
	}
	res.OK = len(res.Violations) == 0
	return res
}

// cycles returns each strongly connected component with more than one module,
// as a sorted-start walk, using Tarjan's algorithm.
func cycles(idx *schema.Index, es [][2]string) [][]string {
	adj := map[string][]string{}
	for _, e := range es {
		adj[e[0]] = append(adj[e[0]], e[1])
	}
	names := make([]string, 0, len(idx.Modules))
	for n := range idx.Modules {
		names = append(names, n)
	}
	sort.Strings(names)

	index := map[string]int{}
	low := map[string]int{}
	on := map[string]bool{}
	var stack []string
	next := 0
	var out [][]string
	var visit func(v string)
	visit = func(v string) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		on[v] = true
		for _, w := range adj[v] {
			if _, seen := index[w]; !seen {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if on[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] == index[v] {
			var comp []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			if len(comp) > 1 {
				sort.Strings(comp)
				out = append(out, comp)
			}
		}
	}
	for _, n := range names {
		if _, seen := index[n]; !seen {
			visit(n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}
