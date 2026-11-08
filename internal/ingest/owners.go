package ingest

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ignore "github.com/sabhiram/go-gitignore"
)

// OwnerRule is one CODEOWNERS line.
type OwnerRule struct {
	Pattern string   `json:"pattern"`
	Owners  []string `json:"owners"`
}

// ModuleOwners maps a module to owners from CODEOWNERS and/or recent authors.
type ModuleOwners struct {
	Module    string   `json:"module"`
	Codeowner []string `json:"codeowners,omitempty"`
	Authors   []string `json:"top_authors,omitempty"`
}

// Owners is the ownership layer.
type Owners struct {
	Rules   []OwnerRule    `json:"rules,omitempty"`
	Modules []ModuleOwners `json:"modules,omitempty"`
}

func collectOwners(o Options) *Owners {
	out := &Owners{}
	var matchers []*ignore.GitIgnore
	if o.Owners {
		out.Rules = readCodeowners(o.Root)
		for _, r := range out.Rules {
			matchers = append(matchers, ignore.CompileIgnoreLines(r.Pattern))
		}
	}
	var authors map[string]map[string]int
	if o.GitOwners {
		authors = gitAuthors(o.Root, o.GitDays, o.Modules)
	}
	mods := append([]string(nil), o.Modules...)
	sort.Strings(mods)
	for _, m := range mods {
		mo := ModuleOwners{Module: m}
		// CODEOWNERS: last matching rule wins, probed with a synthetic file in the module.
		probe := strings.TrimSuffix(m, "/") + "/x"
		for i := len(matchers) - 1; i >= 0; i-- {
			if matchers[i].MatchesPath(probe) {
				mo.Codeowner = out.Rules[i].Owners
				break
			}
		}
		mo.Authors = topN(authors[m], 3)
		if len(mo.Codeowner) > 0 || len(mo.Authors) > 0 {
			out.Modules = append(out.Modules, mo)
		}
	}
	if len(out.Rules) == 0 && len(out.Modules) == 0 {
		return nil
	}
	return out
}

func readCodeowners(root string) []OwnerRule {
	for _, p := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		body, ok := readCapped(root, p)
		if !ok {
			continue
		}
		var rules []OwnerRule
		sc := bufio.NewScanner(strings.NewReader(body))
		for sc.Scan() {
			l := strings.TrimSpace(sc.Text())
			if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "[") {
				continue
			}
			f := strings.Fields(l)
			if len(f) >= 2 {
				rules = append(rules, OwnerRule{Pattern: f[0], Owners: f[1:]})
			}
		}
		return rules
	}
	return nil
}

// gitAuthors counts commit authors per module over the last days. Names only, no emails.
func gitAuthors(root string, days int, modules []string) map[string]map[string]int {
	if days <= 0 {
		days = int(envInt("STACKLIT_OWNERS_DAYS", 180))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "log", fmt.Sprintf("--since=%d.days.ago", days), "--pretty=format:@%aN", "--name-only")
	b, err := cmd.Output()
	if err != nil {
		return nil
	}
	res := map[string]map[string]int{}
	var author string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "@") {
			author = l[1:]
			continue
		}
		l = filepath.ToSlash(strings.TrimSpace(l))
		if l == "" || author == "" {
			continue
		}
		best := ""
		for _, m := range modules {
			mm := strings.TrimSuffix(m, "/")
			if (l == mm || strings.HasPrefix(l, mm+"/")) && len(mm) > len(best) {
				best = m
			}
		}
		if best == "" {
			continue
		}
		if res[best] == nil {
			res[best] = map[string]int{}
		}
		res[best][author]++
	}
	return res
}

func topN(m map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool {
		if s[i].v != s[j].v {
			return s[i].v > s[j].v
		}
		return s[i].k < s[j].k
	})
	var out []string
	for i := 0; i < len(s) && i < n; i++ {
		out = append(out, s[i].k)
	}
	return out
}
