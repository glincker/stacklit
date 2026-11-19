package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractPathClassifiesHeaders(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Plain C header with no C++ neighbour.
	write("plain.h", "int process(const char *msg);\n")
	if fs := ExtractPath(dir, "plain.h", 0); fs == nil || fs.Lang != "c" {
		t.Fatalf("plain.h = %+v, want Lang c", fs)
	}

	// Same header once a C++ translation unit sits beside it.
	write("plain.cc", "int process(const char *msg) { return 0; }\n")
	if fs := ExtractPath(dir, "plain.h", 0); fs == nil || fs.Lang != "cpp" {
		t.Fatalf("plain.h beside .cc = %+v, want Lang cpp", fs)
	}

	// A header that is C++ on its own text.
	write("acquirer.h", "class Acquirer {\npublic:\n  void run();\n};\n")
	if fs := ExtractPath(dir, "acquirer.h", 0); fs == nil || fs.Lang != "cpp" {
		t.Fatalf("acquirer.h = %+v, want Lang cpp", fs)
	}

	// A .cc file itself is C++.
	if fs := ExtractPath(dir, "plain.cc", 0); fs == nil || fs.Lang != "cpp" {
		t.Fatalf("plain.cc = %+v, want Lang cpp", fs)
	}
}

func TestExtractFileWithoutDiskPathUsesContent(t *testing.T) {
	fs := extractFileWith("hdr.h", []byte("namespace n { struct S; }\n"), ExtractOptions{}, nil)
	if fs == nil || fs.Lang != "cpp" {
		t.Fatalf("header with C++ text = %+v, want Lang cpp", fs)
	}
	fs = extractFileWith("hdr.h", []byte("int f(void);\n"), ExtractOptions{}, nil)
	if fs == nil || fs.Lang != "c" {
		t.Fatalf("header with C text = %+v, want Lang c", fs)
	}
}
