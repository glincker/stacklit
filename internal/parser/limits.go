package parser

import (
	"bytes"
	"path"
	"strings"
	"time"
)

// Fallback reasons recorded on FileInfo.Fallback and in the scan report.
const (
	FallbackTimeout   = "timeout"
	FallbackLarge     = "large"
	FallbackGenerated = "generated"
	FallbackMinified  = "minified"
)

// Limits bound the cost of structural (tree-sitter) parsing. Files that exceed
// them are indexed with the lightweight generic parser instead, so one huge or
// pathological file cannot stall or bloat a whole scan. Zero disables a limit.
type Limits struct {
	ParseTimeout time.Duration // wall-clock budget per file
	MaxBytes     int64         // largest file given to tree-sitter
	MaxLineLen   int           // longest line before a file counts as minified or generated
	Generated    bool          // route files marked as generated code to the lightweight path
}

// fallbackReason says why a file should skip tree-sitter, or "" to parse it.
func (l Limits) fallbackReason(rel string, content []byte) string {
	if l.MaxBytes > 0 && int64(len(content)) > l.MaxBytes {
		return FallbackLarge
	}
	if l.Generated && looksGenerated(rel, content) {
		return FallbackGenerated
	}
	if l.MaxLineLen > 0 && longestLine(content) > l.MaxLineLen {
		return FallbackMinified
	}
	return ""
}

// looksGenerated matches common generated-file names and header markers.
func looksGenerated(rel string, content []byte) bool {
	base := strings.ToLower(path.Base(rel))
	for _, s := range []string{".min.js", ".min.css", ".pb.go", ".pb.ts", "_pb.js", "_pb.d.ts", ".bundle.js", ".chunk.js"} {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	if strings.Contains(base, ".gen.") || strings.Contains(base, ".generated.") {
		return true
	}
	head := content
	if len(head) > 1024 {
		head = head[:1024]
	}
	head = bytes.ToLower(head)
	return bytes.Contains(head, []byte("code generated")) ||
		bytes.Contains(head, []byte("automatically generated")) ||
		bytes.Contains(head, []byte("auto-generated")) ||
		bytes.Contains(head, []byte("@generated")) ||
		bytes.Contains(head, []byte("do not edit"))
}

func longestLine(content []byte) int {
	longest := 0
	for len(content) > 0 {
		n := bytes.IndexByte(content, '\n')
		if n < 0 {
			n = len(content)
		}
		if n > longest {
			longest = n
		}
		if n+1 >= len(content) {
			break
		}
		content = content[n+1:]
	}
	return longest
}
