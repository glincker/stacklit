// Symbols tab: searchable list with signature, location and callers. Runs after the main script.
(function () {
  var el = document.getElementById('stacklit-graph');
  var G = null;
  try { G = JSON.parse(el.textContent); } catch (e) { G = null; }
  var SL = window.StackLit;
  var tab = document.getElementById('tab-symbols');
  if (!G || !G.nodes || !G.nodes.length || !tab || !SL) return;
  tab.hidden = false;

  var h = SL.h, plural = SL.plural, clear = SL.clear;
  var searchEl = document.getElementById('sym-search'), commEl = document.getElementById('sym-comm'), kindEl = document.getElementById('sym-kind');
  var listEl = document.getElementById('sym-list'), infoEl = document.getElementById('sym-info'), statEl = document.getElementById('sym-stat');
  var N = G.nodes, E = G.edges, KINDS = G.kinds || [], PAGE = 100;
  var out = N.map(function () { return []; }), inn = N.map(function () { return []; });
  E.forEach(function (e) { out[e[0]].push(e); inn[e[1]].push(e); });
  var deg = N.map(function (_, i) { return out[i].length + inn[i].length; });
  var results = [], shown = 0, current = -1;
  var cnt = {}; N.forEach(function (n) { cnt[n.c] = (cnt[n.c] || 0) + 1; });
  var commRank = {}; Object.keys(cnt).sort(function (a, b) { return cnt[b] - cnt[a]; }).forEach(function (c, i) { commRank[c] = i; });
  function commCls(c) { var r = commRank[c]; return r === undefined || r > 6 ? '' : ' c' + (r + 1); }

  function loc(n) { return G.files[n.f] + ':' + n.l; }
  function fits(n, q) {
    if (commEl.value !== '' && String(n.c) !== commEl.value) return false;
    if (kindEl.value !== '' && n.k !== kindEl.value) return false;
    if (!q) return true;
    return n.n.toLowerCase().indexOf(q) >= 0 || (n.s || '').toLowerCase().indexOf(q) >= 0 || (n.d || '').toLowerCase().indexOf(q) >= 0 || G.files[n.f].toLowerCase().indexOf(q) >= 0;
  }
  function item(i) {
    var n = N[i], b = h('button', 'sym-item'); b.type = 'button';
    b.setAttribute('role', 'listitem');
    if (i === current) b.setAttribute('aria-current', 'true');
    var tt = h('b'); tt.appendChild(SL.dot(commCls(n.c)));
    tt.appendChild(document.createTextNode(n.n)); b.appendChild(tt);
    b.appendChild(h('span', null, n.k + ', ' + loc(n)));
    b.addEventListener('click', function () { open(i); });
    return b;
  }
  function more() {
    var end = Math.min(results.length, shown + PAGE), old = listEl.querySelector('.more');
    if (old) old.remove();
    for (; shown < end; shown++) listEl.appendChild(item(results[shown]));
    if (shown < results.length) {
      var b = h('button', 'more', 'Show ' + Math.min(PAGE, results.length - shown).toLocaleString() + ' more'); b.type = 'button';
      b.addEventListener('click', more); listEl.appendChild(b);
    }
  }
  function refresh() {
    var q = searchEl.value.trim().toLowerCase();
    results = [];
    for (var i = 0; i < N.length; i++) if (fits(N[i], q)) results.push(i);
    results.sort(function (a, b) {
      if (q) { var ea = N[a].n.toLowerCase() === q ? 1 : 0, eb = N[b].n.toLowerCase() === q ? 1 : 0; if (ea !== eb) return eb - ea; }
      return deg[b] - deg[a] || a - b;
    });
    clear(listEl); shown = 0; more();
    statEl.textContent = plural(results.length, 'symbol') + (G.total > N.length ? ' (the ' + N.length.toLocaleString() + ' most connected of ' + G.total.toLocaleString() + ')' : '');
    if (!results.length) listEl.appendChild(h('p', 'empty-note', 'No symbols match.'));
  }
  function relRows(parent, title, edges, ends) {
    if (!edges.length) return;
    var box = h('div'); box.appendChild(h('h3', null, title + ' (' + edges.length.toLocaleString() + ')'));
    edges.slice().sort(function (a, b) { return b[3] - a[3]; }).slice(0, 60).forEach(function (e) {
      var o = e[ends], n = N[o], b = h('button', 'link-row'); b.type = 'button';
      b.appendChild(h('span', null, n.n));
      b.appendChild(h('span', 'meta', (KINDS[e[2]] || 'links') + (e[4] ? ', inferred' : '') + ', ' + loc(n)));
      b.addEventListener('click', function () { open(o); });
      box.appendChild(b);
    });
    if (edges.length > 60) box.appendChild(h('p', 'empty-note', plural(edges.length - 60, 'more') + ' not shown.'));
    parent.appendChild(box);
  }
  function show(i) {
    var n = N[i];
    clear(infoEl);
    infoEl.appendChild(h('h2', 'sym-name', n.n));
    infoEl.appendChild(h('p', 'note', n.k + ' at ' + loc(n) + (n.e > n.l ? ' to line ' + n.e : '')));
    var mi = SL.moduleForFile(G.files[n.f]);
    if (mi !== undefined) {
      var mb = h('button', 'more', 'Open module ' + SL.moduleId(mi)); mb.type = 'button';
      mb.addEventListener('click', function () { SL.selectModule(SL.moduleId(mi)); });
      infoEl.appendChild(mb);
    }
    if (n.s) { var pre = h('pre', 'sig'); pre.appendChild(SL.code(n.s, SL.langOfFile(G.files[n.f]))); infoEl.appendChild(pre); }
    if (n.d) infoEl.appendChild(h('p', 'mod-purpose', n.d));
    if (G.comms[n.c]) infoEl.appendChild(h('p', 'note', 'Cluster: ' + G.comms[n.c].label));
    var cols = h('div', 'cols');
    relRows(cols, 'Used by', inn[i], 0);
    relRows(cols, 'Uses', out[i], 1);
    infoEl.appendChild(cols);
    if (!inn[i].length && !out[i].length) infoEl.appendChild(h('p', 'empty-note', 'No callers or callees were found for this symbol.'));
  }
  function open(i, quiet) {
    if (i < 0 || i >= N.length) return;
    current = i; show(i);
    SL.state.sym = i;
    listEl.querySelectorAll('.sym-item').forEach(function (b) { b.removeAttribute('aria-current'); });
    var idx = results.indexOf(i);
    if (idx >= 0 && idx < shown) { var b = listEl.querySelectorAll('.sym-item')[idx]; if (b) b.setAttribute('aria-current', 'true'); }
    if (!quiet) { SL.state.mod = -1; SL.switchTab('symbols'); }
    SL.writeHash();
  }

  G.comms.forEach(function (c, i) {
    var o = document.createElement('option'); o.value = String(i); o.textContent = c.label; commEl.appendChild(o);
  });
  var kinds = {};
  N.forEach(function (n) { kinds[n.k] = 1; });
  Object.keys(kinds).sort().forEach(function (k) { var o = document.createElement('option'); o.value = k; o.textContent = k; kindEl.appendChild(o); });
  [searchEl, commEl, kindEl].forEach(function (c) { c.addEventListener('input', refresh); });
  infoEl.appendChild(h('p', 'note', 'Pick a symbol to see its signature, where it lives and what calls it.'));
  refresh();

  SL.symbolProvider = function (q, score) {
    var hits = [];
    for (var i = 0; i < N.length; i++) {
      var s = score(q, N[i].n);
      if (s > 0) hits.push({ s: s - 2, kind: 'Symbol', label: N[i].n, sub: loc(N[i]), run: (function (k) { return function () { open(k); }; })(i) });
    }
    hits.sort(function (a, b) { return b.s - a.s; });
    return hits.slice(0, 15);
  };
  document.addEventListener('stk:open-sym', function (ev) { open(ev.detail, true); });
  document.addEventListener('stk:module-symbols', function (ev) {
    searchEl.value = ev.detail === '.' ? '' : ev.detail + '/'; refresh(); SL.switchTab('symbols'); searchEl.focus();
  });
  document.dispatchEvent(new CustomEvent('stk:ready'));
})();
