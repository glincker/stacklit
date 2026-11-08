# Stacklit brand kit

The mark is three stacked bars, each shifted sideways, with the top bar lit in the accent color. It reads as layers of a codebase seen from the side.

## Files

| File | Use |
| --- | --- |
| `stacklit-mark.svg` | One color mark, uses `currentColor`. Use it where the surrounding text color should decide. |
| `stacklit-mark-light.svg` | Mark with the accent baked in, for light backgrounds. |
| `stacklit-mark-dark.svg` | Mark with the accent baked in, for dark backgrounds. |
| `stacklit-logo.svg` | Mark and wordmark, horizontal. |
| `stacklit-logo-stacked.svg` | Mark above the wordmark. |
| `favicon.svg` | 32 px app icon on a dark tile. |
| `social-preview.png` | 1280 by 640 link preview. |

The wordmark is live text set in a system font stack (Avenir Next, Segoe UI Variable, system-ui). Convert it to outlines in a vector editor before sending the logo to print.

## Color tokens

These match the tokens in `assets/template.html`.

| Token | Light | Dark |
| --- | --- | --- |
| Accent | `#0a6b5b` | `#5fd0b5` |
| Ink | `#1b1a17` | `#ece9df` |
| Paper | `#f5f3ee` | `#151410` |

The accent on paper reaches 6:1 contrast in light and 9:1 in dark. Use the accent for the top bar and for one action color. Do not add a second accent.

## Clear space and size

- Keep clear space equal to the height of one bar (a fifth of the mark height) on every side.
- Minimum size: 16 px tall for the mark, 20 px tall for the logo with wordmark. Below 24 px use the single color mark.

## Don'ts

- Do not rotate, skew, outline or add shadows or gradients.
- Do not recolor the top bar to anything but the accent, or make it the same color as the other bars in the two color versions.
- Do not stretch the mark or change the bar offsets.
- Do not set the wordmark in another typeface.

## Making .ico and PNG sizes

Render `favicon.svg` at each size. With ImageMagick:

```sh
for s in 16 32 48 180 192 512; do magick -background none -density 1200 favicon.svg -resize ${s}x${s} favicon-$s.png; done
magick favicon-16.png favicon-32.png favicon-48.png favicon.ico
```

`rsvg-convert -w 180 favicon.svg > apple-touch-icon.png` works too.
