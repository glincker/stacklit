# Privacy

Stacklit runs locally. It reads your source files on your machine, writes its outputs into your repository, and makes no network requests.

- No telemetry, no analytics, no account, no upload.
- The generated HTML map is self-contained and loads nothing from the network.
- The only network activity in the whole project happens outside the program: `npx stacklit` and `npm install -g stacklit` download the release binary from GitHub when you install.
- The one opt-in exception is `stacklit init --summary`. It pipes a snapshot of the index (module names, exports, dependency edges, entrypoints; no file contents) to a command you control, `claude -p` by default or whatever `STACKLIT_SUMMARY_CMD` names. Whether that command talks to a network is up to that command. Without the flag, nothing is sent anywhere.

`stacklit.json` lists names of modules, files and exported symbols, and detected environment variable names (not values). Review it before committing a private repo's index to a public place.
