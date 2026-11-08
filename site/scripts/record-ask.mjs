// Records the real output behind "Ask the map" on the home page. Not run by the build.
// Usage: node scripts/record-ask.mjs <stacklit-binary> <gin-v1.10.0-checkout>
// The checkout must already be indexed (stacklit init, then stacklit graph). Output is plain text, never edited by hand.
import { execFileSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';
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
