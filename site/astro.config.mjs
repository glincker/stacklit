import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import tailwindcss from '@tailwindcss/vite';

// Starlight serves only the docs routes (/docs/...). The landing page, demos and 404
// are plain Astro pages in src/pages, so Starlight never owns their layout.
const slugs = [
  'quickstart', 'how-it-works', 'commands', 'mcp', 'assistants', 'configuration', 'sessions',
  'languages', 'privacy', 'faq', 'format', 'ci', 'benchmarks', 'agent-test', 'install', 'changelog',
];

export default defineConfig({
  site: 'https://stacklit.glinr.com',
  output: 'static',
  trailingSlash: 'always',
  build: { format: 'directory', inlineStylesheets: 'auto' },
  compressHTML: true,
  devToolbar: { enabled: false },
  prefetch: false,
  vite: { plugins: [tailwindcss()] },
  integrations: [
    starlight({
      title: 'Stacklit',
      description: 'Stacklit maps your repo once so AI coding agents search less. Documentation.',
      favicon: '/favicon.svg',
      logo: { light: './src/assets/mark-light.svg', dark: './src/assets/mark-dark.svg', alt: 'Stacklit' },
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/glincker/stacklit' }],
      customCss: ['./src/styles/docs.css'],
      lastUpdated: false,
      pagination: true,
      disable404Route: true,
      sidebar: [{ label: 'Documentation', items: slugs.map((s) => ({ slug: `docs/${s}` })) }],
    }),
  ],
});
