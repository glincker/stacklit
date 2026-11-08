package graph

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// ---- Python ----

var pySpec = &tsSpec{
	grammar: "python", lang: "python",
	defs: map[string]defRule{
		"function_definition": {kind: KindFunc, callable: true},
		"class_definition":    {kind: KindClass, container: true, heritage: pyHeritage},
	},
	calls: map[string]callRule{"call": {fn: "function"}},
	decls: map[string]declRule{
		"typed_parameter":         {fn: pyTypedParam},
		"typed_default_parameter": {name: "name", typ: "type"},
		"assignment":              {fn: pyAssign},
	},
	special:  map[string]func(w *tsw, n *gts.Node) bool{"import_statement": pyImport, "import_from_statement": pyFrom},
	comments: set("comment"),
	wrappers: set("decorated_definition"),
	builtins: wordSet("len range str int float bool list dict set tuple frozenset isinstance issubclass super open enumerate zip map filter sorted reversed min max sum any all getattr setattr hasattr delattr type repr format iter next abs round id hash vars dir bytes bytearray object property staticmethod classmethod callable input divmod pow chr ord hex bin oct slice print __init__"),
	docFn:    pyDoc,
	finish:   pyLayoutCheck,
	vis: func(w *tsw, n *gts.Node, name string) string {
		if strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "__") || strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
			return "private"
		}
		return "public"
	},
}

// pyLayoutCheck flags trees where a nested definition is not indented deeper
// than its parent, which means the grammar runtime lost track of a block.
func pyLayoutCheck(w *tsw) {
	for i, p := range w.parents {
		if p >= 0 && w.cols[i] <= w.cols[p] {
			w.fs.unreliable = true
			return
		}
	}
}

func pyHeritage(w *tsw, n *gts.Node, idx int) {
	sc := w.field(n, "superclasses")
	if sc == nil {
		return
	}
	for _, c := range namedKids(w, sc) {
		if t := w.typeOf(c); t == "identifier" || t == "attribute" {
			w.emitHeritage(idx, EdgeInherits, c)
		}
	}
}

func namedKids(w *tsw, n *gts.Node) []*gts.Node {
	var out []*gts.Node
	for i := 0; i < n.ChildCount(); i++ {
		if c := n.Child(i); c != nil && c.IsNamed() && !w.spec.comments[w.typeOf(c)] {
			out = append(out, c)
		}
	}
	return out
}

func pyTypedParam(w *tsw, n *gts.Node) (string, string) {
	id := w.kidOf(n, "identifier")
	return w.text(id), typeName(w.fieldText(n, "type"))
}

func pyAssign(w *tsw, n *gts.Node) (string, string) {
	left := w.field(n, "left")
	if left == nil {
		return "", ""
	}
	typ := typeName(w.fieldText(n, "type"))
	if typ == "" {
		typ = w.valueType(w.field(n, "right"))
	}
	switch w.typeOf(left) {
	case "identifier":
		return w.text(left), typ
	case "attribute":
		// self.repo = Repo() types the field for every method of the class.
		if typ != "" && w.typ != "" && w.fieldText(left, "object") == "self" {
			w.fields[w.typ+"\x00"+w.fieldText(left, "attribute")] = typ
		}
	}
	return "", ""
}

func pyDoc(w *tsw, n *gts.Node) string {
	body := w.field(n, "body")
	if body == nil || body.NamedChildCount() == 0 {
		return ""
	}
	st := body.NamedChild(0)
	s := st
	if st != nil && w.typeOf(st) == "expression_statement" && st.NamedChildCount() > 0 {
		s = st.NamedChild(0)
	}
	if s == nil || w.typeOf(s) != "string" {
		return ""
	}
	t := strings.Trim(w.text(s), "\"' \n\t")
	for _, l := range strings.Split(t, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return firstSentence(l)
		}
	}
	return ""
}

func pyImport(w *tsw, n *gts.Node) bool {
	for _, c := range namedKids(w, n) {
		switch w.typeOf(c) {
		case "dotted_name":
			spec := w.text(c)
			w.fs.Imports = append(w.fs.Imports, Import{Spec: spec, Alias: firstSeg(spec)})
		case "aliased_import":
			w.fs.Imports = append(w.fs.Imports, Import{Spec: w.fieldText(c, "name"), Alias: w.fieldText(c, "alias")})
		}
	}
	return true
}

func pyFrom(w *tsw, n *gts.Node) bool {
	imp := Import{Spec: w.fieldText(n, "module_name")}
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		switch {
		case w.typeOf(c) == "wildcard_import":
			imp.Names = append(imp.Names, "*")
		case n.FieldNameForChild(i, w.lang) == "name":
			if w.typeOf(c) == "aliased_import" {
				imp.Names = append(imp.Names, w.fieldText(c, "name")+"="+w.fieldText(c, "alias"))
			} else {
				imp.Names = append(imp.Names, w.text(c))
			}
		}
	}
	if imp.Spec != "" {
		w.fs.Imports = append(w.fs.Imports, imp)
	}
	return true
}

// ---- TypeScript and JavaScript ----

var tsFnValue = set("arrow_function", "function_expression", "function", "generator_function")

func newTSSpec(lang string) *tsSpec {
	return &tsSpec{
		grammar: lang, lang: lang,
		defs: map[string]defRule{
			"function_declaration":           {kind: KindFunc, callable: true},
			"generator_function_declaration": {kind: KindFunc, callable: true},
			"class_declaration":              {kind: KindClass, container: true, heritage: tsHeritage},
			"abstract_class_declaration":     {kind: KindClass, container: true, heritage: tsHeritage},
			"interface_declaration":          {kind: KindInterface, container: true, heritage: tsHeritage},
			"type_alias_declaration":         {kind: KindType},
			"enum_declaration":               {kind: KindEnum},
			"method_definition":              {kind: KindFunc, callable: true, names: classMemberName},
			"method_signature":               {kind: KindFunc, callable: true, names: classMemberName},
			"abstract_method_signature":      {kind: KindFunc, callable: true, names: classMemberName},
		},
		calls: map[string]callRule{
			"call_expression": {fn: "function"},
			"new_expression":  {fn: "constructor", ctor: true},
		},
		decls: map[string]declRule{
			"required_parameter":      {fn: tsParam},
			"optional_parameter":      {fn: tsParam},
			"variable_declarator":     {name: "name", typ: "type", value: "value"},
			"public_field_definition": {name: "name", typ: "type", value: "value"},
		},
		special: map[string]func(w *tsw, n *gts.Node) bool{
			"import_statement":        tsImport,
			"export_statement":        tsExport,
			"variable_declarator":     tsVarDecl,
			"public_field_definition": tsField,
			"field_definition":        tsField,
		},
		comments: set("comment"),
		wrappers: set("export_statement"),
		builtins: wordSet("String Number Boolean Array Object Symbol Date Error Promise Map Set parseInt parseFloat isNaN setTimeout setInterval clearTimeout clearInterval BigInt RegExp super require import"),
		vis:      tsVis,
	}
}

func classMemberName(w *tsw, n *gts.Node) (string, string) {
	if w.typ == "" || w.inFunc {
		return "", ""
	}
	return strings.Trim(w.fieldText(n, "name"), "\"'"), ""
}

func tsHeritage(w *tsw, n *gts.Node, idx int) {
	for _, c := range namedKids(w, n) {
		switch w.typeOf(c) {
		case "class_heritage":
			ext := w.kidOf(c, "extends_clause")
			if ext != nil {
				if v := w.field(ext, "value"); v != nil && (w.typeOf(v) == "identifier" || w.typeOf(v) == "member_expression") {
					w.emitHeritage(idx, EdgeInherits, v)
				}
			} else if kids := namedKids(w, c); len(kids) > 0 && (w.typeOf(kids[0]) == "identifier" || w.typeOf(kids[0]) == "member_expression") {
				w.emitHeritage(idx, EdgeInherits, kids[0]) // plain JS: extends <expression>
			}
			if imp := w.kidOf(c, "implements_clause"); imp != nil {
				for _, t := range namedKids(w, imp) {
					w.emitHeritage(idx, EdgeImplements, t)
				}
			}
		case "extends_type_clause":
			for _, t := range namedKids(w, c) {
				if w.typeOf(t) != "type_arguments" {
					w.emitHeritage(idx, EdgeInherits, t)
				}
			}
		}
	}
}

func tsParam(w *tsw, n *gts.Node) (string, string) {
	name := lastIdent(w.fieldText(n, "pattern"))
	typ := typeName(w.fieldText(n, "type"))
	if typ != "" && name != "" && w.typ != "" && w.kidOf(n, "accessibility_modifier", "override_modifier") != nil {
		w.fields[w.typ+"\x00"+name] = typ // constructor parameter property
	}
	return name, typ
}

func tsVis(w *tsw, n *gts.Node, name string) string {
	if strings.HasPrefix(name, "#") {
		return "private"
	}
	if w.typ != "" && !w.inFunc {
		if v := w.modVis(n); v != "" {
			return v
		}
		return "public"
	}
	if p := n.Parent(); p != nil && w.typeOf(p) == "export_statement" {
		return "public"
	}
	return "private"
}

func tsStr(w *tsw, n *gts.Node) string {
	if n == nil {
		return ""
	}
	return strings.Trim(w.text(n), "\"'`")
}

func tsImport(w *tsw, n *gts.Node) bool {
	src := tsStr(w, w.field(n, "source"))
	if src == "" {
		return true
	}
	imp := Import{Spec: src}
	if cl := w.kidOf(n, "import_clause"); cl != nil {
		for _, c := range namedKids(w, cl) {
			switch w.typeOf(c) {
			case "identifier":
				imp.Alias = w.text(c)
			case "namespace_import":
				if id := w.kidOf(c, "identifier"); id != nil {
					imp.Alias = w.text(id)
				}
			case "named_imports":
				for _, sp := range w.kidsOf(c, "import_specifier") {
					imp.Names = append(imp.Names, tsSpecifier(w, sp))
				}
			}
		}
	}
	w.fs.Imports = append(w.fs.Imports, imp)
	return true
}

func tsSpecifier(w *tsw, sp *gts.Node) string {
	name := w.fieldText(sp, "name")
	if al := w.fieldText(sp, "alias"); al != "" {
		return name + "=" + al
	}
	return name
}

func tsExport(w *tsw, n *gts.Node) bool {
	src := tsStr(w, w.field(n, "source"))
	if src == "" {
		return false
	}
	imp := Import{Spec: src}
	if cl := w.kidOf(n, "export_clause"); cl != nil {
		for _, sp := range w.kidsOf(cl, "export_specifier") {
			imp.Names = append(imp.Names, tsSpecifier(w, sp))
		}
	}
	w.fs.Imports = append(w.fs.Imports, imp)
	return true
}

// tsVarDecl turns require() bindings into imports and file-level function values into definitions.
func tsVarDecl(w *tsw, n *gts.Node) bool {
	val := w.field(n, "value")
	if val == nil {
		return false
	}
	vt := w.typeOf(val)
	if vt == "call_expression" && w.fieldText(val, "function") == "require" {
		if args := w.field(val, "arguments"); args != nil && args.NamedChildCount() > 0 {
			imp := Import{Spec: tsStr(w, args.NamedChild(0))}
			nm := w.field(n, "name")
			switch {
			case nm == nil:
			case w.typeOf(nm) == "object_pattern":
				for _, c := range namedKids(w, nm) {
					switch w.typeOf(c) {
					case "shorthand_property_identifier_pattern":
						imp.Names = append(imp.Names, w.text(c))
					case "pair_pattern":
						imp.Names = append(imp.Names, w.fieldText(c, "key")+"="+w.fieldText(c, "value"))
					}
				}
			default:
				imp.Alias = w.text(nm)
			}
			w.fs.Imports = append(w.fs.Imports, imp)
			return true
		}
	}
	if tsFnValue[vt] && w.owner == -1 && w.typ == "" && !w.inFunc {
		name := w.fieldText(n, "name")
		span := n
		if p := n.Parent(); p != nil && (w.typeOf(p) == "lexical_declaration" || w.typeOf(p) == "variable_declaration") {
			span = p
		}
		if name != "" {
			w.define(n, defOpts{name: name, kind: KindFunc, callable: true, span: span, walk: []*gts.Node{val}})
			return true
		}
	}
	return false
}

// tsField makes arrow-function class properties methods and records typed fields.
func tsField(w *tsw, n *gts.Node) bool {
	name := w.fieldText(n, "name")
	if name == "" {
		name = w.fieldText(n, "property")
	}
	val := w.field(n, "value")
	if val != nil && tsFnValue[w.typeOf(val)] && name != "" && w.typ != "" {
		w.define(n, defOpts{name: name, kind: KindFunc, callable: true, walk: []*gts.Node{val}})
		return true
	}
	if typ := typeName(w.fieldText(n, "type")); typ != "" && name != "" && w.typ != "" {
		w.bind(name, typ)
	} else if val != nil && name != "" {
		w.bind(name, w.valueType(val))
	}
	return false
}

// ---- Rust ----

var rsSpec = &tsSpec{
	grammar: "rust", lang: "rust",
	defs: map[string]defRule{
		"function_item":           {kind: KindFunc, callable: true},
		"function_signature_item": {kind: KindFunc, callable: true},
		"struct_item":             {kind: KindType, container: true},
		"union_item":              {kind: KindType, container: true},
		"enum_item":               {kind: KindEnum},
		"type_item":               {kind: KindType},
		"trait_item":              {kind: KindTrait, container: true, heritage: rsTraitBounds, sect: "public"},
	},
	calls: map[string]callRule{"call_expression": {fn: "function"}},
	decls: map[string]declRule{
		"parameter":         {name: "pattern", typ: "type"},
		"let_declaration":   {name: "pattern", typ: "type", value: "value"},
		"field_declaration": {name: "name", typ: "type"},
	},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"impl_item":       rsImpl,
		"use_declaration": rsUse,
		"mod_item":        rsMod,
	},
	comments: set("line_comment", "block_comment"),
	attrs:    set("attribute_item"),
	builtins: wordSet("Some None Ok Err Box Vec drop"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if v := w.kidOf(n, "visibility_modifier"); v != nil {
			if t := w.text(v); t != "pub" {
				return strings.TrimPrefix(strings.Trim(t, "pub()"), " ")
			}
			return "public"
		}
		if w.sect == "public" {
			return "public"
		}
		return "private"
	},
	finish: rsFinish,
}

func rsTraitBounds(w *tsw, n *gts.Node, idx int) {
	b := w.field(n, "bounds")
	if b == nil {
		return
	}
	for _, c := range namedKids(w, b) {
		t := w.typeOf(c)
		if t == "lifetime" || w.text(c) == "Sized" {
			continue
		}
		w.emitHeritage(idx, EdgeInherits, c)
	}
}

type rsPendingImpl struct{ target, trait string }

func rsImpl(w *tsw, n *gts.Node) bool {
	_, target := splitType(w.fieldText(n, "type"))
	if target == "" {
		return false
	}
	savedTyp, savedSect, savedFunc := w.typ, w.sect, w.inFunc
	w.typ, w.inFunc, w.sect = target, false, ""
	if tr := w.fieldText(n, "trait"); tr != "" {
		w.sect = "public"
		w.pending = append(w.pending, rsPendingImpl{target, tr})
	}
	if b := w.field(n, "body"); b != nil {
		w.children(b)
	}
	w.typ, w.sect, w.inFunc = savedTyp, savedSect, savedFunc
	return true
}

// rsFinish links a type to the traits it implements once every definition in the file is known.
func rsFinish(w *tsw) {
	for _, p := range w.pending {
		pi, ok := p.(rsPendingImpl)
		if !ok {
			continue
		}
		for i, d := range w.fs.Defs {
			if d.Name == pi.target && d.Recv == "" && d.Kind != KindFunc {
				q, name := splitType(pi.trait)
				w.fs.Refs = append(w.fs.Refs, Ref{From: i, Kind: EdgeImplements, Name: name, Qual: q})
				break
			}
		}
	}
}

func rsUse(w *tsw, n *gts.Node) bool {
	if a := w.field(n, "argument"); a != nil {
		w.fs.Imports = append(w.fs.Imports, expandUse(w.text(a))...)
	}
	return true
}

func rsMod(w *tsw, n *gts.Node) bool {
	if w.field(n, "body") == nil {
		if name := w.fieldText(n, "name"); name != "" {
			w.fs.Imports = append(w.fs.Imports, Import{Spec: "mod:" + name, Alias: name})
		}
		return true
	}
	return false
}

// ---- Java ----

var javaSpec = &tsSpec{
	grammar: "java", lang: "java",
	defs: map[string]defRule{
		"class_declaration":           {kind: KindClass, container: true, heritage: javaHeritage},
		"interface_declaration":       {kind: KindInterface, container: true, heritage: javaHeritage},
		"enum_declaration":            {kind: KindEnum, container: true, heritage: javaHeritage},
		"record_declaration":          {kind: KindClass, container: true, heritage: javaHeritage},
		"annotation_type_declaration": {kind: KindInterface, container: true},
		"method_declaration":          {kind: KindFunc, callable: true},
		"constructor_declaration":     {kind: KindFunc, callable: true, names: javaCtor},
	},
	calls: map[string]callRule{
		"method_invocation":          {obj: "object", name: "name"},
		"object_creation_expression": {fn: "type", ctor: true},
	},
	decls: map[string]declRule{
		"formal_parameter":    {name: "name", typ: "type"},
		"variable_declarator": {name: "name", typeFromParent: true, value: "value"},
		"assignment_expression": {fn: func(w *tsw, n *gts.Node) (string, string) {
			assignField(w, normQual(w.fieldText(n, "left")), w.field(n, "right"))
			return "", ""
		}},
	},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"package_declaration": func(w *tsw, n *gts.Node) bool {
			w.fs.Pkg = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(w.text(n), "package"), ";"))
			return true
		},
		"import_declaration": javaImport,
	},
	comments: set("line_comment", "block_comment"),
	builtins: wordSet("String Integer Long Double Boolean Object Math"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if v := w.modVis(n); v != "" {
			return v
		}
		return "package"
	},
}

func javaCtor(w *tsw, n *gts.Node) (string, string) { return "<init>", "" }

// assignField types this.x = y from y's known type; used by constructors that copy a typed parameter.
func assignField(w *tsw, left string, right *gts.Node) {
	rest := strings.TrimPrefix(left, "this.")
	if rest == left || strings.Contains(rest, ".") || w.typ == "" || right == nil {
		return
	}
	typ := w.valueType(right)
	if typ == "" {
		if q := normQual(w.text(right)); q != "" && !strings.Contains(q, ".") {
			typ = w.locals[q]
		}
	}
	if typ != "" {
		w.fields[w.typ+"\x00"+rest] = typ
	}
}

func javaHeritage(w *tsw, n *gts.Node, idx int) {
	if sc := w.kidOf(n, "superclass"); sc != nil {
		for _, t := range namedKids(w, sc) {
			w.emitHeritage(idx, EdgeInherits, t)
		}
	}
	emitList := func(holder *gts.Node, kind string) {
		for _, tl := range namedKids(w, holder) {
			if w.typeOf(tl) == "type_list" {
				for _, t := range namedKids(w, tl) {
					w.emitHeritage(idx, kind, t)
				}
			} else {
				w.emitHeritage(idx, kind, tl)
			}
		}
	}
	if si := w.kidOf(n, "super_interfaces"); si != nil {
		emitList(si, EdgeImplements)
	}
	if ei := w.kidOf(n, "extends_interfaces"); ei != nil {
		emitList(ei, EdgeInherits)
	}
}

func javaImport(w *tsw, n *gts.Node) bool {
	t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(w.text(n)), "import")), ";"))
	if strings.HasPrefix(t, "static ") {
		return true
	}
	if strings.HasSuffix(t, "*") {
		w.fs.Imports = append(w.fs.Imports, Import{Spec: strings.TrimSuffix(t, ".*") + ".*"})
		return true
	}
	t = strings.Join(strings.Fields(t), "")
	w.fs.Imports = append(w.fs.Imports, Import{Spec: t, Alias: lastSeg(t)})
	return true
}
