package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func rubyMimeTable(n int) string {
	var b strings.Builder
	b.WriteString("module Rack\n  module Mime\n    def mime_type(ext)\n      MIME_TYPES[ext]\n    end\n\n    MIME_TYPES = {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "      \".e%d\" => \"application/x-%d\",\n", i, i)
	}
	b.WriteString("    }\n    def after; end\n  end\nend\n")
	return b.String()
}

func TestRubyLargeHashLiteralKeepsSymbolsAndIsFast(t *testing.T) {
	src := rubyMimeTable(900)
	start := time.Now()
	info, err := ParseContent("lib/rack/mime.rb", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("parse took %v, expected the data table to be skipped", d)
	}
	defs := info.TypeDefs["Rack::Mime"]
	for _, want := range []string{"mime_type", "after"} {
		if !strings.Contains(defs, want) {
			t.Errorf("Rack::Mime methods %q missing %q (exports %v)", defs, want, info.Exports)
		}
	}
}

func TestMaskRubyDataRunsKeepsLayout(t *testing.T) {
	t.Setenv("STACKLIT_DATA_RUN_LINES", "5")
	src := []byte(rubyMimeTable(20))
	out := maskRubyDataRuns(src)
	if len(out) != len(src) || strings.Count(string(out), "\n") != strings.Count(string(src), "\n") {
		t.Fatal("masking changed length or line count")
	}
	if strings.Contains(string(out), "application/x-3") {
		t.Error("data run was not masked")
	}
	t.Setenv("STACKLIT_DATA_RUN_LINES", "0")
	if string(maskRubyDataRuns(src)) != string(src) {
		t.Error("limit 0 should disable masking")
	}
}
