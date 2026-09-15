# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repository.

Lines are wrapped. Keep them wrapped — a paragraph on one line makes every edit a
whole-line diff.

This file is the map: the rules that break things silently when forgotten, and
where the reasoning lives. The reasoning itself is in `docs/architecture.md`
(how each subsystem is shaped and why), `docs/decisions.md` (rejected
alternatives and post-mortems), `docs/deferred.md` (what was thought through and
not built) and `docs/todo.md` (what is known and open). Read the relevant
section of `docs/architecture.md` before changing anything structural.

## What this is

**XCIII** is a desktop board that runs coding agents from the board. One Go
module built with **Wails v3**, the board server running **in-process**; the same
code builds a headless server (`-tags server`) that serves the board to a browser.

- `server/` — the board server, a fork of Mattermost's Focalboard. It is **ours**,
  not a fork kept recognisable: Focalboard is archived and nothing imports
  Mattermost any more. A directory of this module (`github.com/artipop/xciii/
  server/…`), not a module of its own. Cutting it is ordinary;
  `docs/fork-shrink-removed.md` is the ledger of what went and how to bring it
  back. A removal is one commit with its tests, mocks and doc edits. Login,
  public sharing and multi-team are kept on purpose.
- `webapp/` — its own npm project, built with Vite, rewritten from React to
  **SolidJS**. Upstream and this repository's early history are React and are no
  recipe any more; `docs/webapp.md` describes the page as it is.
- `mobile/` — the phone app, a **second Go module on purpose** (see below).
- README.md, "What this app requires of the frontend": the output layout
  `go:embed` expects and the three feature-detected globals the page consumes.

## Build & run

`docs/build-and-platforms.md` has the details and the traps already paid for.

- `wails3 dev` — the dev loop; needs `npm ci` in `webapp/` once. A dev build (no
  `frontend` tag) serves `webapp/pack` off disk, kept current by the watcher in
  `build/config.yml`.
- `wails3 task build` / `package` (`darwin:package:dmg`, `windows:package`,
  `linux:package`) / `build:server`. `EDITION=lifetime` on any of them.
- Build tags travel as `EXTRA_TAGS`, default `json1,sqlite3,frontend`; Linux adds
  `gtk3`.
- **Which SQLite is compiled in is cgo's answer.** `sqlite3 && cgo` is
  `mattn/go-sqlite3`; anything else is `modernc.org/sqlite`, registered under the
  same name (`sqlstore/sqlite_pure.go`), with `SQLiteDSN` spelling the settings
  for whichever it is. **Windows builds without cgo by default** and
  cross-compiles from anywhere; macOS and Linux desktops need cgo for Wails itself.
- **A packaged app is not a child of a shell.** launchd hands the `.app` a bare
  PATH and no `TERM`; `internal/userpath` restores the login shell's PATH for the
  process, and `terminalEnv` writes `TERM`/`COLORTERM` for the pty. A bug that
  exists only in the installed app is usually this.
- **The desktop app is a WKWebView, and CSS prefixes are the build's job**
  (autoprefixer in `vite.config.ts`, targets in `browserslist`). WebKit there
  does not know unprefixed `user-select`. Never write prefixes by hand — stylelint
  forbids it.
- **`webapp/pack` must never stop existing**: `go mod tidy` resolves the
  `go:embed all:` pattern in parallel with the frontend build, so a committed
  `.gitkeep` holds it and Vite clears around it (`emptyPack`).

Tests:

- `npm test` in `webapp/` — vitest under jsdom, no coverage (`npm run
  test:coverage` for it), `npm run updatesnapshot` to rewrite snapshots.
- `go test -tags "json1 sqlite3" ./...` — on mattn, as macOS and Linux ship.
  Without tags or cgo it runs on modernc, as Windows ships. `go vet -tags "server
  json1 sqlite3" .` checks the headless build, which has files of its own.
  `./...` also walks `webapp/node_modules`; that noise is cosmetic.
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

`frontdoor.go` is our HTTP server in front of everything: `/wails/` to the Wails
runtime, the rest — HTML, `/api`, `/files`, `/ws` — to the board. A desktop build
hands it to Wails as an `AssetServerTransport` on loopback; a server build puts
it in front of Wails' own HTTP server. It exists because **Wails' asset server
answers a WebSocket upgrade with 501**.

- On a desktop **nothing authenticates a user**: the protection is refusing a
  foreign `Origin`/`Sec-Fetch-Site` and a `Host` the door was not published
  under. Keep a server build on localhost unless something in front of it
  authenticates.
- **Who is asked is the build** (`identity.go` + `identity_desktop.go`/
  `identity_server.go`, `docs/teamwork.md`): a desktop is one person with a
  single-user token; a hosted server is a team with `/login` open, and there
  `/wails/` and both sockets require a session (`requireSession`) — except the
  runtime module itself (`openToEverybody`). A desktop joins a server by logging
  in (`attach.go`) and keeps its agents.
- `tsnetdoor.go` is the only supported way off the machine: the same front door
  published as a node of the user's tailnet (`tsnet`, `ListenTLS`), gated by
  `WhoIs` in front of *everything*, page included, because the page hands out the
  board's session token. It gets a front door built for its own authority.

### The page talks to Go through a shim, not through generated bindings

The bootstrap script in `proxy.go` loads `/wails/runtime.js` and rebuilds
`window.go.main.App` as a Proxy over `Call.ByName('main.App.<name>')`. No bindings
are generated: **adding a method to `App` makes it callable.**

**Events come back over a socket, not the Wails bus** (`eventsws.go`,
`/acp/events/ws`), because the bus only reaches windows the app owns and a page
served through the tailnet hears nothing. `emitter.go` sends every event both
ways; the page listens only on the socket (`components/acp/agentEvents.ts`). A
new UI event needs nothing but `Emit`.

**`/m` is the board on a phone** (`pages/mobile/`): four screens, and it asks
nothing of the board's own API — everything comes from `main.App.*` and the event
socket (`bindings/boards.ts`). `router.test.tsx` guards that the board's
catch-all route does not swallow `/m`. **`mobile/` is a second Go module** because
`wails3 ios` compiles `package main` from the module root, and this root's main
is a board server with cgo SQLite; the phone app is a window per machine onto
`https://<machine>.<tailnet>/m`. `mobile/README.md` has the commands.

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
- **`@dnd-kit/solid` is Solid 1 only** and runs on `webapp/solid1/`, a compat
  layer pointed at that package alone by importer path. Applied globally it
  rewrites what `@solidjs/web` itself imports. It goes when dnd-kit ships Solid 2.
- State is `src/store`: `createAppStore(deps, initialState)`, per-domain action
  methods, no dispatch, no thunk; clients arrive through `deps` so tests pass
  mocks. A path write goes through `storePath`; a mutation in place — a
  `produce`-style body or a `reconcile` — is a draft callback
  (`setState((d) => { reconcile(x)(d.boards.boards) })`), **never `storePath`**,
  which takes an updater's return value and so writes the branch away.
  `storePath(…, reconcile(x))` emptied every board once, silently.
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

### A stage is the agent's own CLI, and it asks in its own words

`internal/acp` is the agent integration and is board-agnostic;
`internal/boardadapter` is the only package importing both it and the board
server.

- **An agent stage of a route is a terminal, not a session**
  (`stageterminal.go`): the vendor CLI in a pty with the card's task on its
  command line, drawing its own plan, questions and permission prompts. **Nothing
  of ours is drawn over the CLI's screen.** Which agents qualify is
  `stageRunsInTerminal` (board tools available, an interactive CLI exists and is
  installed); the rest, and every deploy and test, stay ACP sessions
  (`question.go`).
- **The stage ends when the agent calls `finish_work`**, and the task text says
  so too (`finishWorkBrief`) because a CLI may defer loading the tools. A CLI
  exiting without it is a stall with a reason, not a failed stage.
- **Attention** (`acp:attention`): silence on a stage's terminal
  (`AttentionTerminal`, `terminalQuietFor`) is the floor; a permission hook
  (`internal/acp/toolhook.go`, `hook.go`, `docs/attention-hooks.md`) is the
  precise signal, where the CLI supports one. The hook never takes the question
  from the terminal, and **every hook failure is silence** (exit 0). Output after
  the box settles withdraws the wait (`withdrawWhenAnsweredOnScreen`, measured on
  `workedAt`, not `lastOutput`, because a resize repaints).
- `AttentionCard` (`cardwait.go`) is the route parking a card where nothing runs;
  the ack *ends* it. A wait is addressed to the card's assignee in team mode
  (`attentionAudience.ts`).
- **Ack is kept by Go** (`attentionack.go`), so one click clears every window and
  the phone; only real work, not the repaint our own looking provoked
  (`resizeEcho`), drops it.
- The notification stack is drawn only in the board window, never on a terminal
  page. OS notifications and the tray (`alerts.go`, `alerts_desktop.go`) re-read
  `Manager.Attention()` on each event. Traps: the notification service is started
  by hand, never registered in `Options.Services` (it errors outside a bundle,
  i.e. in every `wails3 dev`); `RemoveDeliveredNotification`, not the stub
  `RemoveNotification`; the tray icons are two inks, not a template icon.
- **The app outlives its windows**: closing hides; «Выход» quits. Quitting is
  per-platform — `appShouldQuit` says yes on darwin, or ⌘Q is refused.
- The card's corner button (`KanbanCard__terminal`) opens the card's terminal;
  its colour and glyph say what is happening (`StallKindConversation` for a cut
  stage). The terminal page draws nothing over the screen.
- **A stage writes one comment, at the end** (`stageComment`). "Nothing happened"
  is state, never a comment: one stall record per card (`card_stall`,
  `stall.go`), route dead-ends written softly so they never bury the root cause.
  Comments are drawn only in team mode.
- The card's changes are git and nothing else (`cardDiff.tsx`, `carddiff.go`),
  never cached, the committed half read by branch name because the copy is
  folded away after a stage; the button is drawn on a stored count
  (`card_changes`).
- **The model is a graph**: nodes (a column is a node's face; the node id is the
  board option id and never regenerated), edges are routes, crew and `Prompt`
  inherit node → column. A board can be viewed one route at a time
  (`boardFlows.ts`; which route a card is on is Go's answer, `CardFlowsForBoard`,
  after `SeedBoard`); the intake column is kept and divided off
  (`intakeColumn`), found as the view's first group, never by name.
- Editing: `automationEditor.tsx` over the board's own columns, inside
  `automationDialog.tsx`; types and pure helpers in `automation.ts`. The panel
  edits the column on «Колонки» and the stage on a route tab, each stage field
  naming the column's answer it falls back to. A template is shown, not edited
  (`templateEditor.tsx`); made by saving a board (`saveAsTemplate.ts`).
- Templates (`docs/templates.md`): the marker (`TemplateMarkerProperty`) is the
  app's to give, and copies are disowned at launch; which templates are ours is
  `createdBy`. Files are English and translated on import through
  `templates/i18n/*.json` keyed by the English string
  (`TestEveryTemplateStringIsTranslated`). Language is `ui-settings.json`;
  switching reinstalls (`App.retemplate`); the page reports the system locale via
  `UseLanguage` and awaits it before the first render. **Four names stay Russian
  in the files** (`appGivenNames`: the inbox view and column names, «Исход» and
  its values), because something finds them by title.
- **What an agent is told first is two texts** (`promptLead`,
  `boardprompts.go`): the board's `prompt`, then the agent's own. A third layer
  per (board, agent) was built and removed; do not bring it back.
- **Automation lives on the board** (`columns`/`flows` in board properties,
  written through by `persistBoardLocked`); the registry in memory is a working
  copy, and `config.json` does not carry it.
- **A setting lives where its owner is**: machine settings in
  `settings/appSettingsDialog.tsx`; deploy targets, agents and proxies are
  machine registries offered from a board's ⋯ menu on its setup plan; UI prefs
  are kept by the install in `<dataDir>/ui-settings.json` (`installKept` in
  `userSettings.ts`), hydrated into localStorage before first paint.
- **A theme is a Mattermost theme config** (`webapp/src/theme/`,
  `docs/themes.md`), shipped as pairs (light/dark) with `themeMode` choosing the
  half; `_tokens.scss` is the first paint and a test holds it equal to the
  shipped configs.
- **Board setup questions are each a ⋯ menu item**, and which ones a board has is
  read off its stages (`setupSteps`, `BoardSetupPlan`), not its template. The
  first is the board's name, unique case-insensitively (`boardTitle.ts`).
  **Every question may be left unanswered** ("relaxed mode", a word and not a
  flag): the default must be sayable in one sentence where the question is asked
  and must keep a person able to work.
- **A card names its agent by its assignee** («Кто занимается») and nothing else;
  the stage writes the resolved worker there (`assignCardAgent`), and
  `humanAssignee` vetoes a session on a person's card. Agent accounts are made
  when the agent is registered (`EnsureAgentAccounts`); `AgentUsername` keeps
  letters of any script.

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
- **The branch is the product, the copy is the workshop**: `FoldWorktree` after a
  stage, remade from the branch on the next terminal; the branch is written onto
  the card, the path never. Where a stage runs is `FlowNode.RunIn`.

### An agent talks back through MCP, not through its output

`internal/boardmcp` gives an agent the board as tools, served by the app itself
over HTTP on the front door (`/acp/board/mcp`, `boardapi.go`).

- Everything is **named, not identified**, except a card id, which defaults to
  the card the run stands on.
- `finish_work` ends a stage; `describe_conversation`/`name_conversation` exist
  because nothing else can know what a pty conversation is about.
- `move_card` is the one write that lets the board notify — a card moved at an
  agent's request must fire the column's automation. A card's description is not
  editable by an agent.
- **The server carries a grant token, not a board id** (`boardtools.go`), minted
  per run; every card id is checked against the grant's board (`grantedCard`).
  The handler is stateless.
- A session declares the server in `session/new`; a terminal gets a config file
  (`cliMCPArgs`). A deploy or test session deliberately does not get the tools
  (an agent moving its own card is a loop).

### A source brings cards in, and the app decides what they become

`internal/sources` (`docs/sources.md`) is board-agnostic and **not part of
`internal/acp`** — it must work with no agents at all.

- A source is a **plugin process** speaking JSON-RPC over stdio
  (`sources/protocol`), which never sees the board; rules and columns are ours.
  `ingest.go` is the token-guarded route for scripts and webhooks. **An MCP
  server is a plugin with no code**: a manifest maps a tool's rows to items
  (`mcpsource.go`); shipped manifests re-invoke this binary, user manifests in
  `<dataDir>/sources/manifests/` win by name.
- The OS share sheet is a source (`share.go`, `pages/share/`), the one allowed to
  pick its board.
- Arrivals land in the inbox column, the one column the app adds to a board
  itself (`EnsureColumn`, add-only; the column property is asked of the board,
  never assumed by name). People meet it as **a view** (`EnsureInbox`), grouped
  by author — each source has a board account (`EnsureSourceUser`). Sources are
  configured only from that view's ⋯ menu.
- A card is carried to another board as a **real move** keeping its id
  (`app.MoveCardToBoard`); `insertBlock` cannot do it because it keys on
  `id AND board_id`. Properties travel by name.

### A terminal is where the work happens, watched or not

`internal/acp/terminal.go` + `terminalws.go`: the agent's own CLI in a pty in the
card's workspace, xterm.js over `/acp/terminal/{id}/ws`.

- Drawn **beside the card** (`cardTerminal.tsx` panel, lazily loaded), opened
  from the dialog toolbar; the window is the panel's open-in-new button.
- **A conversation is keyed (card, node)**, node = column option id (`@none` for
  no column). A stage **adopts** the conversation already open on its node
  (`startStageTerminal`).
- **Two declared kinds**: work (`StartCardTerminal`, claims the workspace,
  refuses with `errNoWorkdir` when no folder) and talk (`StartCardTalk`,
  `@talk`, may stand in «черновики доски», `<dataDir>/boards/<boardID>`). Never
  infer the kind.
- Where a conversation runs is `cardPlace`. A card returning to a node gets a
  brief, not the task again (`returnBrief`); a stage is told what the branch
  already holds (`branchstate.go`) and what the card says (`cardIntro`).
- **Properties are the dataflow**: a stage declares `Writes`/`Reads`; writes come
  through `finish_work` and are written before the report is delivered; reads
  fall back to what upstream stages write (`UpstreamWrites`). The brief and
  `list_card_fields` state the contract to the agent.
- **A transition can ask** (`cardask.go`, `FlowEdge.Ask`), including the entry
  edge (`EntryNode`); `AskWorkMode` lands on `FlowState.WorkMode`.
- **How a stage ended is the «Исход» field, two values**, written by the engine
  for every stage before edges are read, silently and by id; a person writes it
  from the route strip (`MarkCardOutcome` via `UpdateCard`, the one notifying
  write). No outcome columns.
- Tools per column/node (`MCPServers`), added to the agent's own; dokku's and
  `boardmcp.ServerName` are refused names.
- A refused resume restarts fresh once (`restartFresh`); a terminal drops the
  kind's `dropEnv` like a session (`CLAUDE_CODE_CHILD_SESSION` disables
  transcripts). A terminal a person opened raises no attention; a stage's does.

### The app replaces itself, and trusts one key while doing it

`updates.go` on the framework's `app.Updater` (`docs/release.md`).

- Provider `endpoint` with a signed manifest checked against the embedded
  `build/updater.key.pub`. **The manifest URL cannot be taken back** — every
  installed copy asks there; it is `edition.ManifestURL`, per edition.
- `<dataDir>/updates.json` keeps the switch, the skipped version and last check;
  the timer is ours; a poll only checks, never downloads.
- UI is ours (`updater.WindowNone`): Go re-emits one `acp:update` with the full
  state read fresh off `Updater.State()`; drawn by `settings/updatesPanel.tsx`.
- `updater.HandleHelperMode()` is **the first line of `main()`**.
- The version is a constant (`version.go`); `version_test.go` checks every other
  file that states it; `wails3 task version:set` writes them all. Not
  `common:update:build-assets` — it drops the hand-written `CFBundleURLTypes`.

### An edition is a build, not a flag

`internal/edition`, `docs/editions.md`: `base` and `lifetime` differ by a build
tag, **never by anything read at runtime**. Extra templates are a tagged
`go:embed`; the page is not told which edition it is. A template the build does
not ship is deleted at startup. Constants in `internal/edition` are one `const`
per line because CI reads them with `sed`.

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
- `conversation` and `agent_session` are two tables on purpose. The schema says
  `workspace`/`checkout`; the Go side still says `WorkdirEntry`.

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
- **User-facing text is plain documentation**, in `webapp/i18n/*.json`,
  component defaults, `site/` and `docs/guide/`: name the thing, say what it
  does, say what to do. No aphorisms, mirrored halves, metaphors where a plain
  verb exists, or headings that make statements. Product vocabulary (маршрут,
  карточка едет, «Входящие») is terminology and stays.
- **Russian is never a key.** Nothing finds, matches or branches on a Russian
  word: the column property is what a view groups by, author/link properties are
  found by type, folders by the id the board records. The one exception is
  `NormalizeVerdict`.
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
  `webapp/i18n/ru.json`, never from memory. `docs/guide/en/` mirrors it page for
  page and quotes `en.json`; names the app gives stay Russian there too.
- Commit messages: a plain subject saying what changed, a body saying why — the
  reasoning, the rejected alternative, what was verified. No emoji.
- **Verify before claiming.** A feature is done when it has been run, not when
  it compiles (`docs/verifying.md`).
- **Run the tests around what changed, not the whole suite**: the package or
  component plus what imports it. The whole suite is for changes across the tree
  and before a release. A clean merge that changed no source needs no run.
  Snapshots deserve a wider pass (`-u` reports which moved).
