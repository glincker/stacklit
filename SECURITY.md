# Security policy

## Reporting a vulnerability

Please report security issues privately through GitHub: open the repository's **Security** tab and choose **Report a vulnerability** (private vulnerability reporting). Do not open a public issue or discussion for a suspected vulnerability.

Include what you found, how to reproduce it, the version (`stacklit --version`) and the platform. You can expect an acknowledgement within a few days. This is a volunteer-maintained project, so there is no formal response-time guarantee, but reports are taken seriously and credited if you wish.

## Supported versions

Fixes are made on the latest release and the default branch. Older versions are not patched; upgrade with `npm install -g stacklit@latest` or `go install github.com/glincker/stacklit/cmd/stacklit@latest`.

## Threat model

Stacklit is a local command-line tool that reads a repository you point it at and writes files into that repository.

What it does:

- Reads source files and `.gitignore`, `.stacklitrc.json` and package manifests in the working directory.
- Reads git history by running `git`.
- Writes `stacklit.json`, `DEPENDENCIES.md` and `stacklit.html`, edits `CLAUDE.md`, `.cursorrules`, `.aider.conf.yml`, `.mcp.json` and `.cursor/mcp.json` when you run `derive --inject` or `setup`, and creates or appends to `.git/hooks/post-commit` when you ask for the hook.
- Serves read-only MCP tools over stdio from `stacklit.json`.

What it does not do:

- It makes no network requests of its own and has no telemetry.
- It does not execute code from the repository it scans. Files are parsed, not run.
- It does not read environment variable values from your environment; it only records variable names that appear in source text.

The one opt-in path that leaves the process is `stacklit init --summary`, which runs a command you configure (`claude -p` by default, or `STACKLIT_SUMMARY_CMD`) and passes it an index snapshot on stdin. It contains module names, exports, dependency edges and entrypoints, not file contents. Do not set `STACKLIT_SUMMARY_CMD` to a command you do not trust.

## Things worth knowing

- `stacklit.json` lists module paths, file names and exported symbol names. If you commit it to a public repo, that information becomes public. Review it for private repos.
- The git hook runs `stacklit generate` after commits. It is a plain shell script you can read in `.git/hooks/post-commit`.
- Scanning an untrusted repository parses its files with tree-sitter and Go parsers. Parser bugs on hostile input are in scope for reports. Scan untrusted code in a sandbox or container if that concerns you.
- The npm package downloads a release binary from GitHub Releases on install. Prefer `go install` or a checked download if you need to verify provenance.

## Out of scope

Reports that stacklit "reads my source code" (that is its job), findings that require an attacker who already controls your machine or your repository's git hooks, and denial of service from scanning extremely large repositories.
