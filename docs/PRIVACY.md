# Privacy

Stacklit runs on your machine. It reads your source files, writes its outputs into your repository, and makes no network requests of its own.

## What it does not do

- No telemetry, no analytics, no account and no upload.
- The program makes no network requests of its own. The two exceptions are below: the HTML map and `--summary`.
- Network activity during install is separate. `npx stacklit` and `npm install -g stacklit` download the release binary from GitHub.

## The HTML map

The map is one file. It has no scripts or fonts from other hosts. The only thing it fetches is small language logos.

When you open the map, it requests one icon file per language and framework. For example, `https://thesvg.org/icons/go/default.svg`. A copy of each icon is on cdn.jsdelivr.net as a backup.

- The requests carry only the icon name. They send no repo data, no cookies and no referrer.
- A Content-Security-Policy blocks every other request.
- The page works offline. It shows letters where the logos would be.
- To leave no remote URL in the page, set `STACKLIT_MAP_ICONS=off`, or `"map_icons": "off"` in `.stacklitrc.json`.

## The one opt-in exception

`stacklit init --summary` is the only feature that sends data to another program. It pipes a snapshot of the index to a command you control. The snapshot has module names, exports, dependency edges and entrypoints. It has no file contents.

The command is `claude -p` by default. You can change it with `STACKLIT_SUMMARY_CMD`. Whether that command talks to a network is up to that command. Without the flag, nothing is sent anywhere.

## What is in `stacklit.json`

The index lists the names of modules, files and exported symbols. It also lists detected environment variable names, but never their values.

If your repo is private, review the index before you commit it to a public place.
