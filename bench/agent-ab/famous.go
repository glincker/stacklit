package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Thresholds of the famous repos study, fixed in PREREGISTRATION-famous.md
// before any session ran. They are constants here so the report cannot drift
// from the document.
const (
	famousMemoryMax        = 0.20 // a repo is contaminated above this share of memory-correct tasks
	famousCostUpper        = 1.00 // a repo shows savings only if the cost ratio's upper bound is below this
	famousHarmLower        = 1.00 // a repo shows harm if the cost ratio's lower bound is above this
	famousRepoCorrectLower = -0.10
	famousPoolCorrectLower = -0.05
	famousMinRepos         = 3    // repos that must show savings before a general claim is allowed
	famousLargeFiles       = 5000 // tracked files above which a repo counts as large
)

// famousArms are the treatment arms of the study, each compared with control.
var famousArms = []string{"stacklit", "stacklit-e"}

// applyFamousPreset fills every setting the user did not pass explicitly with
// the pre-registered value, so `-repo famous` reproduces the study.
func applyFamousPreset(c *config, set map[string]bool) {
	c.famous = true
	str := func(flagName string, dst *string, v string) {
		if !set[flagName] {
			*dst = v
		}
	}
	str("repos", &c.reposFile, "bench/agent-ab/repos-famous.json")
	str("tasks", &c.tasksDir, "bench/agent-ab/tasks-famous")
	str("results", &c.resultsDir, "bench/agent-ab/famous/results")
	str("ledger", &c.ledger, "bench/agent-ab/famous/ledger.jsonl")
	str("arms", &c.arms, "control,stacklit,stacklit-e")
	str("model", &c.model, "claude-sonnet-5-5")
	str("small-repos", &c.smallRepos, "")
	if c.mode == "memory" && !set["results"] {
		c.resultsDir = filepath.Join(c.resultsDir, "memory")
	}
	if !set["trials"] {
		c.trials = 3
	}
	if !set["seed"] {
		c.seed = 20261109
	}
	if !set["max-budget-usd"] {
		c.maxBudget = 0.25
	}
	if !set["cap-usd"] {
		c.cap = 20
	}
	c.excludeMem = !c.includeMem
	if !set["reuse-index"] {
		c.reuseIndex = true
	}
}

// memoryCorrectTasks lists the tasks the model answered correctly with every
// tool disabled. resultsDir is the study's results directory.
func memoryCorrectTasks(resultsDir, repoName string, tasks map[string]task) map[string]bool {
	out := map[string]bool{}
	for _, r := range loadRecords(filepath.Join(resultsDir, "memory"), repoName, "memory") {
		if gradeAudited(r.Answer, tasks[r.Task]) {
			out[r.Task] = true
		}
	}
	return out
}

// memoryRate is the share of memory-checked tasks of one repo answered
// correctly, with the number checked.
func memoryRate(resultsDir, repoName string, tasks map[string]task) (k, n int) {
	for _, r := range loadRecords(filepath.Join(resultsDir, "memory"), repoName, "memory") {
		n++
		if gradeAudited(r.Answer, tasks[r.Task]) {
			k++
		}
	}
	return k, n
}

// contaminated reports whether a repo's memory check exceeds the limit. A repo
// without a memory check is not eligible: the study cannot claim anything
// about it.
func contaminated(k, n int) bool {
	return n == 0 || float64(k)/float64(n) > famousMemoryMax
}

type famousRow struct {
	repo, arm                       string
	files, memK, memN, excluded     int
	cost, corr                      *cmp
	savings, harm                   bool
	eligible                        bool
	costObs, corrObs, status, claim string
}

func famousRows(s summary, names []string, files map[string]int, excluded map[string]int) []famousRow {
	var rows []famousRow
	for _, arm := range famousArms {
		for _, n := range names {
			row := famousRow{repo: n, arm: arm, files: files[n], excluded: excluded[n]}
			if m, ok := s.Memorised[n]; ok {
				row.memK, row.memN = m.K, m.N
			}
			row.eligible = !contaminated(row.memK, row.memN)
			sc := findScope(s, n, arm, "control")
			if sc != nil {
				row.cost, row.corr = findCmp(sc.Compare, "cost_usd"), findCmp(sc.Compare, "correct_audited")
			}
			switch {
			case sc == nil || row.cost == nil || row.corr == nil:
				row.status, row.costObs, row.corrObs = "no data", "no data", "no data"
			default:
				row.costObs = fmt.Sprintf("%.2f [%.2f, %.2f]", row.cost.Ratio, row.cost.RatioCI.Lo, row.cost.RatioCI.Hi)
				row.corrObs = fmt.Sprintf("%+.1f points [%+.1f, %+.1f]", 100*row.corr.Diff, 100*row.corr.DiffCI.Lo, 100*row.corr.DiffCI.Hi)
				row.savings = row.eligible && row.cost.RatioCI.Hi < famousCostUpper && row.corr.DiffCI.Lo > famousRepoCorrectLower
				row.harm = row.eligible && row.cost.RatioCI.Lo > famousHarmLower
				row.status = "ok"
			}
			if !row.eligible {
				row.status = "contaminated or unchecked: excluded from every claim"
			}
			switch {
			case row.savings:
				row.claim = "lower cost on this repo"
			case row.harm:
				row.claim = "higher cost on this repo"
			case row.status == "ok":
				row.claim = "no claim (interval includes 1.00 or correctness not shown)"
			default:
				row.claim = "no claim"
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// famousReport evaluates the pre-registered claim rules. names are the repos
// of the study, excluded counts the tasks dropped for memorisation per repo.
func famousReport(s summary, names []string, files map[string]int, excluded map[string]int) string {
	rows := famousRows(s, names, files, excluded)
	var b strings.Builder
	b.WriteString("# Famous repos decision report\n\nGenerated from summary.json by the rule in PREREGISTRATION-famous.md. Every repo is listed; none is dropped except by the memorisation rule, and then it is still shown.\n\n")
	for _, arm := range famousArms {
		fmt.Fprintf(&b, "## Arm %s against control\n\n| repo | tracked files | memory check | tasks dropped | cost ratio [95%% CI] | correctness diff [95%% CI] | status | what may be said |\n|---|---|---|---|---|---|---|---|\n", arm)
		saving, savingLarge := 0, 0
		eligible := 0
		var harmed []string
		for _, r := range rows {
			if r.arm != arm {
				continue
			}
			fmt.Fprintf(&b, "| %s | %d | %d of %d | %d | %s | %s | %s | %s |\n", r.repo, r.files, r.memK, r.memN, r.excluded, r.costObs, r.corrObs, r.status, r.claim)
			if r.eligible {
				eligible++
			}
			if r.savings {
				saving++
				if r.files > famousLargeFiles {
					savingLarge++
				}
			}
			if r.harm {
				harmed = append(harmed, r.repo)
			}
		}
		pooled := findScope(s, "pooled", arm, "control")
		poolCost, poolCorr := "no data", "no data"
		general, noLoss := false, false
		if pooled != nil {
			if c := findCmp(pooled.Compare, "cost_usd"); c != nil {
				poolCost = fmt.Sprintf("%.2f [%.2f, %.2f]", c.Ratio, c.RatioCI.Lo, c.RatioCI.Hi)
				general = saving >= famousMinRepos && c.RatioCI.Hi < famousCostUpper
			}
			if c := findCmp(pooled.Compare, "correct_audited"); c != nil {
				poolCorr = fmt.Sprintf("%+.1f points [%+.1f, %+.1f]", 100*c.Diff, 100*c.DiffCI.Lo, 100*c.DiffCI.Hi)
				noLoss = c.DiffCI.Lo > famousPoolCorrectLower
			}
		}
		fmt.Fprintf(&b, "\nPooled over the %d eligible repos: cost ratio %s; correctness difference %s.\n\n", eligible, poolCost, poolCorr)
		sort.Strings(harmed)
		fmt.Fprintf(&b, "- Repos showing lower cost with correctness shown (rule R1): %d of %d eligible.\n", saving, eligible)
		fmt.Fprintf(&b, "- General claim \"lower cost on famous repos\" (rule R2: at least %d repos and pooled upper bound below %.2f): %s.\n", famousMinRepos, famousCostUpper, yesNo(general))
		fmt.Fprintf(&b, "- Wording \"on large famous repos\" (rule R2 holds and at least %d of the repos with savings have more than %d tracked files; %d do): %s.\n", famousMinRepos, famousLargeFiles, savingLarge, yesNo(general && savingLarge >= famousMinRepos))
		fmt.Fprintf(&b, "- Claim \"no loss of correctness\" (rule R3: pooled lower bound above %+.0f points): %s.\n", 100*famousPoolCorrectLower, yesNo(noLoss))
		fmt.Fprintf(&b, "- Repos with higher cost (rule R4, to be reported as harm): %s.\n\n", listOrNone(harmed))
	}
	b.WriteString("Anything not marked as allowed above is not claimed. Null and negative results are published in the same format.\n")
	return b.String()
}

func yesNo(v bool) string {
	if v {
		return "ALLOWED"
	}
	return "not allowed"
}

func listOrNone(l []string) string {
	if len(l) == 0 {
		return "none"
	}
	return strings.Join(l, ", ")
}

// saveGenerated copies the files a command created in a git checkout (its
// untracked files) to dst, keeping their relative paths.
func saveGenerated(dir, dst string) error {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all", "-z")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range strings.Split(string(out), "\x00") {
		if !strings.HasPrefix(e, "?? ") {
			continue
		}
		rel := strings.TrimPrefix(e, "?? ")
		if err := copyFile(filepath.Join(dir, rel), filepath.Join(dst, rel)); err != nil {
			return err
		}
	}
	return nil
}

// copyTree copies every file under src into dst, keeping relative paths.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		return copyFile(p, filepath.Join(dst, rel))
	})
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}
