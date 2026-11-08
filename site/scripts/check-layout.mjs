// Fails when a landing section skips the layout primitives or the styling rules.
// Every file in src/sections must be built from <Section> (which wraps <Container>) and no
// source file may use a raw <section>, an inline style, a literal colour or a font family.
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, resolve, dirname, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const src = resolve(dirname(fileURLToPath(import.meta.url)), '../src');
const walk = (d) => readdirSync(d).flatMap((f) => (statSync(join(d, f)).isDirectory() ? walk(join(d, f)) : [join(d, f)]));
const files = walk(src);
const problems = [];
const rel = (f) => relative(resolve(src, '..'), f);

for (const f of files.filter((x) => x.startsWith(join(src, 'sections')) && x.endsWith('.astro'))) {
  const t = readFileSync(f, 'utf8');
  if (!/<Section[\s>]/.test(t)) problems.push(`${rel(f)}: a landing section must be built from <Section>`);
  if (/<section[\s>]/.test(t)) problems.push(`${rel(f)}: raw <section>, use <Section>`);
  if (/max-w-site|\bcontainer\b/.test(t)) problems.push(`${rel(f)}: width caps belong to <Container>, not to a section`);
}
for (const f of files.filter((x) => /\.(astro|css|ts|mjs)$/.test(x) && !x.endsWith('tokens.css') && !x.includes('/data/'))) {
  const t = readFileSync(f, 'utf8');
  if (/\sstyle=["{]/.test(t)) problems.push(`${rel(f)}: inline style attribute`);
  if (/#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b(?![\w-])/.test(t.replace(/&#\d+;|href="#[^"]*"|.*theme-color.*/g, '')) && !/\.(ts|mjs)$/.test(f)) problems.push(`${rel(f)}: literal colour, use a token from tokens.css`);
  if (/font-family\s*:\s*(?!var\()\S/.test(t)) problems.push(`${rel(f)}: font-family outside tokens.css`);
}
const pages = files.filter((x) => x.startsWith(join(src, 'pages')) && x.endsWith('.astro'));
for (const f of pages) {
  const t = readFileSync(f, 'utf8');
  if (/<section[\s>]/.test(t) || /<Section[\s>]/.test(t)) problems.push(`${rel(f)}: pages compose sections from src/sections`);
}
if (problems.length) {
  console.error(problems.join('\n'));
  process.exit(1);
}
console.log(`layout check passed (${files.filter((x) => x.startsWith(join(src, 'sections'))).length} sections)`);
