// Exports the compact file graph behind the hero picture. Not run by the build; the result is checked in.
// Usage: node scripts/export-hero-graph.mjs <gin-v1.10.0-checkout>
// The checkout is a shallow clone of the pinned tag, already indexed by a binary built from this repo
// (stacklit init, then stacklit graph), the same setup as record-ask.mjs.
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = process.argv[2];
if (!repo) { console.error('usage: export-hero-graph.mjs <gin checkout>'); process.exit(1); }
const g = JSON.parse(readFileSync(join(repo, 'stacklit.graph.json'), 'utf8'));

// One node per file, with its symbol count; edges between files are the symbol links summed.
const files = g.nodes.filter((n) => n.kind === 'file').map((n) => n.file).sort();
const index = new Map(files.map((f, i) => [f, i]));
const symbols = new Array(files.length).fill(0);
const community = new Array(files.length).fill(0);
for (const n of g.nodes) {
  const i = index.get(n.file);
  if (i === undefined) continue;
  if (n.kind === 'file') community[i] = n.community;
  else symbols[i] += 1;
}
const weights = new Map();
for (const e of g.edges) {
  const a = index.get(g.nodes[e.f].file);
  const b = index.get(g.nodes[e.t].file);
  if (a === undefined || b === undefined || a === b) continue;
  const key = a < b ? `${a}:${b}` : `${b}:${a}`;
  weights.set(key, (weights.get(key) ?? 0) + 1);
}
const edges = [...weights.entries()].map(([k, w]) => [...k.split(':').map(Number), w]).sort((x, y) => x[0] - y[0] || x[1] - y[1]);
const out = {
  repo: 'gin v1.10.0',
  commit: g.commit,
  files: files.map((f, i) => [f, community[i], symbols[i]]),
  edges,
  communities: g.communities.map((c) => ({ id: c.id, label: c.label })),
};
const file = resolve(dirname(fileURLToPath(import.meta.url)), '../src/data/hero-graph.json');
writeFileSync(file, JSON.stringify(out) + '\n');
console.log(`wrote ${file}: ${files.length} files, ${edges.length} edges`);
