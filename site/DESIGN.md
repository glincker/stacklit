---
version: alpha
name: Stacklit
description: Plain, warm, technical. A paper page that is a map of the product: coordinates, contour hairlines, and real output as the terrain.
colors:
  primary: "#0a6b5b"
  on-primary: "#ffffff"
  accent-text: "#075446"
  accent-soft: "#d5eae3"
  paper: "#f5f3ee"
  surface: "#fbfaf7"
  raised: "#ffffff"
  sunk: "#ebe8e0"
  line: "#d9d5c9"
  ink: "#1b1a17"
  muted: "#55524a"
  faint: "#615d53"
  bad: "#9b2c1d"
  term: "#1b1a17"
  term-text: "#ece9df"
  dark-paper: "#151410"
  dark-ink: "#ece9df"
  dark-primary: "#5fd0b5"
  dark-on-primary: "#0a1f1a"
typography:
  hero:
    fontFamily: system-ui
    fontSize: 4rem
    fontWeight: 700
    lineHeight: 1.04
    letterSpacing: -0.03em
  h2:
    fontFamily: system-ui
    fontSize: 2.5rem
    fontWeight: 650
    lineHeight: 1.1
    letterSpacing: -0.02em
  h3:
    fontFamily: system-ui
    fontSize: 1.25rem
    fontWeight: 650
    lineHeight: 1.25
  lead:
    fontFamily: system-ui
    fontSize: 1.375rem
    fontWeight: 400
    lineHeight: 1.5
  body:
    fontFamily: system-ui
    fontSize: 1rem
    fontWeight: 400
    lineHeight: 1.6
  caption:
    fontFamily: system-ui
    fontSize: 0.875rem
    fontWeight: 400
    lineHeight: 1.5
  stat:
    fontFamily: system-ui
    fontSize: 2.25rem
    fontWeight: 700
    lineHeight: 1.05
    letterSpacing: -0.02em
  code:
    fontFamily: ui-monospace
    fontSize: 0.8125rem
    fontWeight: 400
    lineHeight: 1.65
  display:
    fontFamily: system-ui
    fontSize: 6.5rem
    fontWeight: 700
    lineHeight: 0.96
    letterSpacing: -0.045em
  label:
    fontFamily: ui-monospace
    fontSize: 0.75rem
    fontWeight: 500
    letterSpacing: 0.14em
    textTransform: uppercase
rounded:
  sm: 4px
  md: 8px
  lg: 12px
  full: 999px
spacing:
  1: 4px
  2: 8px
  3: 12px
  4: 16px
  6: 24px
  8: 32px
  12: 48px
  16: 64px
  24: 96px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    height: 44px
    padding: 16px
  button-secondary:
    backgroundColor: "{colors.raised}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    height: 44px
    padding: 16px
  code-block:
    backgroundColor: "{colors.term}"
    textColor: "{colors.term-text}"
    typography: "{typography.code}"
    rounded: "{rounded.lg}"
    padding: 16px
---

# Stacklit design system

## Concept card

```
Idea:        The page is a map. Stacklit writes a map of a codebase, so the site is laid out as one.
Metaphor:    Cartography. Coordinates for sections, contour lines for dividers, real maps and real output as the terrain.
Anchor:      The product. stacklit.json is literally a map of modules, and every claim on the page is a reading taken from it.
Signature:   ASK THE MAP. Five preset questions, each showing the real recorded `stacklit query` output on gin v1.10.0.
             One chip is an exact-name question where the honest answer is "grep is enough here".
Emotion:     Calm, exact, trustworthy. A surveyor's sheet, not a sales page.
Motion verb: settle. Things drift 12px into place once and rest. The chapter marker slides to the current section and stops.
Screenshot:  the hero: a tight headline and the primary action on the left, a real gin v1.10.0 file graph cropped on the right.
```

Swap test: replace "Stacklit" with another product in "this page is a map of the thing it describes". It falls apart for anything that does not itself produce a map. The coordinates (the chapter numbers), the hero graph and the screens carousel all map to real parts of the product (modules, the map file, command output). The page is not a template with the name changed.

Result: pass. The idea depends on the product writing a committed map, the same fact the headline sells.

## Overview

Stacklit tells the truth about itself, so the page reads like a surveyor's sheet: warm paper, tight system type, tiny monospace kickers for the section numbers only, hairlines instead of boxes, one deep green accent. Real command output and real maps are the pictures.

Borrowed as a style from an editorial studio page (giant tight display type, a metadata layer of mono labels, numbered sections with a circled index, a floating pill nav, hairlines, one large rounded frame, a huge typographic footer). Borrowed as trust patterns from a developer tool home page: a slim release banner with the real version, the star count in the nav, a Works with row, a stats strip, a copy-setup-prompt box that is real. Not borrowed: any colour, font, copy or image, the gradient field, highlighter blocks, dashed borders, wireframe art, cookie banners, inflated counters, backed-by badges.

## Role palette

| Role | Token | Use |
| --- | --- | --- |
| ground | paper | The page. Warm, never white. |
| panel | surface, raised, sunk | The editor window and its tab bar, the carousel slides, the alternate section bands. |
| ink | ink, muted, faint | Text. All above 4.5 to 1 on every ground. |
| route | accent, accent-text | The one link, button and "you are here" colour. |
| drift | bad, warn | Only the unfavourable stat cells. Never decoration. |
| terrain | code-* tokens | Syntax colours inside the editor window, light and dark. |

## Typography

System fonts only, no font files, no third party request. Display: the system sans at weight 700, size up to 104px, tracking -0.045em, leading 0.96. Body 16px. Kickers: the system monospace, 12px, uppercase, tracking 0.14em, on the section kickers only (an owner approved override of Hallmark 67). Every other label, caption, button and table head is sentence case in the body face. Code is the monospace at 13px and never wraps (the window scrolls).

## Layout

12 columns inside one container (76rem, gutter 16px growing to 40px). The hero is a split: text left, a real map cropped on the right and bleeding off the edge at 1024px and up, below the text on smaller screens. Chapters then vary: a big statement with a carousel, a lead beside a quote, a ledger of numbers beside the method, logos beside a note, facts beside the FAQ. Hairlines separate sections. Chapter rail on wide screens: a thin left rail of coordinates with a marker that settles on the current section (scroll linked, progressive enhancement). The page has nine chapters: Overview, The problem, How it works, Ask the map, Results, Benchmark, Works with, Facts and questions, Try it. No legend tables.

## Shape and depth

One radius per level: 4px inline code, 8px controls inside a frame, 20px for the one large frame, full for pills. No shadows except a faint ring on the floating pill nav. Flat everywhere else, hairlines not boxes.

## Motion

Only transform and opacity. Two scroll effects only: reveal on scroll and the chapter rail (navigation state). There is no pill border change and no reading progress line. Scroll linked reveals use `animation-timeline: view()` inside `@supports`, so a browser without it shows everything at once, and `prefers-reduced-motion: reduce` turns them off. Pill nav hover is a background fade. The hero map drifts slowly and its hub links pulse, only inside prefers-reduced-motion: no-preference; otherwise it is a still frame. The carousel never advances by itself. No JavaScript is needed for any of it. The only scripts are the theme toggle, the copy buttons and the carousel buttons and counter (the strip itself scrolls and snaps without a script).

## Trust checklist

- Every number links to its source (agent test or benchmark page). Stat cells carry a source link.
- No invented stats, testimonials or logos. The assistants and language rows are the real support lists.
- The facts that matter (version, MIT, runs on your machine, no telemetry) are one quiet line under the hero, not a table. Limits are on the page: the unfavourable stat cells, "where it did not help" and the exact-name question in Ask the map.
- Real output, not mockups: the derive output, the query output, .stacklitrc.json and the install commands are recorded from the real tool.
- One type scale, one spacing scale, one radius per level.
- Fast and accessible: AA contrast in both themes (checked), visible focus, keyboard tabs, no JavaScript needed to read.
- No decorative gradients, glow, glass, grain, particles or 3D.
- Restrained palette: green plus neutrals, amber and red only for unfavourable cells.
- Provenance links in the facts chapter: source, release checksums, npm provenance, license and the privacy page.
- A Verify note links the checksum file and the npm provenance page.

## Components

- **Header:** logo left, a centred floating pill nav with an active state, a pill GitHub button with the star count right. On phones the pill nav docks to the bottom of the screen. CSS only.
- **CodeWindow:** build time Shiki, three quiet dots, a CSS only tab bar (radio inputs), line numbers, line highlight and diff marks, a copy button and a mono caption row per tab. Code never wraps.
- **Kicker and SectionHead:** the circled index, the mono kicker and a statement heading in two sizes.
- **HeroMap:** the real gin v1.10.0 file graph, drawn at build time from checked in Stacklit output (src/data/hero-graph.json, exported by scripts/export-hero-graph.mjs). Two views switched by radio inputs: dependencies with hubs sized by PageRank, and a folder and community block map. A seeded layout, no runtime physics, labelled gin v1.10.0, real Stacklit output.
- **Carousel:** a scroll-snap strip of real screens (recorded terminal output and the demo maps), each with a one line caption naming the repo and version. Prev and next buttons, a counter, arrow keys on the focused strip, no autoplay.
- **AskTheMap:** preset question chips and recorded output. Labelled as recorded, not live.
- **Setup prompt:** one click copies a prompt for your coding agent. Its text is tested against the real commands.
- **Release banner:** slim, the real current version and date from the GitHub release at build time, with a fallback to the changelog.
- **Section and Container:** the only way to place content. A lint script fails the build otherwise.

## Do's and Don'ts

- Do show real output. Do show the numbers that make Stacklit look worse next to the ones that make it look better.
- Do keep claims inside the benchmark. Never imply a general token or cost saving.
- Don't use em or en dashes, gradients, glass, emoji icons, dashed borders or highlighter blocks behind words.
- Don't add a colour or a font outside the tokens in `src/styles/tokens.css`.
- Tracked uppercase mono only on section kickers. Never for sentences, captions or buttons.
