export function GET() {
  return new Response('User-agent: *\nAllow: /\n\nSitemap: https://stacklit.glinr.com/sitemap.xml\n', { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
}
