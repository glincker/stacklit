package graph

import (
	"strings"
	"sync"
	"time"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// tsSpec describes one language for the shared tree-sitter driver. Everything
// language specific lives in these tables and a few small hooks, so adding a
// language does not touch the walker.
type tsSpec struct {
	grammar string
	lang    string
	defs    map[string]defRule
	calls   map[string]callRule
	decls   map[string]declRule
	special map[string]func(w *tsw, n *gts.Node) bool
	// comments are node types holding doc comments; attrs are node types that
	// may sit between a doc comment and its definition (decorators, attributes).
	comments map[string]bool
	attrs    map[string]bool
	// wrappers are parents whose own position is the one a doc comment precedes.
	wrappers map[string]bool
	builtins map[string]bool
	skip     map[string]bool
	nameKids []string // child node types tried when a node has no "name" field
	vis      func(w *tsw, n *gts.Node, name string) string
	docFn    func(w *tsw, n *gts.Node) string
	prepare  func(w *tsw, root *gts.Node)
	finish   func(w *tsw)
	// mask rewrites source the grammar cannot read, keeping every byte offset.
	mask func(src []byte) []byte
	// qualAlias maps receiver keywords (parent, static, Self) to the names the resolver knows.
	qualAlias map[string]string
}

type defRule struct {
	kind      string
	callable  bool
	container bool
	// names returns the definition name and an owner override (empty = current scope).
	names    func(w *tsw, n *gts.Node) (string, string)
	heritage func(w *tsw, n *gts.Node, idx int)
	kindFn   func(w *tsw, n *gts.Node) string
	sect     string // default access section inside a container (C++ and Ruby style)
}

// callRule says where the callee of a call-like node lives.
type callRule struct {
	fn     string // field with the callee expression; "" = first named child
	obj    string // with name: receiver field
	name   string // with obj: method name field
	ctor   bool   // object creation: the callee names a type
	custom func(w *tsw, n *gts.Node) (qual, name string)
}

// declRule extracts a typed name (parameter, local, field) for receiver inference.
type declRule struct {
	name, typ, value string // field names; empty means not available
	typeFromParent   bool
	fn               func(w *tsw, n *gts.Node) (name, typ string)
}

type tsw struct {
	spec    *tsSpec
	lang    *gts.Language
	src     []byte
	fs      *FileSyms
	owner   int
	typ     string
	inFunc  bool
	sect    string
	locals  map[string]string
	fields  map[string]string
	seen    map[string]bool
	extra   map[string]bool
	pending []any
	cols    []uint32 // start column per def, for layout checks
	parents []int
}

var tsParsers sync.Map // grammar name -> *sync.Pool of *gts.Parser

func acquireTS(name string, lang *gts.Language) *gts.Parser {
	v, _ := tsParsers.LoadOrStore(name, &sync.Pool{})
	if p, ok := v.(*sync.Pool).Get().(*gts.Parser); ok {
		return p
	}
	return gts.NewParser(lang)
}

func releaseTS(name string, p *gts.Parser) {
	if v, ok := tsParsers.Load(name); ok {
		v.(*sync.Pool).Put(p)
	}
}

// extractTS parses src with the language's grammar and walks the tree once.
// It returns nil when the grammar is missing or parsing fails, so callers can fall back.
func extractTS(spec *tsSpec, grammar, file string, src []byte, timeout time.Duration) (fs *FileSyms) {
	defer func() {
		if r := recover(); r != nil {
			fs = nil
		}
	}()
	entry := grammars.DetectLanguageByName(grammar)
	if entry == nil {
		return nil
	}
	lang := entry.Language()
	p := acquireTS(grammar, lang)
	defer releaseTS(grammar, p)
	// Some inputs make the GLR parser blow up (heavy preprocessor branching); a
	// deadline keeps one file from stalling the whole build.
	p.SetTimeoutMicros(uint64(timeout / time.Microsecond))
	if spec.mask != nil {
		src = spec.mask(src)
	}
	tree, err := p.Parse(src)
	if err != nil || tree == nil {
		return nil
	}
	// Release hands the node arena back to the library; extraction copies every
	// string it keeps, so nothing outlives the tree.
	defer tree.Release()
	if tree.ParseStoppedEarly() {
		tree.Release()
		return nil
	}
	root := tree.RootNode()
	if root == nil {
		return nil
	}
	w := &tsw{
		spec: spec, lang: lang, src: src, owner: -1,
		fs:     &FileSyms{Path: file, Lang: spec.lang},
		locals: map[string]string{}, fields: map[string]string{},
		seen: map[string]bool{}, extra: map[string]bool{},
	}
	if spec.prepare != nil {
		spec.prepare(w, root)
	}
	w.fs.syntaxErrors = root.HasError()
	w.visit(root)
	if spec.finish != nil {
		spec.finish(w)
	}
	w.fs.Imports = dedupImports(w.fs.Imports)
	return w.fs
}

func (w *tsw) text(n *gts.Node) string {
	if n == nil {
		return ""
	}
	s, e := int(n.StartByte()), int(n.EndByte())
	if e > len(w.src) {
		e = len(w.src)
	}
	if s > e {
		return ""
	}
	return string(w.src[s:e])
}

func (w *tsw) typeOf(n *gts.Node) string { return n.Type(w.lang) }

func (w *tsw) field(n *gts.Node, name string) *gts.Node { return n.ChildByFieldName(name, w.lang) }

func (w *tsw) fieldText(n *gts.Node, name string) string { return w.text(w.field(n, name)) }

// kidOf returns the first direct child whose type is one of kinds.
func (w *tsw) kidOf(n *gts.Node, kinds ...string) *gts.Node {
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		t := c.Type(w.lang)
		for _, k := range kinds {
			if t == k {
				return c
			}
		}
	}
	return nil
}

// kidsOf returns all direct children whose type is one of kinds.
func (w *tsw) kidsOf(n *gts.Node, kinds ...string) []*gts.Node {
	var out []*gts.Node
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		t := c.Type(w.lang)
		for _, k := range kinds {
			if t == k {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func (w *tsw) visit(n *gts.Node) {
	if n == nil {
		return
	}
	t := n.Type(w.lang)
	if w.spec.skip[t] || w.spec.comments[t] {
		return
	}
	if f := w.spec.special[t]; f != nil && f(w, n) {
		return
	}
	if r, ok := w.spec.defs[t]; ok {
		name, recv := "", ""
		if r.names != nil {
			name, recv = r.names(w, n)
		} else {
			name = w.defaultName(n)
		}
		if name != "" {
			kind := r.kind
			if r.kindFn != nil {
				if k := r.kindFn(w, n); k != "" {
					kind = k
				} else {
					w.children(n)
					return
				}
			}
			w.define(n, defOpts{name: name, recv: recv, kind: kind, callable: r.callable, container: r.container, heritage: r.heritage, sect: r.sect})
			return
		}
	}
	if r, ok := w.spec.calls[t]; ok {
		w.call(n, r)
	}
	if r, ok := w.spec.decls[t]; ok {
		w.decl(n, r)
	}
	w.children(n)
}

func (w *tsw) children(n *gts.Node) {
	for i := 0; i < n.ChildCount(); i++ {
		w.visit(n.Child(i))
	}
}

func (w *tsw) defaultName(n *gts.Node) string {
	if nm := w.field(n, "name"); nm != nil {
		return w.text(nm)
	}
	for _, k := range w.spec.nameKids {
		if c := w.kidOf(n, k); c != nil {
			return w.text(c)
		}
	}
	return ""
}

type defOpts struct {
	name, recv, kind    string
	callable, container bool
	span                *gts.Node
	walk                []*gts.Node
	proto               bool // declaration without a body
	noWalk              bool // leave the body unvisited (prototypes, type bindings)
	heritage            func(w *tsw, n *gts.Node, idx int)
	sect                string
}

// define records a definition and walks its body with the right owner and scope.
func (w *tsw) define(n *gts.Node, o defOpts) int {
	span := n
	if o.span != nil {
		span = o.span
	}
	// Some grammars fold the doc comment into the declaration node; start at the code.
	lead, first := w.leadingComments(span)
	sp, ep := first.StartPoint(), span.EndPoint()
	end := int(ep.Row) + 1
	if ep.Column == 0 && ep.Row > sp.Row {
		end = int(ep.Row)
	}
	d := Def{Name: o.name, Qual: o.name, Kind: o.kind, Line: int(sp.Row) + 1, End: end, Sig: w.sig(span, first)}
	owner := w.typ
	if o.recv != "" {
		owner = o.recv
	}
	switch {
	case o.callable && !w.inFunc && owner != "" && (o.kind == KindFunc || o.kind == KindMethod):
		d.Kind, d.Recv, d.Qual = KindMethod, owner, owner+"."+o.name
	case isTypeKind(o.kind) && w.typ != "" && !w.inFunc:
		d.Qual = w.typ + "." + o.name
	}
	d.Doc = w.docFor(span, lead)
	d.Decl = o.proto
	if w.spec.vis != nil {
		d.Vis = w.spec.vis(w, span, o.name)
	}
	w.fs.Defs = append(w.fs.Defs, d)
	idx := len(w.fs.Defs) - 1
	w.cols, w.parents = append(w.cols, sp.Column), append(w.parents, w.owner)
	if o.heritage != nil {
		o.heritage(w, n, idx)
	}
	savedOwner, savedTyp, savedFunc, savedLocals, savedSect := w.owner, w.typ, w.inFunc, w.locals, w.sect
	switch {
	case o.container:
		w.owner, w.typ, w.inFunc = idx, o.name, false
		w.sect = o.sect
	case o.callable:
		w.owner, w.inFunc, w.locals = idx, true, map[string]string{}
		if d.Recv != "" {
			w.typ = d.Recv
		}
	}
	if o.noWalk {
	} else if len(o.walk) > 0 {
		for _, c := range o.walk {
			w.visit(c)
		}
	} else {
		w.children(n)
	}
	w.owner, w.typ, w.inFunc, w.locals, w.sect = savedOwner, savedTyp, savedFunc, savedLocals, savedSect
	return idx
}

// sig is the declaration text up to its body, on one line.
func (w *tsw) sig(n, first *gts.Node) string {
	start, end := int(first.StartByte()), int(n.EndByte())
	hasBody := false
	if b := w.field(n, "body"); b != nil && int(b.StartByte()) > start {
		end, hasBody = int(b.StartByte()), true
	}
	if end > len(w.src) {
		end = len(w.src)
	}
	if end-start > 400 {
		end = start + 400
	}
	s := string(w.src[start:end])
	if i := strings.IndexByte(s, '{'); i >= 0 {
		s = s[:i]
	}
	if !hasBody {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[:i]
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimRight(s, " {=>")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// leadingComments returns comment children at the start of n and the first real child
// (or n itself when there are none). Some grammars let a node start at the comment
// before it, so the first real child gives the true start of the code.
func (w *tsw) leadingComments(n *gts.Node) ([]*gts.Node, *gts.Node) {
	var lead []*gts.Node
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		if w.spec.comments[c.Type(w.lang)] || c.IsExtra() {
			lead = append(lead, c)
			continue
		}
		if c.StartByte() > n.StartByte() {
			return lead, c
		}
		break
	}
	return nil, n
}

// docFor returns the first sentence of the doc comment directly above n.
func (w *tsw) docFor(n *gts.Node, lead []*gts.Node) string {
	if w.spec.docFn != nil {
		if s := w.spec.docFn(w, n); s != "" {
			return s
		}
	}
	if len(lead) > 0 {
		var parts []string
		for _, c := range lead {
			parts = append(parts, w.text(c))
		}
		if s := firstDocLine(parts); s != "" {
			return s
		}
	}
	cur := n
	for p := cur.Parent(); p != nil && w.spec.wrappers[p.Type(w.lang)]; p = cur.Parent() {
		cur = p
	}
	row := cur.StartPoint().Row
	var block []string
	for s := cur.PrevSibling(); s != nil; s = s.PrevSibling() {
		t := s.Type(w.lang)
		if w.spec.attrs[t] || !s.IsNamed() {
			row = s.StartPoint().Row
			continue
		}
		if !w.spec.comments[t] || s.EndPoint().Row+1 < row {
			break
		}
		block = append(block, w.text(s))
		row = s.StartPoint().Row
	}
	for i, j := 0, len(block)-1; i < j; i, j = i+1, j-1 {
		block[i], block[j] = block[j], block[i]
	}
	return firstDocLine(block)
}

// firstDocLine returns the first meaningful sentence of a run of comment texts, top first.
func firstDocLine(block []string) string {
	for _, b := range block {
		for _, l := range strings.Split(b, "\n") {
			l = cleanDocLine(l)
			if l != "" && !strings.HasPrefix(l, "@") {
				return firstSentence(l)
			}
		}
	}
	return ""
}

var docMarkers = []string{"///", "//!", "//", "/**", "/*", "*/", "(**", "(*", "*)", "--|", "-- |", "--", "#", "\"\"\"", "*"}

func cleanDocLine(l string) string {
	l = strings.TrimSpace(l)
	for changed := true; changed; {
		changed = false
		for _, m := range docMarkers {
			if strings.HasPrefix(l, m) {
				l, changed = strings.TrimSpace(l[len(m):]), true
			}
		}
	}
	l = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(l, "*/"), "*)"))
	for {
		i := strings.IndexByte(l, '<')
		j := strings.IndexByte(l, '>')
		if i < 0 || j < i || strings.HasPrefix(l[i:], "<=") {
			break
		}
		l = strings.TrimSpace(l[:i] + l[j+1:])
	}
	return l
}

// splitType normalises a type or path expression to (qualifier, name).
func splitType(t string) (qual, name string) {
	t = stripAngles(strings.TrimSpace(t))
	if i := strings.IndexByte(t, '('); i >= 0 {
		t = t[:i]
	}
	t = normQual(t)
	t = strings.Trim(t, ".")
	if t == "" {
		return "", ""
	}
	return firstQual(t), lastSeg(t)
}

func stripAngles(s string) string {
	if !strings.ContainsAny(s, "<[") {
		return s
	}
	var b strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '<' || c == '[':
			depth++
		case (c == '>' || c == ']') && depth > 0:
			depth--
		case depth == 0:
			b.WriteByte(c)
		}
	}
	return b.String()
}

var sepReplacer = strings.NewReplacer("::", ".", "->", ".", "?.", ".", "!.", ".", "\\", ".", ":", ".", "$", "", " ", "", "\n", "", "\t", "")

// normQual turns a receiver expression into a dotted chain, or "" when the
// expression is not a plain chain of names (a call result, an index, a literal).
func normQual(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s[0] == '@' {
		s = "this." + strings.TrimLeft(s, "@")
	}
	s = sepReplacer.Replace(s)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(isIdentByte(c) || c == '.') {
			return ""
		}
	}
	return strings.Trim(s, ".")
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// splitCallee splits "a.b.c" style callee text into receiver chain and name.
func splitCallee(t string) (qual, name string) {
	t = strings.TrimSpace(stripAngles(t))
	i := len(t)
	for i > 0 && isIdentByte(t[i-1]) {
		i--
	}
	name = strings.TrimLeft(t[i:], "$")
	if name == "" || name[0] >= '0' && name[0] <= '9' {
		return "", ""
	}
	rest := strings.TrimRight(t[:i], ".:>-?\\ !")
	return normQual(rest), name
}

func (w *tsw) call(n *gts.Node, r callRule) {
	var qual, name string
	switch {
	case r.custom != nil:
		qual, name = r.custom(w, n)
	case r.obj != "":
		name = strings.TrimLeft(w.fieldText(n, r.name), "$")
		qual = normQual(w.fieldText(n, r.obj))
	default:
		var c *gts.Node
		if r.fn != "" {
			c = w.field(n, r.fn)
		} else if n.NamedChildCount() > 0 {
			c = n.NamedChild(0)
		}
		if c == nil {
			return
		}
		qual, name = splitCallee(w.text(c))
		if r.ctor {
			_, name = splitType(w.text(c))
			qual = ""
		}
	}
	w.emitCall(name, qual)
}

func (w *tsw) emitCall(name, qual string) {
	if name == "" || (qual == "" && w.spec.builtins[name]) {
		return
	}
	if a, ok := w.spec.qualAlias[qual]; ok {
		qual = a
	}
	qual = w.infer(qual)
	key := itoa(w.owner+1) + "\x00" + qual + "\x00" + name
	if w.seen[key] {
		return
	}
	w.seen[key] = true
	w.fs.Refs = append(w.fs.Refs, Ref{From: w.owner, Kind: EdgeCalls, Name: name, Qual: qual})
}

func (w *tsw) emitHeritage(idx int, kind string, n *gts.Node) {
	if n == nil {
		return
	}
	q, name := splitType(w.text(n))
	if name == "" || name == "object" {
		return
	}
	w.fs.Refs = append(w.fs.Refs, Ref{From: idx, Kind: kind, Name: name, Qual: q})
}

// typeName reduces a type annotation to a bare class name, or "" for builtins
// and anything that does not look like a user type.
func typeName(t string) string {
	t = stripAngles(t)
	t = strings.NewReplacer("&", " ", "*", " ", "?", " ", "!", " ", ":", " ", "mut ", " ", "const ", " ", "dyn ", " ", "impl ", " ", "final ", " ", "var ", " ", "val ", " ").Replace(t)
	f := strings.Fields(t)
	if len(f) == 0 {
		return ""
	}
	t = f[len(f)-1]
	t = lastSeg(strings.NewReplacer("::", ".", "\\", ".").Replace(t))
	if t == "" || t[0] < 'A' || t[0] > 'Z' {
		return ""
	}
	for i := 0; i < len(t); i++ {
		if !isIdentByte(t[i]) {
			return ""
		}
	}
	return t
}

// decl records a typed name so later receiver chains can be typed.
func (w *tsw) decl(n *gts.Node, r declRule) {
	var name, typ string
	if r.fn != nil {
		name, typ = r.fn(w, n)
	} else {
		if r.name != "" {
			name = lastIdent(w.fieldText(n, r.name))
		}
		if r.typ != "" {
			typ = typeName(w.fieldText(n, r.typ))
		} else if r.typeFromParent && n.Parent() != nil {
			typ = typeName(w.fieldText(n.Parent(), "type"))
		}
		if typ == "" && r.value != "" {
			typ = w.valueType(w.field(n, r.value))
		}
	}
	w.bind(name, typ)
}

// bind stores name's type in the local scope, or as a field of the enclosing type.
func (w *tsw) bind(name, typ string) {
	if name == "" || typ == "" {
		return
	}
	if !w.inFunc && w.typ != "" {
		w.fields[w.typ+"\x00"+name] = typ
		return
	}
	w.locals[name] = typ
}

func lastIdent(s string) string {
	s = strings.TrimSpace(s)
	i := len(s)
	for i > 0 && isIdentByte(s[i-1]) {
		i--
	}
	return strings.TrimLeft(s[i:], "$")
}

// valueType infers the class of a constructor-style initializer such as Foo(), new Foo() or Foo::new().
func (w *tsw) valueType(v *gts.Node) string {
	if v == nil {
		return ""
	}
	r, ok := w.spec.calls[v.Type(w.lang)]
	if !ok {
		return ""
	}
	var qual, name string
	switch {
	case r.custom != nil:
		qual, name = r.custom(w, v)
	case r.obj != "":
		name, qual = w.fieldText(v, r.name), normQual(w.fieldText(v, r.obj))
	default:
		var c *gts.Node
		if r.fn != "" {
			c = w.field(v, r.fn)
		} else if v.NamedChildCount() > 0 {
			c = v.NamedChild(0)
		}
		if c == nil {
			return ""
		}
		qual, name = splitCallee(w.text(c))
		if r.ctor {
			_, name = splitType(w.text(c))
			qual = ""
		}
	}
	if qual == "" {
		return typeName(name)
	}
	switch name {
	case "new", "create", "default", "from", "of", "init", "build":
		return typeName(lastSeg(qual))
	}
	return ""
}

// infer rewrites a receiver chain to "@Type" when the receiver is a typed local or field.
func (w *tsw) infer(qual string) string {
	if qual == "" || strings.HasPrefix(qual, "@") {
		return qual
	}
	switch first := firstSeg(qual); first {
	case "this", "self", "cls", "Self":
		rest := strings.TrimPrefix(qual, first+".")
		if rest != qual && !strings.Contains(rest, ".") && w.typ != "" {
			if t := w.fields[w.typ+"\x00"+rest]; t != "" {
				return "@" + t
			}
		}
		return qual
	}
	if !strings.Contains(qual, ".") {
		if t := w.locals[qual]; t != "" {
			return "@" + t
		}
		if w.typ != "" {
			if t := w.fields[w.typ+"\x00"+qual]; t != "" {
				return "@" + t
			}
		}
	}
	return qual
}

// kwOf returns the keyword token that introduces a declaration (class, struct, extension, ...).
func (w *tsw) kwOf(n *gts.Node) string {
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c != nil && !c.IsNamed() {
			return c.Type(w.lang)
		}
	}
	return ""
}

// modVis finds an access keyword among a declaration's modifier children.
func (w *tsw) modVis(n *gts.Node) string {
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		switch c.Type(w.lang) {
		case "modifiers", "modifier", "visibility_modifier", "accessibility_modifier", "access_modifier", "visibility", "member_modifier", "access_specifier":
			t := w.text(c)
			for _, k := range [...]string{"public", "private", "protected", "internal", "external", "fileprivate", "open"} {
				if strings.Contains(t, k) {
					return k
				}
			}
		}
	}
	return ""
}

func dedupImports(in []Import) []Import {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, im := range in {
		k := im.Spec + "\x00" + im.Alias + "\x00" + strings.Join(im.Names, ",")
		if !seen[k] {
			seen[k] = true
			out = append(out, im)
		}
	}
	return out
}
