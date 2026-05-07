package renderer

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/schema"
)

// RenderMarkdown returns a human-readable markdown overview of idx, designed
// for pasting into PR descriptions, GitHub issues, or chat. It's deterministic:
// modules and dependencies are sorted, so the output diffs cleanly between
// runs and between machines.
func RenderMarkdown(idx *schema.Index) string {
	var sb strings.Builder

	writeHeader(&sb, idx)
	writeOverview(&sb, idx)
	writeEntrypoints(&sb, idx)
	writeKeyDirectories(&sb, idx)
	writeModulesTable(&sb, idx)
	writeModuleDetails(&sb, idx)
	writeDependencies(&sb, idx)
	writeHotFiles(&sb, idx)
	writeHints(&sb, idx)
	writeFooter(&sb, idx)

	return sb.String()
}

// WriteMarkdown writes the rendered markdown to path. If the file already
// matches the new content byte-for-byte, the file is left untouched so commit
// hooks don't see spurious changes.
func WriteMarkdown(idx *schema.Index, path string) error {
	data := []byte(RenderMarkdown(idx))
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, 0644)
}

func writeHeader(sb *strings.Builder, idx *schema.Index) {
	name := idx.Project.Name
	if name == "" {
		name = "Project"
	}
	fmt.Fprintf(sb, "# %s\n\n", name)

	parts := []string{}
	if idx.Tech.PrimaryLanguage != "" {
		parts = append(parts, idx.Tech.PrimaryLanguage)
	}
	if idx.Project.Type != "" {
		parts = append(parts, idx.Project.Type)
	}
	if idx.Structure.TotalFiles > 0 {
		parts = append(parts, fmt.Sprintf("%d files", idx.Structure.TotalFiles))
	}
	if idx.Structure.TotalLines > 0 {
		parts = append(parts, fmt.Sprintf("%s lines", formatThousands(idx.Structure.TotalLines)))
	}
	if len(parts) > 0 {
		sb.WriteString(strings.Join(parts, " · "))
		sb.WriteString("\n\n")
	}
}

func writeOverview(sb *strings.Builder, idx *schema.Index) {
	hasLangs := len(idx.Tech.Languages) > 0
	hasFrameworks := len(idx.Tech.Frameworks) > 0
	if !hasLangs && !hasFrameworks && idx.Architecture.Pattern == "" {
		return
	}

	sb.WriteString("## Overview\n\n")

	if idx.Architecture.Pattern != "" {
		fmt.Fprintf(sb, "Pattern: %s\n\n", idx.Architecture.Pattern)
	}

	if hasLangs {
		sb.WriteString("| Language | Files | Lines |\n")
		sb.WriteString("|----------|-------|-------|\n")
		langs := sortedKeys(idx.Tech.Languages)
		for _, lang := range langs {
			s := idx.Tech.Languages[lang]
			fmt.Fprintf(sb, "| %s | %d | %s |\n", escapePipe(lang), s.Files, formatThousands(s.Lines))
		}
		sb.WriteString("\n")
	}

	if hasFrameworks {
		sb.WriteString("Frameworks: ")
		sb.WriteString(strings.Join(idx.Tech.Frameworks, ", "))
		sb.WriteString("\n\n")
	}
}

func writeEntrypoints(sb *strings.Builder, idx *schema.Index) {
	if len(idx.Structure.Entrypoints) == 0 {
		return
	}
	sb.WriteString("## Entrypoints\n\n")
	entries := append([]string(nil), idx.Structure.Entrypoints...)
	sort.Strings(entries)
	for _, e := range entries {
		fmt.Fprintf(sb, "- `%s`\n", e)
	}
	sb.WriteString("\n")
}

func writeKeyDirectories(sb *strings.Builder, idx *schema.Index) {
	if len(idx.Structure.KeyDirectories) == 0 {
		return
	}
	sb.WriteString("## Key directories\n\n")
	keys := sortedKeys(idx.Structure.KeyDirectories)
	for _, k := range keys {
		fmt.Fprintf(sb, "- `%s` — %s\n", k, idx.Structure.KeyDirectories[k])
	}
	sb.WriteString("\n")
}

func writeModulesTable(sb *strings.Builder, idx *schema.Index) {
	if len(idx.Modules) == 0 {
		return
	}
	sb.WriteString("## Modules\n\n")
	sb.WriteString("| Module | Purpose | Files | Lines | Depends on |\n")
	sb.WriteString("|--------|---------|-------|-------|------------|\n")

	names := sortedKeys(idx.Modules)
	for _, n := range names {
		m := idx.Modules[n]
		deps := "—"
		if len(m.DependsOn) > 0 {
			d := make([]string, 0, len(m.DependsOn))
			d = append(d, m.DependsOn...)
			sort.Strings(d)
			// Escape pipes per-element so `foo|bar` deps don't break the row.
			for i, s := range d {
				d[i] = escapePipe(s)
			}
			deps = "`" + strings.Join(d, "`, `") + "`"
		}
		fmt.Fprintf(sb, "| `%s` | %s | %d | %s | %s |\n",
			escapePipe(n),
			escapePipe(m.Purpose),
			m.Files,
			formatThousands(m.Lines),
			deps,
		)
	}
	sb.WriteString("\n")
}

// writeModuleDetails emits a collapsible <details> block per module that has
// exports or type definitions. Keeps the top-level overview short while
// still letting readers drill in.
func writeModuleDetails(sb *strings.Builder, idx *schema.Index) {
	names := sortedKeys(idx.Modules)
	hasAny := false
	for _, n := range names {
		m := idx.Modules[n]
		if len(m.Exports) == 0 && len(m.TypeDefs) == 0 {
			continue
		}
		hasAny = true
		break
	}
	if !hasAny {
		return
	}

	sb.WriteString("## Module details\n\n")

	for _, n := range names {
		m := idx.Modules[n]
		if len(m.Exports) == 0 && len(m.TypeDefs) == 0 {
			continue
		}
		fmt.Fprintf(sb, "<details><summary><code>%s</code></summary>\n\n", n)
		if m.Purpose != "" {
			fmt.Fprintf(sb, "%s\n\n", m.Purpose)
		}
		if len(m.Exports) > 0 {
			sb.WriteString("**Exports**\n\n")
			exports := append([]string(nil), m.Exports...)
			sort.Strings(exports)
			for _, e := range exports {
				fmt.Fprintf(sb, "- `%s`\n", e)
			}
			sb.WriteString("\n")
		}
		if len(m.TypeDefs) > 0 {
			sb.WriteString("**Types**\n\n")
			tnames := sortedKeys(m.TypeDefs)
			for _, t := range tnames {
				fmt.Fprintf(sb, "- `%s` — %s\n", t, m.TypeDefs[t])
			}
			sb.WriteString("\n")
		}
		sb.WriteString("</details>\n\n")
	}
}

func writeDependencies(sb *strings.Builder, idx *schema.Index) {
	if len(idx.Dependencies.Edges) == 0 &&
		len(idx.Dependencies.MostDepended) == 0 &&
		len(idx.Dependencies.Isolated) == 0 {
		return
	}
	sb.WriteString("## Dependencies\n\n")

	if len(idx.Dependencies.MostDepended) > 0 {
		sb.WriteString("Most depended on: ")
		md := make([]string, 0, len(idx.Dependencies.MostDepended))
		md = append(md, idx.Dependencies.MostDepended...)
		sort.Strings(md)
		for i, s := range md {
			md[i] = escapePipe(s)
		}
		sb.WriteString("`" + strings.Join(md, "`, `") + "`")
		sb.WriteString("\n\n")
	}

	if len(idx.Dependencies.Isolated) > 0 {
		sb.WriteString("Isolated: ")
		iso := make([]string, 0, len(idx.Dependencies.Isolated))
		iso = append(iso, idx.Dependencies.Isolated...)
		sort.Strings(iso)
		for i, s := range iso {
			iso[i] = escapePipe(s)
		}
		sb.WriteString("`" + strings.Join(iso, "`, `") + "`")
		sb.WriteString("\n\n")
	}

	if len(idx.Dependencies.Edges) > 0 {
		sb.WriteString("Edges:\n\n")
		edges := append([][2]string(nil), idx.Dependencies.Edges...)
		sort.Slice(edges, func(i, j int) bool {
			if edges[i][0] != edges[j][0] {
				return edges[i][0] < edges[j][0]
			}
			return edges[i][1] < edges[j][1]
		})
		for _, e := range edges {
			fmt.Fprintf(sb, "- `%s` → `%s`\n", escapePipe(e[0]), escapePipe(e[1]))
		}
		sb.WriteString("\n")
	}
}

func writeHotFiles(sb *strings.Builder, idx *schema.Index) {
	if len(idx.Git.HotFiles) == 0 {
		return
	}
	sb.WriteString("## Hot files (last 90 days)\n\n")
	sb.WriteString("| File | Commits |\n")
	sb.WriteString("|------|---------|\n")
	for _, h := range idx.Git.HotFiles {
		fmt.Fprintf(sb, "| `%s` | %d |\n", escapePipe(h.Path), h.Commits90d)
	}
	sb.WriteString("\n")
}

func writeHints(sb *strings.Builder, idx *schema.Index) {
	h := idx.Hints
	if h.AddFeature == "" && h.TestCmd == "" && len(h.EnvVars) == 0 && len(h.DoNotTouch) == 0 {
		return
	}
	sb.WriteString("## Hints\n\n")
	if h.AddFeature != "" {
		fmt.Fprintf(sb, "- Add a feature: %s\n", h.AddFeature)
	}
	if h.TestCmd != "" {
		fmt.Fprintf(sb, "- Run tests: `%s`\n", h.TestCmd)
	}
	if len(h.EnvVars) > 0 {
		env := append([]string(nil), h.EnvVars...)
		sort.Strings(env)
		fmt.Fprintf(sb, "- Env vars: `%s`\n", strings.Join(env, "`, `"))
	}
	if len(h.DoNotTouch) > 0 {
		dnt := append([]string(nil), h.DoNotTouch...)
		sort.Strings(dnt)
		fmt.Fprintf(sb, "- Do not touch: `%s`\n", strings.Join(dnt, "`, `"))
	}
	sb.WriteString("\n")
}

func writeFooter(sb *strings.Builder, idx *schema.Index) {
	sb.WriteString("---\n")
	sb.WriteString("Generated by [stacklit](https://github.com/glincker/stacklit).")
	// Don't hard-code "stacklit.json" here: --source can point at any file.
	// The timestamp is what's load-bearing.
	if idx.GeneratedAt != "" {
		fmt.Fprintf(sb, " Generated at %s.", idx.GeneratedAt)
	}
	sb.WriteString("\n")
}

// sortedKeys returns the keys of m sorted lexicographically. Used to keep the
// rendered markdown deterministic regardless of Go's map iteration order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatThousands(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// escapePipe replaces "|" with "\|" so user-supplied strings can be embedded
// in markdown table cells without breaking the row.
func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}
