// Fails when a built page links to an internal path or fragment that does not exist.
import { readdirSync, readFileSync, statSync, existsSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const dist = resolve(dirname(fileURLToPath(import.meta.url)), '../dist');
const walk = (d) => readdirSync(d).flatMap((f) => (statSync(join(d, f)).isDirectory() ? walk(join(d, f)) : [join(d, f)]));
const pages = walk(dist).filter((f) => f.endsWith('.html') && !f.includes('/demos/'));
const ids = new Map();
const idsOf = (file) => {
  if (!ids.has(file)) ids.set(file, new Set([...readFileSync(file, 'utf8').matchAll(/\sid="([^"]+)"/g)].map((m) => m[1])));
  return ids.get(file);
};
const toFile = (p) => {
  const clean = decodeURIComponent(p.split('?')[0]);
  const f = join(dist, clean);
  if (existsSync(f) && statSync(f).isFile()) return f;
  const idx = join(f, 'index.html');
  return existsSync(idx) ? idx : null;
};

let bad = 0;
let checked = 0;
for (const file of pages) {
  const html = readFileSync(file, 'utf8');
  const here = '/' + file.slice(dist.length + 1).replace(/index\.html$/, '').replace(/\/$/, '') + '/';
  const self = here === '//' ? '/' : here;
  for (const m of html.matchAll(/\s(?:href|src)="([^"]+)"/g)) {
    const href = m[1];
    if (/^(https?:|mailto:|data:|tel:)/.test(href)) continue;
    checked++;
    const [path, hash] = href.split('#');
    const target = path === '' ? file : toFile(path.startsWith('/') ? path : join(self, path));
    if (!target) { console.error(`${self}: broken link ${href}`); bad++; continue; }
    if (hash && target.endsWith('.html') && !idsOf(target).has(hash)) { console.error(`${self}: missing anchor ${href}`); bad++; }
  }
}
console.log(`checked ${checked} internal links in ${pages.length} pages, ${bad} broken`);
process.exit(bad ? 1 : 0);
