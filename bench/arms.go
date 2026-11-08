package main

import "strings"

// ArmResult is one arm's outcome on one task. Localization tasks use Ranked
// and FirstGold; dependents tasks use Modules, Lenient and Strict.
type ArmResult struct {
	Arm       string
	NA        bool     // the arm has no sensible procedure for this task kind
	CostOnly  bool     // only the token cost is meaningful (full dump)
	Lookup    Cost     // everything the arm emits before any file is opened
	ToFound   Cost     // lookup plus reading ranked files until the gold file is read
	Ranked    []string // files in the order the arm presents them
	Shown     []string // files visible in the printed output; Ranked when nil
	GoldShown bool     // a gold file is visible in the printed output
	FirstGold int      // 1-based rank of the first gold file, 0 when absent
	Modules   []string // dependents: modules the arm reports
	Lenient   bool
	Strict    bool
	Extra     int // dependents: reported modules that are not expected
}

// TaskResult holds every arm's result for one task.
type TaskResult struct {
	Task Task
	Arms map[string]ArmResult
}

// RepoSizes maps every code file of a repo (tests included) to its cost.
type RepoSizes map[string]Cost

func isGold(rel string, task Task) bool {
	for _, e := range task.ExpectPaths {
		if pathMatches(rel, e) {
			return true
		}
	}
	return false
}

// finishLocalization fills FirstGold and ToFound from the ranked list. The
// reader opens ranked files one by one and stops after the first gold file or
// after readCap files; a miss is charged for all reads it made.
func finishLocalization(a *ArmResult, task Task, sizes RepoSizes, readCap int) {
	shown := a.Shown
	if shown == nil {
		shown = a.Ranked
	}
	a.GoldShown = false
	for _, f := range shown {
		if isGold(f, task) {
			a.GoldShown = true
			break
		}
	}
	a.FirstGold = 0
	for i, f := range a.Ranked {
		if isGold(f, task) {
			a.FirstGold = i + 1
			break
		}
	}
	reads := readCap
	if a.FirstGold > 0 && a.FirstGold <= readCap {
		reads = a.FirstGold
	}
	if reads > len(a.Ranked) {
		reads = len(a.Ranked)
	}
	a.ToFound = a.Lookup
	for _, f := range a.Ranked[:reads] {
		a.ToFound = a.ToFound.Add(sizes[f])
	}
}

// Found reports whether the first gold file is within the top k.
func (a ArmResult) Found(k int) bool { return a.FirstGold > 0 && a.FirstGold <= k }

// depMatch compares a reported module with an expected one. A trailing slash
// on the expectation means "anything under it". Loose mode also accepts the
// short trailing-segment names that the stacklit index uses.
func depMatch(found, expected string, loose bool) bool {
	if found == expected {
		return true
	}
	if strings.HasSuffix(expected, "/") && strings.HasPrefix(found, expected) {
		return true
	}
	return loose && strings.HasSuffix(expected, "/"+found)
}

// gradeDependents applies the two documented rules. Lenient: every expected
// module is reported (loose names allowed, extras ignored). Strict: every
// expected module is reported by exact name and at most maxExtra reported
// modules are unexpected.
func gradeDependents(a *ArmResult, task Task, maxExtra int) {
	cover := func(loose bool) bool {
		for _, e := range task.ExpectModules {
			ok := false
			for _, f := range a.Modules {
				if depMatch(f, e, loose) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		}
		return len(task.ExpectModules) > 0
	}
	extra := 0
	for _, f := range a.Modules {
		hit := false
		for _, e := range task.ExpectModules {
			if depMatch(f, e, false) {
				hit = true
				break
			}
		}
		if !hit {
			extra++
		}
	}
	a.Extra = extra
	a.Lenient = cover(true)
	a.Strict = cover(false) && extra <= maxExtra
}
