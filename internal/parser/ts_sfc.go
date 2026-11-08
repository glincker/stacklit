package parser

import (
	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// extractVue handles Vue single-file components via their script blocks.
func extractVue(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	return extractSFC(root, lang, src, "vue")
}

// extractSvelte handles Svelte components via their script blocks.
func extractSvelte(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	return extractSFC(root, lang, src, "svelte")
}

// extractSFC parses each <script> body as TypeScript (a superset of JS) and merges the results.
func extractSFC(root *gts.Node, lang *gts.Language, src []byte, name string) *FileInfo {
	info := &FileInfo{Language: name}
	entry := grammars.DetectLanguage("x.ts")
	if entry == nil {
		return info
	}
	tsLang := entry.Language()

	for _, el := range childrenByType(root, lang, "script_element") {
		raw := childByType(el, lang, "raw_text")
		if raw == nil {
			continue
		}
		body := []byte(nodeText(raw, src))
		tree, err := gts.NewParser(tsLang).Parse(body)
		if err != nil || tree.RootNode() == nil {
			continue
		}
		sub := extractTypeScript(tree.RootNode(), tsLang, body, "x.ts")
		info.Imports = append(info.Imports, sub.Imports...)
		info.Exports = append(info.Exports, sub.Exports...)
		for k, v := range sub.TypeDefs {
			if info.TypeDefs == nil {
				info.TypeDefs = map[string]string{}
			}
			info.TypeDefs[k] = v
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	return info
}
