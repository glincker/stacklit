package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHeaderLooksCpp(t *testing.T) {
	cppHeaders := [][]byte{
		[]byte("class Acquirer {\npublic:\n  void run();\n};\n"),
		[]byte("namespace iso { struct Msg; }\n"),
		[]byte("template <typename T>\nT clamp(T v);\n"),
		[]byte("std::string name;\n"),
		[]byte("virtual void process() = 0;\n"),
	}
	for _, src := range cppHeaders {
		if !HeaderLooksCpp(src) {
			t.Errorf("HeaderLooksCpp(%q) = false, want true", src)
		}
	}

	cHeaders := [][]byte{
		[]byte("#ifndef ACQUIRER_H\n#define ACQUIRER_H\nint process(const char *msg);\n#endif\n"),
		[]byte("typedef struct { int id; } acquirer;\n"),
		[]byte(""),
	}
	for _, src := range cHeaders {
		if HeaderLooksCpp(src) {
			t.Errorf("HeaderLooksCpp(%q) = true, want false", src)
		}
	}
}

func TestHeaderLanguageContentOnly(t *testing.T) {
	if got := HeaderLanguage("", []byte("class A {};\n")); got != "cpp" {
		t.Errorf("C++ header = %q, want cpp", got)
	}
	if got := HeaderLanguage("", []byte("int f(void);\n")); got != "c" {
		t.Errorf("C header = %q, want c", got)
	}
}

func TestHeaderLanguageNeighbour(t *testing.T) {
	dir := t.TempDir()
	header := filepath.Join(dir, "acquirer.h")
	if err := os.WriteFile(header, []byte("int process(const char *msg);\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A plain C header with no C++ neighbour stays C.
	if got := HeaderLanguage(header, []byte("int process(const char *msg);\n")); got != "c" {
		t.Errorf("header without neighbours = %q, want c", got)
	}

	// Any translation unit in the directory marks the header as C++.
	if err := os.WriteFile(filepath.Join(dir, "acquirer.cc"), []byte("int process(const char *msg);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := HeaderLanguage(header, []byte("int process(const char *msg);\n")); got != "cpp" {
		t.Errorf("header beside .cc = %q, want cpp", got)
	}
}

func TestSiblingIsCppPrefersSameBaseName(t *testing.T) {
	dir := t.TempDir()
	header := filepath.Join(dir, "acquirer.h")
	if err := os.WriteFile(header, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.c"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if SiblingIsCpp(header) {
		t.Error("SiblingIsCpp = true with only a .c neighbour, want false")
	}
	if err := os.WriteFile(filepath.Join(dir, "acquirer.cxx"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !SiblingIsCpp(header) {
		t.Error("SiblingIsCpp = false with acquirer.cxx present, want true")
	}
	if SiblingIsCpp("") {
		t.Error("SiblingIsCpp(\"\") = true, want false")
	}
}

func TestCLanguageLabelsCCAndHeaders(t *testing.T) {
	cases := map[string]string{
		"a.cc":  "cpp",
		"a.cxx": "cpp",
		"a.hh":  "cpp",
		"a.hxx": "cpp",
		"a.cpp": "cpp",
		"a.c":   "c",
	}
	for path, want := range cases {
		if got := cLanguage(path, nil); got != want {
			t.Errorf("cLanguage(%q) = %q, want %q", path, got, want)
		}
	}
	if got := cLanguage("a.h", []byte("class A {};\n")); got != "cpp" {
		t.Errorf("cLanguage(cpp header) = %q, want cpp", got)
	}
	if got := cLanguage("a.h", []byte("int f(void);\n")); got != "c" {
		t.Errorf("cLanguage(c header) = %q, want c", got)
	}
}
