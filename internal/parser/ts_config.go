package parser

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// Config files can be huge (lockfiles, generated JSON), so key counts and
// parsed size are bounded, with the bounds overridable from the environment.
const (
	envConfigKeyCap   = "STACKLIT_CONFIG_KEY_CAP"
	envConfigMaxBytes = "STACKLIT_CONFIG_MAX_BYTES"
)

func envPositiveInt(name string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name))); err == nil && n > 0 {
		return n
	}
	return def
}

func configKeyCap() int   { return envPositiveInt(envConfigKeyCap, 40) }
func configMaxBytes() int { return envPositiveInt(envConfigMaxBytes, 256*1024) }

// keyCollector gathers unique keys up to the cap.
type keyCollector struct {
	keys []string
	seen map[string]bool
	max  int
}

func newKeyCollector() *keyCollector {
	return &keyCollector{seen: map[string]bool{}, max: configKeyCap()}
}

func (k *keyCollector) add(key string) {
	key = strings.TrimSpace(key)
	if key == "" || k.seen[key] || len(k.keys) >= k.max {
		return
	}
	k.seen[key] = true
	k.keys = append(k.keys, key)
}

// jsonTopKeys streams the top-level object keys of valid JSON without building
// a tree. The GLR parser takes seconds on a 100 KB locale file; this takes
// microseconds. ok is false for invalid JSON so the caller can fall back.
func jsonTopKeys(src []byte) (keys []string, ok bool) {
	if !json.Valid(src) {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(src))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, err == nil
	}
	kc := newKeyCollector()
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, false
		}
		k, isStr := t.(string)
		if !isStr {
			return nil, false
		}
		kc.add(k)
		if err := skipJSONValue(dec); err != nil {
			return nil, false
		}
	}
	return kc.keys, true
}

// skipJSONValue consumes one value, including nested containers.
func skipJSONValue(dec *json.Decoder) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	d, isDelim := t.(json.Delim)
	if !isDelim || (d != '{' && d != '[') {
		return nil
	}
	for depth := 1; depth > 0; {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := t.(json.Delim); ok {
			if d == '{' || d == '[' {
				depth++
			} else {
				depth--
			}
		}
	}
	return nil
}

// extractJSON lists the top-level object keys.
func extractJSON(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "json"}
	if len(src) > configMaxBytes() {
		return info
	}
	kc := newKeyCollector()
	if obj := childByType(root, lang, "object"); obj != nil {
		for _, p := range childrenByType(obj, lang, "pair") {
			if k := childByType(p, lang, "string"); k != nil {
				kc.add(stripQuotes(nodeText(k, src)))
			}
		}
	}
	info.Exports = kc.keys
	return info
}

// extractYAML lists the top-level mapping keys of each document.
func extractYAML(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "yaml"}
	if len(src) > configMaxBytes() {
		return info
	}
	kc := newKeyCollector()
	for _, doc := range childrenByType(root, lang, "document") {
		bn := childByType(doc, lang, "block_node")
		if bn == nil {
			continue
		}
		bm := childByType(bn, lang, "block_mapping")
		if bm == nil {
			continue
		}
		for _, p := range childrenByType(bm, lang, "block_mapping_pair") {
			if k := namedKids(p); len(k) > 0 {
				kc.add(stripQuotes(nodeText(k[0], src)))
			}
		}
	}
	info.Exports = kc.keys
	return info
}

// extractTOML lists root keys, tables and array tables.
func extractTOML(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "toml"}
	if len(src) > configMaxBytes() {
		return info
	}
	kc := newKeyCollector()
	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "pair":
			if k := namedKids(n); len(k) > 0 {
				kc.add(stripQuotes(nodeText(k[0], src)))
			}
		case "table":
			if k := namedKids(n); len(k) > 0 {
				kc.add("[" + nodeText(k[0], src) + "]")
			}
		case "table_array_element":
			if k := namedKids(n); len(k) > 0 {
				kc.add("[[" + nodeText(k[0], src) + "]]")
			}
		}
	}
	info.Exports = kc.keys
	return info
}

var graphqlKinds = map[string]string{
	"object_type_definition":       "type",
	"interface_type_definition":    "interface",
	"enum_type_definition":         "enum",
	"input_object_type_definition": "input",
	"union_type_definition":        "union",
	"scalar_type_definition":       "scalar",
	"directive_definition":         "directive",
	"object_type_extension":        "type",
	"interface_type_extension":     "interface",
	"enum_type_extension":          "enum",
	"input_object_type_extension":  "input",
}

// extractGraphQL reads schema types with their fields and named operations.
func extractGraphQL(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "graphql"}
	acc := newTypeAcc()

	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		t := n.Type(lang)
		if kind, ok := graphqlKinds[t]; ok {
			name := childByType(n, lang, "name")
			if name == nil {
				return gts.WalkContinue
			}
			tn := nodeText(name, src)
			info.Exports = append(info.Exports, kind+" "+tn)
			acc.add(tn, "")
			if fd := childByType(n, lang, "fields_definition"); fd != nil {
				for _, f := range childrenByType(fd, lang, "field_definition") {
					acc.add(tn, truncSig(nodeText(f, src), 80))
				}
			}
			if ev := childByType(n, lang, "enum_values_definition"); ev != nil {
				for _, v := range childrenByType(ev, lang, "enum_value_definition") {
					acc.add(tn, strings.TrimSpace(nodeText(v, src)))
				}
			}
			if iv := childByType(n, lang, "input_fields_definition"); iv != nil {
				for _, v := range childrenByType(iv, lang, "input_value_definition") {
					acc.add(tn, truncSig(nodeText(v, src), 60))
				}
			}
			if u := childByType(n, lang, "union_member_types"); u != nil {
				acc.add(tn, truncSig(nodeText(u, src), 100))
			}
			return gts.WalkSkipChildren
		}
		switch t {
		case "operation_definition":
			if name := childByType(n, lang, "name"); name != nil {
				kind := "query"
				if ot := childByType(n, lang, "operation_type"); ot != nil {
					kind = nodeText(ot, src)
				}
				info.Exports = append(info.Exports, kind+" "+nodeText(name, src))
			}
			return gts.WalkSkipChildren
		case "fragment_definition":
			if fn := childByType(n, lang, "fragment_name"); fn != nil {
				info.Exports = append(info.Exports, "fragment "+strings.TrimSpace(nodeText(fn, src)))
			}
			return gts.WalkSkipChildren
		}
		return gts.WalkContinue
	})
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
