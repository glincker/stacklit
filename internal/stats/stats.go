// Package stats keeps an opt-in, local-only usage log. Nothing here touches
// the network. It records bytes/4 token estimates, never file contents.
package stats

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glincker/stacklit/internal/config"
)

// Entry is one logged call.
type Entry struct {
	TS   string `json:"ts"`
	Cmd  string `json:"cmd"`
	Repo string `json:"repo,omitempty"` // base name only, never a full path
	// Served is the estimated tokens of the answer (bytes/4).
	Served int `json:"served"`
	// ReplacedEst estimates the tokens of the whole files the answer pointed
	// into. It is an estimate of what reading them would cost, not a
	// measurement of what the agent would have done.
	ReplacedEst int `json:"replaced_est"`
	Files       int `json:"files"`
}

// Enabled reports whether logging is on: STACKLIT_STATS=1 or "stats": true in
// .stacklitrc.json. Default off.
func Enabled(root string) bool {
	switch os.Getenv("STACKLIT_STATS") {
	case "1", "true":
		return true
	case "0", "false":
		return false
	}
	c := config.Load(root)
	return c.Stats != nil && *c.Stats
}

// LogPath is the JSONL file under the user cache dir.
func LogPath() string {
	dir := os.Getenv("STACKLIT_CACHE_DIR")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			base = os.TempDir()
		}
		dir = filepath.Join(base, "stacklit")
	}
	return filepath.Join(dir, "stats.jsonl")
}

// Tokens is the bytes/4 estimate used everywhere in stacklit.
func Tokens(s string) int { return (len(s) + 3) / 4 }

var pathRe = regexp.MustCompile(`[A-Za-z0-9_@./\\-]+\.[A-Za-z0-9]{1,8}`)

// maxFiles bounds the stat calls per record.
const maxFiles = 60

// ReplacedEstimate sums the sizes of existing files under root that text
// mentions by path, as tokens, and returns how many files matched.
func ReplacedEstimate(root, text string) (tokens, files int) {
	seen := map[string]bool{}
	for _, m := range pathRe.FindAllString(text, -1) {
		rel := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(m, "./")))
		if seen[rel] || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
			continue
		}
		seen[rel] = true
		if len(seen) > maxFiles {
			break
		}
		st, err := os.Stat(filepath.Join(root, rel))
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		tokens += int((st.Size() + 3) / 4)
		files++
	}
	return tokens, files
}

// Record appends one entry when logging is enabled. Errors are swallowed:
// statistics must never break a command.
func Record(root, cmd, output string) {
	if !Enabled(root) {
		return
	}
	rep, files := ReplacedEstimate(root, output)
	abs, _ := filepath.Abs(root)
	write(Entry{
		TS: time.Now().UTC().Format(time.RFC3339), Cmd: cmd, Repo: filepath.Base(abs),
		Served: Tokens(output), ReplacedEst: rep, Files: files,
	})
}

func write(e Entry) {
	p := LogPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
}

// CmdTotals aggregates one command.
type CmdTotals struct {
	Cmd         string `json:"cmd"`
	Calls       int    `json:"calls"`
	Served      int    `json:"served"`
	ReplacedEst int    `json:"replaced_est"`
}

// Summary is the report over a time window.
type Summary struct {
	Since       string      `json:"since,omitempty"`
	Calls       int         `json:"calls"`
	Served      int         `json:"served"`
	ReplacedEst int         `json:"replaced_est"`
	ByCmd       []CmdTotals `json:"by_cmd"`
}

// Read aggregates the log for entries at or after since (zero means all).
func Read(since time.Time) (Summary, error) {
	var s Summary
	f, err := os.Open(LogPath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	defer f.Close()
	if !since.IsZero() {
		s.Since = since.UTC().Format(time.RFC3339)
	}
	by := map[string]*CmdTotals{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, e.TS); err != nil || t.Before(since) {
			continue
		}
		c := by[e.Cmd]
		if c == nil {
			c = &CmdTotals{Cmd: e.Cmd}
			by[e.Cmd] = c
		}
		c.Calls++
		c.Served += e.Served
		c.ReplacedEst += e.ReplacedEst
		s.Calls++
		s.Served += e.Served
		s.ReplacedEst += e.ReplacedEst
	}
	for _, c := range by {
		s.ByCmd = append(s.ByCmd, *c)
	}
	sort.Slice(s.ByCmd, func(i, j int) bool { return s.ByCmd[i].Cmd < s.ByCmd[j].Cmd })
	return s, sc.Err()
}
