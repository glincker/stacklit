package setup

import (
	"encoding/json"
	"strings"
	"testing"
)

// noDashes reports en or em dashes, which the repo bans in all text.
func noDashes(s string) bool {
	return !strings.ContainsRune(s, 0x2013) && !strings.ContainsRune(s, 0x2014)
}

func TestSteeringVariantsStableAndBounded(t *testing.T) {
	idx := sessIdx()
	for _, v := range SteeringVariants() {
		a, b := BodyFor(idx, v), BodyFor(idx, v)
		if a != b {
			t.Errorf("variant %s not byte stable", v)
		}
		if tok := Tokens(a); tok > digestBudget() {
			t.Errorf("variant %s block %d tokens > budget %d", v, tok, digestBudget())
		}
		t.Logf("variant %s: rules %d tokens, whole block %d tokens", v, Tokens(steeringText(v)), Tokens(a))
	}
}

func TestStrictVariantIsImperativeAndComplete(t *testing.T) {
	d := steeringText("d")
	for _, want := range []string{
		"source of truth", "Do not crawl", `stacklit query "<question>" --budget 800`,
		"explain", "impact", "path", "stacklit slice <symbol>", "recursive listings", "find", "grep",
		"stacklit refresh --files <paths>", "stacklit generate", "Subagents",
	} {
		if !strings.Contains(d, want) {
			t.Errorf("variant d lacks %q", want)
		}
	}
	if !noDashes(d) {
		t.Error("dash character in rules")
	}
	for _, v := range []string{"a", "b", "c"} {
		if steeringText(v) == d {
			t.Errorf("variant %s collapsed into d", v)
		}
	}
	if DefaultSteering != "a" {
		t.Errorf("default steering changed to %s", DefaultSteering)
	}
}

func TestStrictBlockReachesEveryRulesTool(t *testing.T) {
	t.Setenv("STACKLIT_STEERING", "d")
	body := BodyFor(sessIdx(), SteeringFor(t.TempDir()))
	n := 0
	for _, tool := range Registry() {
		if len(tool.Rules) > 0 {
			n++
		}
	}
	if n < 20 || !strings.Contains(body, "Do not crawl") {
		t.Fatalf("strict wording missing (%d rules tools)", n)
	}
}

func TestGuideTextAndJSON(t *testing.T) {
	g := GuideText()
	if g != GuideText() {
		t.Fatal("guide not byte stable")
	}
	if !strings.Contains(g, strictRules) {
		t.Fatal("guide must embed the rules text")
	}
	for _, c := range GuideCommands() {
		if !strings.Contains(g, c.Command) {
			t.Errorf("guide lacks %q", c.Command)
		}
	}
	if !noDashes(g) {
		t.Fatal("dash character in guide")
	}
	var j struct {
		Rules    string         `json:"rules"`
		Commands []GuideCommand `json:"commands"`
		Text     string         `json:"text"`
	}
	if err := json.Unmarshal([]byte(GuideJSON()), &j); err != nil {
		t.Fatal(err)
	}
	if j.Rules != strictRules || j.Text != g || len(j.Commands) != len(GuideCommands()) {
		t.Fatal("json guide mismatch")
	}
	t.Logf("guide: %d tokens, rules text: %d tokens", Tokens(g), Tokens(strictRules))
}

func TestInjectedTextSizes(t *testing.T) {
	sizes := map[string]string{
		"nudge sample": crawlNudgeSample, "deny sample": crawlDenySample,
		"guide": GuideText(), "strict rules": strictRules,
	}
	for name, s := range sizes {
		t.Logf("%s: %d tokens (%d bytes)", name, Tokens(s), len(s))
		if !noDashes(s) {
			t.Errorf("%s has a dash character", name)
		}
	}
	limit := 120 // matches graph.DefaultPointerTokens, the cap for hook pointers
	if Tokens(crawlNudgeSample) > limit || Tokens(crawlDenySample) > limit {
		t.Error("hook text over the pointer cap")
	}
}
