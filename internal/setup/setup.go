// Package setup wires stacklit into AI coding assistants. Every step is
// idempotent, supports dry-run, and can be reversed with uninstall.
package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/schema"
)

// Options controls one setup run.
type Options struct {
	Root      string
	Tools     []Tool
	Index     *schema.Index
	Uninstall bool
	NoMCP     bool
	NoGitHook bool
	// Global lets setup write outside the repo (home directory configs).
	Global bool
	Claude ClaudeOptions
}

// Run applies opts through r and returns the first hard error.
func Run(r *Runner, o Options) error {
	seen := map[string]bool{}
	for _, t := range o.Tools {
		fmt.Fprintf(r.Out, "%s\n", t.Name)
		for _, rule := range t.Rules {
			if seen[rule.Path] {
				continue
			}
			seen[rule.Path] = true
			if err := r.applyRule(rule, o); err != nil {
				return fmt.Errorf("%s: %w", t.ID, err)
			}
		}
		if t.ID == "aider" {
			if err := r.applyAiderConf(o.Uninstall); err != nil {
				return fmt.Errorf("aider: %w", err)
			}
		}
		if o.NoMCP {
			continue
		}
		for _, m := range t.MCP {
			if m.Global {
				if !o.Global {
					fmt.Fprintf(r.Out, "  skip %s (global config, rerun with --global)\n", m.Path)
					continue
				}
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				m.Path = filepath.Join(home, m.Path)
			}
			if seen[m.Path] {
				continue
			}
			seen[m.Path] = true
			if err := r.applyMCP(m, o.Uninstall); err != nil {
				return fmt.Errorf("%s mcp: %w", t.ID, err)
			}
		}
		if t.ID == "claude" {
			if err := r.ApplyClaudeExtras(o.Claude, o.Uninstall); err != nil {
				return fmt.Errorf("claude extras: %w", err)
			}
		}
	}
	if o.NoGitHook {
		return nil
	}
	return r.applyGitHook(o.Uninstall)
}

func (r *Runner) applyRule(rule Rule, o Options) error {
	old, existed, err := r.read(rule.Path)
	if err != nil {
		return err
	}
	if o.Uninstall {
		if !existed {
			return nil
		}
		out, changed := removeBlock(old, rule.Style)
		if !changed {
			return nil
		}
		rest := strings.TrimSpace(out)
		if rest == "" || rest == strings.TrimSpace(rule.Frontmatter) {
			return r.remove(rule.Path)
		}
		return r.write(rule.Path, old, existed, out)
	}
	base := old
	if !existed {
		base = rule.Frontmatter
	}
	return r.write(rule.Path, old, existed, upsertBlock(base, MarkdownBody(o.Index), rule.Style))
}

// applyAiderConf points Aider at CONVENTIONS.md without clobbering a user's own read list.
func (r *Runner) applyAiderConf(uninstall bool) error {
	const path = ".aider.conf.yml"
	old, existed, err := r.read(path)
	if err != nil {
		return err
	}
	if uninstall {
		if !existed {
			return nil
		}
		out, changed := removeBlock(old, StyleHash)
		if !changed {
			return nil
		}
		if strings.TrimSpace(out) == "" {
			return r.remove(path)
		}
		return r.write(path, old, existed, out)
	}
	s, e := markers(StyleHash)
	stripped, _ := replaceRange(old, s, e, "")
	if strings.Contains(stripped, "\nread:") || strings.HasPrefix(stripped, "read:") {
		fmt.Fprintf(r.Out, "  skip %s (already has a read: list, add CONVENTIONS.md by hand)\n", path)
		return nil
	}
	return r.write(path, old, existed, upsertBlock(old, "read: CONVENTIONS.md\n", StyleHash))
}

func (r *Runner) applyGitHook(uninstall bool) error {
	if uninstall {
		if !r.DryRun {
			if _, err := git.RemoveRefreshHooks(r.Root); err != nil {
				return err
			}
		}
		return r.removeGitHook()
	}
	if _, err := os.Stat(r.abs(filepath.Join(".git", "hooks"))); err != nil {
		return nil
	}
	if r.gitHookInstalled() {
		fmt.Fprintln(r.Out, "  unchanged .git/hooks/post-commit")
		if !r.DryRun {
			if _, err := git.InstallRefreshHooks(r.Root); err != nil {
				return err
			}
		}
		return nil
	}
	r.say("install", ".git/hooks/post-commit")
	r.Changed++
	if r.DryRun {
		r.say("install", ".git/hooks/{post-checkout,post-merge,post-rewrite}")
		return nil
	}
	if err := git.InstallHook(r.Root); err != nil {
		return err
	}
	if _, err := git.InstallRefreshHooks(r.Root); err != nil {
		return err
	}
	r.say("install", ".git/hooks/{post-checkout,post-merge,post-rewrite}")
	return nil
}
