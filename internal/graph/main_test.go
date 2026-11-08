package graph

import (
	"os"
	"testing"
)

// Slow CI runners can take longer than the default parse deadline on a cold
// grammar, which would turn a fixture into a file-only node and fail the test.
func TestMain(m *testing.M) {
	if os.Getenv("STACKLIT_GRAPH_PARSE_TIMEOUT_MS") == "" {
		os.Setenv("STACKLIT_GRAPH_PARSE_TIMEOUT_MS", "60000")
	}
	os.Exit(m.Run())
}
