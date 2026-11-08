package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/fsutil"
	"github.com/glincker/stacklit/internal/schema"
)

// StructureHash covers module names, module edges and exported symbol names,
// nothing else. Line counts, timestamps, purposes and churn are excluded, so
// an ordinary edit leaves it unchanged and the injected digest stays byte
// identical (prompt-cache friendly).
func StructureHash(idx *schema.Index) string {
	h := sha256.New()
	names := make([]string, 0, len(idx.Modules))
	for n := range idx.Modules {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		h.Write([]byte("m " + n + "\n"))
		ex := append([]string(nil), idx.Modules[n].Exports...)
		sort.Strings(ex)
		for _, e := range ex {
			h.Write([]byte("x " + n + " " + e + "\n"))
		}
	}
	edges := make([]string, 0, len(idx.Dependencies.Edges))
	for _, e := range idx.Dependencies.Edges {
		edges = append(edges, e[0]+" -> "+e[1])
	}
	sort.Strings(edges)
	for _, e := range edges {
		h.Write([]byte("e " + e + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Injected records which structure hash each session was given.
type Injected struct {
	Latest   string            `json:"latest"`
	Sessions map[string]string `json:"sessions,omitempty"`
}

func injectedPath(dir string) string { return filepath.Join(dir, "injected.json") }

// maxSessions bounds the record so it cannot grow without limit.
const maxSessions = 64

// RecordInjected notes the hash handed to a session.
func RecordInjected(dir, sessionID, hash string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	rec := ReadInjected(dir)
	if rec.Sessions == nil || len(rec.Sessions) >= maxSessions {
		rec.Sessions = map[string]string{}
	}
	rec.Latest = hash
	if sessionID != "" {
		rec.Sessions[sessionID] = hash
	}
	if b, err := json.Marshal(rec); err == nil {
		_ = fsutil.WriteFileAtomic(injectedPath(dir), b, 0o644)
	}
}

// ReadInjected loads the record; a missing or damaged file reads as empty.
func ReadInjected(dir string) Injected {
	var rec Injected
	if b, err := os.ReadFile(injectedPath(dir)); err == nil {
		_ = json.Unmarshal(b, &rec)
	}
	return rec
}

// For returns the hash given to sessionID, or the latest one when id is empty
// or unknown.
func (r Injected) For(sessionID string) string {
	if h, ok := r.Sessions[sessionID]; ok && sessionID != "" {
		return h
	}
	return r.Latest
}

// LoadIndex reads stacklit.json under root.
func LoadIndex(root string) (*schema.Index, error) {
	b, err := os.ReadFile(filepath.Join(root, "stacklit.json"))
	if err != nil {
		return nil, err
	}
	var idx schema.Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// Short trims a commit id for display.
func Short(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
