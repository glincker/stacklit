package ingest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// ConfInferred tags everything an adapter produces. Stacklit cannot verify it.
const ConfInferred = "INFERRED"

// Concept is a node an adapter reported: an idea, entity or topic found in a
// file stacklit does not parse itself (PDF, image, audio, spreadsheet...).
type Concept struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind,omitempty"`
	Summary    string `json:"summary,omitempty"`
	SourceFile string `json:"source_file"`
	Conf       string `json:"confidence"`
	Source     string `json:"source"`
}

// ConceptLink is an edge an adapter reported. From and To name a concept id,
// a file path, or a symbol id from the graph.
type ConceptLink struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind,omitempty"`
	SourceFile string `json:"source_file,omitempty"`
	Conf       string `json:"confidence"`
	Source     string `json:"source"`
}

// AdapterOptions configures one adapter run.
type AdapterOptions struct {
	Root     string
	Command  string   // run through the platform shell
	Name     string   // value for the source field; defaults to the command's program name
	Files    []string // slash paths relative to Root, sent to the command on stdin
	Timeout  time.Duration
	MaxBytes int64
	MaxItems int
}

// AdapterResult is what an adapter returned after validation.
type AdapterResult struct {
	Name     string
	Concepts []Concept
	Links    []ConceptLink
	Skipped  int // lines that were malformed, unknown or over the item cap
}

func adapterTimeout() time.Duration {
	return time.Duration(envInt("STACKLIT_ADAPTER_TIMEOUT_SEC", 60)) * time.Second
}
func adapterMaxBytes() int64 { return envInt("STACKLIT_ADAPTER_MAX_BYTES", 4<<20) }
func adapterMaxItems() int   { return int(envInt("STACKLIT_ADAPTER_MAX_ITEMS", 5000)) }

// stderrKeep bounds how much adapter stderr is kept for error messages.
const stderrKeep = 2048

// adapterLine is the JSONL wire format.
type adapterLine struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Summary    string `json:"summary"`
	SourceFile string `json:"source_file"`
	From       string `json:"from"`
	To         string `json:"to"`
}

// DefaultAdapterName derives a source label from a shell command.
func DefaultAdapterName(command string) string {
	f := strings.Fields(command)
	if len(f) == 0 {
		return "adapter"
	}
	return strings.TrimSuffix(filepath.Base(f[0]), filepath.Ext(f[0]))
}

// ListFiles returns every candidate file under root (slash paths), skipping
// build output and hidden folders, so an adapter can pick non-code inputs.
func ListFiles(root string) []string {
	var out []string
	_ = walk(root, func(rel string) { out = append(out, rel) })
	sort.Strings(out)
	return out
}

// FilterFiles keeps files matching any glob (against the path or the base name).
// An empty glob list keeps everything.
func FilterFiles(files, globs []string) []string {
	if len(globs) == 0 {
		return files
	}
	var out []string
	for _, f := range files {
		for _, g := range globs {
			g = strings.TrimSpace(g)
			if g == "" {
				continue
			}
			p, _ := path.Match(g, f)
			b, _ := path.Match(g, path.Base(f))
			if p || b {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// RunAdapter feeds file paths to the command on stdin and reads JSONL back.
// Stacklit itself makes no network or model calls; the command is the user's.
func RunAdapter(parent context.Context, o AdapterOptions) (*AdapterResult, error) {
	if strings.TrimSpace(o.Command) == "" {
		return nil, fmt.Errorf("adapter command is empty")
	}
	if o.Timeout <= 0 {
		o.Timeout = adapterTimeout()
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = adapterMaxBytes()
	}
	if o.MaxItems <= 0 {
		o.MaxItems = adapterMaxItems()
	}
	if o.Name == "" {
		o.Name = DefaultAdapterName(o.Command)
	}
	ctx, cancel := context.WithTimeout(parent, o.Timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", o.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", o.Command)
	}
	cmd.Dir = o.Root
	cmd.Env = append(os.Environ(), "STACKLIT_ADAPTER=1", "STACKLIT_ADAPTER_NAME="+o.Name)
	cmd.Stdin = strings.NewReader(strings.Join(o.Files, "\n") + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, left: stderrKeep}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting adapter: %w", err)
	}

	res, perr := parseAdapter(io.LimitReader(stdout, o.MaxBytes+1), o)
	if perr != nil {
		cancel()
	}
	// Drain so the process can exit even when the cap stopped parsing early.
	_, _ = io.Copy(io.Discard, stdout)
	werr := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("adapter timed out after %s (STACKLIT_ADAPTER_TIMEOUT_SEC)", o.Timeout)
	}
	if perr != nil {
		return nil, perr
	}
	if werr != nil {
		return nil, fmt.Errorf("adapter failed: %v: %s", werr, strings.TrimSpace(stderr.String()))
	}
	return res, nil
}

type limitedWriter struct {
	w    io.Writer
	left int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if l.left > 0 {
		k := n
		if k > l.left {
			k = l.left
		}
		_, _ = l.w.Write(p[:k])
		l.left -= k
	}
	return n, nil
}

// parseAdapter validates lines. Bad lines are counted, not fatal, except when
// output exceeds the byte cap, which means the adapter is misbehaving.
func parseAdapter(r io.Reader, o AdapterOptions) (*AdapterResult, error) {
	res := &AdapterResult{Name: o.Name}
	var read int64
	sc := bufio.NewReaderSize(r, 64*1024)
	items := 0
	for {
		line, err := sc.ReadBytes('\n')
		read += int64(len(line))
		if read > o.MaxBytes {
			return nil, fmt.Errorf("adapter output exceeded %d bytes (STACKLIT_ADAPTER_MAX_BYTES)", o.MaxBytes)
		}
		if t := bytes.TrimSpace(line); len(t) > 0 {
			var l adapterLine
			switch {
			case json.Unmarshal(t, &l) != nil:
				res.Skipped++
			case items >= o.MaxItems:
				res.Skipped++
			case l.Type == "node" && l.ID != "" && l.Name != "" && l.SourceFile != "":
				res.Concepts = append(res.Concepts, Concept{
					ID: l.ID, Name: l.Name, Kind: orDefault(l.Kind, "concept"), Summary: l.Summary,
					SourceFile: filepath.ToSlash(l.SourceFile), Conf: ConfInferred, Source: o.Name,
				})
				items++
			case l.Type == "edge" && l.From != "" && l.To != "":
				res.Links = append(res.Links, ConceptLink{
					From: l.From, To: l.To, Kind: orDefault(l.Kind, "related"),
					SourceFile: filepath.ToSlash(l.SourceFile), Conf: ConfInferred, Source: o.Name,
				})
				items++
			default:
				res.Skipped++
			}
		}
		if err != nil {
			break
		}
	}
	return res, nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// ApplyAdapter replaces everything previously produced by the same adapter name
// in ctx, so reruns do not pile up duplicates.
func ApplyAdapter(ctx *Context, r *AdapterResult) {
	var cs []Concept
	for _, c := range ctx.Concepts {
		if c.Source != r.Name {
			cs = append(cs, c)
		}
	}
	var ls []ConceptLink
	for _, l := range ctx.Links {
		if l.Source != r.Name {
			ls = append(ls, l)
		}
	}
	ctx.Concepts = append(cs, r.Concepts...)
	ctx.Links = append(ls, r.Links...)
	sort.SliceStable(ctx.Concepts, func(i, j int) bool {
		a, b := ctx.Concepts[i], ctx.Concepts[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.ID < b.ID
	})
	sort.SliceStable(ctx.Links, func(i, j int) bool {
		a, b := ctx.Links[i], ctx.Links[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
}

// CarryAdapterOutput copies adapter concepts and links from an existing sidecar
// into ctx, so a rerun of the built-in connectors does not erase them.
func CarryAdapterOutput(ctx *Context, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var old Context
	if json.Unmarshal(data, &old) == nil {
		ctx.Concepts, ctx.Links = old.Concepts, old.Links
	}
}
