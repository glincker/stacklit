// Builds the demo gallery: real stacklit maps for pinned public repos.
// Binary: STACKLIT_BIN if set, else built from this checkout (merging
// origin/feat/map-redesign first while that branch is not on the default branch).
// Clones go to the OS temp dir and are deleted at the end.
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync, mkdirSync, copyFileSync, writeFileSync, readFileSync, statSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const siteDir = resolve(here, '..');
const repoRoot = resolve(siteDir, '..');

const REPOS = [
  { id: 'gin', url: 'https://github.com/gin-gonic/gin.git', ref: 'v1.10.0', lang: 'Go' },
  { id: 'flask', url: 'https://github.com/pallets/flask.git', ref: '3.1.2', lang: 'Python' },
  { id: 'hono', url: 'https://github.com/honojs/hono.git', ref: 'v4.6.0', lang: 'TypeScript' },
  { id: 'ripgrep', url: 'https://github.com/BurntSushi/ripgrep.git', ref: '14.1.1', lang: 'Rust' },
  { id: 'gson', url: 'https://github.com/google/gson.git', ref: 'gson-parent-2.11.0', lang: 'Java' },
  { id: 'rack', url: 'https://github.com/rack/rack.git', ref: 'v3.1.8', lang: 'Ruby' },
];
const MAX_BYTES = Number(process.env.DEMO_MAX_BYTES || 3 * 1024 * 1024);

const run = (cmd, args, opts = {}) =>
  execFileSync(cmd, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], ...opts });

const tmp = mkdtempSync(join(tmpdir(), 'stacklit-demos-'));
let worktree = null;
const cleanup = () => {
  try {
    if (worktree) run('git', ['-C', repoRoot, 'worktree', 'remove', '--force', worktree]);
  } catch {}
  try { run('git', ['-C', repoRoot, 'worktree', 'prune']); } catch {}
  rmSync(tmp, { recursive: true, force: true });
};
process.on('exit', cleanup);
process.on('SIGINT', () => process.exit(1));

function buildBinary() {
  if (process.env.STACKLIT_BIN) return resolve(process.env.STACKLIT_BIN);
  const bin = join(tmp, 'stacklit');
  worktree = join(tmp, 'src');
  run('git', ['-C', repoRoot, 'worktree', 'add', '--detach', worktree, 'HEAD']);
  const redesign = 'origin/feat/map-redesign';
  let present = true;
  try { run('git', ['-C', repoRoot, 'rev-parse', '--verify', redesign]); } catch { present = false; }
  let merged = !present;
  if (present) {
    try { run('git', ['-C', repoRoot, 'merge-base', '--is-ancestor', redesign, 'HEAD']); merged = true; } catch {}
  }
  if (!merged) {
    try {
      run('git', ['-c', 'user.name=build', '-c', 'user.email=build@localhost', 'merge', '--no-edit', redesign], { cwd: worktree });
    } catch (e) {
      throw new Error(`merging ${redesign} conflicts; set STACKLIT_BIN to a prebuilt binary (${String(e.stderr || e).slice(0, 200)})`);
    }
  }
  run('go', ['build', '-o', bin, './cmd/stacklit'], { cwd: worktree });
  return bin;
}

const bin = buildBinary();
const version = run(bin, ['--version']).trim();
console.log(`binary: ${version}`);

const outRoot = join(siteDir, 'public', 'demos');
const demos = [];
const env = { ...process.env };
delete env.STACKLIT_MAP_ICONS; // the default: logos requested from thesvg.org

for (const r of REPOS) {
  const dir = join(tmp, r.id);
  run('git', ['clone', '--quiet', '--depth', '1', '--branch', r.ref, r.url, dir]);
  const sha = run('git', ['-C', dir, 'rev-parse', 'HEAD']).trim();
  const t0 = process.hrtime.bigint();
  run(bin, ['generate', '--graph', '--quiet'], { cwd: dir, env });
  const ms = Math.round(Number(process.hrtime.bigint() - t0) / 1e6);
  const idx = JSON.parse(readFileSync(join(dir, 'stacklit.json'), 'utf8'));
  let graph = { nodes: 0, edges: 0 };
  const gp = join(dir, 'stacklit.graph.json');
  if (existsSync(gp)) {
    const g = JSON.parse(readFileSync(gp, 'utf8'));
    graph = { nodes: (g.nodes || g.symbols || []).length, edges: (g.edges || g.links || []).length };
  }
  const langs = idx.tech?.languages || {};
  const files = Object.values(langs).reduce((a, l) => a + (l.files || 0), 0);
  const lines = Object.values(langs).reduce((a, l) => a + (l.lines || 0), 0);
  const html = join(dir, 'stacklit.html');
  const size = statSync(html).size;
  const entry = {
    id: r.id,
    name: r.id,
    language: r.lang,
    ref: r.ref,
    sha: sha.slice(0, 10),
    url: r.url.replace(/\.git$/, ''),
    files,
    lines,
    modules: Object.keys(idx.modules || {}).length,
    graphNodes: graph.nodes,
    graphEdges: graph.edges,
    generateMs: ms,
    htmlBytes: size,
    committed: size <= MAX_BYTES,
  };
  mkdirSync(join(outRoot, r.id), { recursive: true });
  copyFileSync(html, join(outRoot, r.id, 'index.html'));
  demos.push(entry);
  console.log(`${r.id}: ${files} files, ${entry.modules} modules, ${graph.nodes} nodes, ${ms} ms, ${(size / 1024).toFixed(0)} KB`);
  if (r.id === 'gin') {
    // Real command output for the home page, captured from the same checkout.
    const cold = { ...env, STACKLIT_NO_CACHE: '1' };
    const sha = () => run('shasum', ['stacklit.json'], { cwd: dir }).split(/\s+/)[0];
    const g = spawnSync(bin, ['generate', '--graph'], { cwd: dir, env: cold, encoding: 'utf8' });
    const generate = (g.stderr || '') + (g.stdout || '');
    const h1 = sha();
    run(bin, ['generate', '--quiet'], { cwd: dir, env: cold });
    const h2 = sha();
    const question = 'how are route params bound to a handler';
    const mod = idx.modules.render;
    writeFileSync(
      join(siteDir, 'src/data/samples.json'),
      JSON.stringify({
        derive: run(bin, ['derive'], { cwd: dir, env }),
        query: run(bin, ['query', question, '--budget', '500'], { cwd: dir, env }).split('\n').filter((l) => /^(query |\d+ seeds|[\w./-]+\.\w+$|  \d|      calls)/.test(l)).slice(0, 13).join('\n'),
        question,
        diff: run(bin, ['diff'], { cwd: dir, env }).trim(),
        hashFirst: h1,
        hashSecond: h2,
        identical: h1 === h2,
        moduleName: 'render',
        module: { purpose: mod.purpose, language: mod.language, files: mod.files, lines: mod.lines, exports: (mod.exports || []).slice(0, 4), depends_on: mod.depends_on, depended_by: mod.depended_by },
        generateOut: generate.trim(),
      }, null, 2) + '\n',
    );
  }
  rmSync(dir, { recursive: true, force: true });
}

writeFileSync(
  join(siteDir, 'src/data/demos.json'),
  JSON.stringify({ binary: version, demos }, null, 2) + '\n',
);
console.log('wrote src/data/demos.json');
