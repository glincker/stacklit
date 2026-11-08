package parser

import (
	"strings"
	"sync"
	"time"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// TreeSitterParser uses tree-sitter ASTs to extract structured info from source files.
// Handles all languages that have tree-sitter grammars, except Go (which uses stdlib AST).
type TreeSitterParser struct{}

// tsLangMap maps stacklit language names to extraction functions.
var tsLangMap = map[string]func(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo{
	"typescript": extractTypeScript,
	"tsx":        extractTypeScript,
	"javascript": extractTypeScript,
	"jsx":        extractTypeScript,
	"python":     extractPython,
	"rust":       extractRust,
	"java":       extractJava,
	"c_sharp":    extractCSharp,
	"c-sharp":    extractCSharp,
	"ruby":       extractRuby,
	"php":        extractPHP,
	"kotlin":     extractKotlin,
	"swift":      extractSwift,
	"c":          extractC,
	"cpp":        extractC,
	"objc":       extractObjC,
	"elixir":     extractElixir,
	"zig":        extractZig,
	"dart":       extractDart,
	"lua":        extractLua,
	"scala":      extractScala,
	"bash":       extractShell,
	"sql":        extractSQL,
	"gdscript":   extractGDScript,
	"vue":        extractVue,
	"svelte":     extractSvelte,
	"hcl":        extractHCL,
	"proto":      extractProto,
	"haskell":    extractHaskell,
	"ocaml":      extractOCaml,
	"clojure":    extractClojure,
	"r":          extractR,
	"perl":       extractPerl,
	"julia":      extractJulia,
	"nim":        extractNim,
	"erlang":     extractErlang,
	"groovy":     extractGroovy,
	"fsharp":     extractFSharp,
	"fortran":    extractFortran,
	"elm":        extractElm,
	"solidity":   extractSolidity,
	"cmake":      extractCMake,
	"make":       extractMake,
	"dockerfile": extractDockerfile,
	"json":       extractJSON,
	"yaml":       extractYAML,
	"toml":       extractTOML,
	"graphql":    extractGraphQL,
	"powershell": extractPowerShell,
	"gleam":      extractGleam,
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

// jsonFastPath is a var only so tests can compare against the tree-sitter path.
var jsonFastPath = true

// parserPools reuses one parser per grammar. A fresh parser re-allocates its
// GLR scratch buffers (tens of MB) and forgets its arena sizing hints.
var parserPools sync.Map // grammar name -> *sync.Pool

func acquireParser(name string, lang *gts.Language) *gts.Parser {
	v, _ := parserPools.LoadOrStore(name, &sync.Pool{})
	if p, ok := v.(*sync.Pool).Get().(*gts.Parser); ok {
		return p
	}
	return gts.NewParser(lang)
}

func releaseParser(name string, p *gts.Parser) {
	if v, ok := parserPools.Load(name); ok {
		v.(*sync.Pool).Put(p)
	}
}

func (p *TreeSitterParser) Parse(path string, content []byte) (*FileInfo, error) {
	return p.ParseWithTimeout(path, content, 0)
}

// ParseWithTimeout is Parse with a wall-clock budget. When the budget runs
// out the partial tree is discarded and the file gets the generic parser's
// result, marked with FallbackTimeout.
func (p *TreeSitterParser) ParseWithTimeout(path string, content []byte, timeout time.Duration) (*FileInfo, error) {
	entry := detectEntry(path)
	if entry == nil {
		return nil, nil
	}

	if entry.Name == "bash" {
		return scanShell(path, content), nil
	}
	if jsonFastPath && entry.Name == "json" && len(content) <= configMaxBytes() {
		if keys, ok := jsonTopKeys(content); ok {
			return &FileInfo{Path: path, Language: "json", Exports: keys, LineCount: countLines(content)}, nil
		}
	}

	lang := entry.Language()
	parser := acquireParser(entry.Name, lang)
	parser.SetTimeoutMicros(uint64(timeout / time.Microsecond))
	defer func() {
		parser.SetTimeoutMicros(0)
		releaseParser(entry.Name, parser)
	}()
	parseSrc := content
	switch entry.Name {
	case "sql":
		parseSrc = maskDollarQuotes(content)
	case "ruby":
		parseSrc = maskRubyDataRuns(content)
	}
	tree, err := parser.Parse(parseSrc)
	if err != nil {
		// Fallback to generic on parse failure.
		g := &GenericParser{}
		return g.Parse(path, content)
	}

	if timeout > 0 && tree.ParseStopReason() == gts.ParseStopTimeout {
		tree.Release()
		info, err := (&GenericParser{}).Parse(path, content)
		if info != nil {
			info.Fallback = FallbackTimeout
		}
		return info, err
	}

	// Release returns the node arena to the library pool; without it every
	// file allocates a fresh multi-MB arena and the heap balloons.
	defer tree.Release()

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

	info := extractFn(root, lang, parseSrc, path)
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
