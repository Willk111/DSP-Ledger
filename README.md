# DSP Ledger

A desktop tracker for **Dyson Sphere Program**. Log what each planet produces and what
your logistics stations ship between planets. DSP Ledger then shows where you're short,
where you have spare, and how items move across your cluster.

It runs as its own app, with its own window, icon and taskbar entry. It works offline and
keeps everything in one JSON file next to the program.

![icon](DSP-Ledger/assets/icon.png)

## Features

- **Galaxy view.** Planets grouped by star system, with top outputs, import/export counts
  and shortage markers.
- **Planets.** Production lines with rate per minute, building, building count, inputs and
  status (running, building, starved, paused, planned), plus a per-item made, used, in,
  out and net balance.
- **Routes.** ILS, PLS, belt or manual routes with rate, distance and status. Routes are
  flagged when the source planet can't supply them.
- **Route map.** Arrows between planets, with thickness for rate and colour for item.
  Click a planet to focus it, or filter to one item.
- **Items view.** Galaxy-wide totals that catch "made enough, but not where it's needed".
- **Count planned.** Include planned lines and routes to see what a build will need
  before you place it.
- **Safe saves.** Atomic writes, an automatic `.bak` copy, and Export/Import backups.

## Install

Download the latest files from the Releases page.

### Windows 10 / 11

1. Put `DSP-Ledger-1.0.0-windows-x64.exe` in its own folder, e.g. `Documents\DSP Ledger`.
   Your data file is created in the same folder.
2. Double-click it.

- SmartScreen may warn about an unsigned app the first time. Click
  *More info → Run anyway*.
- The window uses Microsoft Edge WebView2, which is built into Windows 10 and 11. If it's
  missing, the app says so and opens in your browser instead.
- To pin it, right-click its taskbar icon and choose *Pin to taskbar*.

### Linux (64-bit)

```sh
tar -xzf dsp-ledger-1.0.0-linux-x64.tar.gz
mv dsp-ledger ~/dsp-ledger      # somewhere permanent; your data is saved here
cd ~/dsp-ledger
./dsp-ledger                    # start it
./install.sh                    # optional: adds DSP Ledger to your app menu with its icon
```

After running `install.sh`, launch it from your app menu like any other app. Run
`install.sh` again if you move the folder.

**If it doesn't start:**

| Message | Fix |
|---|---|
| `Permission denied` | `chmod +x dsp-ledger install.sh` |
| `libwebkit2gtk-4.1.so.0: cannot open shared object file` | Ubuntu/Debian/Mint/Pop: `sudo apt install libwebkit2gtk-4.1-0`<br>Fedora: `sudo dnf install webkit2gtk4.1`<br>Arch: `sudo pacman -S webkit2gtk-4.1` |
| `GLIBC_2.34 not found` | Your distro predates 2021. Build the browser-only version (see *Building*). |

## Using it

- **Planets.** Add one from the Galaxy view, then open it to add production lines and
  imports/exports.
- **Rates** are items per minute, as shown in the game's statistics panel.
- **Count planned** (top bar) includes planned lines and routes in every balance.
- **Export** saves a dated copy of your data into a `backups` folder next to the data
  file. The app shows the path.
- **Import** merges a backup back in. Records with the same id are updated and new ones
  are added. Nothing is deleted.
- **Quit** by closing the window. Starting a second copy tells you it's already open.

### Command-line options

```
--browser          open in your web browser instead of the app window
--no-browser       with --browser: run the server only, don't open a browser
--data <file>      use a different data file, e.g. one in a synced folder
--port 8686        local port (also stops two copies running at once)
--version          print the version
```

## Your data

| File | What it is |
|---|---|
| `dsp-ledger-data.json` | Your planets, lines and routes as plain JSON |
| `dsp-ledger-data.bak.json` | The file as it was before your first change of each session |
| `backups/dsp-ledger-backup-<date>.json` | Copies made with **Export** |

These sit next to the program. If that folder isn't writable, they go to
`%AppData%\dsp-ledger` (Windows) or `~/.config/dsp-ledger` (Linux) instead.

To move to a new PC or a new version, copy `dsp-ledger-data.json` next to the new program.

On first run, the data file is created from `seed.json` built into the program. To ship
a build that starts empty, replace `seed.json` with
`{"version":1,"planets":[],"lines":[],"routes":[]}` before building.

## Building

Needs **Go 1.21+**. All Go dependencies are vendored in `vendor/`, so builds work offline.

```sh
./build.sh      # Linux/macOS: Windows exe, Linux app and Linux browser-only build
build.bat       # Windows: DSP Ledger.exe
```

Output goes to `dist/`:

| File | Notes |
|---|---|
| `DSP Ledger.exe` | Pure Go, no C compiler; cross-compiles from any OS |
| `dsp-ledger` | Linux app window. Uses cgo, so build on Linux with `sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev` |
| `dsp-ledger-browser-only` | Static Linux build, no system libraries, opens in the browser |

### Making a release

1. Bump `appVersion` in `main.go`, plus the version strings in `winres/app.rc` and
   `winres/app.manifest`. Then rebuild the Windows resources (see `winres/README.md`).
2. Run `./build.sh`.
3. Package it for Linux:
   ```sh
   mkdir -p pkg/dsp-ledger
   cp dist/dsp-ledger linux/install.sh pkg/dsp-ledger/
   cp assets/icon.png pkg/dsp-ledger/dsp-ledger.png
   tar -czf dsp-ledger-<version>-linux-x64.tar.gz -C pkg dsp-ledger
   ```
4. Rename `dist/DSP Ledger.exe` to `DSP-Ledger-<version>-windows-x64.exe`.
5. Run `sha256sum` on both, then update `RELEASE_NOTES.md`.

## How it's put together

The UI is a single web page shown in a native window. A small Go server on `127.0.0.1`
serves the page and reads and writes the data file.

```
main.go                  startup, single-instance check, local HTTP API
store.go                 data file: atomic saves, .bak copy, backups
ui_windows.go            app window via WebView2 (github.com/jchv/go-webview2)
ui_linux.go              app window via WebKitGTK (internal/webview), window icon
ui_other.go              fallback for other platforms: open in the browser
internal/webview/        webview_go (MIT), patched for WebKitGTK 4.1
web/index.html           the whole tracker UI (HTML/CSS/JS), embedded in the binary
web/fonts/               Chakra Petch + IBM Plex (SIL OFL), embedded
assets/                  app icon (PNG + ICO)
winres/                  Windows icon/manifest/version resource source
rsrc_windows_amd64.syso  compiled Windows resources, linked automatically
linux/install.sh         adds a menu entry and icon on Linux
seed.json                starting data for a fresh install
vendor/                  vendored Go dependencies
RELEASE_NOTES.md         notes for the current release
```

### API

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/info` | GET | App name, version, data file path, window or browser mode |
| `/api/state` | GET / PUT | Read or replace all data |
| `/api/backup` | POST | Write a dated backup, returns its path |
| `/api/quit` | POST | Stop the app (browser mode) |

### Security

The server listens on `127.0.0.1` only and rejects requests whose `Host` isn't the
loopback address, which blocks DNS rebinding. It sends no CORS headers, so websites open
in your browser can't read or write your data. `POST` actions need a custom
`X-DSP-Ledger` header, which forces a CORS preflight that the server never approves.
Nothing is sent over the internet.

## Roadmap

- **Import from save file.** Read a `.dsv` save to fill in planets, production lines and
  station supply/demand automatically. Being explored.
- **macOS app window.** It builds today but opens in the browser.

## Credits

- Fonts: [Chakra Petch](https://fonts.google.com/specimen/Chakra+Petch) and
  [IBM Plex](https://www.ibm.com/plex/), under the SIL Open Font License
  (`web/fonts/licenses`).
- [go-webview2](https://github.com/jchv/go-webview2) (MIT) for the Windows window.
- [webview_go](https://github.com/webview/webview_go) (MIT) for the Linux window.

DSP Ledger is a fan-made tool and isn't affiliated with Youthcat Studio or Gamera Games.
