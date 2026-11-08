(function () {
  var el = document.getElementById('stacklit-graph');
  var G = null;
  try { G = JSON.parse(el.textContent); } catch (e) { G = null; }
  var tab = document.getElementById('tab-symbols');
  var view = document.getElementById('view-symbols');
  if (!G || !G.nodes || !G.nodes.length || !tab || !view) return;
  tab.hidden = false;

  var canvas = document.getElementById('sym-canvas');
  var ctx = canvas.getContext('2d');
  var searchEl = document.getElementById('sym-search');
  var commEl = document.getElementById('sym-comm');
  var kindEl = document.getElementById('sym-kind');
  var listEl = document.getElementById('sym-list');
  var infoEl = document.getElementById('sym-info');
  var statEl = document.getElementById('sym-stat');

  var N = G.nodes, E = G.edges, KINDS = G.kinds;
  var out = N.map(function () { return []; });
  var inn = N.map(function () { return []; });
  E.forEach(function (e, i) { out[e[0]].push(i); inn[e[1]].push(i); });
  var deg = N.map(function (_, i) { return out[i].length + inn[i].length; });
  var members = G.comms.map(function () { return []; });
  N.forEach(function (n, i) { if (members[n.c]) members[n.c].push(i); });
  members.forEach(function (m) { m.sort(function (a, b) { return deg[b] - deg[a] || a - b; }); });

  var visible = new Set();
  var selected = -1;
  var items = [], links = [], pos = {};
  var panX = 0, panY = 0, scale = 1, W = 0, H = 0, ticks = 0, raf = 0;

  function hue(c) { return (c * 137.508) % 360; }
  function color(c, a) { return 'hsla(' + hue(c).toFixed(0) + ',55%,58%,' + (a || 1) + ')'; }
  function disp(i) { return visible.has(i) ? 's' + i : 'c' + N[i].c; }
  function fits(i) {
    var q = searchEl.value.trim().toLowerCase();
    if (commEl.value !== '' && String(N[i].c) !== commEl.value) return false;
    if (kindEl.value !== '' && N[i].k !== kindEl.value) return false;
    if (q && N[i].n.toLowerCase().indexOf(q) < 0 && (N[i].s || '').toLowerCase().indexOf(q) < 0 && (N[i].d || '').toLowerCase().indexOf(q) < 0) return false;
    return true;
  }

  function rebuild() {
    var map = {};
    items = [];
    G.comms.forEach(function (c, ci) {
      if (commEl.value !== '' && String(ci) !== commEl.value && !members[ci].some(function (i) { return visible.has(i); })) return;
      var rest = members[ci].filter(function (i) { return !visible.has(i); }).length;
      var it = { id: 'c' + ci, comm: ci, label: c.label + (rest < members[ci].length ? ' (+' + rest + ')' : ''), r: 8 + Math.sqrt(members[ci].length) * 2.2, sym: -1 };
      map[it.id] = it; items.push(it);
    });
    visible.forEach(function (i) {
      var it = { id: 's' + i, comm: N[i].c, label: N[i].n, r: 5 + Math.min(10, Math.sqrt(deg[i])), sym: i };
      map[it.id] = it; items.push(it);
    });
    items.forEach(function (it) {
      var p = pos[it.id];
      if (!p) { var a = Math.random() * 6.283; p = pos[it.id] = { x: Math.cos(a) * 160 + (Math.random() - 0.5) * 20, y: Math.sin(a) * 160 }; }
      it.x = p.x; it.y = p.y; it.vx = 0; it.vy = 0;
    });
    var agg = {};
    E.forEach(function (e) {
      if (e[2] === 1) return;
      var a = disp(e[0]), b = disp(e[1]);
      if (a === b || !map[a] || !map[b]) return;
      var k = a + '|' + b;
      if (!agg[k]) agg[k] = { a: map[a], b: map[b], n: 0, inf: true };
      agg[k].n++;
      if (!e[4]) agg[k].inf = false;
    });
    links = Object.keys(agg).map(function (k) { return agg[k]; });
    statEl.textContent = visible.size + ' symbols shown of ' + N.length + (G.total > N.length ? ' (top ' + N.length + ' of ' + G.total + ')' : '');
    ticks = 0; kick();
  }

  function step() {
    var n = items.length, i, j, a, b, dx, dy, d2, d, f;
    for (i = 0; i < n; i++) {
      a = items[i];
      for (j = i + 1; j < n; j++) {
        b = items[j]; dx = a.x - b.x; dy = a.y - b.y; d2 = dx * dx + dy * dy + 0.01;
        if (d2 > 90000) continue;
        f = 1800 / d2; d = Math.sqrt(d2);
        a.vx += dx / d * f; a.vy += dy / d * f; b.vx -= dx / d * f; b.vy -= dy / d * f;
      }
      a.vx -= a.x * 0.004; a.vy -= a.y * 0.004;
    }
    links.forEach(function (l) {
      dx = l.b.x - l.a.x; dy = l.b.y - l.a.y; d = Math.sqrt(dx * dx + dy * dy) + 0.01;
      f = (d - 90) * 0.01;
      l.a.vx += dx / d * f; l.a.vy += dy / d * f; l.b.vx -= dx / d * f; l.b.vy -= dy / d * f;
    });
    items.forEach(function (it) {
      it.vx *= 0.6; it.vy *= 0.6; it.x += it.vx; it.y += it.vy;
      pos[it.id] = { x: it.x, y: it.y };
    });
  }

  function draw() {
    var dpr = window.devicePixelRatio || 1;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, W, H);
    ctx.setTransform(dpr * scale, 0, 0, dpr * scale, dpr * (W / 2 + panX), dpr * (H / 2 + panY));
    links.forEach(function (l) {
      ctx.beginPath(); ctx.moveTo(l.a.x, l.a.y); ctx.lineTo(l.b.x, l.b.y);
      ctx.strokeStyle = l.inf ? 'rgba(139,148,158,0.35)' : 'rgba(139,148,158,0.7)';
      ctx.setLineDash(l.inf ? [4, 3] : []);
      ctx.lineWidth = Math.min(4, 0.6 + Math.log(1 + l.n)) / scale + 0.2;
      ctx.stroke();
    });
    ctx.setLineDash([]);
    ctx.font = (11 / scale) + 'px sans-serif';
    ctx.textAlign = 'center';
    items.forEach(function (it) {
      var dim = it.sym >= 0 && !fits(it.sym);
      ctx.beginPath(); ctx.arc(it.x, it.y, it.r, 0, 6.283);
      ctx.fillStyle = color(it.comm, it.sym < 0 ? 0.35 : dim ? 0.2 : 0.95); ctx.fill();
      ctx.lineWidth = (it.sym === selected ? 3 : 1) / scale;
      ctx.strokeStyle = it.sym === selected ? '#f0f6fc' : color(it.comm, 1); ctx.stroke();
      ctx.fillStyle = dim ? 'rgba(230,237,243,0.35)' : '#e6edf3';
      ctx.fillText(it.label, it.x, it.y + it.r + 12 / scale);
    });
  }

  function kick() { if (!raf) raf = requestAnimationFrame(loop); }
  function loop() {
    raf = 0;
    if (!view.classList.contains('active')) return;
    if (ticks < 160) { step(); ticks++; }
    draw();
    if (ticks < 160) kick();
  }

  function resize() {
    var r = canvas.parentNode.getBoundingClientRect();
    var dpr = window.devicePixelRatio || 1;
    W = Math.max(200, r.width); H = Math.max(200, r.height);
    canvas.width = W * dpr; canvas.height = H * dpr;
    kick();
  }
  function toWorld(ev) {
    var r = canvas.getBoundingClientRect();
    return { x: (ev.clientX - r.left - W / 2 - panX) / scale, y: (ev.clientY - r.top - H / 2 - panY) / scale };
  }
  function hit(ev) {
    var p = toWorld(ev), best = null;
    for (var i = items.length - 1; i >= 0; i--) {
      var it = items[i];
      if ((p.x - it.x) * (p.x - it.x) + (p.y - it.y) * (p.y - it.y) <= (it.r + 3) * (it.r + 3)) { best = it; break; }
    }
    return best;
  }

  function expandCommunity(ci) {
    members[ci].slice(0, 40).forEach(function (i) { visible.add(i); });
  }
  function expandSymbol(i) {
    visible.add(i);
    out[i].concat(inn[i]).slice(0, 80).forEach(function (ei) {
      var e = E[ei];
      if (e[2] === 1) return;
      visible.add(e[0]); visible.add(e[1]);
    });
  }

  function rel(i) {
    var rows = [];
    out[i].forEach(function (ei) { rows.push([E[ei], E[ei][1], 'calls ']); });
    inn[i].forEach(function (ei) { rows.push([E[ei], E[ei][0], 'used by ']); });
    return rows;
  }

  function show(i) {
    selected = i;
    var n = N[i];
    infoEl.textContent = '';
    function add(tag, text, cls) { var d = document.createElement(tag); d.textContent = text; if (cls) d.className = cls; infoEl.appendChild(d); return d; }
    add('div', n.n, 'sym-name');
    add('div', n.k + '  ' + G.files[n.f] + ':' + n.l + (n.e > n.l ? '-' + n.e : ''), 'sym-meta');
    add('div', 'community: ' + G.comms[n.c].label, 'sym-meta');
    if (n.s) add('code', n.s, 'sym-sig');
    if (n.d) add('div', n.d, 'sym-meta');
    rel(i).slice(0, 40).forEach(function (r) {
      var e = r[0], o = r[1];
      var verb = KINDS[e[2]] || 'links';
      var d = add('div', verb + (r[2] === 'used by ' ? ' (incoming) ' : ' ') + N[o].n + '  ' + (e[4] ? 'I' : 'E') + e[3].toFixed(2), 'sym-rel');
      d.addEventListener('click', function () { expandSymbol(o); show(o); rebuild(); });
    });
  }

  function refreshList() {
    var q = searchEl.value.trim();
    listEl.textContent = '';
    if (!q && commEl.value === '' && kindEl.value === '') return;
    var res = [];
    for (var i = 0; i < N.length && res.length < 30; i++) if (fits(i)) res.push(i);
    res.sort(function (a, b) { return deg[b] - deg[a] || a - b; });
    res.forEach(function (i) {
      var d = document.createElement('div');
      d.className = 'sym-hit';
      d.textContent = N[i].n + '  ' + G.files[N[i].f] + ':' + N[i].l;
      d.addEventListener('click', function () { expandSymbol(i); show(i); rebuild(); });
      listEl.appendChild(d);
    });
  }

  G.comms.forEach(function (c, i) {
    var o = document.createElement('option'); o.value = String(i); o.textContent = c.label + ' (' + members[i].length + ')'; commEl.appendChild(o);
  });
  var kinds = {};
  N.forEach(function (n) { kinds[n.k] = 1; });
  Object.keys(kinds).sort().forEach(function (k) { var o = document.createElement('option'); o.value = k; o.textContent = k; kindEl.appendChild(o); });
  [searchEl, commEl, kindEl].forEach(function (c) { c.addEventListener('input', function () { refreshList(); kick(); }); });

  var drag = null, moved = false;
  canvas.addEventListener('mousedown', function (ev) { drag = { x: ev.clientX, y: ev.clientY, px: panX, py: panY }; moved = false; });
  window.addEventListener('mousemove', function (ev) {
    if (!drag) return;
    if (Math.abs(ev.clientX - drag.x) + Math.abs(ev.clientY - drag.y) > 3) moved = true;
    panX = drag.px + ev.clientX - drag.x; panY = drag.py + ev.clientY - drag.y; kick();
  });
  window.addEventListener('mouseup', function () { drag = null; });
  canvas.addEventListener('wheel', function (ev) { ev.preventDefault(); scale = Math.min(4, Math.max(0.2, scale * (ev.deltaY < 0 ? 1.1 : 0.9))); kick(); }, { passive: false });
  canvas.addEventListener('click', function (ev) {
    if (moved) return;
    var it = hit(ev);
    if (!it) return;
    if (it.sym < 0) { expandCommunity(it.comm); } else { expandSymbol(it.sym); show(it.sym); }
    rebuild();
  });

  tab.addEventListener('click', function () {
    document.querySelectorAll('.tab-btn').forEach(function (b) { b.classList.toggle('active', b === tab); });
    document.querySelectorAll('.view').forEach(function (v) { v.classList.toggle('active', v === view); });
    var fit = document.getElementById('btn-fit'); if (fit) fit.style.display = 'none';
    resize(); rebuild();
  });
  window.addEventListener('resize', function () { if (view.classList.contains('active')) resize(); });
})();
