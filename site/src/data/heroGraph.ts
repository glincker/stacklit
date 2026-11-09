// Build time geometry for the hero picture, from the real gin v1.10.0 file graph in hero-graph.json.
// Deterministic: a seeded start and a fixed number of force steps, so the same data always draws the same picture.
// No runtime physics and no dependency. Class names are written out in full so Tailwind finds them.
import raw from './hero-graph.json';

export const W = 520;
export const H = 520;

interface Graph {
  repo: string;
  files: [string, number, number][];
  edges: [number, number, number][];
}
const g = raw as Graph;
const n = g.files.length;

// Small seeded generator (mulberry32).
function rng(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// PageRank on the weighted, undirected file links.
function pageRank(): number[] {
  const adj: [number, number][][] = Array.from({ length: n }, () => []);
  for (const [a, b, w] of g.edges) { adj[a].push([b, w]); adj[b].push([a, w]); }
  const out = adj.map((l) => l.reduce((s, [, w]) => s + w, 0));
  let r = new Array(n).fill(1 / n);
  for (let it = 0; it < 60; it++) {
    const next = new Array(n).fill(0.15 / n);
    for (let i = 0; i < n; i++) {
      if (out[i] === 0) { for (let j = 0; j < n; j++) next[j] += (0.85 * r[i]) / n; continue; }
      for (const [j, w] of adj[i]) next[j] += (0.85 * r[i] * w) / out[i];
    }
    r = next;
  }
  return r;
}
const rank = pageRank();
const maxRank = Math.max(...rank);

// Force layout: links pull, nodes repel, a weak gravity keeps the cloud together. Files with no links are
// not part of the simulation; they sit on a loose outer ring, so the picture stays honest and readable.
function layout(): [number, number][] {
  const rand = rng(85);
  const linked = new Set<number>();
  for (const [a, b] of g.edges) { linked.add(a); linked.add(b); }
  const ids = [...linked].sort((a, b) => a - b);
  const comms = [...new Set(g.files.map((f) => f[1]))].sort((a, b) => a - b);
  const pos: [number, number][] = g.files.map((f) => {
    const ang = (comms.indexOf(f[1]) / comms.length) * Math.PI * 2;
    return [Math.cos(ang) * 0.5 + (rand() - 0.5) * 0.4, Math.sin(ang) * 0.5 + (rand() - 0.5) * 0.4];
  });
  const k = 0.25;
  for (let step = 0; step < 500; step++) {
    const cool = 0.06 * (1 - step / 500) + 0.002;
    const disp = new Map<number, [number, number]>(ids.map((i) => [i, [0, 0]]));
    for (let x = 0; x < ids.length; x++) {
      for (let y = x + 1; y < ids.length; y++) {
        const i = ids[x], j = ids[y];
        let dx = pos[i][0] - pos[j][0];
        let dy = pos[i][1] - pos[j][1];
        const d = Math.max(Math.hypot(dx, dy), 0.01);
        const f = (k * k) / d;
        dx /= d; dy /= d;
        disp.get(i)![0] += dx * f; disp.get(i)![1] += dy * f;
        disp.get(j)![0] -= dx * f; disp.get(j)![1] -= dy * f;
      }
    }
    for (const [a, b, w] of g.edges) {
      const dx = pos[a][0] - pos[b][0];
      const dy = pos[a][1] - pos[b][1];
      const d = Math.max(Math.hypot(dx, dy), 0.01);
      const f = ((d * d) / k) * (0.5 + Math.min(w, 6) * 0.2);
      disp.get(a)![0] -= (dx / d) * f; disp.get(a)![1] -= (dy / d) * f;
      disp.get(b)![0] += (dx / d) * f; disp.get(b)![1] += (dy / d) * f;
    }
    for (const i of ids) {
      const dd = disp.get(i)!;
      dd[0] -= pos[i][0] * 1.2; dd[1] -= pos[i][1] * 1.2;
      const d = Math.max(Math.hypot(dd[0], dd[1]), 0.001);
      const m = Math.min(d, cool);
      pos[i][0] += (dd[0] / d) * m;
      pos[i][1] += (dd[1] / d) * m;
    }
  }
  const xs = ids.map((i) => pos[i][0]);
  const ys = ids.map((i) => pos[i][1]);
  const [x0, x1, y0, y1] = [Math.min(...xs), Math.max(...xs), Math.min(...ys), Math.max(...ys)];
  const inner = 0.74;
  const s = Math.min((W * inner) / (x1 - x0), (H * inner) / (y1 - y0));
  const ox = (W - (x1 - x0) * s) / 2;
  const oy = (H - (y1 - y0) * s) / 2;
  const out: [number, number][] = pos.map((p) => [ox + (p[0] - x0) * s, oy + (p[1] - y0) * s]);
  const lone = g.files.map((_, i) => i).filter((i) => !linked.has(i));
  lone.forEach((i, n) => {
    const ang = -Math.PI / 2 + ((n + 0.5) / lone.length) * Math.PI * 2 + (rand() - 0.5) * 0.2;
    out[i] = [W / 2 + Math.cos(ang) * (W / 2 - 34), H / 2 + Math.sin(ang) * (H / 2 - 34)];
  });
  return out;
}
const pos = layout();
const r1 = (v: number) => Math.round(v * 10) / 10;

const order = rank.map((v, i) => [v, i] as const).sort((a, b) => b[0] - a[0]);
const hubSet = new Set(order.slice(0, 6).map(([, i]) => i));
const base = (p: string) => p.split('/').pop() as string;

// Node fills by community, from tokens only. Literal strings so Tailwind generates them.
const nodeFill = ['fill-accent', 'fill-ink', 'fill-muted', 'fill-faint', 'fill-line-strong', 'fill-line-strong'];
const blockFill = ['fill-accent/45', 'fill-ink/25', 'fill-muted/25', 'fill-faint/20', 'fill-line-strong/25', 'fill-line-strong/25'];

export const nodes = g.files.map((f, i) => ({
  x: r1(pos[i][0]),
  y: r1(pos[i][1]),
  r: r1(3 + Math.sqrt(rank[i] / maxRank) * 15),
  cls: nodeFill[Math.min(f[1], nodeFill.length - 1)],
  hub: hubSet.has(i),
  label: f[0],
  short: base(f[0]),
}));
// Hub edges are drawn apart so a slow pulse can run over them.
const hubEdges: string[] = [];
const plain: string[] = [];
for (const [a, b] of g.edges) {
  const d = `M${r1(pos[a][0])} ${r1(pos[a][1])}L${r1(pos[b][0])} ${r1(pos[b][1])}`;
  (hubSet.has(a) || hubSet.has(b) ? hubEdges : plain).push(d);
}
export const edgePath = plain.join('');
export const hubEdgePath = hubEdges.join('');
export const hubs = order.slice(0, 6).map(([, i]) => ({ ...nodes[i], r: nodes[i].r }));

// Folder and community block map: a squarified treemap, folders first, then files, area by symbol count.
interface Item { name: string; size: number; children?: Item[]; cls?: string }
function squarify(items: Item[], x: number, y: number, w: number, h: number): { item: Item; x: number; y: number; w: number; h: number }[] {
  const sorted = [...items].sort((a, b) => b.size - a.size || a.name.localeCompare(b.name));
  const total = sorted.reduce((s, i) => s + i.size, 0);
  const out: { item: Item; x: number; y: number; w: number; h: number }[] = [];
  let rest = sorted;
  let [cx, cy, cw, ch] = [x, y, w, h];
  while (rest.length) {
    const area = (cw * ch) / rest.reduce((s, i) => s + i.size, 0);
    const side = Math.min(cw, ch);
    let row: Item[] = [];
    let worst = Infinity;
    for (const it of rest) {
      const next = [...row, it];
      const sum = next.reduce((s, i) => s + i.size, 0) * area;
      const mx = Math.max(...next.map((i) => i.size)) * area;
      const mn = Math.min(...next.map((i) => i.size)) * area;
      const ratio = Math.max((side * side * mx) / (sum * sum), (sum * sum) / (side * side * mn));
      if (ratio > worst) break;
      worst = ratio;
      row = next;
    }
    const sum = row.reduce((s, i) => s + i.size, 0) * area;
    const thick = sum / side;
    let off = 0;
    for (const it of row) {
      const len = (it.size * area) / thick;
      out.push(cw >= ch ? { item: it, x: cx, y: cy + off, w: thick, h: len } : { item: it, x: cx + off, y: cy, w: len, h: thick });
      off += len;
    }
    if (cw >= ch) { cx += thick; cw -= thick; } else { cy += thick; ch -= thick; }
    rest = rest.slice(row.length);
  }
  void total;
  return out;
}
const folderOf = (p: string) => (p.includes('/') ? p.slice(0, p.lastIndexOf('/')) : 'root');
const folders = new Map<string, Item>();
g.files.forEach((f, i) => {
  const key = folderOf(f[0]);
  const it = folders.get(key) ?? { name: key, size: 0, children: [] };
  it.children!.push({ name: base(f[0]), size: f[2] + 3, cls: blockFill[Math.min(f[1], blockFill.length - 1)] });
  it.size += f[2] + 3;
  folders.set(key, it);
});
const gap = 3;
export const blocks: { x: number; y: number; w: number; h: number; cls: string }[] = [];
export const folderLabels: { x: number; y: number; w: number; text: string }[] = [];
const top = squarify([...folders.values()], 0, 0, W, H);
for (const f of top) {
  const ix = f.x + gap, iy = f.y + gap, iw = f.w - gap * 2, ih = f.h - gap * 2;
  folderLabels.push({ x: r1(ix + 6), y: r1(iy + 17), w: iw, text: f.item.name });
  const head = ih > 60 ? 24 : 0;
  for (const c of squarify(f.item.children!, ix, iy + head, iw, ih - head)) {
    blocks.push({ x: r1(c.x + 0.75), y: r1(c.y + 0.75), w: r1(Math.max(c.w - 1.5, 1)), h: r1(Math.max(c.h - 1.5, 1)), cls: c.item.cls as string });
  }
}
export const folderOutlines = top.map((f) => ({ x: r1(f.x + gap), y: r1(f.y + gap), w: r1(f.w - gap * 2), h: r1(f.h - gap * 2) }));

export const stats = { repo: g.repo, files: n, edges: g.edges.length, folders: folders.size };
export const hubNames = hubs.map((h) => h.label);
