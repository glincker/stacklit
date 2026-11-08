// Line classes for recorded command output, so the file headers read first and the detail lines recede.
export function queryClasses(text: string, offset = 0): Record<number, string> {
  const out: Record<number, string> = {};
  text.split('\n').forEach((l, i) => {
    const n = i + 1 + offset;
    if (/^\d+\. /.test(l)) out[n] = 'ln-file';
    else if (/^\s+calls /.test(l) || /^\[truncated/.test(l) || /^\d+ seeds/.test(l)) out[n] = 'ln-dim';
    else if (/^best match/.test(l)) out[n] = 'ln-key';
  });
  return out;
}
