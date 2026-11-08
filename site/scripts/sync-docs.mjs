// Builds src/content/docs and src/data/bench.json from the Markdown already in
// the repo, so the README, usage guide, architecture notes, bench results and
// changelog stay the single source of truth. Output is generated and gitignored.
// Every heading or table this script depends on is looked up by name and the
// build fails with a clear message when one is renamed or removed.
import { readFileSync, writeFileSync, mkdirSync, rmSync, existsSync } from 'node:fs';
import { resolve, dirname, posix } from 'node:path';
import { fileURLToPath } from 'node:url';
import GithubSlugger from 'github-slugger';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const out = resolve(here, '../src/content/docs/docs');
const GH = 'https://github.com/glincker/stacklit';

class SyncError extends Error {}
const fail = (msg) => { throw new SyncError(msg); };
process.on('uncaughtException', (e) => {
  if (!(e instanceof SyncError)) throw e;
  console.error(`\nsync-docs failed: ${e.message}\n`);
  process.exit(1);
});

const headingRe = /^(#{1,6})\s+(.*?)\s*#*\s*$/;

// A source file: its repo-relative path and its text.
function load(path) {
  const abs = resolve(root, path);
  if (!existsSync(abs)) fail(`missing source file ${path}. Update site/scripts/sync-docs.mjs if it moved.`);
  return { path, md: readFileSync(abs, 'utf8') };
}

function headings(md) {
  let inFence = false;
  const found = [];
  md.split('\n').forEach((line, i) => {
    if (/^```/.test(line)) inFence = !inFence;
    if (inFence) return;
    const m = headingRe.exec(line);
    if (m) found.push({ i, level: m[1].length, title: m[2] });
  });
  return found;
}

// Returns the body under a heading up to the next heading of the same or higher level.
// Throws when the heading is missing or its body is empty.
function section(doc, title, level = 2, { keepHeading = false } = {}) {
  const lines = doc.md.split('\n');
  const hs = headings(doc.md);
  const at = hs.findIndex((h) => h.level === level && h.title === title);
  if (at < 0) {
    const have = hs.filter((h) => h.level === level).map((h) => h.title).join(' | ');
    fail(`${doc.path}: no "${'#'.repeat(level)} ${title}" heading. Headings at that level: ${have || '(none)'}. A rename needs a matching edit in site/scripts/sync-docs.mjs.`);
  }
  const next = hs.slice(at + 1).find((h) => h.level <= level);
  const start = hs[at].i;
  const end = next ? next.i : lines.length;
  const body = lines.slice(start + 1, end).join('\n').trim();
  if (!body) fail(`${doc.path}: section "${title}" is empty.`);
  return lines.slice(keepHeading ? start : start + 1, end).join('\n').trim();
}

// The whole file without its top-level title.
const whole = (doc) => {
  const body = doc.md.replace(/^# .*\n/, '').trim();
  if (!body) fail(`${doc.path}: file is empty.`);
  return body;
};

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

const USAGE = load('docs/USAGE.md');
const README = load('README.md');
const ARCH = load('docs/ARCHITECTURE.md');
const FAQ = load('docs/FAQ.md');
const PRIVACY = load('docs/PRIVACY.md');
const LANGUAGES = load('docs/LANGUAGES.md');
const RESULTS = load('bench/RESULTS.md');
const AB_RESULTS = load('bench/agent-ab/RESULTS.md');
const AB_CLAIMS = load('bench/agent-ab/PROPOSED_CLAIMS.md');
const CHANGELOG = load('CHANGELOG.md');

// A page body is a list of parts, each remembering the file it came from so
// relative links are resolved from the right directory.
const part = (doc, md) => ({ src: doc.path, md });
const pages = [];
const add = (slug, title, description, order, source, parts) =>
  pages.push({ slug, title, description, order, source, parts, body: parts.map((p) => p.md).join('\n\n') });
const sec = (doc, title, level = 2, opts = {}) => part(doc, section(doc, title, level, opts));

add('quickstart', 'Quickstart',
  'Install Stacklit and generate a codebase index in one command.', 1, 'docs/USAGE.md', [
    part(USAGE, 'One command maps your repo. Other install routes are on the [install page](/docs/install/).'),
    sec(USAGE, 'First run', 2, { keepHeading: true }),
    part(README, '## What you get\n\n' + section(README, 'What you get')),
  ]);

add('how-it-works', 'How it works',
  'The Stacklit pipeline: scan, parse with tree-sitter, build modules and a dependency graph, render outputs.', 2,
  'docs/ARCHITECTURE.md', [part(ARCH, whole(ARCH))]);

add('commands', 'Commands reference',
  'Every stacklit command and flag: init, generate, view, diff, derive, export, setup, serve, query, check and more.', 3,
  'docs/USAGE.md', [part(USAGE, shift(section(USAGE, 'Command reference'), -1))]);

add('mcp', 'MCP tools',
  'The read-only tools the Stacklit MCP server exposes over the index and the symbol graph.', 4,
  'docs/USAGE.md', [sec(USAGE, 'MCP server')]);

// The README block carries logo markup for GitHub; the site shows the text only.
const assistantsText = section(README, 'Assistants').replace(/<p\b[\s\S]*?<\/p>\s*/g, '').trim();
if (!assistantsText) fail('README.md: the Assistants section has only logos, no text.');
add('assistants', 'Assistants and setup',
  'Wire Claude Code, Cursor, Aider and other AI tools to a Stacklit index with one command.', 5,
  'README.md', [
    part(README, '## Supported tools\n\n' + assistantsText),
    part(USAGE, shift(section(USAGE, 'stacklit setup', 3, { keepHeading: true }), -1)),
  ]);

add('configuration', 'Configuration',
  'The .stacklitrc.json keys and STACKLIT_ environment variables that control a Stacklit scan.', 6, 'docs/USAGE.md', [
    sec(USAGE, 'Configuration', 2, { keepHeading: true }),
    sec(USAGE, 'Environment variables', 2, { keepHeading: true }),
  ]);

const hasSessions = existsSync(resolve(root, 'docs/SESSIONS.md'));
const sessionsDoc = hasSessions ? load('docs/SESSIONS.md') : null;
add('sessions', 'Sessions and hooks',
  'Session persistence and hooks for Stacklit. Coming with 0.5.0.', 7,
  hasSessions ? 'docs/SESSIONS.md' : 'CHANGELOG.md',
  [hasSessions
    ? part(sessionsDoc, shift(whole(sessionsDoc), 0))
    : part(CHANGELOG, 'This page is coming with 0.5.0. Session persistence and the related hooks are not in a released version yet, so there is nothing to document as stable.\n\nThe git hook that regenerates the index after each commit is available today. See [Quickstart](/docs/quickstart/) and the `stacklit init --hook` flag in the [commands reference](/docs/commands/). Progress is tracked in the [changelog](/docs/changelog/).')]);

add('languages', 'Languages',
  'Which languages and formats Stacklit indexes and what it extracts from each.', 8,
  'docs/LANGUAGES.md', [part(LANGUAGES, whole(LANGUAGES))]);

add('privacy', 'Privacy',
  'Stacklit runs locally, has no telemetry and no account. What it reads, writes and sends.', 9,
  'docs/PRIVACY.md', [
    part(PRIVACY, whole(PRIVACY)),
    part(PRIVACY, 'To report a security issue, follow the [security policy](../.github/SECURITY.md).'),
  ]);

add('faq', 'FAQ',
  'Answers on small repos, grep, dependency accuracy, unsupported languages and git hooks.', 10,
  'docs/FAQ.md', [part(FAQ, whole(FAQ))]);

add('format', 'Index format',
  'How to read stacklit.json, and how Stacklit handles monorepos and several repos at once.', 11, 'docs/USAGE.md',
  ['Reading stacklit.json', 'Monorepo support', 'Polyrepo scanning'].map((t) => sec(USAGE, t, 2, { keepHeading: true })));

add('ci', 'CI and troubleshooting',
  'Keep the index fresh in CI with the GitHub Action or a hook, and fix common problems.', 12, 'docs/USAGE.md',
  ['CI/CD', 'Troubleshooting'].map((t) => sec(USAGE, t, 2, { keepHeading: true })));

add('benchmarks', 'Benchmarks',
  'A deterministic token-cost proxy on six pinned repos and 58 tasks, with intervals, the methodology, the misses and the limitations.', 13,
  'bench/RESULTS.md', [part(RESULTS, whole(RESULTS).replace(/<!--.*?-->\n?/g, ''))]);

// The real agent test: the results, then the wording the data does and does not support.
add('agent-test', 'Agent test',
  'A real Claude Code A/B on three public repos, 450 sessions: fewer tool calls, no overall cost saving. Results, method, limits and the claims we do and do not make.', 14,
  'bench/agent-ab/RESULTS.md', [
    part(AB_RESULTS, whole(AB_RESULTS)),
    part(AB_CLAIMS, '## What we claim, and what we do not\n\n' + shift(whole(AB_CLAIMS), 1)),
  ]);

add('install', 'Install',
  'Install Stacklit with npm, go install, a release binary or Homebrew.', 15, 'docs/USAGE.md', [sec(USAGE, 'Install', 2, { keepHeading: true })]);

add('changelog', 'Changelog', 'Release history for Stacklit.', 16, 'CHANGELOG.md', [part(CHANGELOG, whole(CHANGELOG))]);

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
  'docs/USAGE.md': '/docs/quickstart/',
  'docs/FAQ.md': '/docs/faq/',
  'docs/PRIVACY.md': '/docs/privacy/',
  'docs/LANGUAGES.md': '/docs/languages/',
  'docs/ARCHITECTURE.md': '/docs/how-it-works/',
  'bench/RESULTS.md': '/docs/benchmarks/',
  'bench/agent-ab/RESULTS.md': '/docs/agent-test/',
  'bench/agent-ab/PROPOSED_CLAIMS.md': '/docs/agent-test/',
  'CHANGELOG.md': '/docs/changelog/',
};
// Usage guide headings that moved to a differently named page.
const usageSection = { 'mcp-server': 'mcp', cicd: 'ci', 'monorepo-support': 'format' };

function fixLinks(p) {
  let fence = false;
  return p.parts
    .map((pt) => {
      const d = posix.dirname(pt.src);
      const sourceDir = d === '.' ? '' : d;
      return pt.md
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
            if (resolved === 'docs/USAGE.md' && hash) {
              const target = usageSection[hash] || anchorPage.get(hash) || 'commands';
              return `${a}/docs/${target}/${pageSlugs.get(target).has(hash) ? '#' + hash : ''}${c}`;
            }
            if (fileToPage[resolved]) return `${a}${fileToPage[resolved]}${hash && resolved !== 'docs/USAGE.md' ? '#' + hash : ''}${c}`;
            const kind = resolved.endsWith('/') || !/\.[a-z]+$/i.test(resolved) ? 'tree' : 'blob';
            return `${a}${GH}/${kind}/master/${resolved.replace(/\/$/, '')}${hash ? '#' + hash : ''}${c}`;
          });
        })
        .join('\n');
    })
    .join('\n\n');
}

rmSync(out, { recursive: true, force: true });
mkdirSync(out, { recursive: true });
for (const p of pages) {
  const body = fixLinks(p);
  const fm = [
    '---',
    `title: ${JSON.stringify(p.title)}`,
    `description: ${JSON.stringify(p.description)}`,
    'sidebar:',
    `  order: ${p.order}`,
    `source: ${JSON.stringify(p.source)}`,
    '---',
    '',
  ].join('\n');
  writeFileSync(resolve(out, `${p.slug}.md`), fm + body + '\n');
}
writeFileSync(resolve(out, 'index.md'), [
  '---', 'title: "Documentation"', 'description: "Quickstart, commands, MCP tools, configuration, privacy, FAQ and benchmarks for Stacklit."', 'sidebar:', '  hidden: true', '---', '',
  'Everything here is generated from the Markdown in the repository, so the site and the repo cannot drift apart.', '',
  ...pages.map((p) => `- [${p.title}](/docs/${p.slug}/): ${p.description}`), '',
].join('\n'));
console.log(`synced ${pages.length} docs pages`);

// Numbers for the home page come from the tables the benchmark harness generates
// in bench/RESULTS.md. Each table is found by its heading and each column by its
// header text, so a changed layout fails the build instead of showing stale data.
function table(doc, title) {
  const lines = doc.md.split('\n');
  const h = headings(doc.md).find((x) => x.level === 3 && x.title === title);
  if (!h) fail(`${doc.path}: no "### ${title}" heading. The harness (bench/report.go) or this script changed.`);
  const rows = [];
  for (let i = h.i + 1; i < lines.length; i++) {
    if (headingRe.test(lines[i]) && /^#{1,3}\s/.test(lines[i])) break;
    if (lines[i].startsWith('|')) rows.push(lines[i].split('|').slice(1, -1).map((c) => c.trim()));
  }
  if (rows.length < 3) fail(`${doc.path}: table under "${title}" has no data rows.`);
  const [header, , ...data] = rows;
  const col = (name) => {
    const i = header.indexOf(name);
    if (i < 0) fail(`${doc.path}: table "${title}" has no column "${name}". Columns: ${header.join(' | ')}`);
    return i;
  };
  return { col, data };
}

{
  const clean = (s) => s.replace(/`/g, '');
  const loc = table(RESULTS, 'All localization tasks (locate plus add-feature)');
  const want = {
    grep: 'grep+read',
    bm25: 'bm25 top-k',
    map: 'stacklit map+lookup',
    query: 'stacklit query 1000',
  };
  const cols = { arm: loc.col('Arm'), tasks: loc.col('Tasks'), f1: loc.col('found@1 [95% CI]'), f5: loc.col('found@5 [95% CI]'), tok: loc.col('tokens to found [95% CI]') };
  const arms = {};
  for (const [key, name] of Object.entries(want)) {
    const r = loc.data.find((x) => x[cols.arm] === name);
    if (!r) fail(`bench/RESULTS.md: arm "${name}" is missing from the localization table.`);
    arms[key] = { found1: r[cols.f1], found5: r[cols.f5], tokens: r[cols.tok] };
    arms[key].tasks = Number(r[cols.tasks]);
  }
  const taskCount = arms.grep.tasks;
  if (!taskCount) fail('bench/RESULTS.md: could not read the localization task count.');

  const paired = table(RESULTS, 'Paired differences (all localization tasks)');
  const pc = { a: paired.col('First arm'), b: paired.col('Second arm'), f1: paired.col('found@1 difference'), f5: paired.col('found@5 difference') };
  const pr = paired.data.find((x) => x[pc.a] === want.query && x[pc.b] === want.grep);
  if (!pr) fail('bench/RESULTS.md: paired row "stacklit query 1000 vs grep+read" is missing.');
  const pairedVsGrep = { found1: pr[pc.f1], found5: pr[pc.f5] };

  const gen = table(RESULTS, 'Repos and generation cost');
  const g = {};
  for (const [k, name] of Object.entries({ repo: 'Repo', lang: 'Lang', ref: 'Pinned commit', tasks: 'Tasks', lines: 'Lines', modules: 'Modules', indexTokens: 'stacklit.json tokens', deriveTokens: 'derive map tokens' })) g[k] = gen.col(name);
  const repos = gen.data.map((r) => ({
    repo: r[g.repo], lang: r[g.lang], ref: clean(r[g.ref]).replace(/^.*\((.*)\)$/, '$1'), tasks: r[g.tasks],
    lines: r[g.lines], modules: r[g.modules], indexTokens: r[g.indexTokens], deriveTokens: r[g.deriveTokens],
  }));
  if (!repos.length) fail('bench/RESULTS.md: no repos in the generation table.');

  const date = (RESULTS.md.match(/Run on (\d{4}-\d{2}-\d{2})/) || [])[1];
  if (!date) fail('bench/RESULTS.md: no "Run on <date>" line in the generated block.');
  const total = (RESULTS.md.match(/^(\d+) tasks \((\d+) localization, (\d+) dependents\), (\d+) repos, (\d+) arms/m) || []);

  writeFileSync(resolve(here, '../src/data/bench.json'), JSON.stringify({
    date, localizationTasks: taskCount, totalTasks: Number(total[1]) || null, repoCount: repos.length,
    arms, pairedQueryVsGrep: pairedVsGrep, repos,
  }, null, 2) + '\n');
}
