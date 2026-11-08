package setup

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/glincker/stacklit/internal/schema"
)

// Tokens approximates token count as bytes/4, the documented estimate used
// everywhere stacklit reports overhead.
func Tokens(s string) int { return (len(s) + 3) / 4 }

// digestBudget is the token cap for the injected block. Override with STACKLIT_DIGEST_TOKENS.
func digestBudget() int {
	if v, err := strconv.Atoi(os.Getenv("STACKLIT_DIGEST_TOKENS")); err == nil && v >= 100 {
		return v
	}
	return 500
}

const digestHeader = "## Codebase map (stacklit)\nCheck `stacklit.json` or the stacklit MCP tools before grep/glob. Refresh: `stacklit generate`.\n\n"

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = strings.TrimSpace(s[:max-1]) + "~"
	}
	return s
}

// MarkdownBody builds the always-on orientation digest. It is deterministic
// (sorted, no timestamps, no churn counters) so reruns are byte-identical and
// do not invalidate prompt caches, and it is capped to the token budget.
func MarkdownBody(idx *schema.Index) string {
	var head strings.Builder
	head.WriteString(digestHeader)
	head.WriteString("```\n")
	line1 := fmt.Sprintf("%s | %s | %d modules", idx.Project.Name, idx.Tech.PrimaryLanguage, len(idx.Modules))
	if len(idx.Tech.Frameworks) > 0 {
		line1 += " | " + oneLine(strings.Join(idx.Tech.Frameworks, ", "), 60)
	}
	head.WriteString(line1 + "\n")
	var meta []string
	if len(idx.Structure.Entrypoints) > 0 {
		meta = append(meta, "entry: "+idx.Structure.Entrypoints[0])
	}
	if idx.Hints.TestCmd != "" {
		meta = append(meta, "test: "+oneLine(idx.Hints.TestCmd, 60))
	}
	if len(meta) > 0 {
		head.WriteString(strings.Join(meta, " | ") + "\n")
	}
	if idx.Hints.AddFeature != "" {
		head.WriteString("add: " + oneLine(idx.Hints.AddFeature, 120) + "\n")
	}
	if len(idx.Hints.DoNotTouch) > 0 {
		head.WriteString("do-not-touch: " + oneLine(strings.Join(idx.Hints.DoNotTouch, ", "), 100) + "\n")
	}
	head.WriteString("modules:\n")

	// Rank by blast radius, then size, then name, so the cut is stable.
	rank := map[string]int{}
	for i, m := range idx.Dependencies.MostDepended {
		rank[m] = i + 1
	}
	names := make([]string, 0, len(idx.Modules))
	for n := range idx.Modules {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := names[i], names[j]
		ra, rb := rank[a], rank[b]
		if (ra > 0) != (rb > 0) {
			return ra > 0
		}
		if ra != rb {
			return ra < rb
		}
		if la, lb := idx.Modules[a].Lines, idx.Modules[b].Lines; la != lb {
			return la > lb
		}
		return a < b
	})

	const tail = "```\n"
	budget := digestBudget()*4 - len(head.String()) - len(tail) - 40
	var kept []string
	for _, n := range names {
		m := idx.Modules[n]
		l := "  " + n + "  " + oneLine(strings.ToLower(m.Purpose), 48)
		if len(m.DependsOn) > 0 && len(m.DependsOn) <= 3 {
			l += " > " + strings.Join(baseNames(m.DependsOn), ",")
		}
		l += "\n"
		if budget-len(l) < 0 {
			continue
		}
		budget -= len(l)
		kept = append(kept, n+"\x00"+l)
	}
	// Display alphabetically so the layout is predictable.
	sort.Strings(kept)
	var b strings.Builder
	b.WriteString(head.String())
	for _, k := range kept {
		b.WriteString(k[strings.IndexByte(k, 0)+1:])
	}
	if more := len(names) - len(kept); more > 0 {
		fmt.Fprintf(&b, "  (+%d more in stacklit.json)\n", more)
	}
	b.WriteString(tail)
	return b.String()
}

func baseNames(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = s[strings.LastIndex(s, "/")+1:]
	}
	return out
}
