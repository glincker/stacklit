// Fails when a built page would load anything from another host: images, scripts, styles, fonts,
// frames or CSS url(). Plain links to other sites are fine, nothing is fetched until a visitor clicks.
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const dist = resolve(dirname(fileURLToPath(import.meta.url)), '../dist');
const walk = (d) => readdirSync(d).flatMap((f) => (statSync(join(d, f)).isDirectory() ? walk(join(d, f)) : [join(d, f)]));
const remote = /(?:https?:)?\/\/[\w.-]+\.[a-z]{2,}/i;
let bad = 0;
for (const f of walk(dist).filter((x) => /\.(html|css)$/.test(x))) {
  const t = readFileSync(f, 'utf8');
  const hits = [
    ...[...t.matchAll(/<(?:img|script|iframe|source|video|audio|embed)\b[^>]*?\s(?:src|srcset)="([^"]+)"/g)].map((m) => m[1]),
    ...[...t.matchAll(/<link\b[^>]*?rel="(?:stylesheet|preload|prefetch|modulepreload|icon)"[^>]*?href="([^"]+)"/g)].map((m) => m[1]),
    ...[...t.matchAll(/url\(\s*["']?([^"')]+)/g)].map((m) => m[1]),
    ...[...t.matchAll(/@import\s+["']([^"']+)/g)].map((m) => m[1]),
  ].filter((u) => remote.test(u) && !u.startsWith('data:'));
  for (const u of hits) { console.error(`${f.slice(dist.length)}: loads ${u}`); bad++; }
}
console.log(`external request check: ${bad} found`);
process.exit(bad ? 1 : 0);
