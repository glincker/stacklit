package parser

import (
	"os"
	"strings"
	"testing"
)

// langCase describes the facts a fixture must yield. Exports match by substring
// because signatures carry parameter text; imports and type keys match exactly.
type langCase struct {
	name        string
	file        string
	language    string
	imports     []string
	exports     []string
	notExports  []string
	typeDefs    map[string]string // type name -> substring of its summary
	noTypeNames []string
}

func TestMoreLanguages(t *testing.T) {
	cases := []langCase{
		{
			name: "elixir", file: "sample.ex", language: "elixir",
			imports:    []string{"GenServer", "Ecto.Query", "MyApp.Repo", "MyApp.Accounts.User", "MyApp.Accounts.Role", "Logger"},
			exports:    []string{"MyApp.Accounts", "get_user(id)", "create_user(attrs", "log_it(msg)", "MyApp.Printable"},
			notExports: []string{"secret"},
			typeDefs:   map[string]string{"MyApp.Accounts": "defstruct", "MyApp.Printable": "def print(data)"},
		},
		{
			name: "zig", file: "sample.zig", language: "zig",
			imports:    []string{"std", "config.zig"},
			exports:    []string{"Point", "Color", "Shape", "fn add(a: i32, b: i32) i32", "fn main()"},
			notExports: []string{"privateHelper"},
			typeDefs:   map[string]string{"Point": "fn len(self: Point) f32"},
		},
		{
			name: "dart", file: "sample.dart", language: "dart",
			imports:    []string{"dart:async", "package:flutter/material.dart", "src/utils.dart", "src/models.dart"},
			exports:    []string{"Shape", "Circle", "Printable", "Status", "StringExt", "String greet(String name)", "void main()"},
			notExports: []string{"_private"},
			typeDefs:   map[string]string{"Circle": "double area()", "Status": "active"},
		},
		{
			name: "kotlin", file: "sample.kt", language: "kotlin",
			imports:    []string{"kotlin.collections.List", "com.example.util.*"},
			exports:    []string{"Repository", "User", "Service", "Registry", "Level", "fun topLevel(a: Int, b: String): Boolean"},
			notExports: []string{"Internal", "secret", "hidden"},
			typeDefs:   map[string]string{"Registry": "fun register", "Service": "fun load(id: Int): User?", "Level": "HIGH", "User": "val id: Int"},
		},
		{
			name: "lua", file: "sample.lua", language: "lua",
			imports:    []string{"cjson", "app.utils"},
			exports:    []string{"M.add(a, b)", "M:greet(name)", "global_fn(a)", "M.sub(a, b)"},
			notExports: []string{"helper"},
		},
		{
			name: "scala", file: "sample.scala", language: "scala",
			imports:    []string{"scala.collection.mutable", "java.util.List", "java.util.Map", "cats.effect._"},
			exports:    []string{"Greeter", "Person", "Point", "Main", "Color", "Alias"},
			notExports: []string{"secret"},
			typeDefs:   map[string]string{"Person": "def greet(name: String): String", "Main": "val answer", "Color": "Red"},
		},
		{
			name: "shell", file: "sample.sh", language: "shell",
			imports:    []string{"./lib/common.sh", "./lib/other.sh"},
			exports:    []string{"build()", "deploy()"},
			notExports: []string{"_private_helper"},
		},
		{
			name: "sql", file: "sample.sql", language: "sql",
			exports:  []string{"table users", "table public.orders", "view active_users", "function add_one"},
			typeDefs: map[string]string{"users": "email VARCHAR(255)", "public.orders": "user_id INT"},
		},
		{
			name: "gdscript", file: "sample.gd", language: "gdscript",
			exports:    []string{"Player", "signal died", "SPEED", "func take_damage(amount: int)", "Player.Inner"},
			notExports: []string{"_ready", "_private_like"},
			typeDefs:   map[string]string{"Player": "extends CharacterBody2D", "Player.Inner": "func foo()"},
		},
		{
			name: "vue", file: "sample.vue", language: "vue",
			imports: []string{"vue", "./Child.vue"},
			exports: []string{"Props", "helper"},
		},
		{
			name: "svelte", file: "sample.svelte", language: "svelte",
			imports: []string{"svelte", "./Child.svelte"},
			exports: []string{"name", "reset", "Counter"},
		},
		{
			name: "terraform", file: "sample.tf", language: "hcl",
			imports:  []string{"./modules/vpc"},
			exports:  []string{"aws_s3_bucket.logs", "data.aws_ami.ubuntu", "var.region", "output.bucket_name", "module.vpc", "provider.aws"},
			typeDefs: map[string]string{"aws_s3_bucket.logs": "bucket", "var.region": "default"},
		},
		{
			name: "protobuf", file: "sample.proto", language: "protobuf",
			imports:  []string{"google/protobuf/timestamp.proto", "other.proto"},
			exports:  []string{"User", "User.Address", "Role", "UserService"},
			typeDefs: map[string]string{"UserService": "rpc GetUser(GetUserRequest) returns (User)", "Role": "ADMIN"},
		},
	}

	p := &TreeSitterParser{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := "../../testdata/langs/" + tc.file
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if !p.CanParse(path) {
				t.Fatalf("CanParse(%s) = false", tc.file)
			}
			info, err := p.Parse(path, content)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if info.Language != tc.language {
				t.Errorf("Language = %q, want %q", info.Language, tc.language)
			}
			if info.LineCount == 0 {
				t.Error("LineCount = 0")
			}
			for _, want := range tc.imports {
				if !containsExact(info.Imports, want) {
					t.Errorf("import %q missing from %q", want, info.Imports)
				}
			}
			for _, want := range tc.exports {
				if !containsSub(info.Exports, want) {
					t.Errorf("export %q missing from %q", want, info.Exports)
				}
			}
			for _, bad := range tc.notExports {
				if containsSub(info.Exports, bad) {
					t.Errorf("private symbol %q leaked into exports %q", bad, info.Exports)
				}
			}
			for name, sub := range tc.typeDefs {
				got, ok := info.TypeDefs[name]
				if !ok {
					t.Errorf("type %q missing from %v", name, info.TypeDefs)
					continue
				}
				if !strings.Contains(got, sub) {
					t.Errorf("type %q = %q, want substring %q", name, got, sub)
				}
			}
		})
	}
}

func TestMoreLanguagesDetectedByExtension(t *testing.T) {
	p := &TreeSitterParser{}
	for _, f := range []string{"a.ex", "a.exs", "a.zig", "a.dart", "a.kt", "a.lua", "a.scala", "a.sh", "a.sql", "a.gd", "a.vue", "a.svelte", "main.tf", "a.hcl", "a.proto"} {
		if !p.CanParse(f) {
			t.Errorf("CanParse(%q) = false", f)
		}
	}
}

func containsExact(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func containsSub(list []string, want string) bool {
	for _, s := range list {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}
