# DSP Ledger

A tracker for Dyson Sphere Program: planets, production lines, logistics routes,
per-planet import/export balances and a route map.

It runs as a small local program. It serves the tracker page on `127.0.0.1`, opens it in
your browser, and saves everything to `dsp-ledger-data.json` next to the program.

## Running it

**Windows**: double-click `dsp-ledger-windows-x64.exe`. A console window opens and the
tracker opens in your browser. Keep the window open while you play. Close it (or click
**Quit** in the tracker) to stop.
Windows SmartScreen may warn about an unsigned app the first time. Click
*More info → Run anyway*.

**Linux**:
```sh
chmod +x dsp-ledger-linux-x64
./dsp-ledger-linux-x64
```
If you launch it from a file manager without a terminal, use **Quit** in the tracker to stop it.

Running it a second time while it's open just reopens the tracker page.

### Options
```
--port 8686        port to use on 127.0.0.1 (falls back to a free one if taken)
--data <file>      use a different data file, e.g. one in a synced folder
--no-browser       don't open the browser automatically
--version          print the version
```

## Your data

- `dsp-ledger-data.json` is plain JSON with three lists: `planets`, `lines`, `routes`.
- Each time you launch, the first save keeps a copy of the previous file as
  `dsp-ledger-data.bak.json`.
- **Export** in the tracker downloads a dated backup. **Import** merges a backup back
  in: records with the same id are updated and new ones are added. Nothing is deleted.
- If the program's folder isn't writable (e.g. Program Files), the data goes to your
  user config folder instead (`%AppData%\dsp-ledger` or `~/.config/dsp-ledger`). The
  console window shows the exact path.
- On first run the data file is created from `seed.json`, which holds the factory you'd
  entered when this build was made. Replace `seed.json` with
  `{"version":1,"planets":[],"lines":[],"routes":[]}` before building to start empty.

## Building from source

Needs Go 1.21 or newer and nothing else: it uses only the standard library.

```sh
./build.sh        # Linux/macOS
build.bat         # Windows
```
Both put binaries in `dist/`. For a single platform:
```sh
GOOS=windows GOARCH=amd64 go build -o dsp-ledger.exe .
```

## Layout

```
main.go          local web server, file storage, browser launcher
web/index.html   the whole tracker UI (HTML, CSS, JS in one file), embedded into the binary
seed.json        starting data for a fresh install
build.sh / .bat  cross-platform build scripts
```

The UI also runs as a plain HTML file. Opened straight from disk, it saves to the
browser's localStorage instead of the data file.

## Security notes

The server listens on 127.0.0.1 only and rejects requests whose `Host` isn't the local
address, which stops DNS-rebinding. It sends no CORS headers, so other websites can't
read or write your data. Quit requires a custom header, which forces a CORS preflight
that cross-site requests can't pass.
# DS-Ledger
# DS-Ledger
