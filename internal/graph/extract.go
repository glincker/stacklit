package graph

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Def is one symbol definition found in a file.
type Def struct {
	Name    string
	Qual    string // Owner.Name for members, else Name
	Kind    string
	Line    int
	End     int    // last line of the definition body
	Doc     string // first sentence of the doc comment
	Sig     string
	Recv    string   // owning type for methods
	Methods []string // interface method names (Go structural matching)
}

// Import is one import statement, kept as written; the resolver maps it to files.
type Import struct {
	Spec  string
	Alias string   // local name that qualifies uses, may be empty
	Names []string // directly imported names (JS named, Python from-import, Rust use)
}

// Ref is a use of a name inside a definition (From >= 0) or at file scope (From = -1).
type Ref struct {
	From int
	Kind string // calls, inherits, implements
	Name string
	Qual string // receiver or namespace chain, "this" for own-type access
}

// FileSyms is the extraction result for a single file.
type FileSyms struct {
	Path    string
	Lang    string
	Pkg     string // Go package directory or Java package, for same-package lookup
	Defs    []Def
	Imports []Import
	Refs    []Ref
}

// ExtractOptions tunes extraction.
type ExtractOptions struct {
	IncludeTests bool
	MaxFileBytes int
}

var langByExt = map[string]string{
	".go": "go", ".ts": "typescript", ".tsx": "typescript", ".mts": "typescript", ".cts": "typescript",
	".js": "javascript", ".jsx": "javascript", ".mjs": "javascript", ".cjs": "javascript",
	".py": "python", ".rs": "rust", ".java": "java",
}

// LangOf returns the graph language for a path, or "" when unsupported.
func LangOf(path string) string { return langByExt[strings.ToLower(filepath.Ext(path))] }

func isTestFile(path string) bool {
	b := filepath.Base(path)
	switch {
	case strings.HasSuffix(b, "_test.go"):
		return true
	case strings.Contains(b, ".test.") || strings.Contains(b, ".spec."):
		return true
	case strings.HasPrefix(b, "test_") && strings.HasSuffix(b, ".py"), strings.HasSuffix(b, "_test.py"):
		return true
	case strings.HasSuffix(b, "Test.java") || strings.HasSuffix(b, "Tests.java"):
		return true
	}
	return false
}

// ExtractFile parses one file's content into symbols and references.
func ExtractFile(path string, src []byte) *FileSyms {
	lang := LangOf(path)
	switch lang {
	case "go":
		return extractGo(path, src)
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

// Extract reads and parses the given root-relative files in parallel.
// The result is sorted by path so the graph is deterministic.
func Extract(root string, files []string, opts ExtractOptions) []*FileSyms {
	if opts.MaxFileBytes == 0 {
		opts.MaxFileBytes = 1 << 20
	}
	var todo []string
	for _, f := range files {
		if LangOf(f) == "" || (!opts.IncludeTests && isTestFile(f)) {
			continue
		}
		todo = append(todo, f)
	}
	out := make([]*FileSyms, len(todo))
	var wg sync.WaitGroup
	ch := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ch {
				out[i] = ExtractPath(root, todo[i], opts.MaxFileBytes)
			}
		}()
	}
	for i := range todo {
		ch <- i
	}
	close(ch)
	wg.Wait()
	res := out[:0]
	for _, fs := range out {
		if fs != nil {
			res = append(res, fs)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Path < res[j].Path })
	return res
}

// ExtractPath reads one root-relative file and extracts it; nil on any read problem.
func ExtractPath(root, rel string, maxBytes int) *FileSyms {
	st, err := os.Stat(filepath.Join(root, rel))
	if err != nil || st.IsDir() || (maxBytes > 0 && st.Size() > int64(maxBytes)) {
		return nil
	}
	src, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil
	}
	return ExtractFile(filepath.ToSlash(rel), src)
}

// stripNoise blanks comments and string contents with spaces, keeping newlines
// and quote characters so line numbers and brace counts stay exact.
func stripNoise(src []byte, lang string) []byte {
	out := make([]byte, len(src))
	copy(out, src)
	hashComments := lang == "python"
	blank := func(i int) {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}
	n := len(src)
	for i := 0; i < n; i++ {
		c := src[i]
		switch {
		case !hashComments && c == '/' && i+1 < n && src[i+1] == '/':
			for ; i < n && src[i] != '\n'; i++ {
				blank(i)
			}
			i--
		case !hashComments && c == '/' && i+1 < n && src[i+1] == '*':
			blank(i)
			blank(i + 1)
			i += 2
			for ; i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/'); i++ {
				blank(i)
			}
			if i < n {
				blank(i)
				blank(i + 1)
				i++
			}
		case hashComments && c == '#':
			for ; i < n && src[i] != '\n'; i++ {
				blank(i)
			}
			i--
		case hashComments && (c == '"' || c == '\'') && i+2 < n && src[i+1] == c && src[i+2] == c:
			i += 3
			for ; i < n && !(src[i] == c && i+2 < n && src[i+1] == c && src[i+2] == c); i++ {
				blank(i)
			}
			i += 2
		case c == '"' || c == '\'' || (c == '`' && lang != "python" && lang != "java"):
			if lang == "rust" && c == '\'' && !(i+2 < n && (src[i+2] == '\'' || (src[i+1] == '\\' && i+3 < n && src[i+3] == '\''))) {
				continue // lifetime, not a char literal
			}
			q := c
			for i++; i < n; i++ {
				if src[i] == '\\' && i+1 < n {
					blank(i)
					i++
					blank(i)
					continue
				}
				if src[i] == q {
					break
				}
				if src[i] == '\n' && q != '`' {
					break
				}
				blank(i)
			}
		}
	}
	return out
}

func splitLines(b []byte) []string { return strings.Split(string(b), "\n") }

// braceDepths returns the brace depth at the start of each line.
func braceDepths(lines []string) []int {
	d := make([]int, len(lines)+1)
	cur := 0
	for i, l := range lines {
		d[i] = cur
		cur += strings.Count(l, "{") - strings.Count(l, "}")
		if cur < 0 {
			cur = 0
		}
	}
	d[len(lines)] = cur
	return d
}

// blockEnd returns the last line of the brace block opened at or just after
// start, or start when no block opens within a few lines (expression bodies, abstract members).
func blockEnd(lines []string, start int) int {
	depth := 0
	opened := false
	for i := start; i < len(lines); i++ {
		l := lines[i]
		if !opened && i > start+6 {
			return start
		}
		if !opened && i > start && strings.Contains(l, ";") && !strings.Contains(l, "{") {
			return start
		}
		for j := 0; j < len(l); j++ {
			switch l[j] {
			case '{':
				depth++
				opened = true
			case '}':
				depth--
				if opened && depth <= 0 {
					return i
				}
			}
		}
		if !opened && i == start && strings.HasSuffix(strings.TrimSpace(l), ";") {
			return start
		}
	}
	if opened {
		return len(lines) - 1
	}
	return start
}

func sigOf(line string) string {
	s := strings.Join(strings.Fields(line), " ")
	s = strings.TrimSuffix(strings.TrimSpace(s), "{")
	s = strings.TrimSpace(s)
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// refScanner finds call sites inside a text line. group 1 is the qualifier chain
// (with trailing separator), group 2 the called name.
type refScanner struct {
	re       *regexp.Regexp
	keywords map[string]bool
	builtins map[string]bool
}

func wordSet(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

// scan appends calls found in text to refs. skipPrefixes are keywords that make a
// following name a declaration, not a call (e.g. "function").
func (s *refScanner) scan(text string, from int, seen map[string]bool, refs *[]Ref) {
	for _, m := range s.re.FindAllStringSubmatchIndex(text, -1) {
		name := text[m[4]:m[5]]
		if s.keywords[name] {
			continue
		}
		qual := ""
		if m[2] >= 0 {
			qual = text[m[2]:m[3]]
		}
		if qual == "" {
			pre := strings.TrimSpace(text[:m[4]])
			if strings.HasSuffix(pre, "function") || strings.HasSuffix(pre, "fn") || strings.HasSuffix(pre, "def") || strings.HasSuffix(pre, "class") || strings.HasSuffix(pre, "func") {
				continue
			}
			if s.builtins[name] {
				continue
			}
		}
		qual = cleanQual(qual)
		key := qual + "\x00" + name
		if seen[key] {
			continue
		}
		seen[key] = true
		*refs = append(*refs, Ref{From: from, Kind: EdgeCalls, Name: name, Qual: qual})
	}
}

// cleanQual normalises a chain like "a::b." or "a?.b." to "a.b".
func cleanQual(q string) string {
	if q == "" {
		return ""
	}
	q = strings.ReplaceAll(q, "::", ".")
	q = strings.ReplaceAll(q, "?.", ".")
	q = strings.ReplaceAll(q, " ", "")
	return strings.Trim(q, ".")
}

// ownerMap maps each line to the innermost callable def covering it (-1 = file scope).
func ownerMap(nLines int, spans []span) []int {
	own := make([]int, nLines)
	for i := range own {
		own[i] = -1
	}
	for _, s := range spans {
		for l := s.start; l <= s.end && l < nLines; l++ {
			own[l] = s.def
		}
	}
	return own
}

type span struct{ start, end, def int }

// scanBodies runs the call scanner over every line, attributing hits to owners.
// Declaration lines of a def are scanned only after their opening brace.
func scanBodies(lines []string, own []int, declLine map[int]bool, sc *refScanner) []Ref {
	var refs []Ref
	seen := map[int]map[string]bool{}
	for i, l := range lines {
		text := l
		if declLine[i] {
			idx := strings.IndexAny(text, "{")
			if k := strings.Index(text, "=>"); k >= 0 && (idx < 0 || k < idx) {
				idx = k + 1
			}
			if idx < 0 {
				continue
			}
			text = text[idx+1:]
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		o := own[i]
		if seen[o] == nil {
			seen[o] = map[string]bool{}
		}
		sc.scan(text, o, seen[o], &refs)
	}
	return refs
}

// fillDefs sets End and Doc for definitions whose extractor did not, using the
// stripped lines for structure and the original lines for comment text.
func fillDefs(fs *FileSyms, orig, lines []string) {
	for i := range fs.Defs {
		d := &fs.Defs[i]
		at := d.Line - 1
		if at < 0 || at >= len(lines) {
			continue
		}
		if d.End == 0 {
			if fs.Lang == "python" {
				d.End = pyBlockEnd(lines, at) + 1
			} else {
				d.End = blockEnd(lines, at) + 1
			}
		}
		if d.Doc == "" {
			d.Doc = docAbove(orig, at)
			if d.Doc == "" && fs.Lang == "python" {
				d.Doc = pyDocBelow(orig, at)
			}
		}
	}
}

func pyBlockEnd(lines []string, i int) int {
	indent := indentOf(lines[i])
	h, bal := i, 0
	for j := i; j < len(lines) && j < i+60; j++ {
		bal += strings.Count(lines[j], "(") + strings.Count(lines[j], "[") - strings.Count(lines[j], ")") - strings.Count(lines[j], "]")
		h = j
		if bal <= 0 {
			break
		}
	}
	end := h
	for j := h + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "" {
			continue
		}
		if indentOf(lines[j]) <= indent {
			break
		}
		end = j
	}
	return end
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ". "); i > 0 {
		s = s[:i+1]
	}
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

// docAbove returns the first line of the comment block directly above line at,
// skipping decorators and annotations.
func docAbove(orig []string, at int) string {
	j := at - 1
	for j >= 0 && (strings.HasPrefix(strings.TrimSpace(orig[j]), "@") || strings.HasPrefix(strings.TrimSpace(orig[j]), "#[")) {
		j--
	}
	var block []string
	for ; j >= 0; j-- {
		t := strings.TrimSpace(orig[j])
		switch {
		case strings.HasPrefix(t, "///"), strings.HasPrefix(t, "//"), strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "#["):
			block = append(block, strings.TrimLeft(t, "/# "))
			continue
		case strings.HasSuffix(t, "*/"), strings.HasPrefix(t, "*"), strings.HasPrefix(t, "/**"):
			block = append(block, strings.Trim(strings.TrimSuffix(strings.TrimLeft(t, "/* "), "*/"), " *"))
			if strings.HasPrefix(t, "/*") {
				j = -1
			}
			continue
		}
		break
	}
	for k := len(block) - 1; k >= 0; k-- {
		if block[k] != "" && !strings.HasPrefix(block[k], "@") {
			return firstSentence(block[k])
		}
	}
	return ""
}

func pyDocBelow(orig []string, at int) string {
	for j := at + 1; j < len(orig) && j < at+10; j++ {
		t := strings.TrimSpace(orig[j])
		if t == "" || strings.HasSuffix(t, ":") || strings.HasPrefix(t, ")") {
			continue
		}
		for _, q := range []string{`"""`, "'''", `"`, "'"} {
			if strings.HasPrefix(t, q) {
				return firstSentence(strings.Trim(strings.TrimPrefix(t, q), "\"' "))
			}
		}
		return ""
	}
	return ""
}
