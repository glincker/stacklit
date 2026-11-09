// Reads the round 2 agent A/B numbers from bench/agent-ab/round2/summary.json at build time.
// Round 1 lives in agentAb.ts and is left alone. Keys are looked up by name and a missing one stops the build.
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

const path = resolve(process.cwd(), '../bench/agent-ab/round2/summary.json');

function fail(msg: string): never {
  throw new Error(`agent A/B round 2 summary (bench/agent-ab/round2/summary.json): ${msg}. The harness output changed or site/src/data/agentAbRound2.ts needs a matching edit.`);
}
if (!existsSync(path)) fail('file not found');

interface Ci { lo: number; hi: number }
interface Comparison { metric: string; ratio: number; ratio_ci95: Ci }
interface Rate { rate: number }
interface Scope { name: string; arm: string; comparisons: Comparison[]; correctness: Record<string, Rate> }
interface Summary { sessions_per_arm: Record<string, number>; scopes: Scope[] }

const raw = JSON.parse(readFileSync(path, 'utf8')) as Summary;
if (!raw || !Array.isArray(raw.scopes) || !raw.sessions_per_arm) fail('no "scopes" array or "sessions_per_arm"');

const num = (v: unknown, where: string): number => (typeof v === 'number' && Number.isFinite(v) ? v : fail(`${where} is not a number`));

// The file also holds an e-against-a block under the same scope names; the first match is the one against control.
function cmp(name: string, arm: string, metric: string): Comparison {
  const s = raw.scopes.find((x) => x.name === name && x.arm === arm);
  const c = s?.comparisons.find((x) => x.metric === metric);
  if (!s || !c) return fail(`no "${metric}" for scope "${name}" arm "${arm}"`);
  num(c.ratio, `${name}/${arm}/${metric}.ratio`);
  num(c.ratio_ci95?.lo, `${name}/${arm}/${metric}.lo`);
  num(c.ratio_ci95?.hi, `${name}/${arm}/${metric}.hi`);
  return c;
}
const r2 = (n: number) => n.toFixed(2);
export interface Interval { ratio: number; text: string }
function iv(c: Comparison): Interval {
  return { ratio: c.ratio, text: `${r2(c.ratio)} [${r2(c.ratio_ci95.lo)}, ${r2(c.ratio_ci95.hi)}]` };
}
function rate(scopeArm: string, who: string): string {
  const s = raw.scopes.find((x) => x.name === 'pooled' && x.arm === scopeArm);
  return (num(s?.correctness?.[who]?.rate, `pooled/${scopeArm} correctness.${who}`) * 100).toFixed(1);
}

export const REPOS = [
  { name: 'levelrail', size: 'about 16,000 files' },
  { name: 'theauth', size: 'about 2,900 files' },
  { name: 'theauth-go', size: 'about 1,300 files' },
  { name: 'featuredrop', size: 'about 400 files' },
] as const;

const repoRows = REPOS.map((r) => ({
  ...r,
  a: iv(cmp(r.name, 'stacklit', 'cost_usd')),
  e: iv(cmp(r.name, 'stacklit-e', 'cost_usd')),
}));
const smallRatios = repoRows.filter((r) => r.name !== 'levelrail').flatMap((r) => [r.a.ratio, r.e.ratio]);
const pooledA = iv(cmp('pooled', 'stacklit', 'cost_usd'));
const pooledE = iv(cmp('pooled', 'stacklit-e', 'cost_usd'));
const pctUp = (ratio: number) => String(Math.round((ratio - 1) * 100));
const pctDown = (ratio: number) => String(Math.round((1 - ratio) * 100));

export const ab2 = {
  sessions:
    num(raw.sessions_per_arm.control, 'control') + num(raw.sessions_per_arm.stacklit, 'stacklit') +
    num(raw.sessions_per_arm['stacklit-e'], 'stacklit-e') + num(raw.sessions_per_arm['stacklit-e-small'], 'stacklit-e-small'),
  repoRows,
  pooledA,
  pooledE,
  levelrailDropA: pctDown(repoRows[0].a.ratio),
  levelrailDropE: pctDown(repoRows[0].e.ratio),
  smallRiseMin: pctUp(Math.min(...smallRatios)),
  smallRiseMax: pctUp(Math.max(...smallRatios)),
  pooledRiseE: pctUp(pooledE.ratio),
  pooledRiseA: pctUp(pooledA.ratio),
  grepFewerA: pctDown(cmp('pooled', 'stacklit', 'grep_glob_calls').ratio),
  toolFewerA: pctDown(cmp('pooled', 'stacklit', 'tool_calls').ratio),
  correctControl: rate('stacklit', 'control'),
  correctA: rate('stacklit', 'stacklit'),
  correctE: rate('stacklit-e', 'stacklit-e'),
};
