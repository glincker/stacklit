import { getCollection } from 'astro:content';
import { SITE } from '../data/site';

export async function GET() {
  const docs = await getCollection('docs');
  // Entry ids already start with docs/, and the docs index has the id docs.
  const paths = ['/', '/demos/', ...docs.map((d) => `/${d.id}/`)];
  const body = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${paths.map((p) => `  <url><loc>${SITE}${p}</loc></url>`).join('\n')}\n</urlset>\n`;
  return new Response(body, { headers: { 'Content-Type': 'application/xml; charset=utf-8' } });
}
