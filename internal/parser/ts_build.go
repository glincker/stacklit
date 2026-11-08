package parser

import (
	"fmt"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractCMake reads include/subdirectory/package dependencies and declared targets.
func extractCMake(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "cmake"}

	firstArg := func(args *gts.Node) string {
		if args == nil {
			return ""
		}
		if a := namedKids(args); len(a) > 0 {
			return nodeText(a[0], src)
		}
		return ""
	}

	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		switch n.Type(lang) {
		case "normal_command":
			id := childByType(n, lang, "identifier")
			if id == nil {
				return gts.WalkContinue
			}
			arg := firstArg(childByType(n, lang, "argument_list"))
			if arg == "" {
				return gts.WalkContinue
			}
			switch strings.ToLower(nodeText(id, src)) {
			case "include", "add_subdirectory", "find_package":
				info.Imports = append(info.Imports, arg)
			case "project":
				info.Exports = append(info.Exports, "project "+arg)
			case "add_library":
				info.Exports = append(info.Exports, "library "+arg)
			case "add_executable":
				info.Exports = append(info.Exports, "executable "+arg)
			case "add_custom_target":
				info.Exports = append(info.Exports, "target "+arg)
			}
		case "function_command", "macro_command":
			kind := "function"
			if n.Type(lang) == "macro_command" {
				kind = "macro"
			}
			args := childByType(n, lang, "argument_list")
			if name := firstArg(args); name != "" {
				info.Exports = append(info.Exports, truncSig(kind+" "+name+"("+cmakeRestArgs(args, src)+")", 100))
			}
		}
		return gts.WalkContinue
	})
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	return info
}

func cmakeRestArgs(args *gts.Node, src []byte) string {
	var out []string
	for i, a := range namedKids(args) {
		if i > 0 {
			out = append(out, nodeText(a, src))
		}
	}
	return strings.Join(out, " ")
}

// extractMake reads include directives and explicit rule targets.
func extractMake(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "make"}

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "include_directive":
			if l := childByType(n, lang, "list"); l != nil {
				for _, w := range namedKids(l) {
					info.Imports = append(info.Imports, nodeText(w, src))
				}
			}
		case "rule":
			t := childByType(n, lang, "targets")
			if t == nil {
				continue
			}
			var prereqs []string
			if p := childByType(n, lang, "prerequisites"); p != nil {
				for _, w := range namedKids(p) {
					prereqs = append(prereqs, nodeText(w, src))
				}
			}
			for _, w := range namedKids(t) {
				name := nodeText(w, src)
				// Pattern rules and special targets (.PHONY) are not user-facing entry points.
				if strings.HasPrefix(name, ".") || strings.Contains(name, "%") {
					continue
				}
				sig := name
				if len(prereqs) > 0 {
					sig += ": " + strings.Join(prereqs, " ")
				}
				info.Exports = append(info.Exports, truncSig(sig, 100))
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	return info
}

// extractDockerfile reads base images and build stages.
func extractDockerfile(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "dockerfile"}
	stage := 0
	stages := map[string]bool{}

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "from_instruction":
			image := ""
			if sp := childByType(n, lang, "image_spec"); sp != nil {
				image = strings.TrimSpace(nodeText(sp, src))
			}
			label := fmt.Sprintf("stage %d", stage)
			if al := childByType(n, lang, "image_alias"); al != nil {
				label = "stage " + nodeText(al, src)
				stages[nodeText(al, src)] = true
			}
			stage++
			info.Exports = append(info.Exports, label+" from "+image)
			// A FROM naming an earlier stage is not an external dependency.
			if image != "" && image != "scratch" && !stages[image] {
				info.Imports = append(info.Imports, image)
			}
		case "expose_instruction":
			info.Exports = append(info.Exports, "expose "+strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(nodeText(n, src)), "EXPOSE")))
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	return info
}
