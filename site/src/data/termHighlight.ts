// A small build time highlighter for Stacklit's own terminal output. Colour carries meaning, one class per role:
// prompt, command, flag, path, symbol, string or number, success, warning, error, dim, ranked file, score.
// The roles map to tokens in styles/tokens.css and styles/code.css. No client code.
const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
const sp = (c: string, t: string) => (t === '' ? '' : `<span class="${c}">${esc(t)}</span>`);
const plain = (t: string) => esc(t);

function command(rest: string): string {
  let first = true;
  return rest.replace(/("[^"]*"|\s+|\S+)/g, (m) => {
    if (/^\s+$/.test(m)) return m;
    if (m.startsWith('"')) return sp('t-str', m);
    if (m.startsWith('-')) return sp('t-flag', m);
    if (first) { first = false; return sp('t-cmd', m); }
    return plain(m);
  });
}

function signature(sig: string): string {
  const m = sig.match(/^(func|type|struct|interface)\b(.*)$/);
  if (!m) return plain(sig);
  let rest = m[2];
  let out = sp('t-kw', m[1]);
  if (m[1] === 'func') {
    const r = rest.match(/^(\s+\([^)]*\))?(\s+)(\w+)(.*)$/);
    if (r) return out + plain(r[1] ?? '') + plain(r[2]) + sp('t-sym', r[3]) + plain(r[4]);
  } else {
    const r = rest.match(/^(\s+)(\w+)(.*)$/);
    if (r) return out + plain(r[1]) + sp('t-sym', r[2]) + plain(r[3]);
  }
  return out + plain(rest);
}

function target(t: string): string {
  const [file, sym] = t.split('::');
  return sym ? sp('t-path', file) + sp('t-dim', '::') + sp('t-sym', sym) : sp('t-sym', t);
}

function line(l: string): string {
  let m: RegExpMatchArray | null;
  if ((m = l.match(/^\$ (.*)$/))) return sp('t-prompt', '$ ') + command(m[1]);
  if ((m = l.match(/^(\d+)\. (\S+)$/))) return sp('t-dim', `${m[1]}. `) + sp('t-rank', m[2]);
  if ((m = l.match(/^best match: (\S+) (\S+)$/))) return sp('t-dim', 'best match: ') + sp('t-path', m[1]) + ' ' + sp('t-sym', m[2]);
  if (/^\d+ seeds, \d+ candidates$/.test(l) || /^query "/.test(l) || /^injected digest/.test(l)) return sp('t-dim', l);
  if ((m = l.match(/^\[truncated: (.*?)(--budget \d+)(.*)$/))) return sp('t-dim', `[truncated: ${m[1]}`) + sp('t-flag', m[2]) + sp('t-dim', m[3]);
  if ((m = l.match(/^(\s+)(create|install) (.*)$/))) return plain(m[1]) + sp('t-ok', m[2]) + ' ' + sp('t-path', m[3]);
  if ((m = l.match(/^(\s+)calls (\S+) ([EI]\d\.\d+)$/))) return plain(m[1]) + sp('t-dim', 'calls ') + target(m[2]) + ' ' + sp('t-score', m[3]);
  if ((m = l.match(/^(\s+)(\d+(?:-\d+)?) (.*)$/))) {
    const [sig, ...c] = m[3].split('  // ');
    return plain(m[1]) + sp('t-dim', m[2]) + ' ' + signature(sig) + (c.length ? sp('t-dim', '  // ' + c.join('  // ')) : '');
  }
  if ((m = l.match(/^([\w./-]+):(\d+)$/))) return sp('t-path', m[1]) + sp('t-dim', ':' + m[2]);
  if ((m = l.match(/^(\.\/[^:]+):(\d+):(.*)$/))) return sp('t-path', m[1]) + sp('t-dim', ':') + sp('t-num', m[2]) + sp('t-dim', ':') + sp('t-str', m[3]);
  if ((m = l.match(/^impact (.*?): (.*)$/))) return sp('t-dim', 'impact ') + sp('t-path', m[1]) + sp('t-dim', ': ' + m[2]);
  if ((m = l.match(/^\[stacklit\] (.*)$/))) return sp('t-dim', '[stacklit] ') + plain(m[1]).replace(/\b(\d+)\b/g, '<span class="t-num">$1</span>');
  if (/^Done, /.test(l)) return sp('t-ok', l);
  if (/^do-not-touch:/.test(l)) return sp('t-warn', 'do-not-touch:') + sp('t-path', l.slice('do-not-touch:'.length));
  if (/^(test|modules|env):/.test(l) || /\| env:/.test(l)) return plain(l).replace(/\b(test|env|modules):/g, '<span class="t-dim">$1:</span>');
  if ((m = l.match(/^(\w+) \| (\w+) \| (.*)$/))) return sp('t-rank', m[1]) + sp('t-dim', ' | ') + plain(m[2]) + sp('t-dim', ' | ') + sp('t-dim', m[3]);
  if ((m = l.match(/^(\s+)(\S+)(\s+)(\S+)(.*)$/))) {
    const tail = plain(m[5]).replace(/(depends:|depended-by:)/g, '<span class="t-dim">$1</span>');
    return plain(m[1]) + sp('t-path', m[2]) + plain(m[3]) + sp('t-sym', m[4]) + tail;
  }
  if (/^[A-Z][A-Za-z ]+$/.test(l)) return sp('t-cmd', l);
  return plain(l);
}

export function termHtml(code: string): string {
  return code.split('\n').map((l) => `<span class="line">${line(l)}</span>`).join('\n');
}
