package graph

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Parse cost model, matching the main scanner: the GLR parser holds roughly
// 6.5 KB per source byte plus a fixed floor while a tree is alive.
const (
	parseFloorBytes   = 24 << 20
	parseBytesPerByte = 6500
	// defaultMaxParseBytes bounds one tree-sitter parse when nothing is configured.
	defaultMaxParseBytes = 192 << 10
	// defaultParseMemoryMB bounds concurrent parses when nothing is configured.
	defaultParseMemoryMB = 512
)

func maxParseBytes(o ExtractOptions) int {
	if o.MaxParseBytes > 0 {
		return o.MaxParseBytes
	}
	if v, err := strconv.Atoi(os.Getenv("STACKLIT_GRAPH_MAX_PARSE_BYTES")); err == nil && v > 0 {
		return v
	}
	return defaultMaxParseBytes
}

func parseBudget(o ExtractOptions) int64 {
	if o.MemoryBudget != 0 {
		return o.MemoryBudget
	}
	if v := os.Getenv("STACKLIT_PARSE_MEMORY_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			if n == 0 {
				return -1
			}
			return int64(n) << 20
		}
	}
	return defaultParseMemoryMB << 20
}

// OptionsFromEnv returns extraction options read from the environment.
func OptionsFromEnv() ExtractOptions {
	o := ExtractOptions{}
	if v := os.Getenv("STACKLIT_GRAPH_INCLUDE_TESTS"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			o.IncludeTests = b
		}
	}
	return o
}

// parseGate is a weighted semaphore over the estimated memory of live parses.
// A request larger than the whole budget waits for an empty gate, then runs alone.
type parseGate struct {
	mu     sync.Mutex
	cond   *sync.Cond
	budget int64
	used   int64
}

func newParseGate(budget int64) *parseGate {
	if budget <= 0 {
		return nil
	}
	g := &parseGate{budget: budget}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *parseGate) acquire(n int64) {
	if g == nil {
		return
	}
	g.mu.Lock()
	for g.used > 0 && g.used+n > g.budget {
		g.cond.Wait()
	}
	g.used += n
	g.mu.Unlock()
}

func (g *parseGate) release(n int64) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.used -= n
	g.mu.Unlock()
	g.cond.Broadcast()
}

var tsSpecs = map[string]*tsSpec{}
var tsSpecOnce sync.Once

func initTSSpecs() {
	for _, s := range []*tsSpec{
		pySpec, newTSSpec("typescript"), newTSSpec("javascript"), rsSpec, javaSpec,
	} {
		tsSpecs[s.lang] = s
	}
	for _, s := range moreSpecs() {
		tsSpecs[s.lang] = s
	}
}

// grammarFor picks the tree-sitter grammar for a file.
func grammarFor(lang, path string, src []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch lang {
	case "typescript":
		if ext == ".tsx" {
			return "tsx"
		}
	case "csharp":
		return "c_sharp"
	case "c":
		if ext == ".h" && (bytes.Contains(src, []byte("class ")) || bytes.Contains(src, []byte("namespace ")) || bytes.Contains(src, []byte("template<")) || bytes.Contains(src, []byte("template <"))) {
			return "cpp"
		}
	}
	return lang
}

// extractFileWith dispatches on language. Tree-sitter handles everything except
// Go; the line scanners stay as the fallback for oversize or unparsable files.
func extractFileWith(path string, src []byte, o ExtractOptions, gate *parseGate) *FileSyms {
	lang := LangOf(path)
	if lang == "" {
		return nil
	}
	if lang == "go" {
		return extractGo(path, src)
	}
	tsSpecOnce.Do(initTSSpecs)
	spec := tsSpecs[lang]
	var fs *FileSyms
	if spec != nil && len(src) <= maxParseBytes(o) {
		cost := int64(parseFloorBytes) + int64(len(src))*parseBytesPerByte
		gate.acquire(cost)
		fs = extractTS(spec, grammarFor(lang, path, src), path, src)
		gate.release(cost)
	}
	legacy := func() *FileSyms {
		switch lang {
		case "typescript", "javascript":
			return extractJS(path, lang, src)
		case "python":
			return extractPython(path, src)
		case "rust":
			return extractRust(path, src)
		case "java":
			return extractJava(path, src)
		}
		return nil
	}
	if fs != nil {
		// The Python grammar runtime can drop definitions without flagging an
		// error, so cross-check it (and any tree with error nodes) against the
		// line scanner and keep whichever found more definitions.
		if lang == "python" || fs.syntaxErrors || fs.unreliable {
			if old := legacy(); old != nil && (fs.unreliable || len(old.Defs) > len(fs.Defs)) {
				return old
			}
		}
		return fs
	}
	if old := legacy(); old != nil {
		return old
	}
	return &FileSyms{Path: path, Lang: lang}
}
