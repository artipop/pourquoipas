# Solid 2

The webapp runs on Solid 2 (`solid-js@2.0.0-rc.1`, `@solidjs/web`,
`@solidjs/router@2`, `@solidjs/vite-plugin@3`). This is what changed, what it
cost, and what is not finished.

## The thing that looked like a blocker and was not

`@dnd-kit/solid` (every drag on the board and in the sidebar) and
`@dschz/solid-flow` (the route canvas) have no Solid 2 release and no
maintained branch heading for one. Both import `solid-js/store` and
`solid-js/web`, which Solid 2 does not publish, and both call primitives it
removed — `on`, `batch`, `splitProps`, `produce`, `Index`, `Context.Provider`,
and the one-argument `createEffect`.

They work anyway, behind `webapp/solid1/` — a re-creation of the Solid 1
surface on top of Solid 2, pointed at **those packages only**, by importer
path, from `solid1Compat()` in `vite.config.ts`. `webapp/solid1/README.md` is
the detail, including the two traps: applied globally the shim rewrites what
`@solidjs/web` itself imports and the Solid 2 renderer ends up built on Solid 1
effect semantics; and `storePath`, which looks like the ready-made answer for
Solid 1 path setters, silently wipes a branch when handed a `produce`
callback, because it takes the updater's **return value**.

A dnd-kit probe under the shim renders byte-for-byte what the same probe
renders on Solid 1. That, rather than the build passing, is why the shim is
trusted.

## What the migration actually changed in our code

Mostly mechanical:

- `solid-js/web` → `@solidjs/web`; the `JSX` namespace comes from there too.
- `solid-js/store` → `solid-js`; `produce` is gone, so every call site is a
  draft callback (`setState(({sidebar}) => …)`), and every path setter is
  wrapped in `storePath`.
- `batch` is gone: Solid 2 batches until the microtask drains, so the wrapper
  was dropped and the body lifted.
- `onMount` → `onSettled`, which refuses `onCleanup` inside it — a cleanup is
  **returned**. An `async` callback therefore cannot be an `onSettled`
  callback at all: its promise is read as the cleanup value.
- `Suspense` → `Loading`, `Index` → `<For keyed={false}>`,
  `<X.Provider value=…>` → `<X value=…>`.
- `classList` is gone: `class` takes the same object form
  (`class={[base, {modifier: on()}]}`).
- `createResource` is gone. Both call sites became a signal fed by an effect
  rather than an async memo, because neither screen has a loading state: an
  async memo throws `NotReadyError` on the first read — through `latest()` as
  well — until a `Loading` boundary catches it, and neither screen had one.

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

- **34 tests in 16 files still fail** (of 1285). Mostly tests that write
  directly and need their own `flush()`, plus the route canvas, whose library
  is the one running through the compat shim.
- **`tsc --noEmit` is not clean**, and was not clean before this either. The
  new errors are concentrated in files where TypeScript gives up and reports
  captured bindings as `never`; the build (esbuild) and the linter are the
  gates this repository actually runs.
- **Nothing has been driven in a browser.** The build is green and the suite
  is nearly green; a real pass over drag-and-drop, the route canvas and the
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
