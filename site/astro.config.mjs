import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://stacklit.glinr.com',
  output: 'static',
  trailingSlash: 'always',
  build: { format: 'directory', inlineStylesheets: 'always' },
  compressHTML: true,
  devToolbar: { enabled: false },
  prefetch: false,
  markdown: { syntaxHighlight: false },
});
