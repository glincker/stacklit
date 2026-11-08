# Privacy

Stacklit runs locally. It reads your source files on your machine, writes its outputs into your repository, and makes no network requests of its own.

- No telemetry, no analytics, no account, no upload.
- The generated HTML map is one file with no scripts or fonts from anywhere. The only thing it fetches is small language logos: when you open it, it requests an icon file per language and framework (for example `https://thesvg.org/icons/go/default.svg`) from thesvg.org, with a copy on cdn.jsdelivr.net as a backup. The requests carry only the icon name, no repo data, cookies or referrer, and a Content-Security-Policy blocks every other request. The page works offline with letters in place of logos. Set `STACKLIT_MAP_ICONS=off` or `"map_icons": "off"` in `.stacklitrc.json` to leave no remote URL in the page.
- The only network activity in the whole project happens outside the program: `npx stacklit` and `npm install -g stacklit` download the release binary from GitHub when you install.
- The one opt-in exception is `stacklit init --summary`. It pipes a snapshot of the index (module names, exports, dependency edges, entrypoints; no file contents) to a command you control, `claude -p` by default or whatever `STACKLIT_SUMMARY_CMD` names. Whether that command talks to a network is up to that command. Without the flag, nothing is sent anywhere.

`stacklit.json` lists names of modules, files and exported symbols, and detected environment variable names (not values). Review it before committing a private repo's index to a public place.
