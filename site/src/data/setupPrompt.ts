// The text a visitor pastes into their coding agent. Every command and file named here was run against the real
// binary (init writes stacklit.json, DEPENDENCIES.md and stacklit.html; setup writes each detected assistant's config
// and installs git hooks; doctor checks the result). Keep it in step with docs/USAGE.md.
export const setupPrompt = `Set up Stacklit in this repository. Stacklit writes a small map of the codebase (stacklit.json) that you can query instead of searching blind.

1. Run: npx stacklit init
   It scans the repo and writes stacklit.json, DEPENDENCIES.md and stacklit.html, then tries to open the visual map in a browser. Nothing leaves the machine.

2. Install the binary so it is on PATH: npm install -g stacklit
   Then run: stacklit setup
   It finds the AI tools set up here and writes the config each one reads (for example CLAUDE.md, .mcp.json, AGENTS.md), and installs git hooks that rebuild the map after commits, checkouts and merges.

3. Run: stacklit doctor
   Then show me which files changed. Do not commit anything.

After that, when you need to find where something lives, run: stacklit query "<plain question>" --budget 1000
When you already know an exact name, plain grep is fine.`;
