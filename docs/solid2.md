# Solid 2

The webapp runs on Solid 2 (`solid-js@2.0.0-rc.1`, `@solidjs/web`,
`@solidjs/router@2`, `@solidjs/vite-plugin@3`). This is what changed, what it
cost, and what is not finished.

## The thing that looked like a blocker and was not

`@dnd-kit/solid` (every drag on the board and in the sidebar) has no Solid 2
release — not on its beta tag either, as of September 2026. It imports
`solid-js/web`, which Solid 2 does not publish, and calls primitives it
removed — `on`, `batch`, `splitProps`, `Context.Provider`, and the one-argument
`createEffect`. `@dschz/solid-flow` (the route canvas) was in the same place
when the move started, and is not any more: its `1.0.0-next` line is built for
Solid 2 and is what the canvas runs on. What that cost is one API change —
the imperative getters are gone, so a node's measured size is read off
`useSolidFlow().internalNodes[id]` rather than `getInternalNode(id)`.

dnd-kit works anyway, behind `webapp/solid1/` — a re-creation of the Solid 1
surface on top of Solid 2, pointed at **that package only**, by importer
path, from `solid1Compat()` in `vite.config.ts`. `webapp/solid1/README.md` is
the detail, including the two traps: applied globally the shim rewrites what
`@solidjs/web` itself imports and the Solid 2 renderer ends up built on Solid 1
effect semantics; and `storePath`, which looks like the ready-made answer for
Solid 1 path setters, silently wipes a branch when handed a `produce`
callback, because it takes the updater's **return value**.

A dnd-kit probe under the shim renders byte-for-byte what the same probe
renders on Solid 1. That, rather than the build passing, is why the shim is
trusted.

**The peer ranges do not agree, so `webapp/.npmrc` sets
`legacy-peer-deps=true`.** `@dnd-kit/solid` still declares solid-js 1, and
`@solidjs/vite-plugin` next declares an older jest-dom; a plain `npm ci`
refuses the lockfile with ERESOLVE. The move was built and tested in a
checkout whose `node_modules` were already installed, so the first clean
`npm ci` — on a fresh checkout, and in CI — was what found it. An `overrides`
entry for dnd-kit was tried first and does not help: npm still checks the
next package's peers.

## What the migration actually changed in our code

Mostly mechanical:

- `solid-js/web` → `@solidjs/web`; the `JSX` namespace comes from there too.
- `solid-js/store` → `solid-js`; `produce` is gone, so every call site is a
  draft callback (`setState(({sidebar}) => …)`), and every plain path setter
  is wrapped in `storePath`. A `reconcile` is **not** a plain path setter — see
  below.
- `batch` is gone: Solid 2 batches until the microtask drains, so the wrapper
  was dropped and the body lifted.
- `onMount` → `onSettled`, which refuses `onCleanup` inside it — a cleanup is
  **returned**. An `async` callback therefore cannot be an `onSettled`
  callback at all: its promise is read as the cleanup value.
- `Suspense` → `Loading`, `Index` → `<For keyed={false}>`,
  `<X.Provider value=…>` → `<X value=…>`.
- `classList` is gone: `class` takes the same object form
  (`class={[base, {modifier: on()}]}`).
- `createResource` is gone, and so is the loader it stood in for — a signal,
  an effect that fetched into it, and a `live` flag so a late answer for the
  board somebody left could not land on the one they moved to. A read is an
  async memo (`createMemo(async () => …)`): the latest question wins by
  itself, `refresh(memo)` re-asks after an event or a write and keeps the old
  answer on screen meanwhile, and a rejection is thrown at the reader, so the
  memo catches its own. Its first read throws `NotReadyError` until a
  `Loading` boundary catches it. Where "nothing yet" and "nothing" look the
  same on screen — a plan with no steps, a count of zero — the memo passes
  `loadingValue` instead and never suspends; where an empty answer says
  something («нет карточек», "no boards yet") it sits under `Loading`, which
  is what keeps that sentence off the screen while the question is still out.
  `eslint-plugin-solid` still knows only Solid 1 and warns "This tracked
  scope should not be async" at every one of them; the warning is wrong here.
  A loader whose answer a form then edits, or whose load error shares a line
  with the errors of the actions beside it (`serverPanel`, `folderChoices`,
  `templateEditor`, the dialogs that seed a draft), stays a signal: what the
  actions write is not a re-read, and splitting one error line in two would
  be the workaround.
- **The open board is loaded the same way** (`boardPage.tsx`): an async memo
  over the board, the team and who is signed in fetches the blocks
  (`fetchBoardBlocks`), and an effect over it applies them
  (`applyBoardBlocks`), joins the board when it came back empty, and asks
  for its members. The apply is an effect rather than a line after the
  `await` because an effect over an async memo is handed the latest answer
  only — the load used to fetch-then-write, so a board left while it loaded
  could land its cards over the one opened. A socket that reconnects
  re-asks it with `refresh`. The store's `loadingBoard` flag is gone:
  `workspace.tsx` puts the centre under `Loading` for the first load and
  asks `isPending` for a later one, and only when there is no board to draw
  — so "Create a board" is never shown for a board still on its way.
- The module caches that count their consumers (`attention.ts`,
  `liveTerminals.ts`, `boardFlows.ts`, `settings/updates.ts`) keep counting
  rather than moving to a signal's `unobserved` option. There is no matching
  "observed" hook, so the subscription would have to start on the first read —
  an untracked read in a click handler included, which nothing would ever
  release — and `unobserved` fires from the runtime's own sweep, whose timing
  against a `For` swapping rows was never measured. The counter is explicit,
  and its one trap (no write during disposal, hence the microtask) is written
  down where it is.

Not mechanical, and the bulk of the work: **77 `createEffect` call sites**.
Solid 1 tracked the whole body; Solid 2 splits it into a tracked compute and an
untracked apply, and the apply's return value is a cleanup. Each site had to
say what it actually depends on.

## The three bugs a blind migration would have shipped

1. **Every `aria-expanded`, `aria-pressed` and `aria-selected` disappeared.**
   Solid 2 types those as enumerated strings; a boolean renders as `""`, which
   ARIA does not read as true — and in practice the attribute was dropped
   entirely. Comboboxes, toggles, menus and the tour were all affected. They
   are `String(...)` now. The snapshots caught it; nothing else would have.
2. **The calendar rendered nothing.** FullCalendar builds each cell by
   rendering a Solid tree of its own, and Solid 2 refuses to create a reactive
   root inside a settle (`PRIMITIVE_IN_FORBIDDEN_SCOPE`). The construction
   moved one microtask past the settle.
3. **A `Setter` passed straight in as an apply function** returns the value it
   set, which Solid 2 reads as a cleanup and rejects. `createEffect(src, setX)`
   has to be `createEffect(src, (v) => { setX(v) })`.

Solid 2 also fixed one: `galleryCard`'s "with an image content" test had been
recording the *fallback* branch.

## The bug that did ship: every board was empty

The initial load writes each branch with `reconcile`, and the migration wrapped
those writes like every other path setter:
`setState(storePath('cards', 'cards', reconcile(cards)))`. `storePath` keeps an
updater's **return value**, and `reconcile` mutates in place and returns
nothing — so boards, views, cards, contents, comments, attachments and the
user's config were fetched, written, and written away again, and the board
drew empty with no error anywhere. The build was green and the suite nearly so.

What should have caught it did: `a preference the server dropped is dropped
from myConfig` failed with `Cannot reconcile null or undefined state`, and was
put down as one of the tests that only wanted a `flush()`. The rest of the store
tests did only want one, which is how the one that did not hid among them.

A `reconcile` is applied to the draft branch now —
`setState((d) => { reconcile(x)(d.cards.cards) })` — and
`a board opened is in the store with its views and cards` fails on the old
loader and passes on this one. Applied that way `reconcile` still does what the
loader's comment counts on: a `For` over the cards creates no row when the same
cards are loaded again, or when one of them is renamed. (Comparing the store
proxies with `===` says otherwise, and is not what `For` looks at.)

## The bug that did ship: no column lit under a dragged card

The wash on a column while a card is held over it went missing, and no test
looked at it. dnd-kit's Solid bindings subscribe to the fields a component has
already read (`useDeepSignal`), from inside a `createEffect` — and the compat
layer ran a Solid 1 effect body as the compute half of a Solid 2 effect, which
runs at creation, before the component has read anything. So `isDropTarget`
never changed. The layer runs it through `createTrackedEffect` now, which is
tracked and runs in the effect phase, the way a Solid 1 effect did, and
`sortableDrag.test.tsx` holds a card over a zone and reads the class.

## The suite

`src/test/flushAfterEvents.ts` drains the queue after every `fireEvent` and
after every fake-timer advance, because Solid 2 stages a write until the next
microtask and the suite is written the Solid 1 way — fire a click, read the DOM
on the next line. A test that writes **directly** — calling a module's own
function — still says `flush()` itself; there is no event to hang it on.

Snapshots were re-recorded once. The diff is `<!---->` placeholders, a dropped
`xmlns` on inline SVG, `spellcheck=""` (which HTML reads as true), and one
fewer wrapper `<div>` around a `Portal`. Nothing in the tree styles that
wrapper.

## What is not finished

- **The suite is green** (1546 of 1552, six skipped on purpose). The last
  fifteen failures were each read before being fixed, and four of them were
  the app, not the test:
  - *The card's terminal panel started in the wrong place.* Picking a folder
    or an agent set a signal and then called `start`, which read the signal
    back — the old value — so the conversation opened in the board's drafts
    with nobody named. `start` now takes what was picked as arguments.
  - *Renaming a property with Enter saved it twice.* The menu closed and
    focus went back to its trigger in the same turn, while the menu — held
    by the staged write — was still on screen; the name box blurred, and it
    confirms on blur. `menuWrapper` flushes the close before moving focus,
    and the keyboard open the same way, so the first option takes focus in
    the turn of the key.
  - *A tap on the phone's tab bar could be lost.* The bar was a keyed `For`
    over entries rebuilt on every count change, so a count arriving mid-tap
    replaced the button being tapped. It is `keyed={false}` now.
  - *The board prompt dialog took typing before it had loaded*, and the
    stored text then arrived over what was typed. It waits under `Loading`.
  The rest were tests that wrote directly and needed `flush()`, awaited
  what they fired, or read a label that was still "Asking the agent…".
  `Portal` no longer wraps what it ports, so the node's parent is `body`.
- **A read right after a write in the same handler returns the old value.**
  The theme picker and the terminal panel both shipped that bug; a scan for
  the shape (a setter followed by its own getter in one handler) finds
  nothing else now, but the scan is a heuristic and the bug class is silent.
- **`tsc --noEmit` is not clean**, and was not clean before this either. The
  new errors are concentrated in files where TypeScript gives up and reports
  captured bindings as `never`; the build (esbuild) and the linter are the
  gates this repository actually runs.
- **Nothing has been driven in a browser.** The build is green and the suite
  is green; a real pass over drag-and-drop, the route canvas and the
  calendar is still owed, and jsdom does no layout — see the note about
  verifying UI in a real browser.
- Solid 2 is at **rc.1** and `@solidjs/router` at `2.0.0-next.17`. Both are
  pinned; neither is a version to float.

## The router

Solid Router 2 is a redesign, not a rename: no `<Router>`, `<Route>`, `<A>`,
`<Navigate>` or `MemoryRouter`. A route tree is data handed to `createRouter`,
which returns the provider component; plain `<a>` is intercepted by default;
`<Navigate>` is a `useNavigate` call in an effect (`Redirect` in `route.tsx`);
`useCurrentMatches` is `useRouteMatches`. `FBRoute` is `fbRoute` and returns a
`RouteDefinition` instead of JSX.
