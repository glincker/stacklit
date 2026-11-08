package setup

import (
	"os"
	"path"
	"strconv"
	"strings"
)

// Crawl describes one recognised repo-wide crawl and the stacklit command
// that answers the same need.
type Crawl struct {
	Kind    string // find, ls, tree, grep, glob, read-dir, subagent
	Ident   string // identifier searched for, when the pattern is one
	Query   string // words to put in a stacklit query, may be empty
	Command string // suggested stacklit command
}

// maxNarrowDepth is the deepest listing still treated as a narrow look.
// STACKLIT_CRAWL_MAX_DEPTH overrides it.
func maxNarrowDepth() int {
	if n, err := strconv.Atoi(os.Getenv("STACKLIT_CRAWL_MAX_DEPTH")); err == nil && n >= 0 {
		return n
	}
	return 1
}

// splitShell splits a command line into pipelines of tokens. It understands
// quotes and the usual separators, and keeps backslashes literal so Windows
// paths survive. The bool per segment marks one that reads a pipe.
type segment struct {
	toks  []string
	piped bool
}

func splitShell(cmd string) []segment {
	var segs []segment
	var cur []string
	var tok strings.Builder
	var quote rune
	have, curPiped := false, false
	flushTok := func() {
		if have {
			cur = append(cur, tok.String())
		}
		tok.Reset()
		have = false
	}
	flushSeg := func(nextPiped bool) {
		flushTok()
		if len(cur) > 0 {
			segs = append(segs, segment{cur, curPiped})
		}
		cur, curPiped = nil, nextPiped
	}
	rs := []rune(cmd)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				tok.WriteRune(c)
			}
		case c == '"' || c == '\'':
			quote, have = c, true
		case c == ' ' || c == '\t' || c == '\r':
			flushTok()
		case c == '\n' || c == ';':
			flushSeg(false)
		case c == '|':
			if i+1 < len(rs) && rs[i+1] == '|' {
				i++
				flushSeg(false)
			} else {
				flushSeg(true)
			}
		case c == '&':
			if i+1 < len(rs) && rs[i+1] == '&' {
				i++
			}
			flushSeg(false)
		default:
			tok.WriteRune(c)
			have = true
		}
	}
	flushSeg(false)
	return segs
}

// cleanPath normalises a path operand for comparison, folding Windows
// separators and trailing slashes.
func cleanPath(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), `\`, "/")
	if p == "" {
		return ""
	}
	return path.Clean(p)
}

// rootish reports whether p names the working directory, its parent, home, or
// an ancestor of root, meaning a search there covers the whole repo.
func rootish(p, root string) bool {
	switch strings.ToLower(p) {
	case "", ".", "./", `.\`, "~", "$pwd", "${pwd}", "$(pwd)", "%cd%", "$pwd/", "$(pwd)/", "$cwd":
		return true
	}
	c := cleanPath(p)
	if c == "." || c == ".." || c == "/" || c == "~" || strings.HasPrefix(c, "../") && strings.Trim(c, "./") == "" {
		return true
	}
	if root != "" && (path.IsAbs(c) || (len(c) > 1 && c[1] == ':')) {
		r := cleanPath(root)
		return strings.EqualFold(c, r) || strings.HasPrefix(strings.ToLower(r), strings.ToLower(c)+"/")
	}
	return false
}

func lowerBase(s string) string {
	s = strings.ReplaceAll(s, `\`, "/")
	s = strings.ToLower(s[strings.LastIndexByte(s, '/')+1:])
	return strings.TrimSuffix(s, ".exe")
}

var wrapperCmds = map[string]bool{"sudo": true, "time": true, "nice": true, "command": true, "env": true, "exec": true, "nohup": true}

// stripPrefix drops env assignments and wrappers in front of the real command.
func stripPrefix(toks []string) []string {
	for len(toks) > 0 {
		t := toks[0]
		if wrapperCmds[lowerBase(t)] || (strings.Contains(t, "=") && !strings.HasPrefix(t, "-") && !strings.ContainsAny(t, "/\\")) {
			toks = toks[1:]
			continue
		}
		break
	}
	return toks
}

// argFlags lists flags whose value is the next token, per tool, so a value is
// never mistaken for a path operand.
var argFlags = map[string]map[string]bool{
	"grep": set("-e", "-f", "-m", "-A", "-B", "-C", "--include", "--exclude", "--exclude-dir", "--color", "--max-count", "--regexp", "--file", "--include-dir"),
	"rg":   set("-e", "-f", "-m", "-A", "-B", "-C", "-g", "--glob", "--iglob", "-t", "-T", "--type", "--type-not", "--max-depth", "-j", "--threads", "-M", "--max-columns", "--regexp", "--file", "--max-count", "--sort", "--sortr", "-r", "--replace", "--colors", "--context"),
}

func set(k ...string) map[string]bool {
	m := map[string]bool{}
	for _, s := range k {
		m[s] = true
	}
	return m
}

// operands splits tokens into the pattern, explicit patterns via -e and the
// remaining path operands, honouring the flag table.
func operands(tool string, toks []string) (flags []string, pats, paths []string) {
	af := argFlags[tool]
	pastDD := false
	havePat := false
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case pastDD:
			paths = append(paths, t)
		case t == "--":
			pastDD = true
		case strings.HasPrefix(t, "--") && strings.Contains(t, "="):
			flags = append(flags, t)
		case strings.HasPrefix(t, "-") && len(t) > 1:
			flags = append(flags, t)
			if af[t] && i+1 < len(toks) {
				if t == "-e" || t == "--regexp" {
					pats = append(pats, toks[i+1])
					havePat = true
				}
				i++
			}
		case !havePat && len(pats) == 0:
			pats = append(pats, t)
			havePat = true
		default:
			paths = append(paths, t)
		}
	}
	return
}

func hasFlag(flags []string, long string, short ...rune) bool {
	for _, f := range flags {
		if f == long || strings.HasPrefix(f, long+"=") {
			return true
		}
		if strings.HasPrefix(f, "-") && !strings.HasPrefix(f, "--") {
			for _, r := range short {
				if strings.ContainsRune(f[1:], r) {
					return true
				}
			}
		}
	}
	return false
}

func allRootish(paths []string, root string) bool {
	for _, p := range paths {
		if !rootish(p, root) {
			return false
		}
	}
	return true
}

// queryWords turns a grep pattern or glob into plain words for a query.
func queryWords(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	ws := strings.Fields(b.String())
	var keep []string
	for _, w := range ws {
		if len(w) >= 3 {
			keep = append(keep, w)
		}
	}
	out := strings.Join(keep, " ")
	if len(out) > 60 {
		out = strings.TrimSpace(out[:60])
	}
	return out
}

func genericCmd() string { return `stacklit query "<question>" --budget 800` }

// commandFor picks the stacklit command for a crawl from its identifier or words.
func commandFor(c *Crawl, ident, words string) {
	c.Ident, c.Query = ident, words
	switch {
	case ident != "":
		c.Command = "stacklit explain " + ident
	case words != "":
		c.Command = `stacklit query "` + words + `" --budget 800`
	default:
		c.Command = genericCmd()
	}
}

// detectSegment judges one command, already stripped of wrappers.
func detectSegment(seg segment, root string) (Crawl, bool) {
	toks := stripPrefix(seg.toks)
	if len(toks) == 0 {
		return Crawl{}, false
	}
	name, args := lowerBase(toks[0]), toks[1:]
	c := Crawl{}
	switch name {
	case "find":
		var paths []string
		i := 0
		for ; i < len(args) && !strings.HasPrefix(args[i], "-") && args[i] != "(" && args[i] != "!"; i++ {
			paths = append(paths, args[i])
		}
		if !allRootish(paths, root) {
			return c, false
		}
		var nameGlob string
		for j := i; j < len(args); j++ {
			switch args[j] {
			case "-delete":
				return c, false
			case "-maxdepth":
				if j+1 < len(args) {
					if n, err := strconv.Atoi(args[j+1]); err == nil && n <= maxNarrowDepth() {
						return c, false
					}
				}
			case "-name", "-iname", "-path", "-ipath":
				if j+1 < len(args) && nameGlob == "" {
					nameGlob = args[j+1]
				}
			}
		}
		c.Kind = "find"
		commandFor(&c, "", queryWords(nameGlob))
		return c, true
	case "ls", "dir", "get-childitem", "gci":
		rec := false
		var paths []string
		for _, a := range args {
			la := strings.ToLower(a)
			switch {
			case la == "--recursive", a == "-R", strings.HasPrefix(la, "-rec"):
				rec = true
			case name != "ls" && la == "-r", name == "dir" && la == "/s", name == "dir" && strings.HasPrefix(la, "/s"):
				rec = true
			case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && name == "ls" && strings.ContainsRune(a[1:], 'R'):
				rec = true
			case strings.HasPrefix(a, "-"), name == "dir" && strings.HasPrefix(a, "/"):
			default:
				paths = append(paths, a)
			}
		}
		if !rec || !allRootish(paths, root) {
			return c, false
		}
		c.Kind = "ls"
		commandFor(&c, "", "")
		return c, true
	case "tree":
		depth := -1
		var paths []string
		for i := 0; i < len(args); i++ {
			if args[i] == "-L" && i+1 < len(args) {
				depth, _ = strconv.Atoi(args[i+1])
				i++
			} else if !strings.HasPrefix(args[i], "-") && !strings.HasPrefix(args[i], "/") {
				paths = append(paths, args[i])
			}
		}
		if (depth >= 0 && depth <= maxNarrowDepth()) || !allRootish(paths, root) {
			return c, false
		}
		c.Kind = "tree"
		commandFor(&c, "", "")
		return c, true
	case "grep", "egrep", "fgrep", "rg", "ag", "ack", "git", "findstr", "select-string":
		return detectSearch(name, args, seg.piped, root)
	case "bash", "sh", "zsh", "cmd", "powershell", "pwsh":
		for i, a := range args {
			if a == "-c" || a == "-lc" || a == "/c" || a == "/C" || strings.EqualFold(a, "-command") {
				return DetectBash(strings.Join(args[i+1:], " "), root)
			}
		}
	}
	return c, false
}

func detectSearch(name string, args []string, piped bool, root string) (Crawl, bool) {
	c := Crawl{Kind: "grep"}
	tool := name
	switch name {
	case "git":
		if len(args) == 0 || args[0] != "grep" {
			return c, false
		}
		args, tool = args[1:], "grep"
	case "egrep", "fgrep":
		tool = "grep"
	case "ag", "ack":
		tool = "rg"
	}
	flags, pats, paths := operands(tool, args)
	if tool == "rg" && hasFlag(flags, "--files") {
		paths, pats = append(pats, paths...), nil
	}
	pat := ""
	if len(pats) > 0 {
		pat = pats[0]
	}
	switch {
	case name == "findstr":
		sub := false
		for _, a := range args {
			if strings.EqualFold(a, "/s") {
				sub = true
			}
		}
		if !sub {
			return c, false
		}
		var ps []string
		for _, a := range args {
			if !strings.HasPrefix(a, "/") {
				ps = append(ps, a)
			}
		}
		// First non-switch is the search string, the rest are masks or dirs.
		if len(ps) > 0 {
			pat, ps = ps[0], ps[1:]
		}
		for _, p := range ps {
			if !rootish(p, root) && strings.ContainsAny(p, `/\`) {
				return c, false
			}
		}
	case name == "select-string":
		return c, false
	case tool == "grep" && name != "git":
		if !hasFlag(flags, "--recursive", 'r', 'R') && !hasFlag(flags, "--dereference-recursive") {
			return c, false
		}
		if len(paths) == 0 && piped {
			return c, false
		}
		if !allRootish(paths, root) {
			return c, false
		}
	case name == "git":
		if !allRootish(paths, root) {
			return c, false
		}
	default: // rg, ag, ack
		if len(paths) == 0 && piped {
			return c, false
		}
		if !allRootish(paths, root) {
			return c, false
		}
	}
	commandFor(&c, identOf(pat), queryWords(pat))
	return c, true
}
