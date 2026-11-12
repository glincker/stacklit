package parser

import "strings"

// extensionLanguageMap maps file extensions to human-readable language names.
var extensionLanguageMap = map[string]string{
	".go":         "go",
	".ts":         "typescript",
	".tsx":        "tsx",
	".js":         "javascript",
	".jsx":        "jsx",
	".mjs":        "javascript",
	".cjs":        "javascript",
	".py":         "python",
	".rb":         "ruby",
	".rs":         "rust",
	".java":       "java",
	".kt":         "kotlin",
	".kts":        "kotlin",
	".swift":      "swift",
	".c":          "c",
	".h":          "c",
	".cpp":        "cpp",
	".cc":         "cpp",
	".cxx":        "cpp",
	".hpp":        "cpp",
	".hh":         "cpp",
	".hxx":        "cpp",
	".cs":         "csharp",
	".php":        "php",
	".sh":         "shell",
	".bash":       "shell",
	".zsh":        "shell",
	".fish":       "shell",
	".ps1":        "powershell",
	".lua":        "lua",
	".r":          "r",
	".scala":      "scala",
	".ex":         "elixir",
	".exs":        "elixir",
	".erl":        "erlang",
	".hrl":        "erlang",
	".hs":         "haskell",
	".ml":         "ocaml",
	".mli":        "ocaml",
	".clj":        "clojure",
	".cljs":       "clojure",
	".dart":       "dart",
	".groovy":     "groovy",
	".json":       "json",
	".yaml":       "yaml",
	".yml":        "yaml",
	".toml":       "toml",
	".xml":        "xml",
	".html":       "html",
	".htm":        "html",
	".css":        "css",
	".scss":       "scss",
	".sass":       "sass",
	".less":       "less",
	".sql":        "sql",
	".md":         "markdown",
	".mdx":        "mdx",
	".txt":        "text",
	".graphql":    "graphql",
	".gql":        "graphql",
	".proto":      "protobuf",
	".zig":        "zig",
	".gd":         "gdscript",
	".tf":         "hcl",
	".hcl":        "hcl",
	".vue":        "vue",
	".svelte":     "svelte",
	".cljc":       "clojure",
	".pl":         "perl",
	".pm":         "perl",
	".jl":         "julia",
	".m":          "objective-c",
	".mm":         "objective-c",
	".nim":        "nim",
	".fs":         "fsharp",
	".fsx":        "fsharp",
	".f90":        "fortran",
	".f95":        "fortran",
	".f03":        "fortran",
	".f08":        "fortran",
	".elm":        "elm",
	".sol":        "solidity",
	".cmake":      "cmake",
	".mk":         "make",
	".gleam":      "gleam",
	".dockerfile": "dockerfile",
}

// basenameLanguageMap names extensionless build files.
var basenameLanguageMap = map[string]string{
	"makefile":       "make",
	"gnumakefile":    "make",
	"dockerfile":     "dockerfile",
	"cmakelists.txt": "cmake",
}

// GenericParser is a fallback parser that identifies languages by file extension
// but does not perform import/export analysis.
type GenericParser struct{}

// CanParse always returns true — the generic parser handles any file.
func (g *GenericParser) CanParse(_ string) bool {
	return true
}

// Parse returns path, language (from extension), and line count only.
func (g *GenericParser) Parse(path string, content []byte) (*FileInfo, error) {
	return &FileInfo{
		Path:      path,
		Language:  detectLanguage(path, content),
		LineCount: countLines(content),
	}, nil
}

// detectLanguage maps the file extension to a language name. A .h header is
// classified from its content. Returns "unknown" for unrecognized extensions.
func detectLanguage(filename string, content []byte) string {
	lower := strings.ToLower(filename)
	base := lower
	if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
		base = base[i+1:]
	}
	if lang, ok := basenameLanguageMap[base]; ok {
		return lang
	}
	if strings.HasPrefix(base, "dockerfile.") {
		return "dockerfile"
	}

	// Walk from longest suffix to shortest to handle multi-part extensions.
	// For simple single extensions, find the last dot.
	idx := strings.LastIndex(lower, ".")
	if idx < 0 {
		return "unknown"
	}

	ext := lower[idx:]
	if ext == ".h" && HeaderLooksCpp(content) {
		return "cpp"
	}
	if lang, ok := extensionLanguageMap[ext]; ok {
		return lang
	}

	return "unknown"
}
