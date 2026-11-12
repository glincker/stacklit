package graph

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glincker/stacklit/internal/parser"
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
	// defaultParseTimeout stops one tree-sitter parse that runs away.
	defaultParseTimeout = 3 * time.Second
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

func parseTimeout(o ExtractOptions) time.Duration {
	if o.ParseTimeout > 0 {
		return o.ParseTimeout
	}
	if v, err := strconv.Atoi(os.Getenv("STACKLIT_GRAPH_PARSE_TIMEOUT_MS")); err == nil && v > 0 {
		return time.Duration(v) * time.Millisecond
	}
	return defaultParseTimeout
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
	}
	return lang
}

// extractFileWith dispatches on language when only the source is available.
// The neighbour check that can mark a header as C++ is skipped.
func extractFileWith(path string, src []byte, o ExtractOptions, gate *parseGate) *FileSyms {
	return extractFileAt(path, "", src, o, gate)
}

// extractFileAt is extractFileWith with the header's on-disk path, so a .h
// header can be classified from the translation units next to it.
func extractFileAt(path, diskPath string, src []byte, o ExtractOptions, gate *parseGate) *FileSyms {
	lang := LangOf(path)
	if lang == "" {
		return nil
	}
	// A .h header is C++ when its own text or its neighbours say so.
	if lang == "c" && strings.EqualFold(filepath.Ext(path), ".h") {
		lang = parser.HeaderLanguage(diskPath, src)
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
		fs = extractGuarded(spec, grammarFor(lang, path, src), path, src, parseTimeout(o))
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

// extractGuarded runs extractTS with a hard deadline. The parser checks its own
// timeout only between steps, and a few inputs stall inside grammar-specific
// recovery, so the caller stops waiting after a grace period and falls back.
func extractGuarded(spec *tsSpec, grammar, path string, src []byte, timeout time.Duration) *FileSyms {
	done := make(chan *FileSyms, 1)
	go func() { done <- extractTS(spec, grammar, path, src, timeout) }()
	select {
	case fs := <-done:
		return fs
	case <-time.After(timeout + timeout/2):
		return nil
	}
}
