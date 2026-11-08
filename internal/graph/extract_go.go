package graph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var goMajorSuffix = regexp.MustCompile(`^v[0-9]+$`)

// extractGo uses go/ast, so definitions and direct calls are exact.
func extractGo(file string, src []byte) *FileSyms {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution|parser.ParseComments)
	if f == nil || err != nil && len(f.Decls) == 0 {
		return nil
	}
	fs := &FileSyms{Path: file, Lang: "go", Pkg: path.Dir(file)}

	for _, imp := range f.Imports {
		spec, uerr := strconv.Unquote(imp.Path.Value)
		if uerr != nil {
			continue
		}
		alias := ""
		switch {
		case imp.Name != nil && (imp.Name.Name == "_" || imp.Name.Name == "."):
			alias = ""
		case imp.Name != nil:
			alias = imp.Name.Name
		default:
			parts := strings.Split(spec, "/")
			alias = parts[len(parts)-1]
			if goMajorSuffix.MatchString(alias) && len(parts) > 1 {
				alias = parts[len(parts)-2]
			}
			alias = strings.ReplaceAll(strings.TrimPrefix(alias, "go-"), "-", "_")
		}
		fs.Imports = append(fs.Imports, Import{Spec: spec, Alias: alias})
	}

	header := func(start, end token.Pos) string {
		a, b := fset.Position(start).Offset, fset.Position(end).Offset
		if a < 0 || b > len(src) || a >= b {
			return ""
		}
		return sigOf(string(src[a:b]))
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			def := Def{Name: d.Name.Name, Qual: d.Name.Name, Kind: KindFunc, Line: fset.Position(d.Pos()).Line}
			recvVar := ""
			if d.Recv != nil && len(d.Recv.List) > 0 {
				r := d.Recv.List[0]
				def.Kind = KindMethod
				def.Recv = goTypeName(r.Type)
				def.Qual = def.Recv + "." + def.Name
				if len(r.Names) > 0 {
					recvVar = r.Names[0].Name
				}
			}
			end := d.End()
			if d.Body != nil {
				end = d.Body.Lbrace
			}
			def.Sig = header(d.Pos(), end)
			def.End = fset.Position(d.End()).Line
			def.Doc = firstSentence(d.Doc.Text())
			fs.Defs = append(fs.Defs, def)
			if d.Body != nil {
				idx := len(fs.Defs) - 1
				seen := map[string]bool{}
				ast.Inspect(d.Body, func(n ast.Node) bool {
					if c, ok := n.(*ast.CallExpr); ok {
						addGoCall(fs, idx, c, recvVar, seen)
					}
					return true
				})
			}
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, sp := range d.Specs {
				ts, ok := sp.(*ast.TypeSpec)
				if !ok {
					continue
				}
				def := Def{Name: ts.Name.Name, Qual: ts.Name.Name, Kind: KindType, Line: fset.Position(ts.Pos()).Line, End: fset.Position(ts.End()).Line}
				if ts.Doc != nil {
					def.Doc = firstSentence(ts.Doc.Text())
				} else {
					def.Doc = firstSentence(d.Doc.Text())
				}
				var fields *ast.FieldList
				switch t := ts.Type.(type) {
				case *ast.StructType:
					def.Sig = "struct " + ts.Name.Name
					fields = t.Fields
				case *ast.InterfaceType:
					def.Kind = KindInterface
					def.Sig = "interface " + ts.Name.Name
					fields = t.Methods
				default:
					def.Sig = "type " + ts.Name.Name
				}
				fs.Defs = append(fs.Defs, def)
				idx := len(fs.Defs) - 1
				if fields == nil {
					continue
				}
				for _, fld := range fields.List {
					if len(fld.Names) > 0 {
						if _, isFn := fld.Type.(*ast.FuncType); isFn && def.Kind == KindInterface {
							fs.Defs[idx].Methods = append(fs.Defs[idx].Methods, fld.Names[0].Name)
						}
						continue
					}
					// Embedded type: struct embedding or interface embedding.
					name, qual := goEmbedded(fld.Type)
					if name != "" {
						fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeInherits, Name: name, Qual: qual})
					}
				}
			}
		}
	}
	return fs
}

func goTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return goTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return goTypeName(t.X)
	case *ast.IndexListExpr:
		return goTypeName(t.X)
	case *ast.ParenExpr:
		return goTypeName(t.X)
	}
	return ""
}

func goEmbedded(e ast.Expr) (name, qual string) {
	switch t := e.(type) {
	case *ast.StarExpr:
		return goEmbedded(t.X)
	case *ast.Ident:
		return t.Name, ""
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return t.Sel.Name, x.Name
		}
	case *ast.IndexExpr:
		return goEmbedded(t.X)
	}
	return "", ""
}

func addGoCall(fs *FileSyms, from int, c *ast.CallExpr, recvVar string, seen map[string]bool) {
	var name, qual string
	switch fn := c.Fun.(type) {
	case *ast.Ident:
		name = fn.Name
	case *ast.SelectorExpr:
		name = fn.Sel.Name
		if x, ok := fn.X.(*ast.Ident); ok {
			qual = x.Name
			if recvVar != "" && qual == recvVar {
				qual = "this"
			}
		} else {
			qual = "?"
		}
	case *ast.IndexExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			name = id.Name
		}
	}
	if name == "" || goBuiltins[name] && qual == "" {
		return
	}
	key := qual + "\x00" + name
	if seen[key] {
		return
	}
	seen[key] = true
	fs.Refs = append(fs.Refs, Ref{From: from, Kind: EdgeCalls, Name: name, Qual: qual})
}

var goBuiltins = wordSet("append cap clear close complex copy delete imag len make max min new panic print println real recover string int int8 int16 int32 int64 uint uint8 uint16 uint32 uint64 float32 float64 byte rune bool error any")
