package graph

import (
	"path"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

func moreSpecs() []*tsSpec {
	return []*tsSpec{
		csharpSpec, kotlinSpec, swiftSpec, phpSpec, rubySpec, scalaSpec, dartSpec,
		newCSpec("c"), newCSpec("cpp"), elixirSpec, luaSpec, zigSpec, haskellSpec, ocamlSpec, solSpec,
	}
}

var assignRule = declRule{fn: func(w *tsw, n *gts.Node) (string, string) {
	assignField(w, normQual(w.fieldText(n, "left")), w.field(n, "right"))
	return "", ""
}}

// heritageKind guesses inherits vs implements for languages that list both in one clause.
func heritageKind(name string, first bool, ownerKind string) string {
	if ownerKind == KindInterface || ownerKind == KindTrait {
		return EdgeInherits
	}
	if len(name) > 1 && name[0] == 'I' && name[1] >= 'A' && name[1] <= 'Z' {
		return EdgeImplements
	}
	if first {
		return EdgeInherits
	}
	return EdgeImplements
}

// emitBases emits one heritage ref per node, classifying by position and name.
func emitBases(w *tsw, idx int, nodes []*gts.Node) {
	owner := w.fs.Defs[idx].Kind
	first := true
	for _, b := range nodes {
		_, name := splitType(w.text(b))
		if name == "" {
			continue
		}
		w.emitHeritage(idx, heritageKind(name, first, owner), b)
		first = false
	}
}

func skipTypeArgs(w *tsw, ns []*gts.Node) []*gts.Node {
	out := ns[:0:0]
	for _, c := range ns {
		if t := w.typeOf(c); t != "type_argument_list" && t != "type_arguments" && t != "arguments" {
			out = append(out, c)
		}
	}
	return out
}

// ---- C# ----

var csharpSpec = &tsSpec{
	grammar: "c_sharp", lang: "csharp",
	defs: map[string]defRule{
		"class_declaration":         {kind: KindClass, container: true, heritage: csBases},
		"struct_declaration":        {kind: KindType, container: true, heritage: csBases},
		"record_declaration":        {kind: KindClass, container: true, heritage: csBases},
		"record_struct_declaration": {kind: KindType, container: true, heritage: csBases},
		"interface_declaration":     {kind: KindInterface, container: true, heritage: csBases},
		"enum_declaration":          {kind: KindEnum, container: true},
		"method_declaration":        {kind: KindFunc, callable: true},
		"constructor_declaration":   {kind: KindFunc, callable: true, names: javaCtor},
		"local_function_statement":  {kind: KindFunc, callable: true},
	},
	calls: map[string]callRule{
		"invocation_expression":      {fn: "function"},
		"object_creation_expression": {fn: "type", ctor: true},
	},
	decls: map[string]declRule{
		"parameter":             {name: "name", typ: "type"},
		"variable_declarator":   {fn: csDeclarator},
		"assignment_expression": assignRule,
	},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"namespace_declaration":             csNamespace,
		"file_scoped_namespace_declaration": csNamespace,
		"using_directive":                   csUsing,
	},
	comments:  set("comment"),
	builtins:  wordSet("nameof typeof"),
	nameKids:  []string{"identifier"},
	qualAlias: map[string]string{"base": "super"},
	vis: func(w *tsw, n *gts.Node, name string) string {
		if v := w.modVis(n); v != "" {
			return v
		}
		if w.typ != "" && !w.inFunc {
			if w.owner >= 0 && w.fs.Defs[w.owner].Kind == KindInterface {
				return "public"
			}
			return "private"
		}
		return "internal"
	},
}

func csBases(w *tsw, n *gts.Node, idx int) {
	if bl := w.kidOf(n, "base_list"); bl != nil {
		emitBases(w, idx, skipTypeArgs(w, namedKids(w, bl)))
	}
}

func csNamespace(w *tsw, n *gts.Node) bool {
	if w.fs.Pkg == "" {
		w.fs.Pkg = normQual(w.fieldText(n, "name"))
	}
	return false
}

func csUsing(w *tsw, n *gts.Node) bool {
	kids := namedKids(w, n)
	if len(kids) == 0 || strings.Contains(w.text(n), "using static") {
		return true
	}
	target := kids[len(kids)-1]
	imp := Import{Spec: normQual(w.text(target))}
	if nm := w.field(n, "name"); nm != nil {
		imp.Alias = w.text(nm)
	}
	if imp.Spec != "" {
		w.fs.Imports = append(w.fs.Imports, imp)
	}
	return true
}

func csDeclarator(w *tsw, n *gts.Node) (string, string) {
	name := w.fieldText(n, "name")
	typ := ""
	if p := n.Parent(); p != nil {
		typ = typeName(w.fieldText(p, "type"))
	}
	if typ == "" {
		for i := n.NamedChildCount() - 1; i >= 0; i-- {
			if c := n.NamedChild(i); c != nil && w.typeOf(c) != "identifier" {
				typ = w.valueType(c)
				break
			}
		}
	}
	return name, typ
}

// ---- Kotlin ----

var ktType = set("user_type", "nullable_type")

var kotlinSpec = &tsSpec{
	grammar: "kotlin", lang: "kotlin",
	defs: map[string]defRule{
		"class_declaration":    {kind: KindClass, container: true, heritage: ktBases, kindFn: ktKind},
		"object_declaration":   {kind: KindClass, container: true, heritage: ktBases},
		"function_declaration": {kind: KindFunc, callable: true},
	},
	calls: map[string]callRule{"call_expression": {}},
	decls: map[string]declRule{
		"parameter":            {fn: ktParam},
		"class_parameter":      {fn: ktParam},
		"property_declaration": {fn: ktProp},
	},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"package_header": func(w *tsw, n *gts.Node) bool {
			w.fs.Pkg = normQual(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(w.text(n)), "package")))
			return true
		},
		"import_header": ktImport,
	},
	comments: set("multiline_comment", "line_comment"),
	nameKids: []string{"simple_identifier", "type_identifier"},
	skip:     set("delegation_specifier"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if v := w.modVis(n); v != "" {
			return v
		}
		return "public"
	},
}

func ktKind(w *tsw, n *gts.Node) string {
	if w.kwOf(n) == "interface" {
		return KindInterface
	}
	return KindClass
}

func ktBases(w *tsw, n *gts.Node, idx int) {
	for _, ds := range w.kidsOf(n, "delegation_specifier") {
		if ci := w.kidOf(ds, "constructor_invocation"); ci != nil {
			if ut := w.kidOf(ci, "user_type"); ut != nil {
				w.emitHeritage(idx, EdgeInherits, ut)
			}
		} else if ut := w.kidOf(ds, "user_type"); ut != nil {
			kind := EdgeImplements
			if w.fs.Defs[idx].Kind == KindInterface {
				kind = EdgeInherits
			}
			w.emitHeritage(idx, kind, ut)
		}
	}
}

func ktParam(w *tsw, n *gts.Node) (string, string) {
	id := w.kidOf(n, "simple_identifier")
	ut := w.kidOf(n, "user_type", "nullable_type")
	return w.text(id), typeName(w.text(ut))
}

func ktProp(w *tsw, n *gts.Node) (string, string) {
	vd := w.kidOf(n, "variable_declaration")
	if vd == nil {
		return "", ""
	}
	name := w.text(w.kidOf(vd, "simple_identifier"))
	typ := typeName(w.text(w.kidOf(vd, "user_type", "nullable_type")))
	if typ == "" {
		typ = w.valueType(w.kidOf(n, "call_expression"))
	}
	return name, typ
}

func ktImport(w *tsw, n *gts.Node) bool {
	t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(w.text(n)), "import"))
	alias := ""
	if i := strings.Index(t, " as "); i >= 0 {
		t, alias = strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+4:])
	}
	t = strings.Join(strings.Fields(t), "")
	if strings.HasSuffix(t, ".*") {
		w.fs.Imports = append(w.fs.Imports, Import{Spec: t})
		return true
	}
	if alias == "" {
		alias = lastSeg(t)
	}
	w.fs.Imports = append(w.fs.Imports, Import{Spec: t, Alias: alias})
	return true
}

// ---- Swift ----

var swiftSpec = &tsSpec{
	grammar: "swift", lang: "swift",
	defs: map[string]defRule{
		"class_declaration":             {kind: KindClass, container: true, heritage: swiftBases, kindFn: swiftKind},
		"protocol_declaration":          {kind: KindInterface, container: true, heritage: swiftBases},
		"function_declaration":          {kind: KindFunc, callable: true},
		"protocol_function_declaration": {kind: KindFunc, callable: true},
		"init_declaration":              {kind: KindFunc, callable: true, names: func(w *tsw, n *gts.Node) (string, string) { return "init", "" }},
	},
	calls: map[string]callRule{"call_expression": {}},
	decls: map[string]declRule{
		"parameter":            {name: "name", typ: "type"},
		"property_declaration": {fn: swiftProp},
	},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"class_declaration": swiftExtension,
		"import_declaration": func(w *tsw, n *gts.Node) bool {
			m := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(w.text(n)), "import"))
			w.fs.Imports = append(w.fs.Imports, Import{Spec: m, Alias: firstSeg(m)})
			return true
		},
	},
	comments:  set("comment", "multiline_comment"),
	qualAlias: map[string]string{"Self": "self"},
	vis: func(w *tsw, n *gts.Node, name string) string {
		if v := w.modVis(n); v != "" {
			return v
		}
		return "internal"
	},
}

func swiftKind(w *tsw, n *gts.Node) string {
	switch w.kwOf(n) {
	case "class", "actor":
		return KindClass
	case "struct":
		return KindType
	case "enum":
		return KindEnum
	}
	return ""
}

func swiftBases(w *tsw, n *gts.Node, idx int) {
	first := true
	for _, is := range w.kidsOf(n, "inheritance_specifier") {
		t := w.field(is, "inherits_from")
		if t == nil {
			continue
		}
		kind := EdgeImplements
		switch {
		case w.fs.Defs[idx].Kind == KindInterface:
			kind = EdgeInherits
		case first && w.fs.Defs[idx].Kind == KindClass:
			kind = EdgeInherits
		}
		first = false
		w.emitHeritage(idx, kind, t)
	}
}

// swiftExtension scopes the members of "extension T" to T without adding a definition.
func swiftExtension(w *tsw, n *gts.Node) bool {
	if w.kwOf(n) != "extension" {
		return false
	}
	_, target := splitType(w.fieldText(n, "name"))
	if target == "" {
		return true
	}
	savedTyp, savedFunc := w.typ, w.inFunc
	w.typ, w.inFunc = target, false
	if b := w.field(n, "body"); b != nil {
		w.children(b)
	}
	w.typ, w.inFunc = savedTyp, savedFunc
	return true
}

func swiftProp(w *tsw, n *gts.Node) (string, string) {
	p := w.field(n, "name")
	if p == nil {
		return "", ""
	}
	name := lastIdent(w.text(p))
	typ := typeName(w.text(w.kidOf(n, "type_annotation")))
	if typ == "" {
		typ = w.valueType(w.field(n, "value"))
	}
	return name, typ
}

// ---- PHP ----

var phpSpec = &tsSpec{
	grammar: "php", lang: "php",
	defs: map[string]defRule{
		"class_declaration":     {kind: KindClass, container: true, heritage: phpBases},
		"interface_declaration": {kind: KindInterface, container: true, heritage: phpBases},
		"trait_declaration":     {kind: KindTrait, container: true},
		"enum_declaration":      {kind: KindEnum, container: true, heritage: phpBases},
		"function_definition":   {kind: KindFunc, callable: true},
		"method_declaration":    {kind: KindFunc, callable: true},
	},
	calls: map[string]callRule{
		"function_call_expression":        {fn: "function"},
		"member_call_expression":          {obj: "object", name: "name"},
		"nullsafe_member_call_expression": {obj: "object", name: "name"},
		"scoped_call_expression":          {obj: "scope", name: "name"},
		"object_creation_expression":      {custom: phpNew},
	},
	decls: map[string]declRule{
		"simple_parameter":             {name: "name", typ: "type"},
		"property_promotion_parameter": {name: "name", typ: "type"},
		"property_declaration":         {fn: phpProp},
		"assignment_expression":        assignRule,
	},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"namespace_definition": func(w *tsw, n *gts.Node) bool {
			if w.fs.Pkg == "" {
				w.fs.Pkg = normQual(w.fieldText(n, "name"))
			}
			return false
		},
		"namespace_use_declaration": phpUse,
	},
	comments:  set("comment"),
	nameKids:  []string{"name"},
	builtins:  wordSet("array isset empty count strlen print echo die exit unset list compact define"),
	qualAlias: map[string]string{"parent": "super", "static": "self"},
	vis: func(w *tsw, n *gts.Node, name string) string {
		if v := w.modVis(n); v != "" {
			return v
		}
		return "public"
	},
}

func phpNew(w *tsw, n *gts.Node) (string, string) {
	if c := w.kidOf(n, "name", "qualified_name"); c != nil {
		_, name := splitType(w.text(c))
		return "", name
	}
	return "", ""
}

func phpBases(w *tsw, n *gts.Node, idx int) {
	if b := w.kidOf(n, "base_clause"); b != nil {
		kind := EdgeInherits
		for _, c := range namedKids(w, b) {
			w.emitHeritage(idx, kind, c)
		}
	}
	if b := w.kidOf(n, "class_interface_clause"); b != nil {
		for _, c := range namedKids(w, b) {
			w.emitHeritage(idx, EdgeImplements, c)
		}
	}
}

func phpProp(w *tsw, n *gts.Node) (string, string) {
	typ := typeName(w.text(w.field(n, "type")))
	if pe := w.kidOf(n, "property_element"); pe != nil {
		return lastIdent(w.fieldText(pe, "name")), typ
	}
	return "", ""
}

func phpUse(w *tsw, n *gts.Node) bool {
	add := func(prefix string, c *gts.Node) {
		var spec, alias string
		if w.typeOf(c) == "namespace_use_clause" {
			if al := w.field(c, "alias"); al != nil {
				alias = w.text(al)
			}
			if q := w.kidOf(c, "qualified_name", "name"); q != nil {
				spec = normQual(w.text(q))
			}
		}
		if spec == "" {
			return
		}
		if prefix != "" {
			spec = prefix + "." + spec
		}
		if alias == "" {
			alias = lastSeg(spec)
		}
		w.fs.Imports = append(w.fs.Imports, Import{Spec: spec, Alias: alias})
	}
	for _, c := range namedKids(w, n) {
		switch w.typeOf(c) {
		case "namespace_use_clause":
			add("", c)
		case "namespace_name":
			prefix := normQual(w.text(c))
			if g := w.field(n, "body"); g != nil {
				for _, cc := range w.kidsOf(g, "namespace_use_clause") {
					add(prefix, cc)
				}
			}
		}
	}
	return true
}

// ---- Ruby ----

var rubyBuiltins = wordSet("puts print p require require_relative load raise include extend prepend attr_reader attr_writer attr_accessor private public protected module_function lambda proc loop format sprintf pp freeze")

var rubySpec = &tsSpec{
	grammar: "ruby", lang: "ruby",
	defs: map[string]defRule{
		"class":            {kind: KindClass, container: true, names: rubyName, heritage: rubyBases, sect: "public"},
		"module":           {kind: KindType, container: true, names: rubyName, sect: "public"},
		"method":           {kind: KindFunc, callable: true},
		"singleton_method": {kind: KindFunc, callable: true},
	},
	calls: map[string]callRule{"call": {obj: "receiver", name: "method"}},
	decls: map[string]declRule{"assignment": {fn: rubyAssign}},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"call":       rubyCall,
		"identifier": rubyIdent,
	},
	comments: set("comment"),
	builtins: rubyBuiltins,
	vis: func(w *tsw, n *gts.Node, name string) string {
		if w.sect == "private" || w.sect == "protected" {
			return w.sect
		}
		return "public"
	},
}

func rubyName(w *tsw, n *gts.Node) (string, string) {
	t := strings.ReplaceAll(w.fieldText(n, "name"), "::", ".")
	return lastSeg(t), ""
}

func rubyBases(w *tsw, n *gts.Node, idx int) {
	if sc := w.field(n, "superclass"); sc != nil {
		for _, c := range namedKids(w, sc) {
			w.emitHeritage(idx, EdgeInherits, c)
		}
	}
}

func rubyAssign(w *tsw, n *gts.Node) (string, string) {
	left := w.field(n, "left")
	if left == nil {
		return "", ""
	}
	typ := w.valueType(w.field(n, "right"))
	switch w.typeOf(left) {
	case "identifier":
		return w.text(left), typ
	case "instance_variable":
		if typ != "" && w.typ != "" {
			w.fields[w.typ+"\x00"+strings.TrimLeft(w.text(left), "@")] = typ
		}
	}
	return "", ""
}

// rubyIdent tracks private/protected sections and treats bare statement names as calls.
func rubyIdent(w *tsw, n *gts.Node) bool {
	p := n.Parent()
	if p == nil || w.typeOf(p) != "body_statement" {
		return false
	}
	name := w.text(n)
	switch name {
	case "private", "protected", "public":
		w.sect = name
		return true
	}
	if w.inFunc {
		w.emitCall(name, "")
	}
	return true
}

func rubyCall(w *tsw, n *gts.Node) bool {
	method := w.fieldText(n, "method")
	recv := w.field(n, "receiver")
	if recv == nil {
		args := w.field(n, "arguments")
		switch method {
		case "require", "require_relative", "load":
			if args != nil && args.NamedChildCount() > 0 {
				p := tsStr(w, args.NamedChild(0))
				if p != "" {
					if method == "require_relative" && !strings.HasPrefix(p, ".") {
						p = "./" + p
					}
					w.fs.Imports = append(w.fs.Imports, Import{Spec: p, Alias: path.Base(strings.TrimSuffix(p, ".rb"))})
				}
			}
			return true
		case "include", "extend", "prepend":
			if args != nil && w.owner >= 0 && !w.inFunc {
				for _, a := range namedKids(w, args) {
					w.emitHeritage(w.owner, EdgeImplements, a)
				}
			}
			return true
		}
	}
	if method == "new" && recv != nil {
		if _, name := splitType(w.text(recv)); name != "" {
			w.emitCall(name, "")
		}
		w.children(n)
		return true
	}
	qual := ""
	if recv != nil && w.typeOf(recv) != "call" {
		qual = normQual(w.text(recv))
	}
	w.emitCall(method, qual)
	w.children(n)
	return true
}

// ---- Scala ----

var scalaSpec = &tsSpec{
	grammar: "scala", lang: "scala",
	defs: map[string]defRule{
		"class_definition":     {kind: KindClass, container: true, heritage: scalaBases},
		"object_definition":    {kind: KindClass, container: true, heritage: scalaBases},
		"trait_definition":     {kind: KindTrait, container: true, heritage: scalaBases},
		"enum_definition":      {kind: KindEnum, container: true},
		"function_definition":  {kind: KindFunc, callable: true},
		"function_declaration": {kind: KindFunc, callable: true},
	},
	calls: map[string]callRule{
		"call_expression":     {fn: "function"},
		"instance_expression": {ctor: true},
	},
	decls: map[string]declRule{
		"class_parameter": {name: "name", typ: "type"},
		"parameter":       {name: "name", typ: "type"},
		"val_definition":  {name: "pattern", typ: "type", value: "value"},
		"var_definition":  {name: "pattern", typ: "type", value: "value"},
	},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"package_clause": func(w *tsw, n *gts.Node) bool {
			if w.fs.Pkg == "" {
				w.fs.Pkg = normQual(w.fieldText(n, "name"))
			}
			return false
		},
		"import_declaration": scalaImport,
	},
	comments: set("comment", "block_comment"),
	builtins: wordSet("println print require assert"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if v := w.modVis(n); v != "" {
			return v
		}
		return "public"
	},
}

func scalaBases(w *tsw, n *gts.Node, idx int) {
	ext := w.field(n, "extend")
	if ext == nil {
		return
	}
	emitBases(w, idx, skipTypeArgs(w, namedKids(w, ext)))
}

func scalaImport(w *tsw, n *gts.Node) bool {
	var parts []string
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c != nil && c.IsNamed() && n.FieldNameForChild(i, w.lang) == "path" {
			parts = append(parts, w.text(c))
		}
	}
	prefix := strings.Join(parts, ".")
	if prefix == "" {
		return true
	}
	if w.kidOf(n, "namespace_wildcard") != nil {
		w.fs.Imports = append(w.fs.Imports, Import{Spec: prefix + ".*"})
		return true
	}
	if sel := w.kidOf(n, "namespace_selectors"); sel != nil {
		for _, c := range namedKids(w, sel) {
			name := lastIdent(w.text(c))
			if t := w.typeOf(c); t == "identifier" || t == "arrow_renamed_identifier" {
				w.fs.Imports = append(w.fs.Imports, Import{Spec: prefix + "." + name, Alias: name})
			}
		}
		return true
	}
	w.fs.Imports = append(w.fs.Imports, Import{Spec: prefix, Alias: lastSeg(prefix)})
	return true
}

// ---- Dart ----

var dartSpec = &tsSpec{
	grammar: "dart", lang: "dart",
	defs: map[string]defRule{
		"class_definition":  {kind: KindClass, container: true, heritage: dartBases},
		"enum_declaration":  {kind: KindEnum},
		"mixin_declaration": {kind: KindTrait, container: true},
	},
	decls: map[string]declRule{"formal_parameter": {fn: dartParam}},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"program":          dartBody,
		"class_body":       dartBody,
		"selector":         dartSelector,
		"import_or_export": dartImport,
	},
	comments: set("comment", "documentation_comment"),
	builtins: wordSet("print"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if strings.HasPrefix(name, "_") {
			return "private"
		}
		return "public"
	},
}

func dartBases(w *tsw, n *gts.Node, idx int) {
	if sc := w.field(n, "superclass"); sc != nil {
		if t := w.kidOf(sc, "type_identifier"); t != nil {
			w.emitHeritage(idx, EdgeInherits, t)
		}
		if m := w.kidOf(sc, "mixins"); m != nil {
			for _, t := range w.kidsOf(m, "type_identifier") {
				w.emitHeritage(idx, EdgeImplements, t)
			}
		}
	}
	if in := w.field(n, "interfaces"); in != nil {
		for _, t := range w.kidsOf(in, "type_identifier") {
			w.emitHeritage(idx, EdgeImplements, t)
		}
	}
}

func dartParam(w *tsw, n *gts.Node) (string, string) {
	return w.fieldText(n, "name"), typeName(w.text(w.kidOf(n, "type_identifier")))
}

// dartBody pairs each signature with the function_body sibling that follows it,
// since the grammar keeps them apart.
func dartBody(w *tsw, n *gts.Node) bool {
	kids := make([]*gts.Node, 0, n.ChildCount())
	for i := 0; i < n.ChildCount(); i++ {
		if c := n.Child(i); c != nil {
			kids = append(kids, c)
		}
	}
	for i := 0; i < len(kids); i++ {
		c := kids[i]
		t := w.typeOf(c)
		sig := c
		switch t {
		case "method_signature", "declaration":
			sig = w.kidOf(c, "function_signature")
			if sig == nil {
				if t == "declaration" {
					dartField(w, c)
				}
				w.visit(c)
				continue
			}
		case "function_signature":
		default:
			w.visit(c)
			continue
		}
		name := w.fieldText(sig, "name")
		if name == "" {
			w.visit(c)
			continue
		}
		var body *gts.Node
		span := c
		if i+1 < len(kids) && w.typeOf(kids[i+1]) == "function_body" {
			body = kids[i+1]
			i++
		}
		if name == w.typ {
			name = "<init>"
		}
		end := span
		if body != nil {
			end = body
		}
		d := w.define(span, defOpts{name: name, kind: KindFunc, callable: true, walk: dartWalk(sig, body)})
		w.fs.Defs[d].End = int(end.EndPoint().Row) + 1
	}
	return true
}

func dartWalk(sig, body *gts.Node) []*gts.Node {
	out := []*gts.Node{sig}
	if body != nil {
		out = append(out, body)
	}
	return out
}

func dartField(w *tsw, decl *gts.Node) {
	typ := typeName(w.text(w.kidOf(decl, "type_identifier")))
	list := w.kidOf(decl, "initialized_identifier_list")
	if typ == "" || list == nil {
		return
	}
	for _, ii := range w.kidsOf(list, "initialized_identifier") {
		w.bind(w.text(w.kidOf(ii, "identifier")), typ)
	}
}

// dartSelector turns "a.b(x)" selector chains into calls.
func dartSelector(w *tsw, n *gts.Node) bool {
	if w.kidOf(n, "argument_part") == nil {
		return false
	}
	prev := n.PrevSibling()
	if prev == nil {
		return false
	}
	assignable := func(s *gts.Node) string {
		if w.typeOf(s) != "selector" {
			return ""
		}
		if u := w.kidOf(s, "unconditional_assignable_selector"); u != nil {
			return w.text(w.kidOf(u, "identifier"))
		}
		return ""
	}
	var name, qual string
	switch {
	case assignable(prev) != "":
		name = assignable(prev)
		var chain []string
		for c := prev.PrevSibling(); c != nil; c = c.PrevSibling() {
			if a := assignable(c); a != "" {
				chain = append([]string{a}, chain...)
				continue
			}
			if w.typeOf(c) == "identifier" {
				chain = append([]string{w.text(c)}, chain...)
				qual = strings.Join(chain, ".")
			}
			break
		}
	case w.typeOf(prev) == "identifier":
		name = w.text(prev)
	}
	if name != "" {
		w.emitCall(name, qual)
	}
	return false
}

func dartImport(w *tsw, n *gts.Node) bool {
	var spec, alias string
	var find func(x *gts.Node)
	find = func(x *gts.Node) {
		switch w.typeOf(x) {
		case "string_literal":
			if spec == "" {
				spec = strings.Trim(w.text(x), "\"'")
			}
			return
		case "import_specification":
			if id := w.kidOf(x, "identifier"); id != nil {
				alias = w.text(id)
			}
		}
		for i := 0; i < x.ChildCount(); i++ {
			if c := x.Child(i); c != nil {
				find(c)
			}
		}
	}
	find(n)
	if spec != "" {
		w.fs.Imports = append(w.fs.Imports, Import{Spec: spec, Alias: alias})
	}
	return true
}

// ---- C and C++ ----

func newCSpec(lang string) *tsSpec {
	return &tsSpec{
		grammar: lang, lang: lang,
		defs: map[string]defRule{
			"function_definition": {kind: KindFunc, callable: true, names: cFuncName},
			"struct_specifier":    {kind: KindType, container: true, names: cTypeName, heritage: cBases, sect: "public"},
			"union_specifier":     {kind: KindType, container: true, names: cTypeName, sect: "public"},
			"class_specifier":     {kind: KindClass, container: true, names: cTypeName, heritage: cBases, sect: "private"},
			"enum_specifier":      {kind: KindEnum, names: cTypeName},
		},
		calls: map[string]callRule{"call_expression": {fn: "function"}},
		decls: map[string]declRule{
			"parameter_declaration": {fn: cParam},
			"declaration":           {fn: cLocal},
		},
		special: map[string]func(w *tsw, n *gts.Node) bool{
			"preproc_include": cInclude,
			"access_specifier": func(w *tsw, n *gts.Node) bool {
				w.sect = strings.TrimSpace(strings.TrimSuffix(w.text(n), ":"))
				return true
			},
			"type_definition":   cTypedef,
			"declaration":       cProto,
			"field_declaration": cProto,
		},
		comments: set("comment"),
		builtins: wordSet("printf fprintf sprintf snprintf malloc calloc realloc free memcpy memset memmove strlen strcmp strcpy sizeof assert exit"),
		vis: func(w *tsw, n *gts.Node, name string) string {
			if w.sect != "" {
				return w.sect
			}
			if sc := w.kidOf(n, "storage_class_specifier"); sc != nil && w.text(sc) == "static" {
				return "private"
			}
			return "public"
		},
	}
}

// cDeclarator unwraps pointer and reference declarators down to the function declarator.
func cFuncDeclarator(w *tsw, d *gts.Node) *gts.Node {
	for d != nil {
		switch w.typeOf(d) {
		case "function_declarator":
			return d
		case "pointer_declarator", "reference_declarator", "parenthesized_declarator", "attributed_declarator":
			next := w.field(d, "declarator")
			if next == nil && d.NamedChildCount() > 0 {
				next = d.NamedChild(int(d.NamedChildCount()) - 1)
			}
			d = next
		default:
			return nil
		}
	}
	return nil
}

func cDeclName(w *tsw, fd *gts.Node) (string, string) {
	nm := w.field(fd, "declarator")
	if nm == nil {
		return "", ""
	}
	if w.typeOf(nm) == "qualified_identifier" {
		scope := lastSeg(normQual(w.fieldText(nm, "scope")))
		return lastIdent(w.fieldText(nm, "name")), scope
	}
	return strings.TrimSpace(w.text(nm)), ""
}

func cFuncName(w *tsw, n *gts.Node) (string, string) {
	fd := cFuncDeclarator(w, w.field(n, "declarator"))
	if fd == nil {
		return "", ""
	}
	return cDeclName(w, fd)
}

func cTypeName(w *tsw, n *gts.Node) (string, string) {
	if w.field(n, "body") == nil {
		return "", ""
	}
	return w.fieldText(n, "name"), ""
}

func cBases(w *tsw, n *gts.Node, idx int) {
	if bc := w.kidOf(n, "base_class_clause"); bc != nil {
		for _, c := range namedKids(w, bc) {
			if t := w.typeOf(c); t != "access_specifier" && t != "virtual" {
				w.emitHeritage(idx, EdgeInherits, c)
			}
		}
	}
}

func cInclude(w *tsw, n *gts.Node) bool {
	p := w.field(n, "path")
	if p != nil && w.typeOf(p) == "string_literal" {
		spec := strings.Trim(w.text(p), "\"")
		w.fs.Imports = append(w.fs.Imports, Import{Spec: spec})
	}
	return true
}

// cProto records declarations of functions in headers and class bodies, where the definition lives elsewhere.
func cProto(w *tsw, n *gts.Node) bool {
	fd := cFuncDeclarator(w, w.field(n, "declarator"))
	if fd == nil {
		return false
	}
	header := strings.HasSuffix(w.fs.Path, ".h") || strings.HasSuffix(w.fs.Path, ".hpp") || strings.HasSuffix(w.fs.Path, ".hh") || strings.HasSuffix(w.fs.Path, ".hxx")
	if (!header && w.typ == "") || w.inFunc {
		return false
	}
	name, recv := cDeclName(w, fd)
	if name == "" {
		return false
	}
	w.define(n, defOpts{name: name, recv: recv, kind: KindFunc, callable: true, noWalk: true, proto: true})
	return true
}

// cTypedef names "typedef struct { ... } Name;" after its declarator.
func cTypedef(w *tsw, n *gts.Node) bool {
	spec := w.kidOf(n, "struct_specifier", "union_specifier", "enum_specifier")
	if spec == nil || w.field(spec, "name") != nil || w.field(spec, "body") == nil {
		return false
	}
	name := lastIdent(w.fieldText(n, "declarator"))
	if name == "" {
		return false
	}
	kind, container := KindType, true
	if w.typeOf(spec) == "enum_specifier" {
		kind, container = KindEnum, false
	}
	w.define(n, defOpts{name: name, kind: kind, container: container, walk: []*gts.Node{spec}})
	return true
}

func cParam(w *tsw, n *gts.Node) (string, string) {
	return lastIdent(w.fieldText(n, "declarator")), typeName(w.fieldText(n, "type"))
}

func cLocal(w *tsw, n *gts.Node) (string, string) {
	d := w.field(n, "declarator")
	if d == nil {
		return "", ""
	}
	if w.typeOf(d) == "init_declarator" {
		d = w.field(d, "declarator")
	}
	if d == nil || w.typeOf(d) == "function_declarator" {
		return "", ""
	}
	return lastIdent(w.text(d)), typeName(w.fieldText(n, "type"))
}

// ---- Elixir ----

var exDefs = set("def", "defp", "defmacro", "defmacrop", "defdelegate", "defguard", "defguardp")
var exImports = set("alias", "import", "use", "require")
var exKeywords = set("if", "unless", "case", "cond", "with", "for", "try", "receive", "quote", "unquote", "raise", "reraise", "throw", "fn", "defstruct", "defexception", "super", "spawn", "send", "inspect", "to_string", "is_nil", "is_atom", "is_binary", "is_list", "is_map", "length", "hd", "tl", "elem", "put_in", "get_in", "not", "and", "or", "when")

var elixirSpec = &tsSpec{
	grammar: "elixir", lang: "elixir",
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"call":           exCall,
		"unary_operator": func(w *tsw, n *gts.Node) bool { return strings.HasPrefix(w.text(n), "@") },
	},
	comments: set("comment"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if t := w.fieldText(n, "target"); t == "defp" || t == "defmacrop" || t == "defguardp" {
			return "private"
		}
		return "public"
	},
}

func exSnake(mod string) string {
	var out []string
	for _, seg := range strings.Split(mod, ".") {
		var b strings.Builder
		for i := 0; i < len(seg); i++ {
			c := seg[i]
			if c >= 'A' && c <= 'Z' {
				if i > 0 && !(seg[i-1] >= 'A' && seg[i-1] <= 'Z') {
					b.WriteByte('_')
				}
				c += 'a' - 'A'
			}
			b.WriteByte(c)
		}
		out = append(out, b.String())
	}
	return strings.Join(out, ".")
}

func exArgs(w *tsw, n *gts.Node) []*gts.Node {
	if a := w.kidOf(n, "arguments"); a != nil {
		return namedKids(w, a)
	}
	return nil
}

func exCall(w *tsw, n *gts.Node) bool {
	target := w.field(n, "target")
	if target == nil {
		return false
	}
	tname := w.text(target)
	args := exArgs(w, n)
	switch {
	case tname == "defmodule" || tname == "defprotocol" || tname == "defimpl":
		if len(args) == 0 {
			return false
		}
		full := w.text(args[0])
		kind := KindClass
		if tname == "defprotocol" {
			kind = KindInterface
		}
		var walk []*gts.Node
		if b := w.kidOf(n, "do_block"); b != nil {
			walk = append(walk, b)
		}
		w.define(n, defOpts{name: lastSeg(full), kind: kind, container: true, walk: walk, noWalk: len(walk) == 0})
		return true
	case exDefs[tname]:
		if len(args) == 0 {
			return false
		}
		head := args[0]
		if w.typeOf(head) == "binary_operator" { // def f(x) when guard
			head = w.field(head, "left")
		}
		name := ""
		switch {
		case head == nil:
		case w.typeOf(head) == "call":
			name = w.text(w.field(head, "target"))
		case w.typeOf(head) == "identifier":
			name = w.text(head)
		}
		if name == "" {
			return false
		}
		var walk []*gts.Node
		walk = append(walk, args[1:]...)
		if b := w.kidOf(n, "do_block"); b != nil {
			walk = append(walk, b)
		}
		w.define(n, defOpts{name: name, kind: KindFunc, callable: true, walk: walk, noWalk: len(walk) == 0})
		return true
	case exImports[tname]:
		if len(args) > 0 && w.typeOf(args[0]) == "alias" {
			full := w.text(args[0])
			w.fs.Imports = append(w.fs.Imports, Import{Spec: exSnake(full), Alias: lastSeg(full)})
		}
		return true
	}
	if w.typeOf(target) == "dot" {
		w.emitCall(w.fieldText(target, "right"), normQual(w.fieldText(target, "left")))
	} else if !exKeywords[tname] && w.typeOf(target) == "identifier" {
		w.emitCall(tname, "")
	}
	return false
}

// ---- Lua ----

var luaSpec = &tsSpec{
	grammar: "lua", lang: "lua",
	defs: map[string]defRule{
		"function_declaration": {kind: KindFunc, callable: true, names: luaName},
	},
	calls:    map[string]callRule{"function_call": {fn: "name"}},
	special:  map[string]func(w *tsw, n *gts.Node) bool{"function_call": luaRequire},
	comments: set("comment"),
	builtins: wordSet("print require pairs ipairs type tostring tonumber setmetatable getmetatable error assert pcall xpcall select next rawget rawset unpack"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if strings.HasPrefix(strings.TrimSpace(w.text(n)), "local") {
			return "private"
		}
		return "public"
	},
}

func luaName(w *tsw, n *gts.Node) (string, string) {
	nm := w.field(n, "name")
	if nm == nil {
		return "", ""
	}
	switch w.typeOf(nm) {
	case "dot_index_expression":
		return w.fieldText(nm, "field"), w.fieldText(nm, "table")
	case "method_index_expression":
		return w.fieldText(nm, "method"), w.fieldText(nm, "table")
	}
	return w.text(nm), ""
}

func luaRequire(w *tsw, n *gts.Node) bool {
	if w.fieldText(n, "name") != "require" {
		return false
	}
	if args := w.field(n, "arguments"); args != nil && args.NamedChildCount() > 0 {
		if spec := tsStr(w, args.NamedChild(0)); spec != "" {
			w.fs.Imports = append(w.fs.Imports, Import{Spec: spec, Alias: lastSeg(spec)})
		}
	}
	return true
}

// ---- Zig ----

var zigSpec = &tsSpec{
	grammar: "zig", lang: "zig",
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"Decl":       zigDecl,
		"SuffixExpr": zigSuffix,
	},
	comments: set("doc_comment", "line_comment", "comment"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if strings.HasPrefix(strings.TrimSpace(w.text(n)), "pub ") {
			return "public"
		}
		if p := n.PrevSibling(); p != nil && w.text(p) == "pub" {
			return "public"
		}
		return "private"
	},
}

func zigDecl(w *tsw, n *gts.Node) bool {
	if fp := w.kidOf(n, "FnProto"); fp != nil {
		name := w.fieldText(fp, "function")
		if name == "" {
			return false
		}
		w.define(n, defOpts{name: name, kind: KindFunc, callable: true})
		return true
	}
	vd := w.kidOf(n, "VarDecl")
	if vd == nil {
		return false
	}
	name := w.fieldText(vd, "variable_type_function")
	if name == "" {
		return false
	}
	t := w.text(vd)
	if i := strings.Index(t, "@import("); i >= 0 {
		rest := t[i+len("@import("):]
		if j := strings.IndexByte(rest, ')'); j > 0 {
			spec := strings.Trim(rest[:j], "\" ")
			if strings.HasSuffix(spec, ".zig") && !strings.HasPrefix(spec, ".") {
				spec = "./" + spec
			}
			w.fs.Imports = append(w.fs.Imports, Import{Spec: spec, Alias: name})
		}
		return true
	}
	var cd *gts.Node
	var find func(x *gts.Node, depth int)
	find = func(x *gts.Node, depth int) {
		if cd != nil || depth > 4 {
			return
		}
		if w.typeOf(x) == "ContainerDecl" {
			cd = x
			return
		}
		for i := 0; i < x.ChildCount(); i++ {
			if c := x.Child(i); c != nil {
				find(c, depth+1)
			}
		}
	}
	find(vd, 0)
	if cd == nil {
		return false
	}
	kind := KindType
	if strings.HasPrefix(w.text(w.kidOf(cd, "ContainerDeclType")), "enum") {
		kind = KindEnum
	}
	w.define(n, defOpts{name: name, kind: kind, container: true, walk: []*gts.Node{cd}})
	return true
}

// zigSuffix reads call chains such as "std.debug.print(...)" and "self.go()".
func zigSuffix(w *tsw, n *gts.Node) bool {
	var chain []string
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		switch w.typeOf(c) {
		case "IDENTIFIER":
			chain = append(chain, w.text(c))
		case "BUILTINIDENTIFIER":
			chain = nil
		case "FnCallArguments":
			if len(chain) > 0 {
				w.emitCall(chain[len(chain)-1], strings.Join(chain[:len(chain)-1], "."))
			}
			chain = nil
		case "FieldOrFnCall":
			if f := w.field(c, "function_call"); f != nil {
				w.emitCall(w.text(f), strings.Join(chain, "."))
				chain = nil
			} else if f := w.field(c, "field_access"); f != nil {
				chain = append(chain, w.text(f))
			}
		}
	}
	return false
}

// ---- Haskell ----

var haskellSpec = &tsSpec{
	grammar: "haskell", lang: "haskell",
	defs: map[string]defRule{
		"data_type":    {kind: KindType},
		"newtype":      {kind: KindType},
		"type_synonym": {kind: KindType},
		"type_alias":   {kind: KindType},
		"class":        {kind: KindInterface},
		"function":     {kind: KindFunc, callable: true},
		"bind":         {kind: KindFunc, callable: true, names: hsBind},
	},
	calls: map[string]callRule{"apply": {custom: hsApply}},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"function": hsClause,
		"header": func(w *tsw, n *gts.Node) bool {
			w.fs.Pkg = normQual(w.fieldText(n, "module"))
			return true
		},
		"import": hsImport,
	},
	comments: set("comment", "haddock"),
	skip:     set("signature", "context"),
	builtins: wordSet("show print putStrLn putStr map filter foldr foldl return pure fmap not otherwise error head tail length fst snd id"),
	vis:      func(w *tsw, n *gts.Node, name string) string { return "public" },
}

// hsBind names a definition the grammar read as a bare binding, which happens
// when a type signature is followed by its equation after a layout break.
func hsBind(w *tsw, n *gts.Node) (string, string) {
	p := w.field(n, "pattern")
	if p == nil {
		return "", ""
	}
	switch w.typeOf(p) {
	case "signature":
		return w.fieldText(p, "pattern"), ""
	case "variable":
		return w.text(p), ""
	}
	return "", ""
}

func hsApply(w *tsw, n *gts.Node) (string, string) {
	f := w.field(n, "function")
	if f == nil || w.typeOf(f) == "apply" {
		return "", ""
	}
	switch w.typeOf(f) {
	case "variable", "qualified", "constructor", "name":
		return splitCallee(w.text(f))
	}
	return "", ""
}

// hsClause merges the equations of one function into a single definition.
func hsClause(w *tsw, n *gts.Node) bool {
	name := w.fieldText(n, "name")
	if k := len(w.fs.Defs) - 1; k >= 0 && w.owner == -1 && w.fs.Defs[k].Name == name && w.fs.Defs[k].Kind == KindFunc && w.fs.Defs[k].Recv == "" {
		w.fs.Defs[k].End = int(n.EndPoint().Row) + 1
		w.owner = k
		w.children(n)
		w.owner = -1
		return true
	}
	return false
}

func hsImport(w *tsw, n *gts.Node) bool {
	spec := normQual(w.fieldText(n, "module"))
	if spec == "" {
		return true
	}
	alias := normQual(w.fieldText(n, "alias"))
	if alias == "" {
		alias = lastSeg(spec)
	}
	w.fs.Imports = append(w.fs.Imports, Import{Spec: spec, Alias: alias})
	return true
}

// ---- OCaml ----

var ocamlSpec = &tsSpec{
	grammar: "ocaml", lang: "ocaml",
	calls: map[string]callRule{"application_expression": {custom: ocamlApply}},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"value_definition":  ocamlValue,
		"type_definition":   ocamlType,
		"module_definition": ocamlModule,
		"open_module": func(w *tsw, n *gts.Node) bool {
			if m := normQual(w.fieldText(n, "module")); m != "" {
				w.fs.Imports = append(w.fs.Imports, Import{Spec: m, Alias: lastSeg(m)})
			}
			return true
		},
	},
	comments: set("comment"),
	builtins: wordSet("print_string print_endline print_int string_of_int ignore fst snd not failwith raise ref incr decr"),
	vis:      func(w *tsw, n *gts.Node, name string) string { return "public" },
}

func ocamlApply(w *tsw, n *gts.Node) (string, string) {
	f := w.field(n, "function")
	if f == nil || w.typeOf(f) != "value_path" {
		return "", ""
	}
	qual, name := splitCallee(w.text(f))
	if qual != "" && !w.extra[qual] {
		w.extra[qual] = true
		w.fs.Imports = append(w.fs.Imports, Import{Spec: firstSeg(qual), Alias: firstSeg(qual)})
	}
	return qual, name
}

func ocamlValue(w *tsw, n *gts.Node) bool {
	for _, lb := range w.kidsOf(n, "let_binding") {
		pat := w.field(lb, "pattern")
		if pat == nil || w.typeOf(pat) != "value_name" {
			w.visit(lb)
			continue
		}
		body := w.field(lb, "body")
		isFn := w.kidOf(lb, "parameter") != nil
		if body != nil {
			if t := w.typeOf(body); t == "fun_expression" || t == "function_expression" {
				isFn = true
			}
		}
		if !isFn {
			w.visit(lb)
			continue
		}
		w.define(lb, defOpts{name: w.text(pat), kind: KindFunc, callable: true, span: n})
	}
	return true
}

func ocamlType(w *tsw, n *gts.Node) bool {
	for _, tb := range w.kidsOf(n, "type_binding") {
		if name := w.fieldText(tb, "name"); name != "" {
			w.define(tb, defOpts{name: name, kind: KindType, span: n, noWalk: true})
		}
	}
	return true
}

func ocamlModule(w *tsw, n *gts.Node) bool {
	mb := w.kidOf(n, "module_binding")
	if mb == nil {
		return false
	}
	name := w.text(w.kidOf(mb, "module_name"))
	if name == "" {
		return false
	}
	w.define(mb, defOpts{name: name, kind: KindClass, container: true, span: n})
	return true
}

// ---- Solidity ----

var solSpec = &tsSpec{
	grammar: "solidity", lang: "solidity",
	defs: map[string]defRule{
		"contract_declaration":   {kind: KindClass, container: true, heritage: solBases},
		"interface_declaration":  {kind: KindInterface, container: true, heritage: solBases},
		"library_declaration":    {kind: KindClass, container: true},
		"struct_declaration":     {kind: KindType},
		"enum_declaration":       {kind: KindEnum},
		"function_definition":    {kind: KindFunc, callable: true},
		"modifier_definition":    {kind: KindFunc, callable: true},
		"constructor_definition": {kind: KindFunc, callable: true, names: javaCtor},
	},
	calls: map[string]callRule{"call_expression": {fn: "function"}},
	decls: map[string]declRule{
		"parameter":                  {name: "name", typ: "type"},
		"state_variable_declaration": {name: "name", typ: "type"},
	},
	special: map[string]func(w *tsw, n *gts.Node) bool{
		"import_directive": func(w *tsw, n *gts.Node) bool {
			if s := tsStr(w, w.field(n, "source")); s != "" {
				w.fs.Imports = append(w.fs.Imports, Import{Spec: s, Alias: strings.TrimSuffix(path.Base(s), ".sol")})
			}
			return true
		},
	},
	comments: set("comment"),
	builtins: wordSet("require assert revert keccak256 sha256 ecrecover addmod mulmod selfdestruct emit"),
	vis: func(w *tsw, n *gts.Node, name string) string {
		if v := w.kidOf(n, "visibility"); v != nil {
			return w.text(v)
		}
		return "internal"
	},
}

func solBases(w *tsw, n *gts.Node, idx int) {
	var anc []*gts.Node
	for _, is := range w.kidsOf(n, "inheritance_specifier") {
		if a := w.field(is, "ancestor"); a != nil {
			anc = append(anc, a)
		}
	}
	emitBases(w, idx, anc)
}
