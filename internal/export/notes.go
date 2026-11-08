package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/graph"
)

// noteName maps a module path to a flat, filesystem-safe note name.
func noteName(module string) string {
	r := strings.NewReplacer("/", "__", "\\", "__", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", "#", "_", "[", "_", "]", "_")
	return r.Replace(module)
}

// linkStyle renders links for either an Obsidian vault or a plain markdown wiki.
type linkStyle struct {
	obsidian bool
	dir      string // folder holding module notes, relative to the output root
}

func (l linkStyle) module(name string) string {
	if l.obsidian {
		return fmt.Sprintf("[[%s|%s]]", noteName(name), name)
	}
	return fmt.Sprintf("[%s](%s.md)", name, noteName(name))
}

func (l linkStyle) symbol(module, sym string) string {
	if l.obsidian {
		return fmt.Sprintf("[[%s#%s|%s]]", noteName(module), sym, sym)
	}
	return fmt.Sprintf("[%s](%s.md#%s)", sym, noteName(module), anchor(sym))
}

func anchor(s string) string {
	return strings.ToLower(strings.NewReplacer(" ", "-", "(", "", ")", "", ".", "", "/", "").Replace(s))
}

type symbolRow struct {
	idx    int
	node   graph.SymNode
	module string
}

// symbolsByModule groups non-file symbols under the module that owns the file.
func symbolsByModule(in Input) map[string][]symbolRow {
	out := map[string][]symbolRow{}
	if in.Graph == nil {
		return out
	}
	for i, n := range in.Graph.Nodes {
		if n.Kind == graph.KindFile {
			continue
		}
		mod := moduleOf(in.Index, n.File)
		if mod == "" {
			continue
		}
		out[mod] = append(out[mod], symbolRow{idx: i, node: n, module: mod})
	}
	for k := range out {
		rows := out[k]
		sort.SliceStable(rows, func(a, b int) bool {
			if rows[a].node.File != rows[b].node.File {
				return rows[a].node.File < rows[b].node.File
			}
			if rows[a].node.Line != rows[b].node.Line {
				return rows[a].node.Line < rows[b].node.Line
			}
			return rows[a].node.ID < rows[b].node.ID
		})
	}
	return out
}

// modulePage renders one module note, stopping when the token budget is spent.
func modulePage(in Input, name string, syms []symbolRow, ls linkStyle, budget int) string {
	mi := in.Index.Modules[name]
	var b strings.Builder
	if ls.obsidian {
		fmt.Fprintf(&b, "---\ntags: [stacklit, module]\nlanguage: %s\nfiles: %d\nlines: %d\n---\n", mi.Language, mi.Files, mi.Lines)
	}
	fmt.Fprintf(&b, "# %s\n\n", name)
	if mi.Purpose != "" {
		fmt.Fprintf(&b, "%s\n\n", oneLine(mi.Purpose))
	}
	fmt.Fprintf(&b, "%s, %d files, %d lines\n\n", orDash(mi.Language), mi.Files, mi.Lines)
	section := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		for i, it := range items {
			if approxTokens(b.String()) >= budget {
				fmt.Fprintf(&b, "- ... %d more\n", len(items)-i)
				break
			}
			fmt.Fprintf(&b, "- %s\n", it)
		}
		b.WriteString("\n")
	}
	var deps, rdeps []string
	for _, d := range sortedCopy(mi.DependsOn) {
		if _, ok := in.Index.Modules[d]; ok {
			deps = append(deps, ls.module(d))
		}
	}
	for _, d := range sortedCopy(mi.DependedBy) {
		if _, ok := in.Index.Modules[d]; ok {
			rdeps = append(rdeps, ls.module(d))
		}
	}
	section("Depends on", deps)
	section("Depended on by", rdeps)
	section("Exports", sortedCopy(mi.Exports))

	if len(syms) == 0 {
		return b.String()
	}
	b.WriteString("## Symbols\n\n")
	for i, s := range syms {
		if approxTokens(b.String()) >= budget {
			fmt.Fprintf(&b, "... %d more symbols\n", len(syms)-i)
			break
		}
		fmt.Fprintf(&b, "### %s\n\n%s `%s:%d`\n", s.node.Name, s.node.Kind, s.node.File, s.node.Line)
		if uses := useLinks(in, s, ls); len(uses) > 0 {
			fmt.Fprintf(&b, "Uses: %s\n", strings.Join(uses, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// useLinks lists outgoing symbol edges that cross into another module.
func useLinks(in Input, s symbolRow, ls linkStyle) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range in.Graph.Edges {
		if e.From != s.idx {
			continue
		}
		t := in.Graph.Nodes[e.To]
		if t.Kind == graph.KindFile {
			continue
		}
		tm := moduleOf(in.Index, t.File)
		if tm == "" || tm == s.module {
			continue
		}
		l := ls.symbol(tm, t.Name)
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func renderObsidian(in Input, o Options) []File {
	ls := linkStyle{obsidian: true}
	syms := symbolsByModule(in)
	var files []File
	var idx strings.Builder
	fmt.Fprintf(&idx, "---\ntags: [stacklit]\n---\n# %s\n\n", orDash(in.Index.Project.Name))
	for _, name := range sortedModules(in.Index) {
		fmt.Fprintf(&idx, "- %s\n", ls.module(name))
		files = append(files, File{Name: noteName(name) + ".md", Content: modulePage(in, name, syms[name], ls, o.pageTokens())})
	}
	files = append([]File{{Name: "Index.md", Content: idx.String()}}, files...)
	return files
}

func renderWiki(in Input, o Options) []File {
	ls := linkStyle{dir: "modules"}
	syms := symbolsByModule(in)
	var files []File
	var idx strings.Builder
	fmt.Fprintf(&idx, "# %s\n\n", orDash(in.Index.Project.Name))
	fmt.Fprintf(&idx, "%s, %d files, %d lines, %d modules\n\n", orDash(in.Index.Tech.PrimaryLanguage),
		in.Index.Structure.TotalFiles, in.Index.Structure.TotalLines, len(in.Index.Modules))
	idx.WriteString("## Modules\n\n")
	names := sortedModules(in.Index)
	for i, name := range names {
		if approxTokens(idx.String()) >= o.pageTokens() {
			fmt.Fprintf(&idx, "- ... %d more\n", len(names)-i)
			break
		}
		mi := in.Index.Modules[name]
		line := fmt.Sprintf("- %s: %s", ls.module(name), oneLine(mi.Purpose))
		fmt.Fprintln(&idx, strings.TrimSuffix(line, ": "))
	}
	files = append(files, File{Name: "index.md", Content: idx.String()})
	for _, name := range names {
		files = append(files, File{Name: "modules/" + noteName(name) + ".md", Content: modulePage(in, name, syms[name], ls, o.pageTokens())})
	}
	return files
}
