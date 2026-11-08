// Renders the social cards (1280x640) from og.variants in src/data/copy.json into public/og/.
// Colours come from src/styles/tokens.css. Needs the agent-browser CLI (AGENT_BROWSER_BIN to override).
// Usage: npm run og
import { readFileSync, writeFileSync, mkdtempSync, statSync, mkdirSync, copyFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { resolve, dirname, join } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const copy = JSON.parse(readFileSync(resolve(here, '../src/data/copy.json'), 'utf8'));
const css = readFileSync(resolve(here, '../src/styles/tokens.css'), 'utf8');
const outDir = resolve(here, '../public/og');
mkdirSync(outDir, { recursive: true });
const bin = process.env.AGENT_BROWSER_BIN || '/opt/homebrew/bin/agent-browser';
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

const tokens = (sel) => {
  const start = css.indexOf(sel + ' {');
  const body = css.slice(start, css.indexOf('}', start));
  return Object.fromEntries([...body.matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6})/g)].map((m) => [m[1], m[2]]));
};
const light = tokens(':root');
const pal = { light, dark: { ...light, ...tokens(":root[data-theme='dark']") } };
const fontSans = (css.match(/--font-sans:\s*([^;]+);/) || [])[1];
if (!fontSans) throw new Error('--font-sans missing from tokens.css');

const slug = (v) => v.purpose.split(' ')[0].replace(/[^a-z]/g, '') || 'card';
const names = ['default', 'stat', 'story', 'large-repo', 'launch', 'map'];
const must = 'Largest test repo only. Across all three repos, no cost change.';

function page(v, p) {
  const longest = Math.max(v.line1.length, v.line2.length);
  const size = Math.min(104, Math.floor(1120 / (longest * 0.53)));
  return `<!doctype html><html><head><meta charset="utf-8"><style>
*{box-sizing:border-box}html,body{margin:0;width:1280px;height:640px;background:${p.paper};color:${p.ink};font-family:${fontSans};overflow:hidden}
.wrap{padding:72px 80px;height:100%;display:flex;flex-direction:column;justify-content:space-between}
.brand{display:flex;align-items:center;gap:14px;font-size:38px;font-weight:700;letter-spacing:-.03em}
svg{width:48px;height:48px}
h1{margin:0;font-size:${size}px;line-height:1.05;letter-spacing:-.04em;font-weight:700;white-space:nowrap}
h1 span{display:block}h1 .two{color:${p.accent}}
.foot{font-size:30px;line-height:1.3;color:${p.muted}}
.rule{height:4px;width:96px;background:${p.accent};margin-bottom:28px}
</style></head><body><div class="wrap">
<div class="brand"><svg viewBox="0 0 24 24"><rect x="8" y="3" width="13" height="5" rx="1.5" fill="${p.accent}"/><rect x="3" y="9.5" width="18" height="5" rx="1.5" fill="${p.ink}"/><rect x="3" y="16" width="13" height="5" rx="1.5" fill="${p.ink}"/></svg>stacklit</div>
<div><div class="rule"></div><h1><span>${esc(v.line1)}</span><span class="two">${esc(v.line2)}</span></h1></div>
<div class="foot">${esc(v.footer)}</div>
</div></body></html>`;
}

const dir = mkdtempSync(join(tmpdir(), 'stacklit-og-'));
const run = (...a) => execFileSync(bin, a, { stdio: ['ignore', 'pipe', 'inherit'] });
run('set', 'viewport', '1280', '640');
const jobs = copy.og.variants.map((v, i) => ({ v, name: names[i] ?? slug(v), theme: 'light' }));
jobs.push({ v: copy.og.variants[0], name: 'default', theme: 'dark' });
for (const { v, name, theme } of jobs) {
  if (name === 'large-repo' && !v.footer.includes(must)) throw new Error('the large repo card must carry its footer');
  const file = join(dir, `${name}-${theme}.html`);
  writeFileSync(file, page(v, pal[theme]));
  const out = resolve(outDir, `${name}-${theme}.png`);
  run('open', `file://${file}`);
  run('screenshot', out);
  const kb = Math.round(statSync(out).size / 1024);
  if (kb > 300) throw new Error(`${name}-${theme}.png is ${kb} KB, over the 300 KB limit`);
  console.log(`wrote og/${name}-${theme}.png (${kb} KB)`);
}
run('close');
copyFileSync(resolve(outDir, 'default-light.png'), resolve(here, '../public/social-preview.png'));
copyFileSync(resolve(outDir, 'default-dark.png'), resolve(here, '../public/social-preview-dark.png'));
