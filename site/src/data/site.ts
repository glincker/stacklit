export const SITE = 'https://stacklit.glinr.com';
export const REPO = 'https://github.com/glincker/stacklit';
export const NPM = 'https://www.npmjs.com/package/stacklit';
export const NAME = 'Stacklit';
export const TAGLINE = 'A committed map of your codebase, built for AI coding agents.';

// The home page chapters, in page order. The rail and the section kickers read from here.
export const chapters = [
  { id: 'overview', label: 'Overview' },
  { id: 'results', label: 'Results' },
  { id: 'works', label: 'Works with' },
  { id: 'problem', label: 'The problem' },
  { id: 'ask', label: 'Ask the map' },
  { id: 'how', label: 'How it works' },
  { id: 'agent-test', label: 'Agent test' },
  { id: 'helps', label: 'Where it helps' },
  { id: 'benchmark', label: 'Benchmark' },
  { id: 'facts', label: 'Plain facts' },
  { id: 'faq', label: 'Questions' },
  { id: 'try', label: 'Try it' },
] as const;
