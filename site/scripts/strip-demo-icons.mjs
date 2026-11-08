// Makes the committed demo maps request nothing from outside: the same result as generating them
// with STACKLIT_MAP_ICONS=off (empty icon table, no image host in the CSP). Safe to run twice.
import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../public/demos');
let changed = 0;
for (const id of readdirSync(root)) {
  const file = join(root, id, 'index.html');
  if (!existsSync(file)) continue;
  const before = readFileSync(file, 'utf8');
  const after = before
    .replace(/img-src data: https:\/\/thesvg\.org https:\/\/cdn\.jsdelivr\.net;/, 'img-src data:;')
    .replace(/var ICON_BASES = \[[^\]]*\];\nvar MARK_SLUG = '[^']*';\nvar LANG_ICONS = \{[\s\S]*?\n\};/, "var LANG_ICONS = {}; var ICON_BASES = []; var MARK_SLUG = '';");
  if (after !== before) { writeFileSync(file, after); changed++; }
  if (/thesvg|jsdelivr/.test(after.replace(/\/\/[^\n]*thesvg[^\n]*/g, ''))) console.warn(`${id}: still mentions an outside host`);
}
console.log(`stripped remote icons from ${changed} demo maps`);
