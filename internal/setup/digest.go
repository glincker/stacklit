package setup

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/session"
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

// steering holds the rule text variants, selectable with the "steering" config
// key or STACKLIT_STEERING so phrasings can be A/B tested. Each stays byte
// stable and well inside the digest cap.
var steering = map[string]string{
	"a": "Before grepping the repo, run `stacklit query \"<question>\" --budget 800`.\n" +
		"Read code with `stacklit slice <symbol>`, not whole files.\n" +
		"Before editing shared code, run `stacklit impact <symbol>`.\n",
	"b": "Stacklit first: `stacklit query \"<question>\"` before grep, `stacklit slice <symbol>` before reading a file, `stacklit impact <symbol>` before editing shared code.\n",
	"c": "Stacklit answers structure questions in far fewer tokens than grep plus whole-file reads.\n" +
		"- Where is X or how does Y work: `stacklit query \"<question>\" --budget 800`\n" +
		"- Need one function or type: `stacklit slice <symbol>`\n" +
		"- Changing shared code: `stacklit impact <symbol>` lists what depends on it\n",
	"d": strictRules,
	"e": routedRules,
}

// DefaultSteering is the variant used when none is configured.
const DefaultSteering = "a"

// SteeringVariants lists the valid variant names.
func SteeringVariants() []string { return []string{"a", "b", "c", "d", "e"} }

func steeringText(variant string) string {
	if t, ok := steering[strings.ToLower(strings.TrimSpace(variant))]; ok {
		return t
	}
	return steering[DefaultSteering]
}

// SteeringFor resolves the variant: STACKLIT_STEERING wins, then the config key.
func SteeringFor(root string) string {
	if v := os.Getenv("STACKLIT_STEERING"); v != "" {
		return v
	}
	return config.Load(root).Steering
}

// SourceFiles counts the indexed files across all languages.
func SourceFiles(idx *schema.Index) int {
	n := 0
	for _, l := range idx.Tech.Languages {
		n += l.Files
	}
	return n
}

// IsSmallRepo reports whether the repo is below the small repo threshold, so a
// full digest would cost more than the exploration it saves. An empty index or
// a threshold of 0 never counts as small.
func IsSmallRepo(root string, idx *schema.Index) bool {
	limit := config.Load(root).SmallRepoLimit()
	n := SourceFiles(idx)
	return limit > 0 && n > 0 && n < limit
}

// smallRepoLine is the only orientation a small repo gets. It is factual, not
// imperative, like the other hook text.
func smallRepoLine(idx *schema.Index) string {
	return fmt.Sprintf("This repo is small (%d source files), so grep and reading files are enough. Stacklit is available if a question needs more: stacklit query \"<question>\".", SourceFiles(idx))
}

// BodyForRoot is BodyFor for a repo on disk: a small repo gets the short block
// (the note plus the structure token, so the session hook still recognises it),
// every other repo gets the full digest.
func BodyForRoot(root string, idx *schema.Index, variant string) string {
	if !IsSmallRepo(root, idx) {
		return BodyFor(idx, variant)
	}
	return fmt.Sprintf("## Codebase map (stacklit)\n%s\n\n```\n%s | %s | %d modules | %s%s\n```\n",
		smallRepoLine(idx), idx.Project.Name, idx.Tech.PrimaryLanguage, len(idx.Modules), StructureTag, session.StructureHash(idx))
}

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
func MarkdownBody(idx *schema.Index) string { return BodyFor(idx, DefaultSteering) }

// StructureTag prefixes the hash in the digest so a reader (or the session
// hook) can tell which structure a CLAUDE.md block was written from.
const StructureTag = "map "

// BodyFor is MarkdownBody with a chosen steering variant. Nothing in it
// depends on line counts, timestamps or churn, only on structure.
func BodyFor(idx *schema.Index, variant string) string {
	var head strings.Builder
	head.WriteString("## Codebase map (stacklit)\n")
	head.WriteString(steeringText(variant))
	head.WriteString("\n```\n")
	line1 := fmt.Sprintf("%s | %s | %d modules | %s%s", idx.Project.Name, idx.Tech.PrimaryLanguage, len(idx.Modules), StructureTag, session.StructureHash(idx))
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

	// Rank by blast radius, then dependents, then name, so the cut is stable.
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
		if da, db := len(idx.Modules[a].DependedBy), len(idx.Modules[b].DependedBy); da != db {
			return da > db
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
