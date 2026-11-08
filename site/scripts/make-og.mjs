// Regenerates public/social-preview.png (1280x640) from src/data/copy.json.
// Usage: npm run og (add -- --dark for the dark variant). Needs the agent-browser CLI (AGENT_BROWSER_BIN to override).
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
const dark = process.argv.includes('--dark');
const outFile = dark ? resolve(here, '../public/social-preview-dark.png') : out;
const pal = dark
  ? { bg: '#151410', text: '#ece9df', accent: '#5fd0b5' }
  : { bg: '#f5f3ee', text: '#1b1a17', accent: '#0a6b5b' };
const html = `<!doctype html><html><head><meta charset="utf-8"><style>
:root{--bg:${pal.bg};--text:${pal.text};--accent:${pal.accent}}
*{box-sizing:border-box}html,body{margin:0;width:1280px;height:640px;background:var(--bg);color:var(--text);font-family:"Avenir Next","Segoe UI Variable Text","Segoe UI",system-ui,sans-serif;overflow:hidden}
.wrap{padding:80px;height:100%;display:flex;flex-direction:column;justify-content:space-between}
.brand{display:flex;align-items:center;gap:14px;font-size:38px;font-weight:700;letter-spacing:-.03em}
svg{width:48px;height:48px}.t{fill:var(--accent)}.b{fill:var(--text)}
h1{margin:0;font-size:112px;line-height:1.02;letter-spacing:-.045em;font-weight:700;white-space:nowrap}
h1 span{display:block}h1 .two{color:var(--accent)}
.names{margin:30px 0 0;font-size:36px;line-height:1.2;opacity:.7}
.foot{display:flex;justify-content:space-between;align-items:center;font-size:28px;opacity:.7}
</style></head><body><div class="wrap">
<div class="brand"><svg viewBox="0 0 24 24"><rect class="t" x="8" y="3" width="13" height="5" rx="1.5"/><rect class="b" x="3" y="9.5" width="18" height="5" rx="1.5"/><rect class="b" x="3" y="16" width="13" height="5" rx="1.5"/></svg>stacklit</div>
<div><h1><span>${esc(og.line1)}</span><span class="two">${esc(og.line2)}</span></h1><p class="names">${esc(og.assistants)}</p></div>
<div class="foot"><span>${esc(og.footer)}</span><span>${esc(og.url)}</span></div>
</div></body></html>`;

const dir = mkdtempSync(join(tmpdir(), 'stacklit-og-'));
const page = join(dir, 'og.html');
writeFileSync(page, html);
const run = (...a) => execFileSync(bin, a, { stdio: ['ignore', 'pipe', 'inherit'] });
run('set', 'viewport', '1280', '640');
run('open', `file://${page}`);
run('screenshot', outFile);
run('close');
console.log(`wrote ${outFile} (${Math.round(statSync(outFile).size / 1024)} KB)`);
