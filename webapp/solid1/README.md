# Solid 1 compatibility, for two libraries and no more

`@dnd-kit/solid` and `@dschz/solid-flow` are the only dependencies left that
speak Solid 1, and neither has a Solid 2 release. They import `solid-js/store`
and `solid-js/web`, which Solid 2 does not publish, and they call primitives
Solid 2 removed — `on`, `batch`, `splitProps`, `produce`, `Index`,
`Context.Provider`, and the one-argument `createEffect`.

So the three files here re-create that surface on top of Solid 2, and
`solid1Compat()` in `vite.config.ts` points **only those packages** at them, by
importer path. Applied globally the shim would also rewrite what `@solidjs/web`
itself imports, and the Solid 2 renderer would then be built on Solid 1 effect
semantics — which is how the first attempt failed.

This is a bridge with an end date: when either library publishes for Solid 2 it
comes off the list, and when both do the directory goes.

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
