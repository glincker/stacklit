// Reads the real agent A/B numbers from bench/agent-ab/summary.json at build time.
// Every key is looked up by name and the build stops with a clear message when one is
// missing, so a schema change in the harness is caught in the PR build, not on the site.
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

// Builds run from site/, and the bundled chunk has no stable relative location.
const path = resolve(process.cwd(), '../bench/agent-ab/summary.json');

function fail(msg: string): never {
  throw new Error(`agent A/B summary (bench/agent-ab/summary.json): ${msg}. The harness output changed or site/src/data/agentAb.ts needs a matching edit.`);
}
if (!existsSync(path)) fail('file not found');

interface Ci { lo: number; hi: number }
interface Comparison { metric: string; n_pairs: number; ratio: number; ratio_ci95: Ci; mean_diff: number; diff_ci95: Ci; mean_control: number; mean_arm: number }
interface Rate { k: number; n: number; rate: number }
interface Scope {
  name: string; arm: string; n_pairs: number;
  comparisons: Comparison[];
  correctness: Record<string, Rate>;
  correctness_audited: Record<string, Rate>;
  by_kind: Record<string, Comparison[]>;
}
interface Summary { date: string; model: string; sessions_per_arm: Record<string, number>; scopes: Scope[] }

const raw = JSON.parse(readFileSync(path, 'utf8')) as Summary;
if (!raw || !Array.isArray(raw.scopes)) fail('no "scopes" array');
for (const k of ['date', 'model']) if (typeof (raw as unknown as Record<string, unknown>)[k] !== 'string') fail(`no "${k}" string`);
if (!raw.sessions_per_arm) fail('no "sessions_per_arm"');

const num = (v: unknown, where: string): number => (typeof v === 'number' && Number.isFinite(v) ? v : fail(`${where} is not a number`));

function scope(name: string, arm: string): Scope {
  const s = raw.scopes.find((x) => x.name === name && x.arm === arm);
  return s ?? fail(`no scope "${name}" for arm "${arm}"`);
}
function cmp(list: Comparison[] | undefined, metric: string, where: string): Comparison {
  const c = list?.find((x) => x.metric === metric);
  if (!c) return fail(`${where} has no metric "${metric}"`);
  num(c.ratio, `${where}/${metric}.ratio`); num(c.ratio_ci95?.lo, `${where}/${metric}.ratio_ci95.lo`); num(c.ratio_ci95?.hi, `${where}/${metric}.ratio_ci95.hi`);
  num(c.n_pairs, `${where}/${metric}.n_pairs`);
  return c;
}
function rate(s: Scope, set: 'correctness' | 'correctness_audited', arm: string): number {
  const r = s[set]?.[arm];
  if (!r) return fail(`scope ${s.name}/${s.arm} has no ${set}.${arm}`);
  return num(r.rate, `${s.name}/${set}.${arm}.rate`);
}

const r2 = (n: number) => n.toFixed(2);
const pct = (n: number) => (n * 100).toFixed(1);
export interface Interval { ratio: string; lo: string; hi: string; text: string }
function iv(c: Comparison): Interval {
  const ratio = r2(c.ratio), lo = r2(c.ratio_ci95.lo), hi = r2(c.ratio_ci95.hi);
  return { ratio, lo, hi, text: `${ratio} [${lo}, ${hi}]` };
}
// The verdict is read from the interval, never typed: below 1 all the way, above 1 all the way, or neither.
export type Verdict = 'cheaper' | 'dearer' | 'unclear';
const verdict = (c: Comparison): Verdict => (c.ratio_ci95.hi < 1 ? 'cheaper' : c.ratio_ci95.lo > 1 ? 'dearer' : 'unclear');

const pooled = scope('pooled', 'stacklit');
const strict = scope('pooled', 'strict');
const P = (m: string) => cmp(pooled.comparisons, m, 'pooled/stacklit');

const points = (c: Comparison) => (c.mean_diff * 100).toFixed(1);
const pointsCi = (c: Comparison) => `${(c.diff_ci95.lo * 100).toFixed(1)}, ${(c.diff_ci95.hi * 100).toFixed(1)}`;
const corr = cmp(pooled.comparisons, 'correct', 'pooled/stacklit');
const corrAud = cmp(pooled.comparisons, 'correct_audited', 'pooled/stacklit');
const kind = (k: string, m: string) => cmp(pooled.by_kind?.[k], m, `pooled/by_kind/${k}`);

export const ab = {
  date: raw.date,
  model: raw.model,
  sessions: num(raw.sessions_per_arm.control, 'sessions_per_arm.control') + num(raw.sessions_per_arm.stacklit, 'sessions_per_arm.stacklit') + num(raw.sessions_per_arm.strict, 'sessions_per_arm.strict'),
  pairs: pooled.n_pairs,
  cost: iv(P('cost_usd')),
  tokens: iv(P('total_tokens')),
  turns: iv(P('turns')),
  toolCalls: iv(P('tool_calls')),
  grepGlob: iv(P('grep_glob_calls')),
  correct: {
    control: pct(rate(pooled, 'correctness', 'control')), arm: pct(rate(pooled, 'correctness', 'stacklit')),
    diff: points(corr), diffCi: pointsCi(corr),
    auditedControl: pct(rate(pooled, 'correctness_audited', 'control')), auditedArm: pct(rate(pooled, 'correctness_audited', 'stacklit')),
  },
  auditedDiff: points(corrAud),
  repos: (['levelrail', 'theauth-go', 'featuredrop'] as const).map((name) => {
    const c = cmp(scope(name, 'stacklit').comparisons, 'cost_usd', `${name}/stacklit`);
    return { name, pairs: c.n_pairs, cost: iv(c), verdict: verdict(c) };
  }),
  kinds: {
    locate: { cost: iv(kind('locate', 'cost_usd')) },
    add: { cost: iv(kind('add', 'cost_usd')) },
    trace: { cost: iv(kind('trace', 'cost_usd')) },
    impact: { cost: iv(kind('impact', 'cost_usd')) },
  },
  traceCorrect: {
    control: pct(num(rawKindRate(pooled, 'trace', 'control'), 'trace control')),
    arm: pct(num(rawKindRate(pooled, 'trace', 'stacklit'), 'trace stacklit')),
  },
  strict: {
    cost: iv(cmp(strict.comparisons, 'cost_usd', 'pooled/strict')),
    tokens: iv(cmp(strict.comparisons, 'total_tokens', 'pooled/strict')),
    correct: pct(rate(strict, 'correctness', 'strict')),
  },
};

// Correctness by task kind sits in the per-kind comparisons as the mean of the arm.
function rawKindRate(s: Scope, k: string, arm: 'control' | 'stacklit'): number {
  const c = cmp(s.by_kind?.[k], 'correct', `pooled/by_kind/${k}`);
  return arm === 'control' ? num(c.mean_control, `${k}.correct.mean_control`) : num(c.mean_arm, `${k}.correct.mean_arm`);
}

// Fills {name} placeholders in copy strings. An unknown name fails the build.
export function fill(text: string): string {
  const vars: Record<string, string> = {
    sessions: String(ab.sessions), pairs: String(ab.pairs),
    pooledCost: ab.cost.text, pooledTokens: ab.tokens.ratio,
    toolCalls: ab.toolCalls.text, turns: ab.turns.text, grepGlob: ab.grepGlob.text,
    levelrail: ab.repos[0].cost.text, theauth: ab.repos[1].cost.text, featuredrop: ab.repos[2].cost.text, levelrailRatio: ab.repos[0].cost.text,
    locate: ab.kinds.locate.cost.text, add: ab.kinds.add.cost.text, trace: ab.kinds.trace.cost.text, impact: ab.kinds.impact.cost.text,
    traceArm: ab.traceCorrect.arm, traceControl: ab.traceCorrect.control,
    strictCost: ab.strict.cost.text, strictTokens: ab.strict.tokens.ratio, strictCorrect: ab.strict.correct,
  };
  return text.replace(/\{(\w+)\}/g, (_, k: string) => vars[k] ?? fail(`copy placeholder {${k}} has no value`));
}
