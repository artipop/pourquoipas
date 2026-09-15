# Solid 1 compatibility, for one library and no more

`@dnd-kit/solid` is the only dependency left that speaks Solid 1, and it has no
Solid 2 release — not even on its beta tag. It imports `solid-js/web`, which
Solid 2 does not publish, and calls primitives Solid 2 removed — `on`, `batch`,
`splitProps`, `Context.Provider`, and the one-argument `createEffect`.

So the files here re-create that surface on top of Solid 2, and
`solid1Compat()` in `vite.config.ts` points **only that package** at them, by
importer path. Applied globally the shim would also rewrite what `@solidjs/web`
itself imports, and the Solid 2 renderer would then be built on Solid 1 effect
semantics — which is how the first attempt failed.

This is a bridge with an end date. `@dschz/solid-flow` was on it too and came
off with its 1.0 line, which is built for Solid 2; when dnd-kit publishes for
Solid 2 the directory goes.

`store.js` and most of `core.js` were written for solid-flow — the path setter,
`produce`, `Index`, `getListener` — and are kept because dnd-kit's own imports
are the ones checked, not its future ones; they cost nothing while nobody
imports them.

## What is deliberately not faithful

- `batch` runs its callback and nothing else. Solid 2 batches by default.
- `observable` and `from` throw. Nothing in either library calls them; a shim
  that silently returned the wrong thing would be worse than a stack trace.
- `startTransition`/`useTransition` do not transition. Neither library uses
  them for anything a person sees.
- The path setter covers keys, key arrays and predicate functions, which is
  what `@dschz/solid-flow` actually calls. Ranges (`{from, to, by}`) throw.

## Why the path setter is written out rather than handed to `storePath`

Solid 2 ships `storePath`, which walks the same Solid 1 path syntax and covers
ranges as well. It cannot stand in here, because it takes the **return value**
of an updater — and `produce`'s callback returns nothing, so
`set(storePath('list', produce(fn)))` sets `list` to `undefined` and says
nothing about it. `@dschz/solid-flow` writes every one of its store updates
that way. `store.js` walks the draft itself so a `produce` callback mutates in
place, which is what Solid 1 did.

The same trap applies to our own code: a `produce` call site migrates to a
draft callback, never to `storePath`.
