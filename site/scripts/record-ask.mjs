// Records the real output behind "Ask the map" on the home page. Not run by the build.
// Usage: node scripts/record-ask.mjs <stacklit-binary> <gin-v1.10.0-checkout>
// The checkout must already be indexed (stacklit init, then stacklit graph). Output is plain text, never edited by hand.
import { execFileSync } from 'node:child_process';
import { writeFileSync, mkdtempSync, cpSync, writeFileSync as put, chmodSync, mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const [bin, repo] = process.argv.slice(2);
if (!bin || !repo) { console.error('usage: record-ask.mjs <binary> <gin checkout>'); process.exit(1); }
const run = (cmd, args) => execFileSync(cmd, args, { cwd: repo, encoding: 'utf8', maxBuffer: 1 << 24 }).trimEnd();
const tokens = (s) => Math.ceil(Buffer.byteLength(s) / 4);
const budget = '600';

const asks = [
  { id: 'params', chip: 'How are route params bound?', kind: 'query', question: 'how are route params bound to a handler' },
  { id: 'body', chip: 'Where is the body decoded?', kind: 'query', question: 'where is the request body decoded into a struct' },
  { id: 'panic', chip: 'How does it recover from a panic?', kind: 'query', question: 'how does the router recover from a panic' },
  { id: 'imports', chip: 'Who imports bytesconv?', kind: 'impact', name: 'internal/bytesconv' },
];
const out = asks.map((a) => {
  if (a.kind === 'query') {
    const cmd = `stacklit query "${a.question}" --budget ${budget}`;
    const text = run(bin, ['query', a.question, '--budget', budget]);
    return { id: a.id, chip: a.chip, command: cmd, output: text, tokens: tokens(text), verdict: 'map' };
  }
  const cmd = `stacklit impact ${a.name}`;
  const text = run(bin, ['impact', a.name]);
  const gcmd = `grep -rn "${a.name}" --include=*.go .`;
  const g = run('grep', ['-rn', a.name, '--include=*.go', '.']);
  return { id: a.id, chip: a.chip, command: cmd, output: text, tokens: tokens(text), verdict: 'grep', grepCommand: gcmd, grepOutput: g, grepTokens: tokens(g) };
});
const file = resolve(dirname(fileURLToPath(import.meta.url)), '../src/data/ask.json');
writeFileSync(file, JSON.stringify({ repo: 'gin v1.10.0', budget: Number(budget), asks: out }, null, 2) + '\n');
console.log(`wrote ${file}`);

// The first two steps run on a clean copy of the checkout, with a no-op "open" first on PATH so init does not launch a browser.
const work = mkdtempSync(join(tmpdir(), 'stacklit-steps-'));
cpSync(repo, join(work, 'repo'), { recursive: true });
const shim = join(work, 'bin');
mkdirSync(shim);
put(join(shim, 'open'), '#!/bin/sh\nexit 0\n');
put(join(shim, 'xdg-open'), '#!/bin/sh\nexit 0\n');
chmodSync(join(shim, 'open'), 0o755);
chmodSync(join(shim, 'xdg-open'), 0o755);
const clean = join(work, 'repo');
execFileSync('git', ['clean', '-fdxq'], { cwd: clean });
const env = { ...process.env, PATH: `${shim}:${process.env.PATH}` };
const step = (args) => execFileSync(bin, args, { cwd: clean, env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trimEnd();
const initOut = execFileSync(bin, ['init'], { cwd: clean, env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trimEnd();
const setupOut = step(['setup']);
const plain = (t) => t.replaceAll('\u2014', '-').replaceAll('\u2013', '-'); // the site prints no dash characters, so the tool's one em dash becomes a hyphen
const steps = { repo: 'gin v1.10.0', init: plain(initOut).replace(/\n\nOpening visual map\.\.\./, ''), setup: plain(setupOut) };
const stepsFile = resolve(dirname(fileURLToPath(import.meta.url)), '../src/data/steps.json');
writeFileSync(stepsFile, JSON.stringify(steps, null, 2) + '\n');
console.log(`wrote ${stepsFile}`);
