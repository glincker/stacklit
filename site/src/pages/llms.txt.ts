import { getCollection } from 'astro:content';
import { SITE, REPO } from '../data/site';

export async function GET() {
  const docs = (await getCollection('docs')).sort((a, b) => a.data.order - b.data.order);
  const lines = [
    '# Stacklit',
    '',
    '> Stacklit parses a repository with tree-sitter and writes stacklit.json (modules, exports, dependencies, hints), an interactive HTML map and an MCP server. MIT licensed, free, local only, no telemetry, one Go binary, deterministic output.',
    '',
    '## Docs',
    '',
    ...docs.map((d) => `- [${d.data.title}](${SITE}/docs/${d.id}/): ${d.data.description}`),
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
