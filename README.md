# Pourquoi Pas?

A desktop board that runs coding agents from the board itself. Moving a card
into a column starts the agent it is assigned to on it, in a branch and an
environment already set up; the agent reports back on the card and moves it on
itself. One binary, built with [Wails v3](https://v3.wails.io), with the board
server running **in-process** — no spawned server process, no Node.js of our
own — and the same code builds a **headless server** (`-tags server`) that serves
the whole thing to a browser.

Everything a build needs is in this repository: `frontend/` is the frontend, its
own npm project built with Vite, and `server/` is the board server, a fork of
Mattermost's Focalboard with our own patches (`GetUserByUsername` and two
single-user endpoint fixes). It is one Go module — `server/` is a directory in
it, not a module of its own — so **`git clone` and `go build ./...` is the whole
of it**: no second checkout beside this one, no branch of somebody else's, and
nothing to `replace`.

The board server's packages are `github.com/artipop/pourquoipas/server/…`. They kept
upstream's import path for a long time, on the grounds that a rename was a
tree-wide edit buying nothing but the name; the name turned out to be worth it,
and the edit was one pass over 209 files.

## How it works

The Go code is platform-agnostic — the same files build for every OS:

- `server.go` — starts the board server in-process in single-user mode on a
  free port. The SQLite database and uploaded files live under the OS user config
  dir (`os.UserConfigDir()` → `~/Library/Application Support/pourquoipas` on macOS,
  `%AppData%\pourquoipas` on Windows, `~/.config/pourquoipas` on Linux), **not** next
  to the binary, because a signed/packaged app dir is read-only.
- `datadir_production.go` / `datadir_dev.go` — which install that is.
  A packaged build (`-tags production`, which every one of them carries) uses
  `pourquoipas`; `wails3 dev` builds without the tag and uses `pourquoipas-dev`, including
  as the keychain service name, and a single-instance lock of its own, so the
  dev window opens while the installed app runs. One product, two installs, so what is tried out
  in development is not in the app afterwards.
- `frontend_embed.go` / `frontend_disk.go` — the page's bundle is compiled into
  the binary with `go:embed` (release builds, `-tags frontend`) straight from
  `frontend/dist`, where the `build:frontend` task leaves it, as in the Wails
  template. At startup `resolveWebPath` extracts the embedded bundle to
  `<dataDir>/web` and points the server there (the server templates
  `index.html` on read, so it needs files on disk). Without the tag
  (`go build ./...`, tests) it falls back to `frontend/dist` on disk.
- `frontdoor.go` — **the origin everything is served under**: `/wails/` goes to
  the Wails runtime, everything else to the board. In a desktop build the front
  door is a loopback listener of ours on a random port, handed to Wails as an
  `AssetServerTransport`, and the window is pointed at it instead of the
  `wails://` origin. In a server build it stands in front of Wails' own HTTP
  server, which is moved to a private port.

  One origin is what lets the **WebSocket** work: Wails' asset server answers an
  upgrade with `501 Not Implemented` and its response writer cannot be hijacked,
  so `/ws` can never be served through it. Beside it, on a plain `net/http`
  server, an upgrade is ordinary — which is why there is no
  `window.webSocketBaseURL` here, unlike the v2 app, whose page origin (a WebKit
  custom scheme) could not carry a socket at all.

  Because a loopback port is reachable by anything on the machine, `/wails/`
  refuses a request whose `Origin`/`Sec-Fetch-Site` says it came from another
  site (the bound methods start agents; the endpoint carries no credential of
  its own), and the whole front door refuses a `Host` it was not published
  under — the standard answer to DNS rebinding.
- `proxy.go` — a reverse proxy to the in-process server, sitting behind the
  front door on everything but `/wails/`. It injects a bootstrap `<script>` into
  the served HTML that seeds the single-user session token
  and **tells the page there is a Go side** (see below), and nothing else.

  **The bindings** are Wails' own, generated from `App` into
  `frontend/bindings` (`wails3 task generate:bindings`) and committed. The page
  imports them as a template's page does, over the `@wailsio/runtime` it
  bundles, and calls each method by id; `goApp()` in `goBridge.ts` hands them
  out when the bootstrap has set `window.hasGoSide`. The same module wires
  `window.openInNewBrowser` to `App.OpenInBrowser`, with a capture-phase click
  handler that sends **every** absolute http(s) anchor there, same-origin ones
  excepted. It deliberately does not defer to the inline `onclick` that
  `Utils.htmlFromMarkdown` puts on markdown links: when that handler does not
  run, the click is simply lost — the webview cannot navigate to an outside
  origin either — which is what made preview links in card comments dead.
- `internal/acp/` — **the agent integration**: an ACP session per card, started
  by a move, given the board as MCP tools, reporting through them. `trigger.go`
  is the whole rule (a card moved into a column, assigned to an agent, is worked
  by it), `session.go` runs it, `boardtools.go` hands out the tools, and
  `workspace.go` gives it a branch to work on. There was a period when a stage
  was the vendor CLI in a pty instead, with the app drawing a terminal beside
  the card; it is gone, and `docs/decisions.md` says what it cost.
- `app.go` — the bound service. Its exported methods are what the frontend
  calls; `OpenInBrowser` is the one every outward link ends in.
- `mode_desktop.go` / `mode_server.go` — everything that differs between a
  window and a server: the front door, the window, and the folder picker (a
  server build has no native dialog and says so, rather than returning an empty
  path the UI would read as a cancellation).
- `main.go` — wires it all together and runs the app.

**On a phone**, the board is the same board: `frontend/src/pages/mobile` is the
`/m` route — the inbox, one board's cards, and what is waiting for a person,
answered in place — and [`mobile/`](mobile/README.md) is a small Wails app for
iOS and Android that is one window onto it on a board server. It is
its own Go module because the mobile build compiles `package main` from the
module root, and this one is a board server with SQLite and git.

Builds are **native per platform** (each on its own machine/CI runner) — the
desktop build needs cgo on macOS and Linux, for Wails itself, and cgo does not
cross-compile with the host toolchain; `wails3 task setup:docker` builds the
image that can, if a Mac has to produce a Linux binary. Windows is pure Go and
cross-compiles from anywhere; SQLite is `modernc.org/sqlite`, pure Go, everywhere.

**[How a card gets worked on](docs/card-work.md)** walks through the whole thing
for somebody using the board: what starts an agent, what it is told, what it
does at the end, where the work happens, and what to look at when nothing
happens.

## What this app requires of the frontend

The desktop app serves and embeds `frontend`, and knows nothing about what it is
written in. There is no React here, and none of the migration off React-only
libraries touched this module. What the two halves *do* agree on is worth
writing down, because it is the whole of what a port to another framework has to
keep — `frontend/src/types/index.d.ts` declares the shape, and this is the
contract behind it.

**The build output.** `build:frontend` must leave `frontend/dist/index.html`,
Vite's output under `frontend/dist/assets/` and what `public/` holds at the root
of `frontend/dist`, all compiled in with `go:embed`. The board server serves a
path naming a file of the bundle as that file and every other path as the page
(`server/web/webserver.go`), as a Wails asset server does. `index.html` must still carry the Go template
`{{.BaseURL}}`, and asset URLs referenced from JS and CSS must stay relative so
they survive a non-empty base path.

**The bindings.** `frontend/bindings` is generated from the bound `App` service
and committed; `@wailsio/runtime` in `frontend/package.json` is pinned to the
Wails version in `go.mod`. `bindings_test.go` checks both.

**One global**, set by the bootstrap script in `proxy.go`:
`window.hasGoSide` — there is a Go side behind this origin. Nothing but the
front door injects the script, so its presence is the answer, and every call
into Go asks it first (`goApp()`), which is what keeps the desktop-only UI inert
where nothing answers. The page sets `window.openInNewBrowser` itself, because
markup generated by `utils.ts` calls it from an inline handler.

There is deliberately **no `window.webSocketBaseURL`** here, unlike v2:
`frontdoor.go` owns the origin, so `/ws` opens from the page's own origin.

## Prerequisites

- Wails v3 CLI (it carries its own task runner, so no separate `task` binary):
  ```
  go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
  ```
- A C toolchain where the desktop build needs cgo:
  - **macOS**: Xcode Command Line Tools.
  - **Linux**: `gcc`, plus `libgtk-3-dev` and `libwebkit2gtk-4.1-dev` (Wails).
  - **Windows**: none — the build is pure Go (`CGO_ENABLED=0`). NSIS (`makensis`) for the installer, and the
    WebView2 runtime (bundled with modern Windows).
- **Line endings are LF in every checkout**, whatever `core.autocrlf` says on
  the machine — `.gitattributes` pins it, because generated files are compared
  byte for byte with what their generator writes, and a CRLF checkout fails
  those tests. A clone made before that file existed keeps its CRLF copies
  until they are written again; on a clean tree,
  `git rm -r --cached -q . && git reset --hard` rewrites them.
- **Linux and Windows installers** are generated by the v3 CLI itself
  (`build/linux/appimage`, `build/linux/nfpm`, `build/windows/nsis`); nothing
  else has to be installed for them.
- **Building for another platform from a Mac**: `wails3 task windows:build` is
  a plain Go cross-compile and needs nothing. Linux needs cgo, so
  `wails3 task setup:docker` once (~800MB), then `wails3 task linux:build`.
  Binaries only — the installers are native-tool jobs (AppImage shells out to
  `ldd`, NSIS is `makensis`). Native builds on the target platform stay the
  release path.
- **Linux targets GTK 3** (`gtk3` build tag, webkit2gtk-4.1). Wails v3 defaults
  to GTK 4, but its GTK 4 code needs 4.10 — newer than several still-supported
  distributions ship. `GTK_TAG=` drops the tag where GTK 4 is new enough.
- **Templates**: the board selector offers three of them — «Разработка» for
  code, «Контент» for writing, and «Личное» for the ordinary life the same
  machinery turns out to fit: an agent that puts a plan, a list or a draft in a
  folder of notes. Each carries its columns, the fields a card names a folder
  and a branch in, and the questions it wants asked when a board is made from
  it. Every other upstream template is hidden — the server module's own are the
  upstream's examples. Ours live **here**, in
  `internal/boardadapter/templates/`, one `.jsonl` per board, embedded with
  `go:embed` and installed into the global team on launch by `ImportTemplates`.
  A board is recognised on the next launch by the `template` property
  carrying its file's slug (ids are regenerated on import, titles are the
  user's), and `TemplateVersion` in `internal/boardadapter/templates.go` is what
  replaces an installed copy with an edited one. Authoring is not by hand:
  build the board in the app, *Export board archive*, unzip, keep the
  `board.jsonl`.
- **Where a setting lives is decided by whose it is.** The registries are the
  machine's — which agents are installed and how they reach the network — so
  they are one
  dialog opened from the sidebar's *«Настройки → Эта машина…»*, beside the theme
  and the language, and are reachable with no board open at all. What a board
  keeps — the folders its agents work in, what those agents are told first — is
  asked one question at a time from its ⋯ menu, each item being
  one of the questions its setup plan lists. That menu used to
  be the only door to all of the above, which made machine settings look like a
  property of whichever board happened to be open. A setting that belongs to one
  *screen* goes with it: what feeds the inbox is asked on «Входящие», the one
  board every source writes to, whose ⋯ menu offers *«Источники…»* and nothing
  besides.
  Registering an agent does not require going there — the setup wizard offers
  the short form (a name and a kind) where the choice is being made, and the
  full form stays in the settings. A folder marked "on every board" joins only
  boards that already have a «Папка» field.
- **What a board tells its agents first is the board's** (the `prompt` property
  on the board itself, edited in *«Системный промпт доски…»*). It was one string
  shared by every board on the machine, labelled "board system prompt" while
  being nothing of the kind — which meant the household board and the code board
  shared it and so nobody could write anything useful in it.
- **First run**: a board made from a template opens a setup wizard by itself.
  Four questions are every board's — its name, a folder, an agent, and a last
  screen saying what to do next — and a template may declare more of its own in
  `setup`, add a sentence to a step ("the folder with your household notes") or
  insist on one the app calls optional. It may only name steps from the closed
  set `internal/acp/setup.go` implements, so a board cannot ask for something
  nothing can fill.
  **A folder does not have to be a git repository.** What git buys — a branch
  per card, something to publish, every transition that waits for one — is
  offered to the folders that have it and simply absent from the ones that
  do not, so a board of household chores sends an agent into a folder of notes
  and nobody is told to `git init` their shopping list. A step of the plan carries what its answer must satisfy
  (`requires: ["git"]`), and `CheckBoardSetupAnswer` enforces it where the
  question is asked rather than on a card three steps later. `BoardSetupPlan`
  resolves the whole answer in Go: the four questions every board has, whatever
  the board declared of its own, and how far *this board* has got through them. **Setup is per board, not per machine.** Every step's status is
  read from the `board_setup` table, keyed by board — the registries are the
  machine's and say only that a step *can* be answered quickly (`ready`, shown
  as "already registered" with one click to pass it). Reading them as the answer
  is what made every board after the first appear fully set up and get created
  in silence. The page renders that plan and works nothing out for itself; the
  wizard walks exactly what the plan lists. The wizard opens itself once per board —
  closing it half-way answers nothing, so the header goes on saying *«Доска ещё
  не настроена»* until every question that board asks has an answer, and
  *«Пройти настройку заново…»* in the ⋯ menu is the way back in.
  Having been offered it is remembered in the store, not in the page: a desktop page
  keeps no data in the browser (`session.ts`, `userSettings.ts`), so anything
  it has to remember between runs is kept by the board server.
- **What starts an agent** is a card being moved into a column while it is
  assigned to one. Nothing else: a column is an ordinary option of the board's
  status field, and there is no registry saying what any of them does. The agent
  is told which column the card was dropped into and what the board's columns
  are called, and it moves the card on itself when it is done (`move_card`),
  writes what it did in one comment (`comment_card`) and ends the run
  (`finish_work`). A card assigned to a person, or to nobody, starts nothing and
  waits where it is.
- **Taking a card yourself**: assign it to yourself and no agent starts on it.
  Assigning a registered agent, or nobody, hands it back.
- **Templates** (*«Сохранить как шаблон…»* in the board menu, or the pencil in
  the template picker). A template is a board that has not been made yet: saving
  a working board copies it under its own name, and the template editor is a
  form — name, icon, what it is for, and the questions a new board should ask
  about this machine. Templates somebody made are offered beside the ones the
  install ships.

## Develop

From this repository:

```
wails3 dev
```

`build/config.yml` declares the dev loop, the one `wails3 init` writes: Go
edits rebuild and restart the app, and the Vite dev server runs alongside it
(`wails3 task common:dev:frontend`), so a TS edit reaches the window by HMR
without a rebuild. Needs the frontend deps — run `npm ci` in `frontend/` once if
`frontend/node_modules` is missing. `frontend/` is excluded from the Go watch list,
so a TS edit never restarts the app.

The window still opens on the front door, not on Vite: a development build
takes the page and its modules from the address `wails3 dev` hands it
(`FRONTEND_DEVSERVER_URL`), injects the bootstrap into it, and keeps `/api` and
`/ws` for the board (`devserver.go`, `proxy.go`). That is why nothing here needs
to know the board server's port or the session token — both stay random per
launch — and why Vite's own config has no proxy in it.

To run it manually instead:

```
wails3 dev
```

The server build is the fastest way to look at the UI in a real browser with
devtools:

```
wails3 task build:server && SERVER_PORT=8099 bin/pourquoipas-server
```

For pure frontend/CSS iteration the browser loop is still faster than the webview:
run the server build above, then `cd frontend && npm run dev` for HMR at
http://localhost:5173.

## Cut a release

A release is a tag. `.github/workflows/release.yml` builds all four platforms
natively, signs the update manifest and publishes the lot:

```
wails3 task version:set VERSION=1.1.0   # every file that states a version
go test .                               # the guard: they all have to agree
git commit -am "Release 1.1.0"
git tag -a v1.1.0                       # the annotation is the release notes
git push && git push --tags
```

`docs/release.md` is the whole of it — the signing key, what is built where,
and how to dry-run the workflow without publishing anything.

## Build release installers by hand

From the repo root, per platform. The build produces `frontend/dist` itself and
embeds it, so the artifacts are single-binary:

```
wails3 task package            # macOS   → bin/pourquoipas.app
wails3 task darwin:package:dmg # macOS   → bin/pourquoipas.dmg
wails3 task windows:package    # Windows → bin/pourquoipas.exe + NSIS installer
wails3 task linux:package      # Linux   → AppImage + .deb (+ .rpm)
wails3 task build:server       # any     → bin/pourquoipas-server (headless)
```

`wails3 task build` / `linux-app-wails3` build just the `.app` / bare binary.
Installer configs live in `build/` (`darwin/`, `windows/nsis/`,
`linux/appimage/` + `linux/nfpm/`), all generated by `wails3 update
build-assets` from `build/config.yml` — edit the config, not the assets.

The server binary has no window: it publishes the board at
`SERVER_HOST`/`SERVER_PORT` (default `localhost:8080`) and
is opened in a browser. Keep it on localhost unless something in front of it
authenticates: the bound methods start agents and read the filesystem, and the
runtime endpoint has no credential of its own — the front door's cross-origin
check stops a web page from calling it, not a user on that network.

## Sign & notarize macOS (single pass — one binary)

v3 has its own signing path, which reads the identity and the keychain profile
`wails3 setup` stored, so nothing has to be repeated on the command line:

```
wails3 setup                      # once: signing identity + notarytool profile
wails3 task darwin:sign:notarize
```

`wails3 entitlements` writes `build/darwin/entitlements.plist` when the app
needs one (the v2 build carried a hand-written file; v3 generates it). Signing
by hand still works and is one pass, since the app is one binary:

```
codesign --deep --force --options runtime \
  --sign "Developer ID Application: <TEAM>" \
  bin/pourquoipas.app

xcrun notarytool submit bin/pourquoipas.app \
  --apple-id <id> --team-id <team> --password <app-specific-pw> --wait

xcrun stapler staple bin/pourquoipas.app
```

## Out of scope (MVP)

Not yet implemented: the `nativeApp` bridge for persisting user settings, the
What's New dialog, multi-window, in-app downloads / file picker, and
window-position autosave.

## Documentation

- [docs/guide/](docs/guide/index.md) — руководство пользователя, по-русски:
  как этим пользоваться, а не как это устроено. Это же и сайт — VitePress,
  разделами и с поиском; как он собирается — [docs/guide-site.md](docs/guide-site.md).
- [docs/card-work.md](docs/card-work.md) — how a card gets worked on, for somebody using the
  board rather than working on it.
- [docs/templates.md](docs/templates.md) — what a template carries, how it is edited,
  and how a working board becomes one.
- [docs/build-and-platforms.md](docs/build-and-platforms.md) — what to run to get a
  binary on each of the three platforms, which build tags do what, and the traps
  already stepped in.
- [docs/verifying.md](docs/verifying.md) — how a change is actually checked here:
  which level proves what, driving the headless build and a browser, and the
  traps of the harness itself.
- [docs/release.md](docs/release.md) — cutting a release: the version, the signing
  key updates are verified against, what the workflow builds where, and how to
  run it without publishing.
- [docs/frontend.md](docs/frontend.md) — the page: the store that replaced Redux, what
  each React library was replaced by, and how its tests are written.
- [docs/local-and-shared-state.md](docs/local-and-shared-state.md) — which of the
  things this app stores belong to the machine and which to the board. A note to
  think about later: nothing in it bites while the app is one process.
- [docs/sources.md](docs/sources.md) — how an outside event (a mailbox, a
  tracker, a notification from a phone) becomes a card, and the plugin protocol a
  source is written against in Go or TypeScript.
- [docs/deferred.md](docs/deferred.md) — work that was thought through and then put
  down on purpose, with the reasoning that put it down.
- [docs/spec-acp.md](docs/spec-acp.md) — the original specification the agent
  integration was written from. Kept for the reasoning; the implementation has
  moved past parts of it.
