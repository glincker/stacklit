package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIngestAll(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/adr/0001-use-go.md", "# Use Go\n\n## Status\n\nAccepted\n\n## Context\nx\n")
	write(t, root, "docs/rfc/rfc-2.md", "# RFC 2\n\nStatus: Draft\n")
	write(t, root, "node_modules/x/docs/a.md", "# skip me\n")
	write(t, root, "api/openapi.yaml", "openapi: 3.0.0\ninfo:\n  title: Pets\n  version: 1\npaths:\n  /pets:\n    get:\n      operationId: listPets\n      summary: List pets\n    post:\n      operationId: addPet\n  /pets/{id}:\n    get:\n      summary: One pet\n")
	write(t, root, "swagger.json", `{"info":{"title":"J"},"paths":{"/a":{"get":{"operationId":"getA"},"parameters":[]}}}`)
	write(t, root, "db/schema.sql", "CREATE TABLE IF NOT EXISTS users (\n id int primary key,\n email varchar(255) not null,\n created numeric(10,2),\n PRIMARY KEY (id),\n CONSTRAINT u UNIQUE (email)\n);\n")
	write(t, root, "prisma/schema.prisma", "model Post {\n  id Int @id\n  title String\n  @@index([title])\n}\n")
	write(t, root, ".github/CODEOWNERS", "* @org/all\n/internal/auth/ @alice\n")

	ctx, err := Run(Options{Root: root, Docs: true, API: true, Schema: true, Owners: true, Modules: []string{"internal/auth", "internal/cli"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Docs) != 2 {
		t.Fatalf("docs: %+v", ctx.Docs)
	}
	adr := ctx.Docs[0]
	if adr.Kind != "adr" || adr.Title != "Use Go" || adr.Status != "Accepted" {
		t.Fatalf("adr: %+v", adr)
	}
	if ctx.Docs[1].Status != "Draft" || ctx.Docs[1].Kind != "rfc" {
		t.Fatalf("rfc: %+v", ctx.Docs[1])
	}
	if len(ctx.APIs) != 2 {
		t.Fatalf("apis: %+v", ctx.APIs)
	}
	var yamlAPI API
	for _, a := range ctx.APIs {
		if a.File == "api/openapi.yaml" {
			yamlAPI = a
		}
	}
	if yamlAPI.Title != "Pets" || len(yamlAPI.Operations) != 3 || yamlAPI.Operations[0].ID != "listPets" {
		t.Fatalf("yaml api: %+v", yamlAPI)
	}
	if len(ctx.Tables) != 2 {
		t.Fatalf("tables: %+v", ctx.Tables)
	}
	for _, tb := range ctx.Tables {
		if tb.Name == "users" && len(tb.Columns) != 3 {
			t.Fatalf("users cols: %v", tb.Columns)
		}
	}
	if ctx.Owners == nil || len(ctx.Owners.Modules) != 2 {
		t.Fatalf("owners: %+v", ctx.Owners)
	}
	if o := ctx.Owners.Modules[0]; o.Module != "internal/auth" || o.Codeowner[0] != "@alice" {
		t.Fatalf("auth owner: %+v", o)
	}
	if o := ctx.Owners.Modules[1]; o.Codeowner[0] != "@org/all" {
		t.Fatalf("cli owner: %+v", o)
	}
}
