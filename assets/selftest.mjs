// Node check for the map's prompt generator: node assets/selftest.mjs path/to/stacklit.html
// (a generated map page; its embedded index and graph are used).
// It lifts the pure model and prompt code out of template.html, so no browser is needed.
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const tpl = readFileSync(join(here, 'template.html'), 'utf8');
const a = tpl.indexOf('/*PURE_BEGIN*/'), b = tpl.indexOf('/*PURE_END*/');
if (a < 0 || b < 0) { console.error('markers not found in template.html'); process.exit(2); }
const page = readFileSync(process.argv[2], 'utf8');
const grab = (id) => { const m = page.match(new RegExp('<script id="' + id + '" type="application/json">([\\s\\S]*?)</script>')); return m ? JSON.parse(m[1]) : null; };
const data = grab('stacklit-data');
let graph = grab('stacklit-graph');
if (!graph || !graph.nodes || !graph.nodes.length) graph = null;
const src = tpl.slice(a, b) + '\nreturn { selfCheck: selfCheck, ideas: ideas, M: M };';
const plural = (n, one, many) => n.toLocaleString() + ' ' + (n === 1 ? one : (many || one + 's'));
const LIM = { direct: 300 };
const fn = new Function('DATA', 'GRAPH', 'LIM', 'plural', 'catRank', src);
const api = fn(data, graph, LIM, plural, {});
const r = api.selfCheck();
console.log((r.problems.length ? 'FAIL' : 'PASS') + ' ' + r.checked + ' prompts, ' + api.ideas().length + ' findings, ' + api.M.length + ' modules');
r.problems.forEach((p) => console.log('  ' + p));
process.exit(r.problems.length ? 1 : 0);
