package check

import (
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/schema"
)

func idx() *schema.Index {
	return &schema.Index{
		Modules: map[string]schema.ModuleInfo{
			"ui/web":    {DependsOn: []string{"core/db", "core/auth"}},
			"core/db":   {DependsOn: []string{"core/auth"}},
			"core/auth": {DependsOn: []string{"core/db"}},
			"lonely":    {},
		},
		Dependencies: schema.Dependencies{Edges: [][2]string{{"ui/web", "core/db"}}},
	}
}

func rules(r *Result) map[string]int {
	m := map[string]int{}
	for _, v := range r.Violations {
		m[v.Rule]++
	}
	return m
}

func TestForbiddenAndGlob(t *testing.T) {
	r := Run(idx(), &config.CheckConfig{Forbidden: []config.ForbiddenEdge{{From: "ui/**", To: "core/db", Reason: "go through auth"}}})
	if r.OK || rules(r)[RuleForbidden] != 1 || !strings.Contains(r.Violations[0].Message, "go through auth") {
		t.Fatalf("%+v", r)
	}
}

func TestCycleDetected(t *testing.T) {
	r := Run(idx(), &config.CheckConfig{Cycles: true})
	if rules(r)[RuleCycle] != 1 || strings.Join(r.Violations[0].Modules, ",") != "core/auth,core/db" {
		t.Fatalf("%+v", r)
	}
}

func TestFanAndOrphans(t *testing.T) {
	r := Run(idx(), &config.CheckConfig{MaxFanIn: 1, MaxFanOut: 1, Orphans: true})
	got := rules(r)
	if got[RuleFanIn] != 2 || got[RuleFanOut] != 1 || got[RuleOrphan] != 1 {
		t.Fatalf("%v", got)
	}
}

func TestNoConfigIsClean(t *testing.T) {
	if r := Run(idx(), nil); !r.OK || len(r.Violations) != 0 {
		t.Fatal("expected clean")
	}
	if r := Run(idx(), &config.CheckConfig{}); !r.OK {
		t.Fatal("empty rules must pass")
	}
}
