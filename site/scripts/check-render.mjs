// Width and overflow gate for the built site (run after `npm run build`). Serves dist, drives a local
// Chrome over the DevTools protocol (no dependencies; Node 22 has WebSocket built in) and asserts at
// 320 to 2000 px, light and dark: no horizontal scroll, equal side gutters on landing sections, code
// that never wraps. Chrome: CHROME_BIN, else the usual install paths (present on GitHub runners).
import { createServer } from 'node:http';
import { readFileSync, existsSync, statSync, mkdtempSync } from 'node:fs';
import { spawn } from 'node:child_process';
import { join, resolve, dirname, extname } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';

const dist = resolve(dirname(fileURLToPath(import.meta.url)), '../dist');
const chrome = [process.env.CHROME_BIN, '/usr/bin/google-chrome', '/usr/bin/google-chrome-stable', '/usr/bin/chromium', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'].find((p) => p && existsSync(p));
if (!chrome) { console.error('check-render: no Chrome found (set CHROME_BIN)'); process.exit(process.env.CI ? 1 : 0); }

const types = { '.html': 'text/html', '.css': 'text/css', '.js': 'text/javascript', '.svg': 'image/svg+xml', '.png': 'image/png', '.webp': 'image/webp', '.json': 'application/json', '.txt': 'text/plain' };
const server = createServer((req, res) => {
  let p = join(dist, decodeURIComponent(new URL(req.url, 'http://x').pathname));
  if (existsSync(p) && statSync(p).isDirectory()) p = join(p, 'index.html');
  if (!existsSync(p)) { res.writeHead(404); res.end('not found'); return; }
  res.writeHead(200, { 'Content-Type': types[extname(p)] || 'application/octet-stream' });
  res.end(readFileSync(p));
}).listen(0);
const base = `http://127.0.0.1:${server.address().port}`;

const dir = mkdtempSync(join(tmpdir(), 'stacklit-chrome-'));
const proc = spawn(chrome, ['--headless=new', '--no-sandbox', '--disable-gpu', '--remote-debugging-port=0', `--user-data-dir=${dir}`, 'about:blank'], { stdio: ['ignore', 'ignore', 'pipe'] });
const wsUrl = await new Promise((ok, no) => {
  let buf = '';
  const t = setTimeout(() => no(new Error('Chrome did not start')), 15000);
  proc.stderr.on('data', (d) => { buf += d; const m = buf.match(/ws:\/\/[^\s]+/); if (m) { clearTimeout(t); ok(m[0]); } });
});
const ws = new WebSocket(wsUrl);
await new Promise((ok) => ws.addEventListener('open', ok));
let id = 0;
const waiting = new Map();
ws.addEventListener('message', (e) => { const m = JSON.parse(e.data); if (m.id && waiting.has(m.id)) { waiting.get(m.id)(m); waiting.delete(m.id); } });
const send = (method, params = {}, sessionId) => new Promise((ok) => { const n = ++id; waiting.set(n, ok); ws.send(JSON.stringify({ id: n, method, params, sessionId })); });
const { result: { targetId } } = await send('Target.createTarget', { url: 'about:blank' });
const { result: { sessionId } } = await send('Target.attachToTarget', { targetId, flatten: true });
const cmd = (m, p) => send(m, p, sessionId);
await cmd('Page.enable');

const probe = `(() => {
  const iw = innerWidth, out = [];
  const de = document.documentElement;
  if (de.scrollWidth > iw) out.push('horizontal scroll: scrollWidth ' + de.scrollWidth + ' > ' + iw);
  for (const pre of document.querySelectorAll('pre')) {
    const ws = getComputedStyle(pre).whiteSpace;
    if (!/^pre/.test(ws) || ws === 'pre-wrap' || ws === 'pre-line') out.push('code wraps (' + ws + ')');
  }
  for (const s of document.querySelectorAll('main > section > div')) {
    const r = s.getBoundingClientRect(), cs = getComputedStyle(s);
    const left = r.left + parseFloat(cs.paddingLeft), right = iw - (r.right - parseFloat(cs.paddingRight));
    if (Math.abs(left - right) > 1.5) out.push('uneven gutters ' + left.toFixed(1) + ' / ' + right.toFixed(1));
    if (left < 12) out.push('content hugs the edge (' + left.toFixed(1) + 'px)');
  }
  for (const b of document.querySelectorAll('a[class*="min-h-11"], button')) {
    const r = b.getBoundingClientRect();
    if (r.width && r.right > iw + 1) out.push('control past the edge: ' + (b.textContent || '').trim().slice(0, 30));
  }
  return out;
})()`;

const pages = ['/', '/demos/', '/docs/quickstart/', '/docs/agent-test/', '/404.html'];
const widths = [320, 390, 768, 1440, 2000];
let failures = 0, runs = 0;
for (const theme of ['light', 'dark']) {
  for (const w of widths) {
    await cmd('Emulation.setDeviceMetricsOverride', { width: w, height: 900, deviceScaleFactor: 1, mobile: w < 500 });
    await cmd('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: theme }] });
    for (const p of pages) {
      await cmd('Page.navigate', { url: base + p });
      await new Promise((r) => setTimeout(r, 400));
      const res = await cmd('Runtime.evaluate', { expression: probe, returnByValue: true });
      const issues = res.result?.result?.value ?? ['probe failed'];
      runs++;
      for (const i of issues) { failures++; console.error(`${p} at ${w}px ${theme}: ${i}`); }
    }
  }
}
ws.close(); proc.kill(); server.close();
console.log(`render check: ${runs} page and width runs, ${failures} problems`);
process.exit(failures ? 1 : 0);
