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
