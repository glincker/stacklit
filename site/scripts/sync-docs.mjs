// Builds src/content/docs from the Markdown already in the repo, so the
// README, USAGE, ARCHITECTURE, bench results and changelog stay the single
// source of truth. Output is generated and gitignored.
import { readFileSync, writeFileSync, mkdirSync, rmSync, existsSync } from 'node:fs';
import { resolve, dirname, posix } from 'node:path';
import { fileURLToPath } from 'node:url';
import GithubSlugger from 'github-slugger';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const out = resolve(here, '../src/content/docs');
const GH = 'https://github.com/glincker/stacklit';
const read = (p) => readFileSync(resolve(root, p), 'utf8');

const headingRe = /^(#{1,6})\s+(.*?)\s*#*\s*$/;

// Returns the body under `## title` (or the given level) up to the next heading of the same or higher level.
function section(md, title, level = 2, { keepHeading = false } = {}) {
  const lines = md.split('\n');
  let inFence = false;
  let start = -1;
  let end = lines.length;
  for (let i = 0; i < lines.length; i++) {
    if (/^```/.test(lines[i])) inFence = !inFence;
    if (inFence) continue;
    const m = headingRe.exec(lines[i]);
    if (!m) continue;
    if (start < 0) {
      if (m[1].length === level && m[2] === title) start = i;
    } else if (m[1].length <= level) {
      end = i;
      break;
    }
  }
  if (start < 0) throw new Error(`section not found: ${title}`);
  return lines.slice(keepHeading ? start : start + 1, end).join('\n').trim();
}

// Moves every heading by `delta` levels (outside code fences).
function shift(md, delta) {
  let inFence = false;
  return md
    .split('\n')
    .map((l) => {
      if (/^```/.test(l)) inFence = !inFence;
      if (inFence) return l;
      const m = headingRe.exec(l);
      if (!m) return l;
      const n = Math.min(6, Math.max(2, m[1].length + delta));
      return `${'#'.repeat(n)} ${m[2]}`;
    })
    .join('\n');
}

const USAGE = read('USAGE.md');
const README = read('README.md');

const pages = [];
const add = (slug, title, description, order, source, body) =>
  pages.push({ slug, title, description, order, source, body });

add('quickstart', 'Quickstart',
  'Install Stacklit and generate a codebase index in one command.', 1, 'README.md',
  [
    section(USAGE, 'Install', 2, { keepHeading: true }),
    section(USAGE, 'First run', 2, { keepHeading: true }),
    '## What you get\n\n' + section(README, 'What you get'),
    '## Keeping the index fresh\n\n' + section(README, 'Keeping the index fresh'),
  ].join('\n\n'));

add('how-it-works', 'How it works',
  'The Stacklit pipeline: scan, parse with tree-sitter, build modules and a dependency graph, render outputs.', 2,
  'docs/ARCHITECTURE.md', shift(read('docs/ARCHITECTURE.md').replace(/^# .*\n/, ''), 0));

add('commands', 'Commands reference',
  'Every stacklit command and flag: init, generate, view, diff, derive, export, setup, serve, query, check and more.', 3,
  'USAGE.md', shift(section(USAGE, 'Command reference'), -1));

add('mcp', 'MCP tools',
  'The fifteen read-only tools the Stacklit MCP server exposes over the index and the symbol graph.', 4,
  'USAGE.md', section(USAGE, 'MCP server'));

add('assistants', 'Assistants and setup',
  'Wire Claude Code, Cursor, Aider and other AI tools to a Stacklit index with one command.', 5,
  'README.md',
  '## Supported tools\n\n' + section(README, 'Supported AI tools') + '\n\n' + shift(section(README, 'Set up your AI tools'), -1));

add('configuration', 'Configuration',
  'The .stacklitrc.json keys and STACKLIT_ environment variables that control a Stacklit scan.', 6,
  'USAGE.md',
  section(USAGE, 'Configuration', 2, { keepHeading: true }) + '\n\n' + section(USAGE, 'Environment variables', 2, { keepHeading: true }));

const sessionsPath = resolve(root, 'docs/SESSIONS.md');
add('sessions', 'Sessions and hooks',
  'Session persistence and hooks for Stacklit. Coming with 0.5.0.', 7,
  existsSync(sessionsPath) ? 'docs/SESSIONS.md' : 'CHANGELOG.md',
  existsSync(sessionsPath)
    ? shift(read('docs/SESSIONS.md').replace(/^# .*\n/, ''), 0)
    : 'This page is coming with 0.5.0. Session persistence and the related hooks are not in a released version yet, so there is nothing to document as stable.\n\nThe git hook that regenerates the index after each commit is available today. See [Quickstart](/docs/quickstart/) and the `stacklit init --hook` flag in the [commands reference](/docs/commands/). Progress is tracked in the [changelog](/docs/changelog/).');

add('privacy', 'Privacy',
  'Stacklit runs locally, has no telemetry and no account. What it reads, writes and sends.', 8,
  'README.md', section(README, 'Privacy') + '\n\nTo report a security issue, follow [SECURITY.md](SECURITY.md).');

add('faq', 'FAQ',
  'Answers on small repos, grep, dependency accuracy, unsupported languages and git hooks.', 9,
  'README.md', section(README, 'FAQ'));

add('format', 'Index format',
  'How to read stacklit.json, and how Stacklit handles monorepos and several repos at once.', 10,
  'USAGE.md',
  ['Reading stacklit.json', 'Monorepo support', 'Polyrepo scanning'].map((t) => section(USAGE, t, 2, { keepHeading: true })).join('\n\n'));

add('ci', 'CI and troubleshooting',
  'Keep the index fresh in CI with the GitHub Action or a hook, and fix common problems.', 11,
  'USAGE.md',
  ['CI/CD', 'Troubleshooting'].map((t) => section(USAGE, t, 2, { keepHeading: true })).join('\n\n'));

add('benchmarks', 'Benchmarks',
  'A deterministic token-cost proxy on three pinned repos, with the methodology, the misses and the limitations.', 12,
  'bench/RESULTS.md', read('bench/RESULTS.md').replace(/^# .*\n/, '').replace(/<!--.*?-->\n?/g, ''));

add('changelog', 'Changelog', 'Release history for Stacklit.', 13, 'CHANGELOG.md',
  read('CHANGELOG.md').replace(/^# .*\n/, '').replace(/^(?!## ).*\n(?=)/, (m) => m).trim());

// Anchor index: slug -> page, so USAGE cross-references land on the right page.
const anchorPage = new Map();
const pageSlugs = new Map();
for (const p of pages) {
  const s = new GithubSlugger();
  const set = new Set();
  let fence = false;
  for (const l of p.body.split('\n')) {
    if (/^```/.test(l)) fence = !fence;
    if (fence) continue;
    const m = headingRe.exec(l);
    if (m) {
      const id = s.slug(m[2].replace(/[`*_]/g, ''));
      set.add(id);
      if (!anchorPage.has(id)) anchorPage.set(id, p.slug);
    }
  }
  pageSlugs.set(p.slug, set);
}

const fileToPage = {
  'README.md': '/',
  'USAGE.md': '/docs/quickstart/',
  'docs/ARCHITECTURE.md': '/docs/how-it-works/',
  'bench/RESULTS.md': '/docs/benchmarks/',
  'CHANGELOG.md': '/docs/changelog/',
};
const usageSection = { 'mcp-server': 'mcp', cicd: 'ci', 'monorepo-support': 'format' };

function fixLinks(p) {
  const base = posix.dirname(p.source === 'CHANGELOG.md' ? 'CHANGELOG.md' : p.source);
  const sourceDir = p.source.startsWith('bench/') ? 'bench' : p.source.startsWith('docs/') ? 'docs' : '';
  let fence = false;
  return p.body
    .split('\n')
    .map((line) => {
      if (/^```/.test(line)) fence = !fence;
      if (fence) return line;
      return line.replace(/(\]\()([^)\s]+)(\))/g, (all, a, href, c) => {
        if (/^(https?:|mailto:|\/)/.test(href)) return all;
        if (href.startsWith('#')) {
          const id = href.slice(1);
          if (pageSlugs.get(p.slug).has(id)) return all;
          const target = usageSection[id] || anchorPage.get(id);
          return target ? `${a}/docs/${target}/${pageSlugs.get(target).has(id) ? '#' + id : ''}${c}` : `${a}/docs/${p.slug}/${c}`;
        }
        const [pathPart, hash] = href.split('#');
        const resolved = posix.normalize(posix.join(sourceDir, pathPart));
        if (resolved === 'USAGE.md' && hash) {
          const target = usageSection[hash] || anchorPage.get(hash) || 'commands';
          return `${a}/docs/${target}/${pageSlugs.get(target).has(hash) ? '#' + hash : ''}${c}`;
        }
        if (fileToPage[resolved]) return `${a}${fileToPage[resolved]}${hash && resolved !== 'USAGE.md' ? '#' + hash : ''}${c}`;
        const kind = resolved.endsWith('/') || !/\.[a-z]+$/i.test(resolved) ? 'tree' : 'blob';
        return `${a}${GH}/${kind}/master/${resolved.replace(/\/$/, '')}${hash ? '#' + hash : ''}${c}`;
      });
    })
    .join('\n');
}

rmSync(out, { recursive: true, force: true });
mkdirSync(out, { recursive: true });
for (const p of pages) {
  const body = fixLinks(p);
  const fm = [
    '---',
    `title: ${JSON.stringify(p.title)}`,
    `description: ${JSON.stringify(p.description)}`,
    `order: ${p.order}`,
    `source: ${JSON.stringify(p.source)}`,
    '---',
    '',
  ].join('\n');
  writeFileSync(resolve(out, `${p.slug}.md`), fm + body + '\n');
}
console.log(`synced ${pages.length} docs pages`);

// Numbers for the home page come straight from the generated table in bench/RESULTS.md.
{
  const md = read('bench/RESULTS.md');
  const block = md.split('### Index size and generation cost')[1]?.split('\n\n')[1] ?? '';
  const rows = block
    .split('\n')
    .slice(2)
    .filter((l) => l.startsWith('|'))
    .map((l) => l.split('|').slice(1, -1).map((c) => c.trim()));
  const num = (s) => s.replace(/`/g, '');
  const repos = rows.map((r) => ({
    repo: r[0], lang: r[1], ref: num(r[2]), files: r[3], lines: r[4], modules: r[5],
    indexTokens: r[7], deriveTokens: r[8], genMedianMs: r[10],
  }));
  const agg = md.split('### Aggregate')[1] ?? '';
  const all = agg.split('\n').find((l) => l.startsWith('| **all repos, all tasks**'));
  const c = all ? all.split('|').slice(1, -1).map((x) => x.trim()) : [];
  const date = (md.match(/Run on (\d{4}-\d{2}-\d{2})/) || [])[1] || '';
  writeFileSync(resolve(here, '../src/data/bench.json'), JSON.stringify({
    date, repos,
    allTasks: { tasks: c[1], grepLookup: c[2], stacklitLookup: c[4], grepFound: c[7], stacklitFound: c[8] },
  }, null, 2) + '\n');
}
