// Writes src/data/stars.json with the live GitHub star count. Never fails the
// build: on any error the file holds {"stars":null} and the page hides the count.
import { writeFileSync, mkdirSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const out = resolve(dirname(fileURLToPath(import.meta.url)), '../src/data/stars.json');
mkdirSync(dirname(out), { recursive: true });

let stars = null;
try {
  const headers = { Accept: 'application/vnd.github+json', 'User-Agent': 'stacklit-site-build' };
  if (process.env.GITHUB_TOKEN) headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
  const res = await fetch('https://api.github.com/repos/glincker/stacklit', { headers, signal: AbortSignal.timeout(5000) });
  if (res.ok) {
    const n = (await res.json()).stargazers_count;
    if (Number.isInteger(n) && n >= 0) stars = n;
  }
} catch {
  // Offline, rate limited or slow: fall through with no count.
}
writeFileSync(out, JSON.stringify({ stars }) + '\n');
console.log(stars === null ? 'stars: unavailable, count hidden' : `stars: ${stars}`);

// The current release for the banner and the legend: the GitHub release when reachable,
// otherwise the newest versioned heading in CHANGELOG.md, so the build never has an empty version.
import { readFileSync } from 'node:fs';
const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const pretty = (iso) => { const [y, m, d] = iso.slice(0, 10).split('-').map(Number); return `${d} ${months[m - 1]} ${y}`; };
let release = null;
try {
  const res = await fetch('https://api.github.com/repos/glincker/stacklit/releases/latest', { headers: { Accept: 'application/vnd.github+json', 'User-Agent': 'stacklit-site-build', ...(process.env.GITHUB_TOKEN ? { Authorization: `Bearer ${process.env.GITHUB_TOKEN}` } : {}) }, signal: AbortSignal.timeout(5000) });
  if (res.ok) {
    const r = await res.json();
    if (/^v\d+\.\d+\.\d+/.test(r.tag_name) && r.published_at) release = { version: r.tag_name.replace(/^v/, ''), date: pretty(r.published_at), source: 'github release' };
  }
} catch {
  // Offline or rate limited: use the changelog below.
}
if (!release) {
  const log = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../CHANGELOG.md'), 'utf8');
  const m = log.match(/^## (\d+\.\d+\.\d+) \((\d{4}-\d{2}-\d{2})\)/m);
  release = m ? { version: m[1], date: pretty(m[2]), source: 'changelog' } : { version: '0.5.0', date: '8 Nov 2026', source: 'fallback' };
}
writeFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../src/data/release.json'), JSON.stringify(release) + '\n');
console.log(`release: ${release.version} (${release.date}, ${release.source})`);
