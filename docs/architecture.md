# Architecture

How Pourquoi Pas? is put together, and why each piece has the shape it has. This is the
long form of the architecture section that used to live in CLAUDE.md; CLAUDE.md
now keeps the rules and points here for the reasoning. Rejected alternatives
and post-mortems in Russian are in `decisions.md`, what was deliberately not
built is in `deferred.md`.

Eleven ideas hold this together. Read them before changing anything structural.

## The front door owns the origin

**The board server has no port** (`boardwire.go`). Its router is served on an
in-memory listener, and everything of ours that talks to it — the page's proxy
(`proxy.go`), the socket tunnel — dials it through `net.Pipe`. HTTP and the
WebSocket protocol are unchanged; there is nothing on the machine to reach the
board server by except this process.

**A desktop window's page is on `wails://`.** Wails' asset server hands every
request to the board router (`AssetOptions.Handler`). A WebSocket cannot ride a
scheme handler (the asset server answers an upgrade with 501), so **the board's
changes come on the Wails bus**: the board's websocket hub also hands every
broadcast to a sink (`ws.Server.SetSink`), which a desktop sets to
`Event.Emit("board:ws", …)`, and the page's `wsclient.ts` is given a `BusFeed`
in place of a socket (`busSocket.ts`, `window.boardOnBus`). An attached
server's `/ws` is **tunnelled through Go** instead (`sockettunnel.go`,
`BusSocket`): the page subscribes to `socket:<id>`, asks `OpenSocket`, Go dials
the server and puts its frames on the bus; the page's frames go back as
`SendSocket`, in order. The agents' events come off the bus too
(`agentEvents.ts`). The bootstrap says `window.pageOnWails`
and hands the session over in `window.seededSession`. **A desktop page keeps
its data out of the browser**: the session is held in memory (`session.ts`) and
the settings come from the database (`userSettings.ts`), because `wails://`
storage outlives the app and its database, and a remembered board that was
gone opened the error page on a fresh install. What is only the page's own
(emoji-mart, `randomIcons`) stays in `localStorage`. Two traps of the scheme handler, measured on macOS: WKWebView sends no
`Content-Length` and `httputil.ReverseProxy` dropped every body
(`boardRouter.ServeHTTP` says unknown instead); WebKit hands it no body at all
for a `FormData` or a `Blob` (`sendableForm` sends the encoded bytes). In
`wails3 dev` the HMR client connects to Vite itself (`server.hmr.host`).

**`frontdoor.go` is the desktop's loopback door, for other processes**: the
agents' board tools and the column feed (`/acp/board/`, `feedmcp.Path`, each
with a token and refused to a browser by its Origin) and `/sources/ingest`
(a source token). It serves no page, no API, no bindings and no socket.

**A server build has no door of its own** (`mode_server.go`). Wails' own HTTP
server is the door there: it binds the address, refuses a foreign `Host` on a
loopback bind, checks a runtime call's `Origin` against the request's own
`Host`, and takes TLS and a proxy's public origin in `ServerOptions`. Our routes
ride it as `AssetOptions.Middleware`, which runs before the asset server's 501,
so the board's `/ws` and `/acp/events/ws` work through it (gorilla hijacks
through the asset server's writer, which unwraps). A door of ours in front of
Wails, as there was until 2026-10-21, keyed its checks to the address it
listened on and so refused every name a server is really reached by — a domain
behind a TLS proxy got 403 on everything. Wails' `/wails/events` socket sits on
its own router, outside the middleware and so outside the session check; the
emitter keeps our events off the bus in this build (`emitsOnBus`). One
origin for the page, the API and the socket is what removes the
`window.webSocketBaseURL` hack the v2 app needed.

Since a loopback port is reachable by anything on the machine, `/wails/` and `/acp/`
refuse a request whose `Origin`/`Sec-Fetch-Site` names another site, and the front
door refuses a `Host` it was not published under. In the mode every install
starts in that is the whole of the protection: **nothing authenticates a user**.
Keep such a server build on localhost unless something in front of it does.

**Who is asked is a property of the build** (`identity.go` and its
`identity_desktop.go`/`identity_server.go` pair, `docs/teamwork.md`). A desktop
is always one person: it mints a single-user token, the API synthesizes a
session for every request and `/login` is closed. A hosted server is always a
team: it mints none, so `/login` and `/register` are open, and a desktop joins
such a server by logging into it (`attach.go`, `<dataDir>/server/settings.json`)
while its agents stay on the desktop. It was a mode of the install once, a
settings file read at startup, and that made two products wear one coat. On a
server `/wails/` and both sockets
require a live board session (`requireSession`), read off a cookie the page
writes beside the token it already keeps: the Wails runtime makes its own
fetches, and a browser lets nobody set a header on a WebSocket handshake.
**Nothing under that guard is open to everybody.** The runtime module was, for
a while (`openToEverybody`): the page imported `/wails/runtime.js` before
anybody had logged in, a browser caches a rejected dynamic import, and logging
in navigates without reloading, so refusing it left a login page unable to
*become* a logged-in one. The page now bundles the runtime with the generated
bindings (below), so the exception protected nothing and went with the import.

The API's synthesizers for the `single-user` id (`api/users.go`,
`api/teams.go`) are gated on single-user mode: unguarded, they answer every
question about the owner with a made-up user called «Вы».

A desktop publishes nothing off its own machine. The tailnet door that did — the
same front door published as a node of the user's tailnet — was removed, and
`docs/deferred.md` keeps what it was and how to bring it back.

## The page talks to Go through generated bindings

The page calls the bound `App` service through bindings Wails generates from it,
as in a project `wails3 init` writes: `frontend/bindings/…/app.ts`, one function
per exported method, each a `Call.ByID` over the `@wailsio/runtime` package the
page bundles. `wails3 task generate:bindings` writes them, and `build:frontend`
runs it whenever Go changed, as the template does — against the headless build
with cgo off — `App` is the same service in both builds, the output is
identical, and that is the combination that type-checks on any machine.

They replaced a shim, and the reason is the types. The bootstrap in `proxy.go`
used to import `/wails/runtime.js` and build `window.go.main.App` as a Proxy
turning every property read into `Call.ByName('main.App.<name>')`, with the
methods' types written by hand beside it in `IAppWindow`. That hand-written list
drifted from Go silently — a renamed method or a changed argument was found out
at runtime — and the first compile against the generated types found a method
the page was calling that the list did not have (`ListCutOffConversations`).
Now the types are Go's own.

Three things follow.

- **Every exported method of `App` is callable from the page**, the way it
  always was, and now it is also in a file somebody reads. Wiring that `main`
  does itself — `setApplication`, `setOrigin` — is unexported for that reason:
  exported, it was a setter any page could call, and it dragged three packages
  of Wails' own models into the page's bindings.
- **They are committed**, so building or testing the page needs no Go. What that
  costs is that they can fall behind: `bindings_test.go` fails on a method added
  or removed without regenerating, and CI regenerates them with the Wails that
  go.mod names and fails on any difference, which also catches a changed
  signature. The npm runtime is pinned to exactly that Wails version, by the
  same test, because the two are halves of one protocol shipped in two packages.
- **They are imported, as in the template**, and there is no global. The page
  still has to be told whether a Go side answers, so the bootstrap seeds the
  session and sets `window.hasGoSide`, and nothing else; `goApp()`
  (`goBridge.ts`) hands out the service only when it is set, and every call
  site asks it. The outward-link catch-all that used to be in the bootstrap is
  the page's own now (`installGoBridge`), since the bootstrap had reached Go
  through the global. A test does not reach the runtime at all: the setup file
  mocks the bindings module for the whole suite (`src/test/goSideMock.ts`), and
  a test that wants a Go side installs its methods with `fakeGoSide`. In
  browser mode a mocked module exports a fixed set of names, so every binding
  is a forwarder, present as in a real build, and one the fake left out
  answers a promise of undefined. Before
  this the bindings were put at `window.go.main.App` and forty test files wrote
  their fakes there, which kept the call sites untouched and the old seam
  alive.

**Under `wails3 dev` the page is Vite's.** `build/config.yml` is the template's
dev loop — the Vite dev server in the background, with HMR — and `wails3 dev`
hands the app its address in `FRONTEND_DEVSERVER_URL`. A development build's
front door takes the page and every module from there (`devserver.go`,
`newServerProxy`), injects the bootstrap into Vite's HTML exactly as into the
board server's, and leaves `/api/v0` and `/ws` to the board. Vite's HMR client
connects back to the page's own origin, which is the front door, and goes down
the same proxy as an ordinary upgrade. Before this the dev loop rebuilt the
bundle with `vite build --watch` and served it off disk, which cost a rebuild per
edit and made a class of bug of its own — the window read the bundle in the middle
of a rebuild and opened white. A release build never listens to the variable:
the page it serves is the one it embeds.

**Events do not come back that way, though.** The Wails bus delivers an event by
running JS in the windows the application owns, so a page opened in a browser hears
nothing. `eventsws.go` broadcasts the same events over a socket on the
front door instead (`/acp/events/ws`), `emitter.go` sends every event both ways, and
the page listens only on the socket — `components/acp/agentEvents.ts`, one shared
connection for the whole page, with backoff and a "look again" nudge to every
subscriber when it reconnects. A new UI event needs nothing but `Emit`.

**`/m` is the board on a phone**, and deliberately not the board: a few screens
and a row of buttons at the bottom, which is where a thumb is. «Входящие» — what
a source left and nobody has looked at, carried onto a board from there;
«Карточки» — one board's cards as a list, to find out where something got to
without walking to the desk; «Ждут» — what is asking for a person, answered in
place, because a question carries its own options. «Ждут» is what opens, being
the one that cannot wait; the rest count themselves on their own button, which
is why the page and not the tab holds those lists — a tab that only counts once
you are looking at it counts nothing.

It is `pages/mobile/`, lazily routed, and it **asks nothing of the board's own
API**: everything on it, the board included, comes
from `main.App.*` and the event socket, both of which the front door serves to a
phone exactly as to the window. That is what `ListBoards`/`ListInbox`/
`ListBoardCards`/`MoveCardToBoard` are for (`bindings/boards.ts`) —
carrying the store, the board client and the websocket onto a screen that shows
a list and moves one card would be the whole app to do a tenth of it.
`router.test.tsx` guards the one thing that could silently break the page: the
board's catch-all route is `/:boardId?/…`, which `/m` fits.

`mobile/` is the phone app itself, and it is **a second Go module on purpose**:
`wails3 ios overlay:gen` compiles `package main` from the module root, and this
root's main is a board server with cgo SQLite and git. The phone runs
none of that — the app is a window onto `https://<server>/m`, which is what the
desktop's own window is too. It keeps the servers in the platform's secure
store, because a person may work more than one: the window holds **a tab per
server and a frame behind each tab**, which is what keeps every board
same-origin with its own front door and costs the server side nothing. Where
the phone goes next is a plan in `docs/todo.md`. Two things cross that boundary — a `postMessage` with
the number waiting there, so a tab can carry it, and a `no-cors` probe of the
machine's own page, since a frame's failure is not readable from outside it.
`mobile/README.md` has the build commands; `go test ./...` there covers the
address rules and the machine list.

## The page is Solid, and its state is a store rather than Redux

A Solid component is a function that runs once and wires reactive reads, not a
render function that re-runs. The price of that is a bug class the whole page
shares: a value read outside a tracked scope is a value frozen at first run, and
every migration bug found so far has been one — `useAppSelector` returns an
**accessor**, so `foo()` is the value and bare `foo` is a function and therefore
always truthy; props are getters, so destructuring one takes a snapshot.

It is **Solid 2**, and that is a different reactive model rather than a version
bump: an effect is a tracked compute and an untracked apply
(`createEffect(() => dep(), (value) => …)`), the apply's return value is its
cleanup, a write is staged until the microtask drains, `onMount`/`batch`/
`produce`/`createResource`/`classList` are gone, and a boolean handed to
`aria-expanded` drops the attribute. `docs/solid2.md` is what changed, the bugs
the move nearly shipped, and what is still owed.

One dependency is Solid 1 only — `@dnd-kit/solid` — and runs on a compat layer
in `frontend/solid1/`, pointed at that package alone by importer path. Applied
globally it rewrites what `@solidjs/web` itself imports, which builds the Solid 2
renderer on Solid 1 semantics; that is the whole reason it is scoped.
`@dschz/solid-flow` was on it too, until its 1.0 line shipped for Solid 2.

State lives in `src/store`: `createAppStore(deps, initialState)` builds one
store tree with per-domain actions, passed down by `AppStoreProvider`
and read with `useAppSelector(selector)`, which memoizes on the fields the
selector touched. There is no dispatch and no thunk — an action is a method that
writes the store, and the client it calls arrives through `deps` rather than a
module import, which is what lets a test hand it a mock. Every write is a draft
callback — a plain assignment, a `produce`-style body or a `reconcile` alike —
and `storePath` is not used. It takes an updater's return value and so writes
the branch away: `reconcile` returns nothing, and `storePath(…, reconcile(x))`
on every branch of the initial load drew every board empty with no error
anywhere. With one way to write, there is no second way to get wrong.

React-only libraries were replaced rather than wrapped: `@dnd-kit/solid`,
`@solidjs/router`, `@formatjs/intl` behind our own `src/intl.tsx` (same `IntlShape`, same message ids), and headless
Lexical with a typeahead menu of our own under `markdownEditorInput/plugins/`.

**Anything drawn over the page is placed by `@floating-ui/dom`** — the menus,
the tooltips, the combobox list, the typeahead and both tour tips, always the
same shape: `autoUpdate(anchor, floating, () => computePosition(…))` with
`flip`/`shift`, the result written as a `transform`, and a class the stylesheet
answers with `position: fixed`. Placing one inside the page is the bug this
prevents: `.Kanban` scrolls and `.mainFrame` hides its overflow, so an
absolutely positioned menu on a card was clipped by both and read as sliding
under the sidebar. Hand-rolled geometry for a new popover is a rewrite of a
dependency that is already here.

The other half of that move is a rule about the stylesheets left behind it: a
placed element is `position: fixed` with its corner at the top left of the
screen, so **`left`, `right` and a percentage width in a call site's CSS now
mean the viewport**. `.Select .Menu` still carried `right: 0; min-width: 100%`
from when a menu was an absolutely positioned child of its wrapper, which under
the fixed strategy reads as "start at the left edge of the screen, end at the
right" — every dropdown in the app opened the full width of the window, and it
had been that way since the move. A field's list is as wide as its field
because it *asks* to be: `matchAnchorWidth` on `Menu`, floating-ui's `size`
middleware, in the one place that already knows the anchor's box.

**A box sized in pixels has to be measured again when its width changes**
(`editableArea.tsx`). The card's title is a textarea grown to its text by a
hidden twin's `scrollHeight`, and that was recomputed on the value alone — so
opening a panel beside the card rewrapped «Поднять зависимости» onto two lines
while the box kept the height of one, and the title read as «Поднять» with the
rest clipped. A `ResizeObserver` on the twin is the fix, and it covers the
window being resized too. The panel also made the card's own side padding worth
looking at: a fixed 126px each side left 275px of a 527px column, so it is a
share of the column with that 126px as the cap.

**A dialog is not above the sidebar the way it looks.** `.mainFrame` carries a
z-index, so it is a stacking context, and everything a dialog inside it says
about its own z-index is said *within* the frame — while the collapsed
sidebar's ☰ is positioned over the frame on purpose, and therefore over the
dialog too. Raising the frame is not the fix, since the sidebar's own menus have
to stay above the board. So the screen that needs the whole window takes it:
`components/wholeScreen.ts` is a counter, `useWholeScreen()` holds it for as
long as the component lives, and the sidebar draws itself `offscreen` while
anything does. One caller — `automationDialog.tsx`, whose canvas is the width of
the board.

Two rules the drag-and-drop earned the hard way, both silent when broken.
`OptimisticSortingPlugin` is left out of every sortable, because it reorders
nodes the framework owns — and the price is that a sortable's `index` and
`group` no longer move during a drag, so **where a drop landed can only be read
off `event.operation.target`**, never off the source. And **two entities may not
share an id**: dnd-kit's registry is a map keyed by id, so a category that
registered its sortable and its boards drop zone both under the category id had
no drop target of its own. `hooks/sortable.tsx` carries the rest of the
reasoning; `sortableDrag.test.tsx` and `sidebarDrag.test.tsx` drive real drags
with the browser's mouse over real layout (`src/test/drag.ts`) and are the
guards.
Several widgets survived the rewrite untouched because their logic already sat
in a plain module under a thin wrapper — `hotkeys.ts`, `calendar.ts`,
`combobox.ts` beside `widgets/calendar.tsx` and `widgets/combobox.tsx`; keep new
ones that shape. Nothing may import React or Redux again: `no-restricted-imports`
in `eslint.config.ts` fails the lint, which is cheaper than finding a React
component rendered from Solid at runtime — which is exactly how that guard
earned its place.

## A card moved into a column is a session, and it answers through MCP

`internal/acp` is the agent integration, and it is board-agnostic: `internal/
boardadapter` is the only package importing both it and the board server.

**The whole rule the app applies on its own is one sentence** (`trigger.go`): a
card moved into a column, whose assignee is an agent this machine has
registered, is a card that agent starts working on. Nothing configures it. A
column is an option of the board's own select property and means whatever the
people using the board mean by it; which column somebody drags a card into is
their sentence, and the agent reads it along with the card.

What used to be here was a registry saying that this column runs an agent with
this crew and that one deploys, and a graph of routes saying what follows what,
and an engine walking it. It is gone, and so is every question it made a person
answer before the board would do anything. What it bought — a card moving on by
itself when a step passed — the agent now does directly: it is told which
columns the board has and moves the card into one of them with `move_card`.

**The agent is told where the card stands and where it can be put**
(`cardplace.go`, `boardStanding`), in the task rather than in the tools'
instructions, because whether a CLI has loaded its tools is its own business and
the task text always arrives. The columns are read off the board every time
rather than remembered: a column added this morning has to be offered this
afternoon.

**A move the agent made itself starts nothing** (`noteAgentMove`,
`movedByAgent`, `agentMoveWindow`). A move is what starts a run and an agent
ends its run by moving the card, so without this the board would hand the card
straight back to the agent that had just finished with it, for ever. The window
is short and the entry is used once: what it has to survive is the round trip
from the write to the board event, not the rest of the day.

**A move while a session is running cancels it** (`cancelForMove`), and waits
for it to let go of the folder before the next one starts (`cardIdleWait`). The
work that was running was about where the card was; the move is a new
instruction. A card dragged out from under a running agent is a card taken back,
whoever it now belongs to — so the cancel happens even when the card no longer
names an agent at all.

**A stage is an ACP session** (`session.go`, `question.go`), which is what this
was before terminals and is what it is again. The agent runs as an adapter on
stdio, a tool outside `autoAllowTools` arrives as `session/request_permission`,
a decision as a form elicitation, either one **blocks only the request that
asked** — the SDK gives every inbound request its own goroutine — and an
unanswered one refuses on cancellation or shutdown rather than stalling for
ever.

**A stage has two executors, and the card cannot tell them apart**
(`session.go`, `conversation`; `modelconv.go`). An agent is a CLI over ACP, or
a model over its vendor's API with the person's key (`kind: "model"`,
`internal/modelapi`, Anthropic's Messages API and OpenAI's chat completions
through their official SDKs). Everything around a turn — the slot, the timeout,
a cancel that arrived early, the question on the card, the comment, the stall —
is the session's and the same for both; `conversation` is the one seam, and a
model's loop (ask, call the tools it asked for, hand the results back) is ours
instead of the CLI's. A model is given what a CLI is given by the same means:
the board's tools are the same `boardmcp` server, reached in memory instead of
over the front door (`BoardForGrant` is the one adapter for both); the agent's
MCP servers are reached as an MCP client, remote ones included; a question to
a person is `ask_person` → `Manager.ask`. What a CLI brings of its own — files
and a shell — is `modeltools.go`, inside the card's copy and under the same
policy and the same permission question as a CLI's Bash. A model with no key
stalls the card (`ai.no-key`) rather than failing a minute later.

**A hosted server has no machine of anybody's** (`SetHosted`, `hostedBuild`):
no folders, no drafts, no CLI agents, no process a person named —
an agent's MCP server there is an address, and a stdio server is started only
when it is this program itself. A model there works with the board's tools
alone, and the brief says so. The page reads the same answer
(`MachineSettings.hosted`, `buildKind.ts`) and hides folders, proxies, the
changes panel and the CLI kinds.

**A model agent is somebody's** (`agent.owner_id`, NULL on a desktop, where
the one person owns everything): it runs on its owner's key, and the bindings
list and change the signed-in person's agents only (`agentsOf`, `ownAgent`).
The key is a credential presented to somebody else, so it is in the secrets
store under the person (`aikeys.go`: `ai/<provider>` on a desktop,
`ai/<userID>/<provider>` on a server), checked against the provider before it
is stored, and the page only ever hears whether there is one. Settings → AI
holds the switch over all agents and the keys.

**How the app learns the stage is over**: the agent says so, through the board
tools — `finish_work`, which reports done-or-not and one line about what it did.
**And the task text says so too** (`resultBrief`), because whether those tools
arrive *loaded* is the CLI's business and not ours: measured, Claude Code
deferred the whole board server once, so `finish_work` reached the agent as a
name in a list and nothing else — it did the work, wrote a summary and stopped,
and the card stood where it was with nobody able to say it was over. The brief
is the one thing that always arrives, so it carries the instruction as well; it
names no way to load a tool, that being each CLI's own.

**There is one kind of attention, and it is the agent asking** (`attention.go`,
`acp:attention`). It wanted a tool it was not given, or sent an elicitation, and
its turn stays open until somebody answers. The second kind — a card parked
where nothing runs, which the route engine raised when it handed a card to a
person — went with the routes: nothing hands a card to anybody any more, a
person moves it and is looking at it while they do.

**A question is drawn in three places and answered in any of them.** On the
card: an amber badge in its bottom-right corner (`KanbanCard__asking`) and the
question itself under the case stamp in the card's dialog, with its answers
(`attentionAnswers.tsx`). In the notification stack, which is the same component
under a box that can be dismissed. And on the phone's «Ждут» screen. All three
call `AnswerQuestion`, and whichever arrives first is the answer; the others
stop showing it when the wait ends. The card is the one that cannot be
dismissed, because a question outlives being read.

**A wait has an addressee once there is more than one person**
(`attentionAudience.ts`, `docs/teamwork.md`). The card answers it: «Кто
занимается» is the field the whole board says who a card belongs to in, and it
already tells a person from an agent. So a wait on a card assigned to a person
is announced to that person; a card assigned to an agent, or to nobody, names no
person and its wait goes to everybody. Following the card is the other way in.
It is worked out on the page rather than in Go because the board has the card
loaded already, and the Go side would be reading one per wait; and on an install
of one person the question is not asked at all.

**Being told is a thing that happens once** (`internal/acp/attentionack.go`).
The ✕ on a notification and an answer given anywhere are the same answer to the
same question — whether anybody still needs telling — and both call
`AckAttention(key)`. The ack is kept by the Go side, not by the page, so one
click takes the notification off every window and off the phone and a reload
does not bring it back; the card's amber badge is untouched, being part of the
card. It was a list in the page keyed by the wait *and when it was raised*, and
that made a loop where looking at a wait produced a fresh one a minute later. A
wait that ends takes its ack with it.

**The stack belongs to the window with the board in it.** It is mounted outside
the router (`app.tsx`), which is what puts it wherever in the app a person is,
and the list it draws is deduped by the wait's own key: one wait drawn twice is
the notification a person cannot tell from a second agent asking the same thing.

**Every one of those surfaces needs somebody to be looking at a window**, and
the case a wait is raised in is usually the opposite one: the app is minimised,
or behind an editor, or on another space. So the same wait goes out through the
two surfaces a desktop keeps for exactly that — the OS notification centre and
the menu bar (`alerts.go`, `alerts_desktop.go`, desktop build only: a headless
install has no menu bar and would post to whoever is logged into the server).
Both are driven by re-reading `Manager.Attention()` on `acp:attention` rather
than by the payload, since a wait ending and a wait being acknowledged arrive as
the same event and what both surfaces need is the picture afterwards.

The line between them is the setting. **The dot on the tray icon is an
indicator**, like the card's amber button: it interrupts nobody, it is drawn
while anything is waiting whether or not it has been acknowledged, and it is
therefore not a setting. **A notification interrupts**, so it is the switch —
and it is *the switch that already existed*, `agentNotifications`, read out of
the install's settings by the Go side (`agentNotificationsEnabled`) rather than
duplicated, because "tell me when an agent is waiting" is one question and two
of them would need a rule about which wins. It is also suppressed while any
window of this app has focus: announcing a wait to somebody who is looking at
the board is the noise this is meant to be the opposite of. Clicking it opens
the card and acks the wait — `openWait`/`AckAttention`, the same pair
`attention.ts` performs, so going to look takes the notification off every
window and off the phone. **Both buttons open the menu** and the way in is
the open item inside it, because opening the app straight off a left click is not
reachable on this Wails — see `docs/deferred.md`.

**Taking a notification down is not `RemoveNotification`**, which is the call
that reads like it and is a stub returning nil on macOS and on Windows.
`RemoveDeliveredNotification` is the one, delivered being what ours are —
nothing here schedules — and on Linux the two are the same method.

**The app outlives its windows**, which is what makes the icon worth having:
one that dies with the last window is a decoration on a window, and every agent
conversation used to die with it. Closing the board *hides* it — a
`WindowClosing` hook that cancels, so the framework's own destroy listener never
runs and coming back is the same window with the same board rather than a page
loading from scratch. The Dock icon leads back too
(`ApplicationShouldHandleReopen`), and the menu's «Выход» is the way out.

**Which lever stops the app quitting is per-platform, and using one lever costs
⌘Q.** Windows and Linux call `Options.ShouldQuit` from a teardown that *both*
the last window closing and `App.Quit()` go through, so there it has to be a
flag `requestQuit` sets. macOS asks the same hook from
`applicationShouldTerminate` and from nowhere else — the last window closing is
settled separately, by `ApplicationShouldTerminateAfterLastWindowClosed` — so
answering with the flag there refuses ⌘Q, the Dock's Quit and everything else a
person reaches for. `appShouldQuit` says yes on darwin for exactly that reason;
it was measured, not reasoned — the runtime's own Quit answered 200 and the
process stayed up.

Three things about the notifications are traps already fallen into. The notification service is
started **by hand** (`ns.ServiceStartup`) and never registered in
`application.Options.Services`: a service whose startup errors takes the whole
application down, and on macOS this one errors whenever the binary is not a
bundle — which is every `wails3 dev` run there is, so a dev build has the menu
bar and no OS notifications. Permission is asked at the first wait rather than
at launch, so an install whose owner never runs an agent is never interrupted to
be asked. And the icons are **two inks rather than one template icon**
(`build/tray/{idle,waiting}-{light,dark}.png`, drawn by `go run ./tools/appicon`):
macOS renders a template icon from its alpha alone, so the amber badge — the
whole point — would come out the same grey as the mark. macOS and Linux take one
icon and never ask for a second, so which ink is right is ours and is redecided
on `ThemeChanged`; Windows keeps both and swaps them itself.

**The card says an agent is asking, and that is all it says.** An amber badge
in the card's bottom-right corner (`KanbanCard__asking`), whose click opens the
card — where the question is, with its answers under the case stamp. It used to
be a console glyph in three shapes, opening the card's terminal: amber while an
agent asked, quiet ink while a CLI merely ran, and a still pause where a stage
had been cut off mid-work. Two of those said something about a machine rather
than about the work, and the third belonged to a kind of interruption that no
longer happens — a session ends by ending.

**The agent writes one comment, and writes it at the end.** `comment_card` is
in the brief as the first of the four things a finished stage does (`resultBrief`):
one comment, with what was done or decided and what a person has to know next —
not a log of its steps. This side writes one only when the agent did not
(`commentFirstTurn`, `hasCommented`), so the fallback is a fallback rather than
a second comment about the same work.

There were a dozen comments once — started, cancelled, asked, answered, moved
along the route — and a card whose comments are a log of the machinery is a card
nobody reads, with the one thing worth reading buried in it. Everything that was
narrated there is shown instead: the branch and the worktree on the card's
stamp, the question on the card's face, the stall in its own record.

**And the card draws them only where somebody can be talked to**
(`docs/teamwork.md`). A comment is a thing said to a person who will read it
later, and on an install of one person that is nobody. So
the panel and the badge hang off `teamMode` in the client config — one answer
from the Go side, the same one the board server was started with — and an
install of one person writes comments it never draws. What that costs there is that the agent's own comment is written
and not drawn; what the card cannot show for itself is shown on the card
instead.

**The changes are the other thing the panel holds** (`cardDiff.tsx`,
`internal/acp/carddiff.go`). A card comes back to a person to be decided about,
and there was nothing on screen to decide *from*: the branch was a label on the
card and the changes lived in a directory somebody had to go and find. It is git and nothing else — the claim says where the work is,
`git diff <base>...<branch>` and `git diff HEAD` say what it holds, and the
patches are never cached, because a stored answer is wrong the moment the agent
commits again. **The copy is not where the work is kept**: a stage ending folds
it away (`FoldWorktree`) and leaves the branch in the folder's object store —
and the column this is read in is the one the agent moved the card to *after*
that. So the
committed half is read from the folder by branch name, which works whether or
not the copy is still there, and the uncommitted half only while it is, since
what is unsaved in the folder itself belongs to whoever is sitting in it
(`gitPlace`). A
card working in a plain folder records no claim at all (`ClaimWorkspace` writes
none for `WorkModePlain`), so the folder the card names is the fallback — and
that is exactly the card whose changes are deliberately left uncommitted for
somebody to read. It is a reader and nothing else: how the work went is what the agent
wrote in its comment, and where the card stands is where somebody put it.
`/acp/diff/:cardId` is the window, because a column beside a card is the
narrowest place in the app to read code in.

**And who did what is `blocks_history`, read at last** (`server/app/
cardhistory.go`, `GET /cards/{cardID}/history`, `cardHistory.tsx`). The board
has always kept a full copy of every block on every write, stamped with the id
of whoever made it, and nothing ever read it: the card could say what it says
now and never who made it say that. The entries are the difference between
consecutive copies — a title rewritten, a property filled in, a card moved —
plus one per comment. Computed on the server rather than on the page for two
reasons: those rows reach the page through no other door, and naming a property
or an option needs the board's schema, which is there.

**Which is only an answer if the writes are signed.** On a desktop the board
runs single-user, so every write — a person's and an agent's alike — arrived as
`model.SingleUser`, and a history built on that could only ever say "you". An
agent already has a board account (`AgentEntry.UserID`, the same one its cards
name it by), so a run says on the context whose its writes are (`internal/
author`, a leaf package both sides can import; `asagent.go` sets it, `Writer`
reads it). Anything that does not set it writes as the single user, exactly as
before. The comment an agent writes through `comment_card` is signed by it, and
so is the fallback this side writes when it did not.

**And the button is drawn on a count, not on hope** (`card_changes`,
`CardChanges`). It appears only where there is something to read: a button that
opened «ничего не изменено» is a button nobody can learn. The dialog's answer is
counted on the spot — one card, one git run, and a stale answer would be wrong
in both directions — while the row it writes is what a *board* would read, one
row per card instead of a git process per card. The row is also written where
this side already knows the work moved (`releaseSession`), which is the only
moment that can still count the unsaved work: the copy goes with the stage. Counting is deliberately not in `finishSession` — that is the
status transition, and it stands in front of the card's own comment, so a git
run there delays the comment by as long as git takes.

**They are in the panel, not in the card.** The body of a card is what a person
wrote; a comment is about the card rather than part of it, so it is drawn in the
panel beside it (`cardComments.tsx`, a button in `cardDialog.tsx`'s toolbar)
along with the changes.

**"Nothing happened" is state, never a comment.** A stage that would not start,
a card refused because a person holds it, a folder another card is using — each
is true only until somebody fixes the registry or moves the card, and a comment
outlives it as noise (`«Агент не запущен: …»` from SINGLE-USER was the reading
experience this replaces). One stall record per card (`card_stall`, `stall.go`)
holds the current reason: replaced by a newer reason, deleted by any progress,
and handed to the card via `GetCardAgent.stall`. The reason is a code and its
values beside the English (`reason_code`, `reason_args`), which the page words
in its own language (`internal/apperr`, `docs/i18n.md`).

**The board is one projection and the only one.** A column is an option of the
board's own select property, and which property that is is recorded on the board
(`columnProperty`) rather than matched by name, so a board in any language and a
board somebody renamed the status field on both work. There is no second model
over it: no nodes, no edges, no per-node anything. What was there — a route
canvas, a column registry, an editor with a tab per route — is described in
`docs/decisions.md` under what it cost and why it went.

A template is **shown and not edited**: `templateEditor.tsx` is a form — name,
icon, what it is for, and the setup questions — with the board's own columns
listed in words beside them. A template is made by building a board and saving
it («Сохранить как шаблон…», `saveAsTemplate.ts`, which copies the board under
the board's own name), so what it carries is what already worked, and the way to
change it is to change the board and save it again. `docs/templates.md` is that
half written for somebody using it.

**The marker is the app's to give** (`TemplateMarkerProperty`, `template`).
It is how the importer recognises its own copy of a template across launches, so
a copy that carries one is a copy claiming to be it: a board made from
«Разработка» carries the marker, `saveBoardAsTemplate` copies the board's
properties, and three «Разработка» stood in the picker while the importer
maintained whichever came last. A copy that carries one is **disowned** at the next
launch — the property comes off and it is listed among the person's own —
rather than deleted, since it is their board and all that is wrong with it is a
word it inherited. Taken off there rather than at the moment of copying because
there are two doors and only one of them is ours: «Сохранить как шаблон…» goes
through `saveBoardAsTemplate`, and «Новый шаблон из доски» in the sidebar's
board menu is a plain `duplicateBoard` that never touches our code. Read as the
single user, because a template somebody saved is private to them and the store
answers a caller with no user id with the open ones alone. Which templates are
*ours* is read the same way round: `createdBy`, not the version stamp, because a
board carries the version of the template it came from and a copy of that board
carried it too — the picker counted such a copy as shipped and then showed it in
neither list.

**A template is one board and a catalogue of words** (`templatetext.go`,
`templates/i18n/*.json`). The files are written once, in **English**, and
translated on the way in: the catalogue is keyed by the very string the file
carries, so there is no third name to keep in step, and a string with no
translation stays as the file wrote it rather than disappearing. English is the
source for the reason the page's own strings are — a message id there is an
English default and `ru.json` translates it — so both halves of the product are
written the same way round. A copy of the board per language was the
alternative, and it is two descriptions of one board: a column added to one of
them is a column missing from the other, and nothing says so until somebody
opens it. What gets translated is decided by the JSON key (`title`,
`description`, `name`, `value`, `column`, `property`, `hint`, `prompt`), which
is what keeps ids, colours and the slug out of it;
`TestEveryTemplateStringIsTranslated` fails when a template gains a string a
catalogue has not got, and its opposite fails when a catalogue keeps
translating something no template says any more. Both read the templates **off
the disk**, because a base build embeds the paid edition's boards nowhere and a
catalogue covers every edition.

**The templates are stored in English and translated where they are read.**
The installed copies are never in any other language, so the language is each
person's: on a server two people read the same template in two. The page
translates the install's own templates as they arrive — from the REST client
and from the socket (`appWords.ts`), with the words and the list of fields Go
hands it (`App.TemplateWords`), so the walk is the same on both sides. A board
made from one is translated once, by Go, as it is copied: the page sends its
language with the duplicate (`?lang=`), `server/app` copies the board itself
when the source is an install template and passes it through
`TranslateTemplateCopy` before anything is written. From then on it is the
person's data, and no language switch touches it — as in Notion, where a
template is shown in your language and a copy is plain content. **Nobody
writes to an install template** (`Board.IsInstallTemplate`, refused in
`localpermissions`): it is replaced whole on an upgrade, and a word written
into it would be shown untranslated to everybody.

Installing them in the language the page showed was the previous shape, and on
a server it reinstalled every template at every page load of a person reading
in another language — and renamed the one inbox for everybody. A switch now
loads the other catalogue and reads the app's boards again
(`language.rereads`); nothing is reinstalled. The template picker still
refetches when it opens, because an upgrade replaces the boards and nobody is a
member of an open template, so its deletion reaches no client.

Nothing is exempt from that translation any more. Three strings used to be —
«Исход» and its two values, which the engine matched by title inside English
template files — and they went with the field: nothing writes an outcome now,
so nothing has to recognise one.

**What an agent is told first is two texts, and there are two because there are
two owners** (`internal/acp/boardprompts.go`). The board's — `boardPrompts`, on
the board as `prompt`, so it travels with it — goes to everybody working
here, and is «Системный промпт доски…» in the board's ⋯ menu. The agent's own —
`AgentEntry.Prompt`, in the registry — holds on every board this machine has,
and is «Настройки → Агенты». `promptLead` puts them in that order in front of
whatever the caller appends, which is the one place four call sites used to
write the same two blocks by hand.

**A third was built and taken out again**, and the reason is the rule rather
than the feature: a text per (board, agent) is the only one of them that
belongs to no single thing a person can point at — a cell of a table, not a
setting of the board or of the agent — and a person cannot reason about what an
agent was told if the layers outnumber the nouns on screen. What a *folder*
wants said is already said in the folder, in the `AGENTS.md` its own CLI reads,
and what one card wants said is the card. So the dialog prints the order it
composes, and the count it prints is two.

And what a board keeps **lives on the board** — `prompt`, `setup` and the three
records saying which of its fields hold the folder, the branch and the columns,
in the board's own properties, in the board database. That is why a live board
and a template are the same keys, and why a board carried to another machine
takes its instructions with it. `internal/acp` holds a working copy in memory
and writes every edit through to the board (`persistBoardLocked` in
`boardseed.go`); `config.json` does not carry any of it — `BoardPrompts` is
`json:"-"`. A board that refuses the write keeps its entries for the run and the
next edit tries again.

**A setting lives where its owner does**, and that is the rule the whole
settings surface is sorted by. What the machine keeps for itself —
what a card-less conversation opens saying, whether an agent waiting may
interrupt, and the archive that carries every board in and out — is
`settings/appSettingsDialog.tsx`, one dialog of panels opened from
`sidebarSettingsButton.tsx`, reachable with no board open.

**One switch turns the agents off** (`aiSwitch.ts`, the `aiEnabled` install
preference, «Агенты работают с карточками» in `settings/aiPanel.tsx`, the AI
section of the settings, over CLI and model agents alike). Off,
the app is a kanban board and nothing on screen names an agent. The page does
it at the gates it already had: every `is…Available()` of `components/acp`
answers no, the setup plan keeps only the name (`withoutAgents`), nothing
waits (`useAttention`), the agents' card fields are hidden
(`withoutAgentProperties` — the folder and the branch) and the assignee field offers people only. Go reads the same key
(`aiEnabled` in `machinesettings.go`) through `Manager.SetSwitchedOff`: a card move
is dropped before the engine sees it, the folder poll and agent sources stop,
and `startSession` refuses whatever was already queued. What is running is
left to finish — the switch means no more, not undo — and nothing is deleted,
so switching back on is the board as it was. The tray keeps its open and quit
items and loses the dot.

**Two registries are the machine's and are offered from a board anyway**,
because a registry belongs where it means something: the agents and the
proxies under them. An agent means nothing to a board of shopping lists, so it
is a door in that board's ⋯ menu, drawn on the board's own setup plan
(`asks('agent')`) — `agentsDialog.tsx`, beside «Папки…». The lists themselves
are unchanged and still one per machine; only where they are asked for moved.
Deploy hosts were a third, and went with deploying to Dokku, which is a flow
step in Gandvík (`../xxvi`) now.
Agents and proxies are **one door and two sections in it**, agents above: a
proxy exists to be named by an agent (`AgentEntry.proxyName`), so it is the
second half of one question rather than a settings section of its own. The
cosmetics those panels grew inside the settings dialog moved to
`styles/_settingsPanel.scss`, or the same panel would have two designs.

**What a person picked for the UI is the person's**: the theme, the language,
the last team, board and view, the name format live in the board's
`preferences` (`/users/me/config`), beside the tour and the hidden boards. On a
server everybody has their own and takes them from device to device; on a
desktop the person is `SingleUser`, in the same database as the boards the
last-board keys point at, so a remembered board cannot belong to another
database. **What the install decides is the install's**: `aiEnabled`,
`agentNotifications` and `prefillCardFolder` are `system_settings`, read and
written through the bindings (`machinesettings.go`), because the agents run on
the machine and a desktop attached to a server must not switch off somebody
else's; on a server they are the server's, and only an administrator turns
them (`ownsTheInstall`). The page reads both before the first render
(`settingsRead.ts`: the theme and the language have to be right on the first
paint) and again after a login or a logout, since a server's login page is
drawn with nobody's; `UserSettings` holds them in memory and writes through.
Both builds do this the same way. A desktop's old `<dataDir>/ui-settings.json`
is carried into the database at the first launch and deleted
(`uisettings_migrate.go`); a server's was one file for everybody and is not
read. **The theme and the language are settings like the rest of them**,
and are `settings/appPanel.tsx` with the link to the manual: they spent a while
in the corner of the board on the grounds that they are changed while looking at
it, and what that cost was two icon menus and a question mark standing in for
three words, plus a corner that had to be published over the no-board screen to
stay reachable. The way to say something is broken went the same way and for the
same reason — where to say it is looked for once, and it is looked for where the
manual is — so the corner is gone rather than reduced, and `topBar.tsx` with it.
**A theme is a config, and the config is Mattermost's** (`frontend/src/theme/`,
`docs/themes.md`). The keys are the ones its documentation names, so a theme
written for another product is pasted into the field and worn, and one written
here goes back. That is the whole reason for borrowing a format rather than
inventing one: themes for it already exist, and nobody hand-writes twenty-six
colours for a board. `_tokens.scss` keeps its shape and its two theme blocks —
it is the **first paint**, before the module has run — and applying a theme
writes the same raw tokens onto the document element; the two shipped themes
ship *also* as configs, and a test compares them with that file token for
token, because a stylesheet and a config disagreeing is a palette that changes
as the page finishes loading. What a config does not say is derived — the
raised surface, the ten label tints, the shadows — and what it says unreadably
is adjusted, since `awayIndicator` is an eight-pixel dot there and the word
«ждёт» on a card here. Every shipped theme is held to the same contrast bar,
ours included; that bar is what moved the paper theme's amber and its stamp a
few levels.
**A theme is a pair, and which half is a second setting.** «Pourquoi Pas?», «Gruvbox»,
«Acid techno» each come light and dark, and `themeMode` — light, dark, or
whichever the system is set to — says which is worn. One list asked both
questions at once and could answer neither: it said «Gruvbox Dark» and «Как в
системе» in the same column, so choosing a design also fixed a polarity, and
following the system meant giving up every design but ours. A family with one
half is refused by a test, because that is a switch that looks like it works and
moves nothing; the eight themes borrowed from Mattermost's list are published
there in one polarity and exist in both at home, so the missing halves are the
real ones. **The themes you paste are pairs too, and there may be many of them**
(`theme/custom.ts`, one `customThemes` key holding the text of both halves of
each): they stand in the same list, below a line, under a «+ Новая тема» row
that forks the theme on screen. Each is named by a field of its own, and the
editor is in a frame because it is a thing being made rather than a setting
being read. Which half the field is editing is asked there —
asking that twice would be two controls saying one word; a half nobody wrote
falls back to the one they did, and opening the slot forks both halves of the
theme on screen. Which half the *field* is editing is a
third question and is asked only there, by a light/dark of its own: «как в
системе» is a real answer for a board and none at all for a config, there being
no third config to edit. It opens on the half on screen and then stays put —
what is being typed and what is being worn are two questions. The themes are the one
place the "lay the choices out rather than fold them into a menu" rule turned
over — twelve rows in `widgets/select` and not a `<select>`, whose own comment
says why — and the polarity, being three answers, is chips like everything
else. That list is flat: it carried a heading over each of the three kinds and
they were three lines of furniture over twelve entries a person picks by name,
so the order says it instead.
**Neither of those two addresses is the repository.** The panel points at the
guide (`Constants.guideUrl`, the doc site `docs/guide/` publishes) and at an
email address (`Constants.feedbackEmail`), because the repository was standing
in for both while there was no site: somebody who opens «Руководство» wants the
manual and not a source tree, and a bug report should not cost a person an
account on a hosting service. The address is printed on the panel as well as put
in the `mailto:`, since a webview that refuses to open one leaves nothing to
copy.

**What a board had to be asked before it can run is not that, and each question
is its own item in the board's ⋯ menu**: «Системный промпт доски…»
(`boardPromptsDialog.tsx`), «Папки…» (`workdirsDialog.tsx`), «Пройти настройку
заново…» (the wizard). Which of them a board has is **the board's own setup
plan** (`BoardSetupPlan`), so the menu differs by board exactly as the questions
do.

**The first question is the board's own name** (`SetupStepName`), and it is the
only step that is not about this machine and the only one with no way past it.
A board arrives called what its template is called, so the second one made from
«Разработка» is a second «Разработка» in the sidebar — and names are unique:
the step refuses one another board has, and so does renaming a board by its
title. One rule, `boardTitle.ts`, used at both doors a person types a name
through; trimmed and case-insensitive, because two boards nobody can tell apart
on screen are two boards with the same name whatever the bytes say. The wizard
reads the other names through `listBoards()` rather than the store, so it needs
no provider around it in a test.

**Three questions are every board's** (`impliedSetup`): its name, an agent, and
the last screen saying what to do with what was set up. The agent is what a card
needs to be worked at all, and nothing about a board implies more than that any
more. **The folder is not a board question**: it is asked on the card, whose
«Папка» field lists the board's folders and adds one («Добавить папку…»,
`cardFolderValue.tsx` over `acp/cardFolder.ts`) — registered for the board,
mirrored into the field, set on the card. A board declaring the retired
`project` or `deploy` step has it dropped in silence (`retiredSetupSteps`),
since boards made before carry them (docs/decisions.md). A board may ask for
others of its own in `setup` (`BoardPropSetup`): whatever the closed set grows
later. The set is closed on purpose — a board that could
declare arbitrary fields would be a program, and nothing on this side could
promise the answer fits the registry it is going into.

It used to be read off the board's stages, which is how a step could be *implied*
at all. With the routes gone there is nothing to infer from, so the three are asked of everybody
and the rest is the board's to declare.

**Relaxed mode is the word for that, and for the moment it is a word and not a
thing.** Every setup question may be left unanswered — «Пропустить» in the
wizard, or a chip nobody ticked — and the board still works, on a default this
side picks. What makes it a mode rather than a pile of accidents is that each
default is *named where the question is asked*, so leaving one is a choice with
a stated consequence rather than a screen somebody escaped from. There is
nothing in the code called `relaxed`: nothing stores it, nothing branches on
it, and a board is in it by not having said otherwise. Whether it should become
a thing the code knows — a flag, or a state a board reports — is open; the term
exists so the defaults can be argued about together instead of one dialog at a
time.

What it means today, question by question:

- **no folder** — the card's session runs in drafts of its own,
  `<dataDir>/boards/<boardID>/cards/<cardID>` (`cardDrafts`), kept after the
  run because what the agent wrote is there; a planning conversation reads
  «черновики доски» (`<dataDir>/boards/<boardID>`). A folder the card *names*
  and the board no longer offers is not "no folder": that stays a stall
  (`folders.unchosen`), since the drafts would be somewhere nobody chose;
- **nobody assigned** — nothing starts, and the card sits in the column
  somebody put it in. The assignee field offers every agent on the machine, so
  answering it is one click on the card;

The rule a new default has to meet is the one those share: it must be sayable
in one sentence on the screen that leaves the question open, and it must be the
answer that keeps a person able to work — offering too many agents rather than
none, waiting rather than refusing, a folder of drafts rather than an error.

The rest of the ⋯ menu is export — the archive in the settings dialog is every
board there is, and one board's own is the board's own business, which is also
the whole of why import is not offered per board: what an archive brings is
boards, plural, and Trello/Notion/Todoist are instructions for making one rather
than an importer of ours. It is **last, under a separator**: carrying the board
out is not a setting of it, and the two exports used to open the menu with the
questions a person came for underneath them. A separator draws nothing when it
has no group above or below it (`separatorOption.scss`) — every menu here is
built out of `<Show>`s, so the group above a line can be empty, and that rule
lives with the line rather than being re-derived at each call site. **"Сохранить как шаблон…" is parked there**
(`OFFER_SAVE_AS_TEMPLATE`, one line in `viewHeaderActionsMenu.tsx`): the
machinery is untouched and still reached by the template picker's pencil, but
making a template is the rarest thing anybody does on a board and it stood one
slot from the things a board is set up with. Registering an agent needs none
of these screens: `agentQuickAdd.tsx` is the two-field form, used by the card
and the setup wizard alike.

**The registry is not an answer by itself.** A card names its folder with an
option of the board's own field, so anything that adds a folder mirrors the
registry into it (`workdirSync.ts`), the wizard included — and a folder somebody
has already added is offered rather than refused («использовать здесь»:
`AttachAgentWorkdir` for one no board claimed, `ShareAgentWorkdir` for one
another board owns), because one checkout worked from two boards is an ordinary
arrangement. That the option is the folder's *name* is a compromise with a
deadline on it: `docs/deferred.md` says why an id belongs there instead.

**A card names its agent by whom it is assigned to**, and by nothing else. Each
registered agent has a board account under its own name, so «Кто занимается»
answers the question the whole board already asks with that field — and it is
now the *only* thing that decides who works a card: `assignedAgent` matches the
card's assignee against the registry by account id first and by username after,
the id being the answer that survives a rename and the username the one that
still answers for a card assigned before the account was recorded. A card
assigned to a person starts nothing, because `humanAssignee` vetoes the session.
There used to be a second one — an «Agent» select `agentSync.ts` kept in
step with the registry — and two fields for one question meant a rule about
which of them wins and a field that said nothing on a board where nobody had
registered an agent. It is gone, and `resolveSessionAgent` no longer reads
select options at all —
that match outlived the field as a rule with nothing behind it, where any option
anywhere on the board spelled like an agent quietly decided who worked the card.
`humanAssignee` is what keeps "assigned to a person" and "assigned to an agent"
opposite answers rather than the same one. A card property named `agent` went
with them, and for the same reason: nothing in this app creates one, so it was a
third answer only a hand-built board could give.

**The account is made when the agent is registered** (`AddAgent` →
`EnsureAgentAccounts`), because that is the moment it becomes a name a card can
be assigned to — and it needs no board: an account is a row in the board
server's own users table, which is what every board reads when it offers people
to assign a card to. There was a sync instead, `syncAgentsToBoard`, run from
wherever a board happened to be open, and it was a sync in search of an event:
register an agent in the settings, never open a board's own screens, and the
agent had no account anywhere. Running it on every board render was worse —
finding-then-creating is not atomic, so one agent got three accounts. A
registry that predates this is caught up once, at startup (`ensureAgentAccounts`
in `Manager.Start`). Every registered agent may be assigned a card on any board:
the board keeps no crew of its own, so the only thing that narrows the field is
the AI switch (`agentAssignees.ts`). `SyncAgentUsers` survives for the board
*membership*,
which is the board's own business and decides only whether the agent is listed
under «участники доски» or found by search.

The username is `AgentUsername`, and it keeps letters of **any** script. It
folded to `a-z0-9` once, on a board whose every label is Russian: «клаус» became
the empty string, was skipped by `AgentUsers`, got no account and could never be
put in the one field that says who works a card — and «клаус 2» was provisioned
under the name "2".

A folder marked global joins only boards that already have a «Папка» property,
so a board nobody works code on never grows one.

## A folder hands out a workspace, and the card owns it

`internal/acp/workdirs.go` is the registry — named folders on this machine, one
of which a card names in its «Папка» field. The code calls them working
directories (`WorkdirEntry`), the screen calls them папки, and neither ever
calls them projects again: a folder of household notes is not a project, and
the word made every board of shopping lists look like it was missing one. The
keys in `config.json` (`projects`) and on the board (`projectProperty`) keep
their old spelling, because they are stored data.

**A card names its folder by id, and the id is the entry's own**
(`WorkdirEntry.ID`, `cardWorkdir`). The board records which property holds the
folder (`BoardPropProject`, `projectProperty`); the *option* offering a
folder is created under that folder's registry id (`workdirSync.ts`), so a card
that names a folder is a card holding an ordinary select value that happens to
be a reference. Nothing about the entry has to stay still except the id — which
is what makes a rename possible, and what lets a place to work stop being a
folder on this disk at all: a repository to clone, a drive, a machine over ssh
have identities of their own, and for them the "name" is exactly the part
somebody will want to change. `docs/deferred.md` carried this as a plan; it is
this.

It used to be a **name**, matched by scanning every option selected anywhere on
the card and then the name of the column it came from, so a label named after a
repository decided where an agent worked — and since the names were collected by
ranging over the property schema, which is a Go map, which of two matches won
changed between events. The same rule already taken out of
`resolveSessionAgent`, for the same reason. Two fallbacks are left for data that
predates the id: an option made before it is matched by its *name*, and a board
that recorded no folder property at all gets the old scan, that being the only
thing such a board can say. `CardMoved.SelectedOptions` carries property and
option ids, and `CardMoved.Values` is the whole card keyed by property id —
filled on every path a card is read by, which is how any field this app made is
read off a card.

**The field is one choice, and the singular is the type talking.** A card claims
one workspace, works one branch — which the board keeps in a single text field —
and hands its agent one cwd, so a second folder had nowhere to go:
`resolveWorkdir` took the first of the selected options and dropped the rest,
which is a choice made for the person and made in silence. So the field is a
single select (`workdirSync.ts`). It is named in the language the page is
read in when this app makes it (`fieldNames`), and never renamed after: the
board records the field by id, so what it is called is the person's.

**A board with one folder fills the field in** (`soleWorkdirOption`, applied in
`centerPanel.newCardProperties` beside the view's filter and the group a card
was dropped into): one option is not a choice, and making somebody give the same
answer on every card is a question with no second option. Counted off the
*field* rather than the registry, because options are never taken away — a board
that has known two folders offers two, and two is a question only the person can
answer. It writes an ordinary value, visible and clearable, which is what makes
writing it unasked acceptable; the filter and the group still win, and a board
grouped by its folder is left alone entirely, or its empty group could never
take a card. `UserSettings.prefillCardFolder` is the switch, on by default,
one of the install's own (`machinesettings.go`).

**Where a card's work is has one answer** (`Manager.CardWork`). Two places knew:
the claim in this app's own database, which knows the whole of it — directory,
branch, base, how the folder is worked in — and knows it only on this machine;
and the card, which knows one thing, its branch, and knows it everywhere,
because a card travels and its fields go with it. Every reader that picked one
was wrong somewhere: the folder lock read the claim and so let go on the second
machine, the stamp read the card and so spoke before anything was claimed. They
are merged once, with the precedence stated — the claim wins where it exists,
being the fuller record of the same fact — and `CardWork.Started` (work exists
anywhere) is deliberately a different question from `CardWork.Here` (this
machine holds it).

**And it stops being a choice once the card has a workspace**
(`cardDetailProperties.tsx`, read off `CardWork.Started` — work exists, which is
what "работа началась" actually means; the column the card stands in is not, and
neither is «this machine holds the workspace»). Work on a card lives in one place, claimed under the folder
the card named then, so pointing the field at another folder afterwards does not
move the work — it makes the card describe somewhere the work is not. The field
goes read-only with the reason under it, in the one place a person changes it,
rather than being refused later and further away. The card's face carries the
folder too (`KanbanCard__folder`), from the field and never from where a
conversation happens to stand: «черновики доски» is not an answer the card
gives.

**A repository is an ordinary folder with a superpower, not a second kind of
thing.** One registry, one adder, one list: in «Обсудить с агентом» and in «В
какой папке будет работать агент?» the repositories stand among the plain
folders, marked and nothing else (`folderChoices.tsx`). What a folder *is* is
asked of git every time it is listed (`WorkdirStatus`), never remembered — `git
init` in a folder added last month makes it a repository and the registry hears
nothing. `Kind` records only a declaration: the setup step of a board that
publishes a branch demands a repository (`SetupRequiresGit`), and answering it
with a folder that has no git is refused where the answer is given.

**A branch appears because somebody took the folder to work on a card, not
because the folder is a repository.** `workspace.go` is the one entry point —
`ClaimWorkspace(WorkSpec)` — and who asks does not change the answer: every
session a card has, one after another, gets the same directory and the same
branch, because a workspace belongs to the **owner** (the card id, or
`board:<id>` for a conversation with no card) rather than to the run. Each run
used to make its own, so a card worked three times left three branches and three
checkouts. A conversation with no card gets
the folder itself and creates nothing.

Two ways to work in a repository, and the answer is the **folder's, on the board
that offers it** (`WorkdirEntry.Modes`, board id → mode): `worktree` — a copy
and a branch per card, several cards of one repository at once; `branch` — a
branch in the folder itself, one card at a time, the person's own checkout
moving with it. There is no third answer, because "work on whatever is checked
out" is what an ordinary folder already does.

Keyed by *both* on purpose. One answer per board is wrong the moment a board has
two repositories; one answer per folder is wrong for a folder marked «на всех
досках», which is one entry seen from several boards — the board where three
people work it may want a copy per card while the board where one person does
does not. A folder belongs to one board anyway, so for almost every entry there
is exactly one answer and the screen reads as "this folder works like this".
`worktreeDir` and `keepFailedWorktrees` stay in `config.json`, being about this
machine's disk, and `worktreeMode` survives as the default for a folder nobody
has been asked about (`always`→worktree, `never`→branch).

`branch` mode is the one that can refuse: the folder is held by one card
(a `checkout` row) until its branch is merged, and it will not switch under
somebody's uncommitted work — tracked work, since untracked files survive a
switch untouched and every checkout has some. Both refusals are **state, not
failure** —
`errWorkdirBusy`/`errWorkdirDirty` become the card's stall reason and the card
keeps its place, instead of being carried off to «Заблокировано» by something
that is not about the work. A merge is what frees the folder, and it is found
out when the next card asks for it rather than by watching: `folderIsFree`
asks git whether the holder's branch is gone or already in its base — locally,
then on the remote after a fetch, since a pull request is merged there
(`branchIsDone`) — and never while the holder still has a live session, because
a branch with nothing committed yet reads as merged. A watcher that polled
every branch did this once and went with the routes, leaving the folder held
for good. Picking the next card up automatically is deliberately not built
(`docs/deferred.md`).

**The branch is the product, the copy is the workshop.** After a stage finishes
the directory is folded away (`FoldWorktree`) and the branch stays; the next
session on that card remakes the copy from the branch, which is what makes
folding it safe — and it never touches a copy with uncommitted work in it or one
a CLI is still running in. The branch is also written onto the card, into the
text property the board records as `branchProperty`: a machine's database
does not travel, and the card does — to another board (`MoveCardToBoard`) or
another machine. The path is not written, because a path means nothing there.

The branch is named after the card's title, transliterated (`branchSlug`
carries а Cyrillic→Latin table, so «Почини логин» is `pochini-login-…` rather
than a hash) — or by the agent itself when `agentNamedBranches` is on: a short
headless ACP session (`naming.go`, the same shape as a source's run) answers
with one name, and anything wrong with the answer falls back to the title. Machine-wide, because it is about how this machine spends
agent runs.

Where a session works follows from what it is: an agent gets the card's own
workspace, because it writes code; a planning conversation stands in the folder
itself and claims nothing, because it only reads.

## An agent talks back through MCP, not through its output

Everything above is us talking to an agent. `internal/boardmcp` is the way back:
an MCP server giving an agent the board as tools. It is what planning ends with —
a conversation about what to do leaves the cards on the board itself, where before
a person read the screen and retyped them — and it is what finishing ends with
too, since a card carries what happens to it next in its own column.

The surface is the board in the vocabulary a person uses: read it
(`list_columns`, `list_card_fields`, `list_cards`, `get_card`), put work on
it (`create_card`, `create_cards`), change it (`update_card`, `comment_card`),
carry a card on (`move_card`) and end the run (`finish_work`). Everything is
**named, never identified** — a column, a folder, a field and its values —
because names are what
a person typed and ids are what the board's own REST API would have made an agent
learn. A **card id** is the one exception, and it has a default: empty means the
card the run stands on (the grant carries it), which is the card an agent working
on one always means.

**Four tools are what a finished stage does**, in this order, and the task text
says so (`resultBrief`) because a CLI may not have loaded the tools by the time
it is told: `comment_card` with the result, `update_card` for anything the task
asks to be recorded on the card, `move_card` into the column whose name says
what has happened, and `finish_work` — done or not done, and one line — which
ends the stage. `finish_work` is load-bearing: the session's turn ending is not
the same as the work being over, and only the agent knows which. A run with no
card behind it is told so, since the tool is about a card rather than about the
board.

`move_card` is `update_card` with a column in it, and the split is for the model
rather than for the code: moving is the call whose *consequence* is the point.
Which is why `acp.CardEdit` is the one write in `BoardWriter` that lets the board
notify — the rest are the integration's own bookkeeping and must not re-trigger
the agent that produced them, but a card moved because an agent asked for it has
to reach every window and every other reader, exactly as a person's drag does.
What it must *not* do is start the agent again, and that is remembered on the
way out rather than read off the event: `noteAgentMove` before the write,
`movedByAgent` in the trigger loop. The card's
**description is not editable**: it is a person's content, and what an agent has
to say about a card goes where everything else a session says already goes.

**The app serves it, over HTTP, on the front door** (`/acp/board/mcp`,
`boardapi.go`). An MCP server that does work of its own is a subprocess of the
agent, but this one *is* the app: the board lives in this process, and the app is already a
separate process from the agent, already listening on an origin it can reach. A
subprocess in between would have been a proxy of ours to ourselves with the tool
schema written twice. It is not the board's own REST API either, because what it
offers is ours: a column that means something, a folder by name.

The server carries **a grant token, not a board id**
(`internal/acp/boardtools.go`): the token names the board, is minted per agent
run and dies with it, so an agent cannot leave cards anywhere else, and one found
later opens nothing: the model picks steps and never targets. The handler is **stateless**, so every request
carries the grant and no session id outlives the check. Now that the tools reach
existing cards, every call that names one is checked against the grant's board
(`grantedCard`): a card id is something an agent can read anywhere, and without
that check one board's grant would edit every other board's cards.

A session declares MCP servers in `session/new`, where ACP has a field for HTTP
ones — and only when the adapter says it takes them
(`AgentCapabilities.McpCapabilities.Http`). An agent whose adapter does not is
an agent with no way to end a stage, since `finish_work` is a tool: it runs, and
the card waits for a person.

## A source brings cards in, and the app decides what they become

`internal/sources` turns outside events into cards: mail, an issue, a
notification from a phone. It is board-agnostic the way `internal/acp` is, and
deliberately **not part of it** — cards from a phone on a board of household
chores are useful to somebody who has no agent and never will, so a source has
to work with the agent integration switched off.

A source is a **plugin**, not a branch in our code: a separate process speaking
JSON-RPC over stdio (`sources/protocol`, `internal/sources/plugin`), which is
what lets one be written in TypeScript and by somebody else. The plugin does one
thing — hands over items. It never sees the board: rules, columns and cards are
this side's, because a plugin author must not be trusted with the board and
because otherwise every plugin would invent its own filter syntax.
`ingest.go` is the way in for everything that has no plugin — a script, a phone,
a webhook — a route on the front door guarded by a per-source token.
`docs/sources.md` is the whole design.

**An MCP server is the second kind of plugin, and it needs no code at all.**
What MCP has no word for is "the feed" — its tools are described for a model —
so a manifest says which tool to call, with what arguments, and how to read one
row of the answer as an item (`internal/sources/mcpsource.go`, mapped with the
same `text/template` a rule's properties use). Past `dialPlugin` nothing knows
which protocol the process speaks. Manifests come from two places: the ones
this app ships (`internal/sources/manifests`, embedded — each one's server is
this binary re-invoked, `pourquoipas mcp <name>`, so there is nothing to install), and
the ones somebody drops in `<dataDir>/sources/manifests/*.json`, which win by
name. A
service with a server of its own is the second kind, since a manifest then
names a path on that machine.

The system's «Поделиться» is a source too (`internal/sources/share.go`,
`share.go`, `pages/share/`): the share extension in the .app opens
`pourquoipas://share?url=…`, a small window asks which board — the inbox, offered
first and already picked, or another — and the link goes down the same
pipeline. It is the one entry allowed to name a board other than the inbox
(`PickBoard`), because a person is looking at the dialog when it happens. On a
board with columns the card stands in the first one (`boardmodel.IntakeColumn`),
since a card in no column is on the board and on no kanban.

**Everything a source brings lands on one board, «Входящие»**
(`boardadapter.EnsureInboxBoard`). The app makes it at launch — it is also
where a person writes down what they have not decided about yet, so it cannot
wait for the first source — and finds it by a board property it gives it
(`inbox`), never by title: a person may rename it. A source names no
board, and a rule decides only whether an item is claimed, never where it goes.
Collect first, decide later: a card leaves the inbox when a person carries it
to the board it belongs to.

It used to be the other way round — an inbox column and view on every board a
source fed, with the source naming its board and a rule naming a column. That
made the source decide where a thing belongs before anybody had looked at it,
which is the decision an inbox exists to put off (`docs/decisions.md`).

The inbox board has **no column property**. Nothing on it is a stage and nothing
moves between columns: a card is read, then carried away or deleted. Its one
view is a kanban grouped by who brought the card — a column per source — and
those columns are facts about the past rather than places, so a card cannot be
dragged between them (`columnsAreFacts` in `kanban.tsx`). The person's own cards
form a column too, headed "My tasks" rather than by their username
(`kanbanColumnHeader.tsx`, on the inbox board only), because what those cards
are says more than who typed them. The sidebar pins it above every category
(`SidebarInboxItem`), as a row with no views under it and no board menu; a
category never lists it, though the server still files it in one, since it
reorders a category only when given every board it holds.

So the inbox is the screen about **where cards come from**, and that is where
the sources are set up: «Источники…» is on the inbox board's ⋯ menu and nowhere
else, and that menu holds nothing besides (`viewHeaderActionsMenu.tsx`,
`isInboxBoard`, which reads the mark).

The inbox is this machine's board. A desktop attached to a server still files
into its own inbox (`sourcesByBoard.InboxBoard`): the sources are this
machine's registry. Only the share dialog may pick a server's board.

The inbox is **grouped by what brought the card**, and that needs no property of
ours: a source has a board account, exactly as an agent does and for the same
reason (`EnsureSourceUser` beside `EnsureAgentUsers`), so the card is *authored*
by its source and the board groups by "created by" out of the box. What a person
typed stays theirs. The account is named after the source rather than prefixed,
because the board shows the username wherever it names an author.

A card is then carried onto another board — from the card's own menu, or from
«Входящие» on a phone — and that is a **real move** — same card id, so comments come with it and everything outside
the board that remembers the card by id still finds it. It lives in the server
module (`app.MoveCardToBoard`, `POST /cards/{cardID}/move`) because moving a
card is the board's own operation, not ours, and because it cannot be built out
of what was there: `insertBlock` keys its update on `id AND board_id`, so
re-inserting a block under a new board updates nothing and says it worked.
Properties travel by **name**, since the two boards share nothing else.

The same carrying can be left to an agent (`internal/inboxsort`,
`docs/sources.md` §29), switched on from the inbox's ⋯ menu. It decides by each
board's title and **description** — asked with the name when a board is made —
and by a **summary of the board's cards** that the agent writes itself and this
machine keeps (`board_summary`), rewritten when the cards change. A card it
files stands in the board's first column **without notifying**, unlike the
person's move above: the sorter picked a board, not a column's automation.
Every decision, moved or left, is a row of `inbox_sort`. Like the sources, the
package imports neither the board nor the agents; `main.go` hands both in.

## The app replaces itself, and trusts one key while doing it

`updates.go` is self-updating, and the machinery under it is the framework's:
`app.Updater` is on every Wails v3 application with nothing to register, and it
owns the whole risky part — streaming the artifact, checking it, unpacking it,
and the helper process that waits for us to exit and renames the new bundle into
place. What this repository adds is the three things the framework leaves to the
application.

**Which feed to trust.** The provider is `endpoint`, not `github`. `github`
reads a release's assets and can verify a `SHA256SUMS` sidecar, which catches a
corrupted download and nothing else — the hash and the file come from the same
place — and it ties the app to a public repository for ever, which this one is
not. `endpoint` reads a signed `manifest.json`, and the signature is checked
against `build/updater.key.pub` — `go:embed`ed, so the feed has no say in which
key authenticates it. `wails3 updater manifest` in CI is the whole publishing
side.

**The address is the part that cannot be taken back.** `updateManifestURL` is
`edition.ManifestURL` — `https://updates.deffun.org/stable.json` for the base
build — a domain of ours rather than a release page on somebody's platform,
because every copy already installed asks there and nowhere else: where the
*files* are kept is a decision that can be revisited every release, and that
one line cannot. So the manifest sits at one unchanging address and the
artifact links inside it are absolute per version (`…/v1.1.0/base/…`), which is
what lets one never move and the others always. It is also the only place the
release address is written down — the workflow reads the prefix back out of the
constant — so the app and the release cannot come to disagree about where a
release lives. **Which address it is belongs to the edition** (below): one
manifest names one artifact per platform, so a shared feed would hand a
lifetime install the base app under the version number it was waiting for.

**What survives a restart.** Nothing, on the framework's side: the download is
a temp directory the helper deletes, and the version a person skipped is a
field of the running `Updater`. So `<dataDir>/updates.json` keeps the switch,
the skipped version and when it last looked — `enabled` a pointer, because a
file written before the field existed must not read as "turned off". The timer
is ours too rather than `Config.CheckInterval`: `Init` may be called once and
`StopPeriodicCheck` cannot be undone, so the framework's timer would make
«проверять автоматически» a switch that takes effect at the next launch. And
the poll only ever *checks* — spending a hundred megabytes of somebody's
connection unasked is not the same act as telling them there is a new version.

**What a person sees.** Not the framework's window, which is good and entirely
hard-coded English. `updater.WindowNone`, Go subscribes to the eleven
`wails:updater:*` events and re-emits **one** — `acp:update`, carrying the whole
state — through `emitter.go`, which is the front door's socket and therefore
the only path that also reaches a page opened on a phone. The status in that
state is read fresh off `Updater.State()` rather than inferred from which event
arrived, because the bus dispatches each event in a goroutine of its own and a
status that went backwards is a progress bar that did. It is drawn by
`settings/updatesPanel.tsx`, and the only thing outside that dialog is a dot on
the sidebar's settings button: an update is not worth a notification.

`updater.HandleHelperMode()` is **the first line of `main()`**, before
`maybeRunMCP`. `application.New` calls it too, but by then this process has
opened SQLite, taken a port, restored a PATH from the login shell and started
plugin processes — all in the one process whose entire job is to wait for the
old one to die.

The version is a constant (`version.go`), not an `-ldflags` injection: every
Taskfile bakes its own `-ldflags="-w -s"` into a template string, and threading
a value through four of them makes the version a property of how the binary was
built, which is exactly what `wails3 dev` then gets wrong. Every other file that
states a version is listed in `internal/buildversion`, `version_test.go` fails
when they disagree, and `wails3 task version:set` writes all of them. Not
`common:update:build-assets`, which regenerates `build/darwin/Info.plist` from
the CLI's template and drops the hand-written `CFBundleURLTypes` block the share
extension is launched through. `docs/release.md` is the release itself;
`.github/workflows/release.yml` is a tag away from it.

## An edition is a build, not a flag

This tree compiles into two products (`internal/edition`, `docs/editions.md`):
`base`, which everybody gets, and `lifetime`, bought once, whose whole
difference today is that it embeds more board templates. **Which one a binary
is, is a build tag and is never read at runtime** — a licence file, an env var
or a flag makes the difference a thing this process *reads*, and anything read
can be made to read the other answer, while what the paid edition buys is files
that are simply not in the base binary. `templates/lifetime/*.jsonl` is a
second `go:embed` behind the tag (`templates_base.go`, `templates_lifetime.go`)
joined with the shipped set by `shippedTemplates`; `EDITION=lifetime` on any
Taskfile target adds the tag; the release workflow's matrix is `edition ×
platform`.

**The page is not told, and that is the point.** `VISIBLE_TEMPLATE_SLUGS` names
every edition's templates, and in a base install the extra slugs match no board
because no such board was ever imported. A page that knew which edition it was
would be a page that could be told otherwise.

Editions are also why the release feed moved into `edition.ManifestURL`
(above), and why the constants there are one `const` per line: CI reads them
out of the source with `sed`, the same way it reads the version.

The other end of that rule: **a template this build does not ship is deleted at
startup**, which predates editions (`importTemplates`) and is what makes
installing base over lifetime take the two extra templates away. Boards made
from them are the person's and are untouched.

## One database, and the schema is written once

Everything this application knows is about a card or a board, and both of those
are rows in the board's database. So our tables are in it too —
`agent_session`, `card_stall`, `checkout`, `source_item` and the rest — rather than in files beside it, which is where they
began (`acp.db`, `sources.db`). `internal/acp` and `internal/sources` take a
`*sql.DB` (`NewStore(db)`) and create nothing; `server.NewStore` hands
back both the store and the handle under it, and `runServerWithLogger` carries
the pair as `board{srv, db}`.

**The reason is a leak rather than tidiness.** Deleting a card is a real
`DELETE FROM blocks`, and this side never heard about it — `BlockChanged`
handles only `notify.Update` — so a deleted card left its conversations, its
place on a route, its stall and its queue slot behind for ever. A foreign key
onto `blocks(id)` is the fix, and it cannot be added later: SQLite's
`ALTER TABLE` has no `ADD CONSTRAINT`, so a key is written in `CREATE TABLE` or
never. The keys are written **and enforced**: `foreign_keys` travels in the DSN
rather than as a `PRAGMA`, being a per-connection setting — a pragma run once
is silently off on the next connection — spelled for modernc
(`sqlite_dsn.go`), whose DSN ignores a parameter it does not know, so
`internal/appschema` takes the spelling from `sqlstore` rather than writing its
own. What SQLite does not do is check rows that were already
hanging when the setting went on; the one-time sweep for those is still owed
(`docs/deferred.md`, «Одна база: хвосты плана»).

**The schema is Go data, and the SQL is generated.** `tools/schemagen` holds
every table once with dialect-neutral types, and `ariga.io/atlas` renders
both migrations — `000001_board` and `000002_app`, `.up`/`.down` — for SQLite,
MySQL and Postgres. Writing three dialects by hand is what the fork's eighty
migrations did, and every
`{{if .postgres}}JSON{{else}}TEXT{{end}}` is a place somebody has to remember
three answers to one question — here the question is asked once and one table of
types answers it. Atlas is a build-time dependency: what ships is the SQL, in
the repository, and `go test ./tools/schemagen` fails when the two disagree.
**Transactions work on SQLite**, and getting there is a chain worth knowing.
`@withTransaction` was switched off for SQLite because turning it on failed with
`UNIQUE constraint failed: blocks_history.id, blocks_history.insert_at`: the
history tables keyed on `(id, insert_at)`, `insert_at` comes from the database's
own clock, and inside a transaction SQLite hands every statement the same
instant. So the fork's composite operations — insert a block, write its history,
touch the board — ran as separate statements on the database everyone actually
uses, and a failure halfway left the database half-changed. That is what made
two test suites flaky, and it is why deleting a board and undeleting it at once
returned a 500 about a third of the time. The fix is that **`insert_at` comes
from Go** — `utils.NextInsertAt`, a per-process clock that never returns a
millisecond it has already given out — written at each of the eleven places a
history row is made, two of which are `INSERT … SELECT` and carry the stamp as a
literal in the projection. The keys stay, and one reader depends on them:
`undeleteBlockChildren` picks each block's latest history row by
`max(insert_at)`, so two rows sharing an instant hand it the same block twice and
it violates `blocks.id` on the way back in. The SQLite branch is gone from
`generators/transactional_store.go.tmpl`. What monotonicity does not cover is two
processes on one database; nothing here does that, and the honest answer when
something does is a key of the row's own rather than a clock.

**A closed set is a `CHECK`, a growing one is not.** `checkout.mode`,
`workspace_board.mode`, `agent_session.status`, `board_setup.status` and
`source_event.outcome` are constrained in the schema, because what they may
hold is closed by the model — there is no fourth way to work a folder. An agent
kind is a vendor CLI, a `session_event` kind is an event name, and a
`workspace` kind is meant to grow past `git` to a drive or a machine over ssh:
those have no check, because SQLite cannot `ALTER` one in, so a check on a
growing set buys a table rebuild every time somebody adds a value.
`TestAValueOutsideAClosedSetIsRefused` is the guard, and it covers the case
worth naming: a difference in *case* is refused too.

**The three dialects are checked, not assumed** (`internal/dbtest`,
`.github/workflows/test.yml`). The fork's fixture could always be handed a MySQL
or a Postgres on a port, and nothing ever handed it one — no compose file, no CI
job — so every run took the SQLite branch and went green, which is the worst
kind of green. `FOCALBOARD_STORE_TEST_DB_TYPE=postgres go test …` now starts the
container it needs (testcontainers-go, from the tests themselves, so there is no
second description of the containers to drift), and `wails3 task test:db:all`
runs the three in turn. What the first real run found, in order: **the MySQL DDL
would not execute at all** — a key column declared NULL, which SQLite permits
and MySQL refuses outright, so `build()` now forces NOT NULL on every key column
and the golden test carries that as a declared departure; `DEFAULT "NOW(6)"`
written as a *string* because atlas quotes any raw expression on a time column
that does not begin with `current_timestamp`; **board search by property name
had never worked on Postgres**, since `properties` is a text column and `->` has
no text overload; and the last data migration read the collation of a table
called `Channels` — Mattermost's — and took the whole open down with it. Every
one of those is a thing that only exists on the vendor nobody ran.

**golang-migrate still runs it**: this is a generator, not a migration engine,
and the engine already knows about versions, dirty marks and the record the
previous one kept. `go generate ./tools/schemagen` after any schema change.

**The ladder is two rungs.** `000001_board` is the fork's tables as this product
has them and `000002_app` is ours, where there used to be eighty-one files of archaeology leading to it
(`{{if doesColumnExist "boards" "minimum_role"}}`, `ALTER TABLE blocks DROP
PRIMARY KEY`, three data migrations interleaved at particular versions to repair
rows the next step would refuse). None of it means anything to a database made
today. What made deleting it safe is that the result is *checkable*: the
generator can print the schema as plain SQLite DDL, and a test compares it —
column, type, nullability, default, key and index — against a snapshot of what
the ladder actually built (`tools/schemagen/testdata/`). Column widths are
checked separately, because SQLite ignores them and atlas therefore does not
emit them, so a `varchar(64)` narrowed to `varchar(32)` is invisible to the
first check and silently truncates data on the two dialects nothing here can
run. A database built by the old ladder is refused with a message saying to
delete it: there is no release, so the only ones that exist belong to whoever is
working on this.

**No table carries a prefix.** `{{.prefix}}` in the migration, `s.tablePrefix`
at a hundred and thirty query sites, `DBTablePrefix` in the config and the
`{in braces}` our own queries were written with all named the same thing: the
namespace Focalboard's tables needed when they lived inside a Mattermost
database as a plugin. This application never set it — the default was `""` and
nothing overrode it — and the two places that did were tests, which get a
database of their own anyway. It went, and the migration's template actions went
with it except the three `{{if .sqlite}}` that pick a dialect. So did the
helpers those migrations were rendered with: `addColumnIfNeeded`,
`doesColumnExist`, `renameTableIfNeeded` and six more, four hundred lines of
per-dialect SQL string building that the collapsed ladder calls from nowhere.

Two consequences worth knowing before touching a query. **Absence is NULL**
wherever a key looks: a planning conversation has no card, and `''` is a card
id that does not exist; the reads say `COALESCE(...,'')` because in Go absence
here is the zero value. And **no
journal has an autoincrement** any more — `session_event` and
`source_event` take UUIDv7, which sorts by the moment it was made, so
`ORDER BY id` still means "as it happened" and three spellings of
`AUTO_INCREMENT` are gone. `session_event.seq` went with them.

A test needs that schema and must not carry a copy of the DDL, because a copy
drifts — the exact bug all of this is about. `internal/appschema` renders the
same migration out of the same embedded filesystem onto a scratch file; that is
the only thing it is for, and the running application never touches it.

Every run is an `agent_session`: one process, one turn, one verdict. There
used to be a second table, `conversation`, for a terminal's CLI that outlived
its process and was resumed with `claude --continue`; it went with the
terminals, and a planning conversation leaves no row at all.

**The registry's name is applied in the schema, and not yet in the code.** The
tables say it: **`workspace`** is the named place, carrying the git *settings* —
`kind`, `base_branch`, `branch_prefix`, the per-board mode — and **`checkout`**
is what it hands one owner — dir, branch, base, mode, the git *state* — in a
table of that name (it was `workdir_claim`). That reads as what it already is,
because a plain folder records no row at all: `ClaimWorkspace` creates and
writes nothing for `WorkModePlain`, so the table only ever held git copies. The
Go side still says `WorkdirEntry`, and the stored keys still say `project`
(`projects`, `projectProperty`) — they are saved data. Not `project` in anything new: that word was removed for a product
reason which has not changed — a folder of household notes is not a project,
and it made every board of shopping lists look like it was missing one.
