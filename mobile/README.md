# Pourquoi Pas? on a phone

One window onto the board, for iOS and Android.

The phone runs nothing. The board, its API, its sockets and the agent bindings
are all served by the board server, and this app is a window onto it — exactly
what the desktop app's own window is, which is also pointed at its front door
rather than at a `wails://` origin. The page each machine shows is
`/m`, the board's phone view: the inbox, a board's cards, and what is waiting
for a person.

What it reaches is a board server, by the address it is published under, and
the server asks for a login. A desktop no longer publishes anything off its own
machine: the tailnet door that did is gone (`docs/deferred.md`), and where the
phone goes next is a plan in `docs/todo.md`.

## Several machines, several tabs

A person may work more than one board server, and each one serves its own board under
its own name. So the window holds a tab per machine and a **frame** behind each
tab — not a navigation, and not one merged board.

A frame is what keeps this free. Inside it the board's page is same-origin with
its own front door, so the bindings and the event socket work exactly as they do in a browser, and nothing on the server side has to be
opened up to another machine's origin. It is also the only way back: the tabs
live in a page that never leaves.

Two things cross that boundary, and they are the smallest two that could:

- **The number waiting.** Each board posts how many things are asking for a
  person (`pages/mobile/mobilePage.tsx`), and the tab carries it — so which
  server needs you is visible without opening its tab. A message from an origin
  that is not one of the machines on screen is ignored.
- **Whether the machine answers at all.** A frame's failure cannot be read from
  outside it, so each machine is probed directly with a `no-cors` GET of its own
  page: an opaque response means it is up, a rejection means it is not, and a
  machine that has just come back gets its frame reloaded — otherwise it would
  sit on the error page the webview drew while the server was down.

What a merged list across machines would cost instead — a JSON API on the front
door and a cross-origin hole in `sameOrigin` — is in `docs/deferred.md`.

## Why it is a module of its own

The mobile build compiles **package main from the module root** — `wails3 ios
overlay:gen` injects a `main_ios.gen.go` beside it that calls `main()`. The
desktop `main` is the board server with cgo SQLite and git; none of it
builds for iOS and none of it belongs on a phone. So the phone app is its own
module, with its own `go.mod`, outside the parent's `./...`.

What it carries is small enough to read in one sitting:

- `main.go` — the app and its one window, which stays on the app's own page.
- `board.go` — what somebody typed (`board`, `board.example.com`,
  `https://…`) becomes the address to load, and the one word its tab is called.
  Pure, and tested.
- `settings.go` — the service the page calls, and where the machines are kept:
  the platform's own secure store, since a phone has no config file to
  hand-edit. The one address earlier versions saved is read as a list of one, so
  an update does not lose somebody's only machine.
- `store_ios.go` / `store_android.go` / `store_desktop.go` — the one call that
  differs between the platforms (iOS takes a key and a value, Android takes
  JSON), plus a stub so the package builds and tests on the machine it is
  written on.
- `frontend/index.html` — the single page this app has of its own: the tabs, the
  frames behind them, and the panel that asks where a board is. Plain HTML:
  pulling a framework onto a phone to draw two tabs would be absurd, and
  everything inside a frame is the board's own page.

## Build & run

From this directory:

- `wails3 task ios:run` — simulator. Needs full Xcode **and an installed
  simulator runtime** (`xcrun simctl list runtimes` must not be empty).
- `wails3 task ios:package` — a signed `.app` bundle in `bin/`; add
  `IOS_PLATFORM=device CODESIGN_IDENTITY="Apple Development: …"` for a device,
  `ios:package:ipa` for an `.ipa`. `wails3 task ios:xcode` opens the generated
  Xcode project, which is the way to automatic signing and TestFlight.
- `wails3 task android:run` — emulator. Needs the Android SDK **and the NDK**
  (`ANDROID_NDK_HOME`, or one installed under `$ANDROID_HOME/ndk`).
  `android:package` builds the APK.
- `go test ./...` — the address rules and what the app opens with, on the
  machine you are writing on.

There is no npm project here: `build:frontend` is a no-op, and `go:embed
all:frontend` is the whole of the frontend build.

## Not done yet

- **Notifications.** `application.IOS.PostNotification` is there, and the event
  socket the board's page listens on would feed it, but nothing wires the two
  yet — and a phone that is asleep cannot be woken by a socket, so what is
  possible here is "while the app is alive", not push.
- **Android has not been run.** The Go code compiles for iOS and the bundle
  links; the Android half needs an NDK to say the same.
