package parser

import (
	"errors"
	"testing"
)

var fuzzExts = []string{".go", ".ts", ".tsx", ".js", ".py", ".rs", ".java", ".cs", ".rb", ".php", ".swift", ".kt", ".c", ".cpp", ".sh", ".sql", ".txt"}

// FuzzParseContent feeds arbitrary bytes to every parser entrypoint. Parsers
// may return errors but must never panic or return a nil info without one.
func FuzzParseContent(f *testing.F) {
	seeds := []string{
		"package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(1) }\n",
		"import { a } from './a'\nexport const b = 1\nexport default class X {}\n",
		"import os\n\ndef f(x):\n    return x\n\nclass C: pass\n",
		"use std::fmt;\npub fn f() {}\npub struct S;\n",
		"public class A { public static void main(String[] a) {} }\n",
		"",
		"\x00\xff\xfe{{{((",
		"func (",
		"import (",
	}
	for i, s := range seeds {
		f.Add(uint8(i%len(fuzzExts)), []byte(s))
	}
	f.Fuzz(func(t *testing.T, extIdx uint8, content []byte) {
		path := "dir/file" + fuzzExts[int(extIdx)%len(fuzzExts)]
		info, err := ParseContent(path, content)
		var pe *PanicError
		if errors.As(err, &pe) {
			t.Fatalf("parser panicked: %v\n%s", pe.Value, pe.Stack)
		}
		if err == nil && info == nil {
			t.Fatal("nil info without error")
		}
		if info != nil && info.LineCount < 0 {
			t.Fatalf("negative line count %d", info.LineCount)
		}
	})
}
