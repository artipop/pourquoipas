# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repository.

Lines are wrapped. Keep them wrapped — a paragraph on one line makes every edit a
whole-line diff.

This file is the map: the rules that break things silently when forgotten, and
where the reasoning lives. The reasoning itself is in `docs/architecture.md`
(how each subsystem is shaped and why), `docs/decisions.md` (rejected
alternatives and post-mortems), `docs/deferred.md` (what was thought through and
not built) and `docs/todo.md` (what is known and open). What works in which
mode — desktop, desktop attached to a server, server in a browser, phone — is
`docs/modes.md`. Read the relevant section of `docs/architecture.md` before
changing anything structural.

## What this is

**Pourquoi Pas?** is a desktop board that runs coding agents from the board. One Go
module built with **Wails v3**, the board server running **in-process**; the same
code builds a headless server (`-tags server`) that serves the board to a browser.

- `server/` — the board server, a fork of Mattermost's Focalboard. It is **ours**,
  not a fork kept recognisable: Focalboard is archived and nothing imports
  Mattermost any more. A directory of this module (`github.com/artipop/pourquoipas/
  server/…`), not a module of its own. Cutting it is ordinary;
  `docs/fork-shrink-removed.md` is the ledger of what went and how to bring it
  back. A removal is one commit with its tests, mocks and doc edits. Login and
  public sharing are kept on purpose; multi-team is to go
  (`docs/client-server-plan.md`).
- `frontend/` — its own npm project, laid out as `wails3 init` lays one out
  (bindings in `frontend/bindings`, the bundle in `frontend/dist`), built with
  Vite, rewritten from React to **SolidJS**. Upstream and this repository's early history are React and are no
  recipe any more; `docs/frontend.md` describes the page as it is.
- `mobile/` — the phone app, a **second Go module on purpose** (see below).
- README.md, "What this app requires of the frontend": the output layout
  `go:embed` expects and the one global the page consumes.

## Build & run

`docs/build-and-platforms.md` has the details and the traps already paid for.

- `wails3 dev` — the dev loop, the template's: Vite with HMR beside the app;
  the frontend's dependencies install themselves (`install:frontend:deps`). A dev
  build is its own install — data dir, keychain service and single-instance
  lock (`datadir_dev.go`) — or its window hands itself to a running Pourquoi Pas? and
  exits silently. A development build's front door takes the
  page from Vite (`FRONTEND_DEVSERVER_URL`, `devserver.go`) and injects the
  bootstrap into it as into any page; `/api` and `/ws` stay the board's.
- `wails3 task build` / `package` (`darwin:package:dmg`, `windows:package`,
  `linux:package`) / `build:server` / `build:docker`.
- **A server's database is the environment's** (`database_server.go`):
  `PP_DB_TYPE` (`sqlite3`, `postgres`, `mysql`) and `PP_DB_DSN`. A desktop
  is always SQLite. On the other two the board runs and the agents and sources
  are off, because their SQL is SQLite's (`appTablesSupported`).
- `wails3 task generate:bindings` — run by every build when Go changed, as in
  the template; by hand after changing `App` when no build follows. They are
  committed: `bindings_test.go` and CI's regeneration fail when they fall behind.
- Build tags travel as `EXTRA_TAGS`, default `frontend`; Linux adds `gtk3`.
- **SQLite is `modernc.org/sqlite` in every build**, pure Go, registered under
  the name `sqlite3` (`sqlstore/sqlite.go`); `SQLiteDSN` spells its settings as
  `_pragma=…`, and a mattn spelling is silently nothing. **cgo is Wails' alone**:
  macOS and Linux desktops need it, Windows and the server build without it and
  cross-compile from anywhere.
- **A packaged app is not a child of a shell.** launchd hands the `.app` a bare
  PATH; `internal/userpath` restores the login shell's PATH for the process,
  which is what lets an agent's CLI be found at all. A bug that exists only in
  the installed app is usually this.
- **The desktop app is a WKWebView, and CSS prefixes are the build's job**
  (autoprefixer in `vite.config.ts`, targets in `browserslist`). WebKit there
  does not know unprefixed `user-select`. Never write prefixes by hand — stylelint
  forbids it.
- **`frontend/dist` must never stop existing**: `go mod tidy` resolves the
  `go:embed all:` pattern in parallel with the frontend build, so a committed
  `.gitkeep` holds it and Vite clears around it (`emptyDist`).

Tests:

- `npm test` in `frontend/` — vitest in browser mode, headless Chromium
  through Playwright (`npx playwright install chromium` once), no coverage
  (`npm run test:coverage` for it), `npm run updatesnapshot` to rewrite
  snapshots. Real CSS and layout apply: a control shown on hover is found after
  `hoverOver`, a menu placed by floating-ui after `findBy…`, a drag is
  `src/test/drag.ts`. `docs/frontend.md` has the traps.
- `go test ./...` — no tags needed. `go vet -tags server .` checks the headless
  build, which has files of its own. `./...` also walks `frontend/node_modules`;
  that noise is cosmetic.
- Known rare flakes, only under a full `./...`: `TestPatchBoard` (`server/app`)
  and `TestASessionAsksTheCardForAToolOutsideThePolicy`. Re-run the package alone
  before blaming a change.
- `wails3 task test:db DB=postgres` (or `mysql`, `sqlite3`; `test:db:all`) — the
  store and API suites against a vendor in a container the tests start
  themselves (`internal/dbtest`); needs only Docker.
  `FOCALBOARD_STORE_TEST_DB_TYPE` is the whole contract.
- `go generate ./tools/schemagen` — after any change to the schema. It rewrites
  the migrations for all three dialects and the generated `docs/schema/*`;
  `go test ./tools/schemagen` fails when somebody forgets.

## Architecture

Eleven ideas hold this together. Each is a section of `docs/architecture.md`
with the same title; what follows is the part that must not be forgotten.

### The front door owns the origin

**The board server has no port**: its router is served over in-memory pipes
(`boardwire.go`). **The window's page is on `wails://`** and goes through the
asset server to the board router; **the board's changes and the agents' events
come on the Wails bus** (`ws.Server.SetSink` → `board:ws`, `BusFeed`); only an
attached server's `/ws` is tunnelled through Go (`sockettunnel.go`). Through the scheme handler WKWebView sends no `Content-Length` and no body
for a `FormData` — send a form with `sendableForm`. `frontdoor.go` is only the
desktop's loopback door for other processes (agents' tools, the feed,
`/sources/ingest`). **A server build
has none**: Wails' own server is the door, our routes are its asset middleware
(`mode_server.go`), and Wails checks `Host` and `Origin` the way a server
reached by any name needs. Nothing of ours goes on Wails' `/wails/events`, which
no session guards (`emitsOnBus`).

- On a desktop **nothing authenticates a user**: the protection is refusing a
  foreign `Origin`/`Sec-Fetch-Site` and a `Host` the door was not published
  under. Keep a server build on localhost unless something in front of it
  authenticates.
- **Who is asked is the build** (`identity.go` + `identity_desktop.go`/
  `identity_server.go`, `docs/teamwork.md`): a desktop is one person with a
  single-user token; a hosted server is many people with `/login` open, each
  with their own boards (no shared boards: `docs/deferred.md`), and there
  `/wails/` and both sockets require a session (`requireSession`), with no
  exception: the page bundles the Wails runtime, so nothing it needs before a
  login is behind the guard. A desktop joins a server by logging in
  (`attach.go`) and keeps its agents.
- **A desktop publishes nothing off its own machine.** The tailnet door that
  did was removed (`docs/deferred.md`); the way to reach a board from elsewhere
  is a server build. Where the phone goes next is a plan in `docs/todo.md`.

### The page talks to Go through generated bindings

`frontend/bindings` is generated from `App` (`wails3 task generate:bindings`) and
committed; the page calls through it over the `@wailsio/runtime` it bundles,
pinned to go.mod's Wails (`bindings_test.go`). **Every exported method of `App`
is callable from the page**, so wiring `main` does itself stays unexported. The
bootstrap in `proxy.go` only seeds the session and sets `window.hasGoSide`
(and `window.pageOnWails` in a desktop window);
`goApp()` (`goBridge.ts`) hands out the imported service only when it is set,
and every call site asks it. **No test reaches the runtime**: the setup file
mocks the bindings module for the suite, and a test installs its Go side with
`fakeGoSide` (`src/test/goSide.ts`), never a global.

**Events come back over a socket, not the Wails bus** (`eventsws.go`,
`/acp/events/ws`; tunnelled in a desktop window), because the bus only reaches windows the app owns and a page
opened in a browser hears nothing. `emitter.go` sends every event both
ways; the page listens only on the socket (`components/acp/agentEvents.ts`). A
new UI event needs nothing but `Emit`.

**`/m` is the board on a phone** (`pages/mobile/`): four screens, and it asks
nothing of the board's own API — everything comes from `main.App.*` and the event
socket (`bindings/boards.ts`). `router.test.tsx` guards that the board's
catch-all route does not swallow `/m`. **`mobile/` is a second Go module** because
`wails3 ios` compiles `package main` from the module root, and this root's main
is a board server with cgo SQLite; the phone app is a window per server onto
`https://<server>/m`. `mobile/README.md` has the commands.

### The page is Solid, and its state is a store rather than Redux

- A component runs once. **A value read outside a tracked scope is frozen**:
  `useAppSelector` returns an **accessor** (`foo()` is the value, bare `foo` is
  always truthy), and destructuring props takes a snapshot. Every migration bug
  so far was one of these.
- **It is Solid 2**, a different reactive model rather than a version bump
  (`docs/solid2.md`): an effect is a tracked compute and an untracked apply
  (`createEffect(() => dep(), (value) => …)`) whose **return value is its
  cleanup**, so a `Setter` passed straight in as the apply is a bug; a write is
  staged until the microtask drains; `onSettled` refuses `onCleanup` inside it
  and an `async` callback cannot be one; a boolean handed to `aria-expanded`
  drops the attribute — write `String(…)`.
- **No async memo is asked again while on screen** — no `refresh`, no async
  memo whose source changes. In the Solid 2 RC that tears `<Show>`: a binding
  inside runs once with the falsy value, `value()!.field` throws, and the whole
  page stops updating. Reads from Go go through `createLoader`
  (`hooks/loader.ts`); `docs/solid2.md` has the measurements.
- **`@dnd-kit/solid` is Solid 1 only** and runs on `frontend/solid1/`, a compat
  layer pointed at that package alone by importer path. Applied globally it
  rewrites what `@solidjs/web` itself imports. It goes when dnd-kit ships Solid 2.
- State is `src/store`: `createAppStore(deps, initialState)`, per-domain action
  methods, no dispatch, no thunk; clients arrive through `deps` so tests pass
  mocks. **Every write is a draft callback** (`setState((d) => { d.cards.current
  = id })`, `setState((d) => { reconcile(x)(d.boards.boards) })`); `storePath`
  is not used. It takes an updater's return value and so writes the branch
  away: `storePath(…, reconcile(x))` emptied every board once, silently.
- **Anything drawn over the page is placed by `@floating-ui/dom`** (`autoUpdate`
  + `computePosition`, written as a `transform`, `position: fixed`). So `left`,
  `right` and percentage widths in a call site's CSS mean the viewport; a list
  as wide as its field asks for it (`matchAnchorWidth`). Do not hand-roll popover
  geometry.
- A textarea sized to its text must be re-measured when its *width* changes
  (`editableArea.tsx`, `ResizeObserver`).
- **A dialog inside `.mainFrame` is not above the sidebar** — the frame is a
  stacking context. A screen that needs the whole window holds
  `useWholeScreen()` (`components/wholeScreen.ts`).
- Drag and drop: `OptimisticSortingPlugin` is left out of every sortable, so
  **where a drop landed is read off `event.operation.target`**, never the source;
  and **two entities may not share a dnd id**. `hooks/sortable.tsx`,
  `sortableDrag.test.tsx`, `sidebarDrag.test.tsx`.
- Logic goes in a plain module under a thin component (`hotkeys.ts`,
  `calendar.ts`, `combobox.ts`). **Nothing may import React or Redux** —
  `no-restricted-imports` in `eslint.config.ts` fails the lint.

### The board knows nothing of the agents

The board is Focalboard's and is to be contributed back some day as «Focalboard
2»; only our own AI by subscription is closed. **The board calls
`frontend/src/extensions.tsx` and imports nothing that fills it**: sources,
filters, named slots, card side panels, card property rows and settings
sections, registered once from `main.tsx` (`components/acp/install.ts`). A new
agent feature goes behind a seam, never inline in a Focalboard component;
`src/seams.test.ts` fails when a board file starts importing `components/acp`.
In Go, `server/` knows only machine users and the agent flag on a comment;
`internal/boardadapter` is the one bridge.

### A card moved into a column is a session, and it answers through MCP

`internal/acp` is the agent integration and is board-agnostic;
`internal/boardadapter` is the only package importing both it and the board
server.

- **The whole rule is one sentence** (`trigger.go`): a card moved into a column
  whose assignee is a registered agent starts a session for that agent. No
  column registry, no crew, no routes. A move the agent made itself starts
  nothing (`movedByAgent`, `agentMoveWindow`), or an agent would hand itself its
  own work back for ever.
- **A move while a session runs cancels it** (`cancelForMove`) and waits for the
  folder to be let go (`cardIdleWait`) before the next one starts.
- **The agent is told where the card stands and what the board's columns are**
  (`cardplace.go`, `boardStanding`), read off the board every time — a column
  added this morning has to be offered this afternoon. It moves the card on
  itself with `move_card`.
- **A stage is an ACP session** (`session.go`, `question.go`): a tool outside
  `autoAllowTools` arrives as `session/request_permission`, a decision as a form
  elicitation, either one blocks only the request that asked, and an unanswered
  one refuses on cancellation or shutdown.
- **An agent is a CLI over ACP or a model over its API** (`kind: "model"`,
  `internal/modelapi`, `modelconv.go`); the session reaches either through one
  `conversation`, and a model gets the board's tools in memory from the same
  `boardmcp` server (`BoardForGrant`), files and commands from `modeltools.go`
  on a desktop only. It runs on its owner's key (`aikeys.go`, the secrets
  store; the page sees "set" or "not set"). **A hosted server has no machine**
  (`SetHosted`, `buildKind.ts`): no folders, no CLI agents, no stdio MCP of a
  person's choosing. `docs/web-ai-plan.md` has what is left.
- **The stage ends when the agent calls `finish_work`**, and the task text says
  so (`resultBrief`) because a CLI may defer loading the tools.
- **Attention is one reason: the agent is asking** (`attention.go`,
  `acp:attention`). It is drawn on the card (the amber badge, and the question
  itself under the case stamp), in the notification stack, and on the phone's
  «Ждут»; all three answer through `AnswerQuestion` and the first answer wins.
- **Ack is kept by Go** (`attentionack.go`), so one click clears every window
  and the phone.
- The notification stack is drawn in the board window (`app.tsx`, outside the
  router). OS notifications and the tray (`alerts.go`, `alerts_desktop.go`)
  re-read `Manager.Attention()` on each event. Traps: the notification service
  is started by hand, never registered in `Options.Services` (it errors outside
  a bundle, i.e. in every `wails3 dev`); `RemoveDeliveredNotification`, not the
  stub `RemoveNotification`; the tray icons are two inks, not a template icon.
- **The app outlives its windows**: closing hides; «Выход» quits. Quitting is
  per-platform — `appShouldQuit` says yes on darwin, or ⌘Q is refused.
- **The agent writes the comment** (`comment_card`), one, at the end; this side
  writes one only when the agent did not (`commentFirstTurn`, `hasCommented`).
  "Nothing happened" is state, never a comment: one stall record per card
  (`card_stall`, `stall.go`). Comments are drawn in every install.
- **Git is off the screen** (`acp/gitShown.ts`): folder mode, base branch, the
  «Ветка» field, the branch on the stamp and «Изменения» are parked behind
  `SHOW_GIT`; the work is still a branch in a copy of its own.
- The card's changes are git and nothing else (`cardDiff.tsx`, `carddiff.go`),
  never cached, the committed half read by branch name because the copy is
  folded away when a stage ends; the button is drawn on a stored count
  (`card_changes`).
- **The card's history is `blocks_history`, diffed** (`server/app/cardhistory.go`,
  `GET /cards/{cardID}/history`, `cardHistory.tsx`): every write already left a
  copy of the block with its author, and the entries are the difference between
  consecutive copies. **Who** is a board account id, so an agent's writes are
  the agent's: a run says so on the context (`internal/author`, `asagent.go`),
  and `boardadapter.Writer` stamps it instead of `model.SingleUser`.
- **Planning is a conversation with no card** (`planning.go`,
  `planningDialog.tsx`): read-only policy, several turns, and the cards it
  agrees on are created through the board tools.
- Templates (`docs/templates.md`): the marker (`TemplateMarkerProperty`) is the
  app's to give, and copies are disowned at launch; which templates are ours is
  `createdBy`, and nobody may write to one (`Board.IsInstallTemplate`). They
  are stored in English, once, and **translated where they are read**: the page
  translates the install's templates and the inbox's own names as they arrive
  (`appWords.ts`, the catalogue from `App.TemplateWords`), and a board made from
  a template is translated by Go as it is copied (`TranslateTemplateCopy`,
  `?lang=` on duplicate) and is the person's data from then on. The catalogue
  is `templates/i18n/*.json` keyed by the English string
  (`TestEveryTemplateStringIsTranslated`) — every string, with no exemptions.
  A language switch reinstalls and renames nothing; on a server each person
  reads in their own.
- **What an agent is told first is two texts** (`promptLead`,
  `boardprompts.go`): the board's `prompt`, then the agent's own. A third layer
  per (board, agent) was built and removed; do not bring it back.
- **What the board keeps is the board's** (`boardseed.go`): `prompt`, `setup`
  and the three "which field is which" records (`projectProperty`,
  `branchProperty`, `columnProperty`), written through by `persistBoardLocked`.
  `config.json` does not carry them.
- **A setting lives where its owner is**: machine settings in
  `settings/appSettingsDialog.tsx`; **agents are connected there and nowhere
  else** — Settings → AI (`acp/aiPanel.tsx`): one list, one «Подключить
  агента…», one form for a program over ACP or a model by key, the key typed
  into the agent's form and kept per provider; proxies under the agents. No
  board has an agents screen, and nothing quick-adds one. UI prefs are the person's, in
  `preferences` via `/users/me/config`; the install's switches (`aiEnabled`,
  `agentNotifications`, `prefillCardFolder`) are
  `system_settings` via bindings (`machinesettings.go`). Both are read before
  first paint and after a login (`settingsRead.ts`); a desktop page holds the
  session in memory, never in the browser (`session.ts`).
- **A theme is a Mattermost theme config** (`frontend/src/theme/`,
  `docs/themes.md`), shipped as pairs (light/dark) with `themeMode` choosing the
  half; `_tokens.scss` is the first paint and a test holds it equal to the
  shipped configs. **The choice of theme is closed** (`themeChoiceOpen()`): the
  look is made from the app icon, and only light or dark is a person's.
- **A board asks nothing when it is made**: the setup wizard and its Go plan
  were removed, and a board from a template opens as it is. Board names are
  unique case-insensitively (`boardTitle.ts`). **The folder is the card's
  question**, added from its «Папка» field (`cardFolder.ts`); a card without
  one is worked in its own drafts (`cardDrafts`).
  **Every question may be left unanswered** ("relaxed mode", a word and not a
  flag): the default must be sayable in one sentence where the question is asked
  and must keep a person able to work.
- **A card names its agent by its assignee** («Кто занимается») and nothing else
  (`assignedAgent`, by account id then by username); `humanAssignee` vetoes a
  session on a person's card. Agent accounts are made when the agent is
  registered (`EnsureAgentAccounts`); `AgentUsername` keeps letters of any
  script. **No agent is a board member**: a person's agents are offered on
  every board through `extensions.tsx` (`agentAssignees.ts`), somebody else's
  never, and none with the AI switch off.

### A folder hands out a workspace, and the card owns it

`internal/acp/workdirs.go` is the registry of folders (`WorkdirEntry`; the
screen says папки, never "projects"; stored keys `projects`/`projectProperty`
keep that spelling).

- **A card names its folder by the entry's id** (`cardWorkdir`), via an option of
  the board's folder field created under that id (`workdirSync.ts`). Two
  name-matching fallbacks remain for boards that recorded no folder property.
- The field is a single select; a board with one folder prefills it
  (`soleWorkdirOption`); it goes read-only once `CardWork.Started`.
- **Where a card's work is has one answer**: `Manager.CardWork`, merging the
  local claim (wins) with the branch on the card.
- A repository is an ordinary folder that git answers for each time it is listed
  (`WorkdirStatus`). **A branch appears because a card claimed the folder**
  (`ClaimWorkspace(WorkSpec)`, owned by the card, not the run). Mode is per
  (folder, board): `worktree` or `branch` (`WorkdirEntry.Modes`); `branch` holds
  the folder for one card and refuses dirty tracked work — as a stall, not a
  failure.
- **The branch is the product, the copy is the workshop**: `FoldWorktree` when
  a stage ends, remade from the branch on the next one; the branch is written
  onto the card, the path never. Publishing a branch is not this app's: deploying
  to Dokku went to Gandvík (`../xxvi`) as a flow step.

### An agent talks back through MCP, not through its output

`internal/boardmcp` gives an agent the board as tools, served by the app itself
over HTTP on the front door (`/acp/board/mcp`, `boardapi.go`).

- Everything is **named, not identified**, except a card id, which defaults to
  the card the run stands on.
- `finish_work` ends a stage; `comment_card` is how the result reaches a person;
  `update_card` puts values into the card's own fields, and `list_card_fields`
  says what a value may be.
- `move_card` is the one write that lets the board notify, and it is how a stage
  ends up somewhere: the agent names the column whose name says what happened.
  The move is remembered as the agent's own (`noteAgentMove`) so it starts
  nothing. A card's description is not editable by an agent.
- **The server carries a grant token, not a board id** (`boardtools.go`), minted
  per run; every card id is checked against the grant's board (`grantedCard`).
  The handler is stateless.
- A session declares the server in `session/new`, when the adapter advertises
  `mcpCapabilities.http`.

### A source brings cards in, and the app decides what they become

`internal/sources` (`docs/sources.md`) is board-agnostic and **not part of
`internal/acp`** — it must work with no agents at all.

- A source is a **plugin process** speaking JSON-RPC over stdio
  (`sources/protocol`), which never sees the board; rules and boards are ours.
  `ingest.go` is the token-guarded route for scripts and webhooks. **An MCP
  server is a plugin with no code**: a manifest maps a tool's rows to items
  (`mcpsource.go`); shipped manifests re-invoke this binary, user manifests in
  `<dataDir>/sources/manifests/` win by name.
- **The inbox is one board**, made by the app at launch
  (`boardadapter.EnsureInboxBoard`) and found by its `inbox` mark, never by
  title. Every source writes there and names no board; a person carries each
  card on to where it belongs. It has no column property — its one view is a
  kanban grouped by author, and each source has a board account
  (`EnsureSourceUser`). Sources are configured only from its ⋯ menu.
- The OS share sheet is a source (`share.go`, `pages/share/`), the one allowed to
  pick another board (`PickBoard`); the dialog offers the inbox first. A card a
  source puts on a board with columns stands in the first one
  (`boardmodel.IntakeColumn`).
- A card is carried to another board as a **real move** keeping its id
  (`app.MoveCardToBoard`); `insertBlock` cannot do it because it keys on
  `id AND board_id`. Properties travel by name.
- **An agent may do the carrying** (`internal/inboxsort`, `docs/sources.md`
  §29), off until a person turns it on from the inbox's ⋯ menu. It decides by
  each board's description (edited on the board) and a
  summary of its cards the agent writes into `board_summary`. A card it files
  stands in the first column **silently** (`SortBoard.FileCard`) — never through
  the notifying move. Decisions are `inbox_sort`; like sources, the package
  imports neither board nor agents.

### The app replaces itself, and trusts one key while doing it

`updates.go` on the framework's `app.Updater` (`docs/release.md`).

- Provider `endpoint` with a signed manifest checked against the embedded
  `build/updater.key.pub`. **The manifest URL cannot be taken back** — every
  installed copy asks there; it is `updateManifestURL`, one `const` per line
  because CI reads it with `sed`.
- `<dataDir>/updates.json` keeps the switch, the skipped version and last check;
  the timer is ours; a poll only checks, never downloads.
- UI is ours (`updater.WindowNone`): Go re-emits one `acp:update` with the full
  state read fresh off `Updater.State()`; drawn by `settings/updatesPanel.tsx`.
- `updater.HandleHelperMode()` is **the first line of `main()`**.
- The version is a constant (`version.go`); `version_test.go` checks every other
  file that states it; `wails3 task version:set` writes them all. Not
  `common:update:build-assets` — it drops the hand-written `CFBundleURLTypes`.

### Nothing in the app is paid for

**There is one build**: the `lifetime` edition
was removed (`docs/deferred.md`, «Платное издание»). A subscription, when there
is one, is the AI model reached through our key, and nothing else is behind it
(`docs/web-ai-plan.md`).

### One database, and the schema is written once

Our tables live in the board's database (`internal/acp` and `internal/sources`
take a `*sql.DB`), so **a reference is a foreign key onto `blocks(id)`** and a
deleted card takes its rows with it. `foreign_keys` travels in the DSN, spelled
per driver.

- **The schema is Go data** (`tools/schemagen`), rendered by atlas into
  `000001_board` and `000002_app` for three dialects; golang-migrate runs them.
  `go generate ./tools/schemagen` after any change. No table prefix.
- **Transactions work on SQLite** because `insert_at` comes from Go
  (`utils.NextInsertAt`); do not go back to the database's clock.
- **A closed set is a `CHECK`, a growing one is not** (SQLite cannot `ALTER` a
  check in).
- **Absence is NULL** where a key looks (reads `COALESCE(...,'')`); journals use
  UUIDv7 ids, no autoincrement.
- Tests get the schema from `internal/appschema`, never a copied DDL.
- Every run is an `agent_session`; `conversation` went with the terminals. The
  schema says `workspace`/`checkout`; the Go side still says `WorkdirEntry`.

## Conventions

- **Comments say why, not what.** A comment earns its place by recording a
  decision, a constraint, or a trap somebody already fell into. Anything longer
  than a couple of sentences belongs in `docs/decisions.md`.
- **Tests describe behaviour, not implementation.** The name is a sentence about
  the product; the comment above says why it matters.
- **The webapp's testing library is Solid's.** `render(() => wrapIntl(() =>
  <X/>))` — a thunk, or providers do not exist yet. No `act`, no `rerender`.
  Stores from `mockAppStore(state)` under `AppStoreProvider`, routers from
  `TestRouter` (`testUtils`); `fireEvent.input`, not `change`.
- **Mocking is vitest's, and it is ESM.** A `vi.mock` factory for a module with a
  default export returns `{default: …}`. `vi.resetAllMocks` restores spies, so a
  spy that must not call through uses `mockResolvedValue` and hooks clear rather
  than reset.
- **Languages.** Russian in user-facing strings and product docs; English in
  code, comments and commit messages. **A comment carries no Russian at all** —
  name the thing in English («Входящие» is the inbox view); no transliteration.
  **Everything said to an agent is English** — prompts, MCP instructions, tool
  descriptions, schema fields; what a person wrote passes through as data, and
  answers meant for a person ask for the conversation's language.
- **User-facing text is plain documentation**, in `frontend/i18n/*.json`,
  component defaults, `site/` and `docs/guide/`: name the thing, say what it
  does, say what to do. No aphorisms, mirrored halves, metaphors where a plain
  verb exists, or headings that make statements. Product vocabulary (карточка,
  колонка, папка, «Входящие») is terminology and stays.
- **Russian is never a key.** Nothing finds, matches or branches on a Russian
  word: the column property is what a view groups by, author/link properties are
  found by type, folders by the id the board records. There are no exceptions
  left — the last three were «Исход» and its two values, and they went with the
  route engine.
- **A reference is a foreign key**, and nothing is found by name.
  `docs/schema/erd.md`, `app.hcl` and `erd.html` are generated and tested;
  `docs/db-erd.md` is the model in words, `docs/db-schema-review.md` the
  decisions, `docs/sql-dialects.md` what is still per vendor.
- **A rework is not finished until `docs/` says what is now true.** Edit the
  document that described the old shape in the same change; a plan whose work is
  done is rewritten as description or deleted; what was deliberately not done
  keeps its reasoning in `docs/deferred.md`.
- **A feature a person uses is not finished until `docs/guide/` says how**
  (Russian, by screen or task; a VitePress site, `docs/guide-site.md`). New pages
  get a sidebar line in `.vitepress/config.mjs`. Quote the screen from
  `frontend/i18n/ru.json`, never from memory. `docs/guide/en/` mirrors it page for
  page and quotes `en.json`.
- Commit messages: a plain subject saying what changed, a body saying why — the
  reasoning, the rejected alternative, what was verified. No emoji.
- **Verify before claiming.** A feature is done when it has been run, not when
  it compiles (`docs/verifying.md`).
- **Run the tests around what changed, not the whole suite**: the package or
  component plus what imports it. The whole suite is for changes across the tree
  and before a release. A clean merge that changed no source needs no run.
  Snapshots deserve a wider pass (`-u` reports which moved).
