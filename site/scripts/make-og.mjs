// Regenerates public/social-preview.png (1280x640) from src/data/copy.json.
// Usage: npm run og. Needs the agent-browser CLI (AGENT_BROWSER_BIN to override).
import { readFileSync, writeFileSync, mkdtempSync, statSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { resolve, dirname, join } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const copy = JSON.parse(readFileSync(resolve(here, '../src/data/copy.json'), 'utf8'));
const out = resolve(here, '../public/social-preview.png');
const bin = process.env.AGENT_BROWSER_BIN || '/opt/homebrew/bin/agent-browser';
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

const og = copy.og;
const html = `<!doctype html><html><head><meta charset="utf-8"><style>
:root{--bg:#f5f3ee;--text:#1b1a17;--muted:#5b5850;--accent:#0a6b5b;--term:#1b1a17;--termtext:#ece9df}
*{box-sizing:border-box}html,body{margin:0;width:1280px;height:640px;background:var(--bg);color:var(--text);font-family:"Avenir Next","Segoe UI",system-ui,sans-serif;overflow:hidden}
.wrap{padding:72px 80px;height:100%;display:flex;flex-direction:column;justify-content:space-between}
.brand{display:flex;align-items:center;gap:14px;font-size:34px;font-weight:650;letter-spacing:-.03em}
svg{width:44px;height:44px}.t{fill:var(--accent)}.b{fill:var(--text)}
h1{margin:0;font-size:76px;line-height:1.04;letter-spacing:-.035em;font-weight:650;max-width:1020px}
p{margin:22px 0 0;font-size:32px;line-height:1.35;color:var(--muted);max-width:900px}
.foot{display:flex;justify-content:space-between;align-items:center;font-size:26px;color:var(--muted)}
code{font-family:ui-monospace,"SF Mono",Menlo,monospace;background:var(--term);color:var(--termtext);padding:10px 18px;border-radius:8px;font-size:26px}
</style></head><body><div class="wrap">
<div class="brand"><svg viewBox="0 0 24 24"><rect class="t" x="8" y="3" width="13" height="5" rx="1.5"/><rect class="b" x="3" y="9.5" width="18" height="5" rx="1.5"/><rect class="b" x="3" y="16" width="13" height="5" rx="1.5"/></svg>stacklit</div>
<div><h1>${esc(og.headline)}</h1><p>${esc(og.sub)}</p></div>
<div class="foot"><code>$ ${esc(copy.cta.install)}</code><span>${esc(og.url)}</span></div>
</div></body></html>`;

const dir = mkdtempSync(join(tmpdir(), 'stacklit-og-'));
const page = join(dir, 'og.html');
writeFileSync(page, html);
const run = (...a) => execFileSync(bin, a, { stdio: ['ignore', 'pipe', 'inherit'] });
run('set', 'viewport', '1280', '640');
run('open', `file://${page}`);
run('screenshot', out);
run('close');
console.log(`wrote ${out} (${Math.round(statSync(out).size / 1024)} KB)`);
