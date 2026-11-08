package cli

import "testing"

func TestResolveVersionLdflags(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "v1.2.3"
	if got := resolveVersion(); got != "1.2.3" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveVersionFallbackNotEmpty(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "dev"
	if resolveVersion() == "" {
		t.Fatal("empty version")
	}
}
