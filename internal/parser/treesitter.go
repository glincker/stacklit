package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// TreeSitterParser uses tree-sitter ASTs to extract structured info from source files.
// Handles all languages that have tree-sitter grammars, except Go (which uses stdlib AST).
type TreeSitterParser struct{}

// tsLangMap maps stacklit language names to extraction functions.
var tsLangMap = map[string]func(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo{
	"typescript":  extractTypeScript,
	"tsx":         extractTypeScript,
	"javascript":  extractTypeScript,
	"jsx":         extractTypeScript,
	"python":      extractPython,
	"rust":        extractRust,
	"java":        extractJava,
	"c_sharp":     extractCSharp,
	"c-sharp":     extractCSharp,
	"ruby":        extractRuby,
	"php":         extractPHP,
	"kotlin":      extractKotlin,
	"swift":       extractSwift,
	"c":           extractC,
	"cpp":         extractC,
	"objc":        extractObjC,
	"elixir":      extractElixir,
	"zig":         extractZig,
	"dart":        extractDart,
	"lua":         extractLua,
	"scala":       extractScala,
	"bash":        extractShell,
	"sql":         extractSQL,
	"gdscript":    extractGDScript,
	"vue":         extractVue,
	"svelte":      extractSvelte,
	"hcl":         extractHCL,
	"proto":       extractProto,
	"haskell":     extractHaskell,
	"ocaml":       extractOCaml,
	"clojure":     extractClojure,
	"r":           extractR,
	"perl":        extractPerl,
	"julia":       extractJulia,
}

// skipExts are file extensions handled by other parsers or that we skip.
var skipExts = map[string]bool{
	".go": true, // GoParser handles these
}

func (p *TreeSitterParser) CanParse(filename string) bool {
	lower := strings.ToLower(filename)
	for ext := range skipExts {
		if strings.HasSuffix(lower, ext) {
			return false
		}
	}
	entry := detectEntry(filename)
	if entry == nil {
		return false
	}
	_, ok := tsLangMap[entry.Name]
	return ok
}

// detectEntry resolves the grammar for a path. It overrides the registry where
// an extension is ambiguous (.fs is F# far more often than Forth) and adds the
// extensionless build files the registry only matches by exact name.
func detectEntry(path string) *grammars.LangEntry {
	lower := strings.ToLower(path)
	base := lower
	if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
		base = base[i+1:]
	}
	switch {
	case base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") || strings.HasSuffix(base, ".dockerfile"):
		return grammars.DetectLanguageByName("dockerfile")
	case base == "makefile" || base == "gnumakefile" || strings.HasSuffix(base, ".mk"):
		return grammars.DetectLanguageByName("make")
	case strings.HasSuffix(base, ".fs"):
		return grammars.DetectLanguageByName("fsharp")
	case strings.HasSuffix(base, ".m") || strings.HasSuffix(base, ".mm"):
		return grammars.DetectLanguageByName("objc")
	}
	return grammars.DetectLanguage(path)
}

func (p *TreeSitterParser) Parse(path string, content []byte) (*FileInfo, error) {
	entry := detectEntry(path)
	if entry == nil {
		return nil, nil
	}

	lang := entry.Language()
	parser := gts.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		// Fallback to generic on parse failure.
		g := &GenericParser{}
		return g.Parse(path, content)
	}

	root := tree.RootNode()
	if root == nil {
		g := &GenericParser{}
		return g.Parse(path, content)
	}

	extractFn, ok := tsLangMap[entry.Name]
	if !ok {
		g := &GenericParser{}
		return g.Parse(path, content)
	}

	info := extractFn(root, lang, content, path)
	info.Path = path
	info.LineCount = countLines(content)
	return info, nil
}

// nodeText returns the source text for a node.
func nodeText(node *gts.Node, src []byte) string {
	if node == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if int(end) > len(src) {
		end = uint32(len(src))
	}
	return string(src[start:end])
}

// truncSig truncates a signature to maxLen chars.
func truncSig(s string, maxLen int) string {
	// Remove newlines for single-line display.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// childByType finds the first direct child with the given type.
func childByType(node *gts.Node, lang *gts.Language, kind string) *gts.Node {
	for i := 0; i < node.ChildCount(); i++ {
		c := node.Child(i)
		if c.Type(lang) == kind {
			return c
		}
	}
	return nil
}

// childrenByType returns all direct children with the given type.
func childrenByType(node *gts.Node, lang *gts.Language, kind string) []*gts.Node {
	var result []*gts.Node
	for i := 0; i < node.ChildCount(); i++ {
		c := node.Child(i)
		if c.Type(lang) == kind {
			result = append(result, c)
		}
	}
	return result
}

// dedup returns unique strings preserving order.
func dedup(items []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, item := range items {
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
