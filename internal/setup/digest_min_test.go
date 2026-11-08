package setup

import (
	"strings"
	"testing"
)

func TestDigestPayloadIsMinimalAndStable(t *testing.T) {
	idx := sessIdx()
	a, b := digestPayload(idx), digestPayload(idx)
	if a != b {
		t.Fatal("digest payload is not byte stable")
	}
	if n := Tokens(a); n > digestBudget()+80 {
		t.Errorf("payload %d tokens, cap %d plus the tools line", n, digestBudget())
	}
	for _, want := range []string{"stacklit query", "Exact-name lookups and small files"} {
		if !strings.Contains(a, want) {
			t.Errorf("payload lacks %q", want)
		}
	}
	if !strings.Contains(BodyFor(idx, DefaultSteering), "exact name or a small file") {
		t.Error("default steering must say when not to use stacklit")
	}
}
