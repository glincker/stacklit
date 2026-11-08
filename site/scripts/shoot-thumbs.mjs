// Captures gallery thumbnails of each demo map in both themes.
// Needs agent-browser and ImageMagick. Serves public/ locally on a free port.
import { createServer } from 'node:http';
import { readFileSync, existsSync, statSync, mkdtempSync, rmSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { tmpdir } from 'node:os';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
const run = promisify(execFile);
import { fileURLToPath } from 'node:url';

const site = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const pub = join(site, 'public');
const demos = JSON.parse(readFileSync(join(site, 'src/data/demos.json'), 'utf8')).demos;
const AB = process.env.AGENT_BROWSER || 'agent-browser';
const tmp = mkdtempSync(join(tmpdir(), 'stacklit-thumbs-'));

const server = createServer((req, res) => {
  let p = join(pub, decodeURIComponent(req.url.split('?')[0]));
  if (existsSync(p) && statSync(p).isDirectory()) p = join(p, 'index.html');
  if (!existsSync(p)) { res.writeHead(404); return res.end(); }
  res.writeHead(200, { 'Content-Type': p.endsWith('.html') ? 'text/html' : 'application/octet-stream' });
  res.end(readFileSync(p));
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));
const port = server.address().port;
const ab = (...a) => run(AB, a);

try {
  await ab('set', 'viewport', '1280', '800');
  for (const scheme of ['light', 'dark']) {
    await ab('set', 'media', scheme);
    for (const d of demos) {
      await ab('open', `http://127.0.0.1:${port}/demos/${d.id}/`);
      await ab('wait', '1800');
      const png = join(tmp, `${d.id}-${scheme}.png`);
      await ab('screenshot', png);
      await run('magick', [png, '-resize', '800x500^', '-gravity', 'northwest', '-extent', '800x500', '-quality', '70', join(pub, 'demos', d.id, `thumb-${scheme}.webp`)]);
      console.log(`${d.id} ${scheme}`);
    }
  }
} finally {
  server.close();
  try { await ab('close'); } catch {}
  rmSync(tmp, { recursive: true, force: true });
}
