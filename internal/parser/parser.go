package parser

import (
	"bytes"
	"os"
	"strings"
)

// FileInfo holds the result of parsing a source file.
type FileInfo struct {
	Path         string            `json:"path"`
	Language     string            `json:"language"`
	Imports      []string          `json:"imports,omitempty"`
	Exports      []string          `json:"exports,omitempty"`
	TypeDefs     map[string]string `json:"type_defs,omitempty"` // type name -> brief definition
	LineCount    int               `json:"line_count"`
	IsEntrypoint bool              `json:"is_entrypoint,omitempty"`
}

// Parser extracts structured information from a source file.
type Parser interface {
	CanParse(filename string) bool
	Parse(path string, content []byte) (*FileInfo, error)
}

// registry holds all registered parsers in priority order.
// Generic must be last so it acts as a fallback.
var registry []Parser

func init() {
	registry = []Parser{
		&GoParser{},
		&TreeSitterParser{},
		&GenericParser{},
	}
}

// ParseFile reads a file from disk and dispatches to the appropriate parser.
func ParseFile(path string) (*FileInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	for _, p := range registry {
		if p.CanParse(path) {
			return p.Parse(path, content)
		}
	}

	// Should never reach here because GenericParser.CanParse always returns true.
	g := &GenericParser{}
	return g.Parse(path, content)
}

// ParseAll parses multiple files and returns all results.
// Errors from individual files are collected and non-fatal.
func ParseAll(paths []string) ([]*FileInfo, []error) {
	results := make([]*FileInfo, 0, len(paths))
	var errs []error

	for _, path := range paths {
		info, err := ParseFile(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		results = append(results, info)
	}

	return results, errs
}

// countLines counts the number of lines in content.
func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	count := bytes.Count(content, []byte{'\n'})
	// If the file does not end with a newline, add one for the last line.
	if content[len(content)-1] != '\n' {
		count++
	}
	return count
}

// extIs reports whether filename has one of the given extensions (case-insensitive).
func extIs(filename string, exts ...string) bool {
	lower := strings.ToLower(filename)
	for _, ext := range exts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// dataLanguages describe configuration and build files rather than program code.
var dataLanguages = map[string]bool{
	"json": true, "yaml": true, "toml": true, "dockerfile": true, "make": true, "cmake": true,
}

// IsDataLanguage reports whether lang is config or build data, which should not
// decide a project's or module's primary language.
func IsDataLanguage(lang string) bool {
	return dataLanguages[strings.ToLower(lang)]
}
