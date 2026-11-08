---
version: alpha
name: Stacklit
description: Plain, warm, technical. A paper page with one deep green accent and real command output as the main picture.
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
    fontSize: 0.875rem
    fontWeight: 400
    lineHeight: 1.6
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

## Overview

Stacklit is a developer tool that tells the truth about itself, so the page looks like a well made README: warm paper, plain type, one green accent and real command output as the main picture. No stock imagery, no gradients, no decorative motion. The tone is calm and exact. Good news and bad news sit side by side at the same size.

## Colors

- **Primary (#0a6b5b):** deep green, the only accent. Used for the main button, links and the top bar of the mark. 5.8 to 1 on paper.
- **Paper (#f5f3ee):** the page. Surface, raised and sunk step away from it for panels and wells.
- **Ink (#1b1a17):** text. Muted and faint are the two lighter text steps, both above 4.5 to 1 on every surface they sit on.
- **Bad (#9b2c1d):** only for the unfavourable numbers, so they read as results and not as errors.
- **Term (#1b1a17):** the code block surface, dark in both themes so command output looks like a terminal.

Dark theme keeps the same roles: paper becomes #151410, ink #ece9df and the accent lifts to #5fd0b5 to hold contrast on dark. Every pair is checked by `npm run check:contrast`.

## Typography

System fonts only, so the site loads no font files and makes no third party request. The stack starts at Avenir Next and Segoe UI Variable. Sizes step on a fixed scale: caption 14px, body 16px, lead up to 22px, h3 20px, h2 up to 40px, hero up to 64px, stat 36px. Headings are one weight and one colour. Labels are body type, sentence case, never tracked capitals. Code is 14px mono and never wraps.

## Layout

One container for every section: 72rem wide, centred, with a gutter of 16px that grows to 40px. Backgrounds can run full width, content never does. Vertical rhythm comes from one section padding token (64px growing to 112px) and the 4px spacing scale. Text blocks stop at 40rem and headings at 48rem. Grid children carry min-width 0 so code and tables cannot widen a column. Button rows wrap as a row and never stack at odd widths.

## Elevation and Depth

Flat. Panels are separated by a 1px line, not a shadow. Focus is a 2px outline in the accent colour with a 2px offset and appears instantly.

## Shapes

Radii are 4px for inline code, 8px for buttons and inputs, 12px for panels and code blocks, full only for pills.

## Components

- **Button:** one component, 44px high, two variants (primary, secondary) and one quiet link form. Labels never wrap inside a button.
- **Code block:** dark surface, mono 14px, horizontal scroll, copy button, keyboard focusable. Never wraps.
- **Stat row:** a big number, a small label and a source link. Favourable, neutral and unfavourable results are shown at the same size.
- **Section and Container:** the only way to place content on a page. A lint script fails the build when a landing section skips them.

## Do's and Don'ts

- Do show real output. Do show the numbers that make Stacklit look worse next to the ones that make it look better.
- Do keep claims inside the benchmark. Never imply a general token or cost saving.
- Don't use em or en dashes, gradients, glass effects, emoji icons or tracked capitals.
- Don't add a colour or a font outside the tokens in `src/styles/tokens.css`.
