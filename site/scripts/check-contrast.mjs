// Reads the colour tokens from src/styles/tokens.css and fails when a text or accent
// pair drops below WCAG AA (4.5 to 1) in either theme.
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const css = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../src/styles/tokens.css'), 'utf8');
const block = (sel) => {
  const start = css.indexOf(sel + ' {');
  const body = css.slice(start, css.indexOf('}', start));
  return Object.fromEntries([...body.matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6})/g)].map((m) => [m[1], m[2]]));
};
const light = block(':root');
const themes = { light, dark: { ...light, ...block(":root[data-theme='dark']") } };

const lum = (h) => {
  const c = [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
};
const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };

const grounds = ['paper', 'surface', 'raised', 'sunk'];
const pairs = [
  ...['ink', 'muted', 'faint', 'accent', 'accent-text', 'bad', 'warn'].flatMap((fg) => grounds.map((bg) => [fg, bg])),
  ['accent-ink', 'accent'], ['accent-text', 'accent-soft'], ['ink', 'accent-soft'],
  ['term-text', 'term'], ['term-dim', 'term'], ['term-accent', 'term'],
  ['code-keyword', 'sunk'], ['code-fn', 'sunk'], ['code-comment', 'sunk'],
  ...['code-keyword', 'code-fn', 'code-comment', 'tok-string', 'tok-const', 'tok-punct', 'ink'].flatMap((fg) => ['tok-hl', 'tok-add', 'tok-del'].map((bg) => [fg, bg])),
  ...['tok-string', 'tok-const', 'tok-punct'].map((fg) => [fg, 'sunk']),
];
let bad = 0;
for (const [name, t] of Object.entries(themes)) {
  for (const [fg, bg] of pairs) {
    const r = ratio(t[fg], t[bg]);
    const ok = r >= 4.5;
    if (!ok) bad++;
    if (process.argv.includes('--table') || !ok) console.log(`${name.padEnd(5)} ${fg.padEnd(12)} on ${bg.padEnd(8)} ${t[fg]} ${t[bg]} ${r.toFixed(2)} ${ok ? 'AA' : 'FAIL'}`);
  }
}
console.log(`${pairs.length * 2} pairs checked, ${bad} below 4.5:1`);
process.exit(bad ? 1 : 0);
