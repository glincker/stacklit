package parser

import (
	"os"
	"strings"
	"testing"
)

func TestRustParserParse_Imports(t *testing.T) {
	p := &TreeSitterParser{}

	content, err := os.ReadFile("../../testdata/rust-project/src/main.rs")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	info, err := p.Parse("../../testdata/rust-project/src/main.rs", content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if info.Language != "rust" {
		t.Errorf("Language = %q, want %q", info.Language, "rust")
	}

	wantImports := []string{"std::collections", "serde", "crate::config"}
	for _, want := range wantImports {
		found := false
		for _, got := range info.Imports {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("import %q not found in %v", want, info.Imports)
		}
	}
}

func TestRustParserParse_Exports(t *testing.T) {
	p := &TreeSitterParser{}

	content, err := os.ReadFile("../../testdata/rust-project/src/main.rs")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	info, err := p.Parse("../../testdata/rust-project/src/main.rs", content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Exports are now full signatures containing the name.
	wantExports := []string{"AppState", "create_app"}
	for _, want := range wantExports {
		found := false
		for _, got := range info.Exports {
			if strings.Contains(got, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("export %q not found in %v", want, info.Exports)
		}
	}

	// Private function should NOT be exported
	for _, got := range info.Exports {
		if strings.Contains(got, "internal_helper") {
			t.Errorf("private fn %q should not be in exports", got)
		}
	}
}

func TestRustParserParse_Entrypoint(t *testing.T) {
	p := &TreeSitterParser{}

	content, err := os.ReadFile("../../testdata/rust-project/src/main.rs")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	info, err := p.Parse("../../testdata/rust-project/src/main.rs", content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !info.IsEntrypoint {
		t.Error("IsEntrypoint = false, want true for file with fn main()")
	}
}

func TestRustParserParse_NoEntrypoint(t *testing.T) {
	p := &TreeSitterParser{}

	content := []byte(`pub fn helper() {}

pub struct Foo {
    x: i32,
}
`)

	info, err := p.Parse("lib.rs", content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if info.IsEntrypoint {
		t.Error("IsEntrypoint = true, want false for file without fn main()")
	}
}

func TestRustUseTargets(t *testing.T) {
	cases := map[string][]string{
		"std::collections::HashMap":        {"std::collections"},
		"crate::config":                    {"crate::config"},
		"crate::{flags, search::Searcher}": {"crate::flags", "crate::search"},
		"grep_matcher::{Matcher, Match}":   {"grep_matcher"},
		"super::{self, util}":              {"super::util"},
		"std::{ffi::OsString, io}":         {"std"},
		"serde":                            {"serde"},
		"{globset::{Glob, GlobSet}, regex_automata::util::pool::Pool}": {"globset", "regex_automata::util"},
	}
	for in, want := range cases {
		got := rustUseTargets(in)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("rustUseTargets(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRustPubUseImport(t *testing.T) {
	p := &TreeSitterParser{}
	info, err := p.Parse("lib.rs", []byte("pub use crate::engine::{Run, Stop};\npub(crate) use self::util::x;\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"crate::engine", "self::util"} {
		if !containsExact(info.Imports, want) {
			t.Errorf("imports %v missing %q", info.Imports, want)
		}
	}
}
