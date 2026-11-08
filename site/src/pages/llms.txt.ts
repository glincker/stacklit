import { getCollection } from 'astro:content';
import { SITE, REPO } from '../data/site';
import copy from '../data/copy.json';

export async function GET() {
  const docs = (await getCollection('docs'))
    .filter((d) => d.id !== 'docs')
    .sort((a, b) => (a.data.sidebar.order ?? 99) - (b.data.sidebar.order ?? 99));
  const lines = [
    '# Stacklit',
    '',
    `> ${copy.meta.llms}`,
    '',
    '## Docs',
    '',
    ...docs.map((d) => `- [${d.data.title}](${SITE}/${d.id}/): ${d.data.description}`),
    '',
    '## More',
    '',
    `- [Demos](${SITE}/demos/): generated maps for gin, flask, hono, ripgrep, gson and rack`,
    `- [Source code](${REPO}): MIT license`,
    `- [Security policy](${REPO}/blob/master/.github/SECURITY.md)`,
    '',
  ];
  return new Response(lines.join('\n'), { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
}
