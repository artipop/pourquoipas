# The board: title, description, menu

This page is about the board itself: the header, the menu, the views. Columns
and routes are covered in ["Columns and routes"](./automation/index.md).

## Title and icon

The title is edited in place: click it and type. A board with no title shows
"Untitled board".

The icon is the emoji left of the title. If there is none, an **"Add icon"**
button above the title sets a random one. Clicking the icon itself opens a
menu — **"Pick icon"**, **"Random"**, **"Remove icon"**.

## Description

A board has a description — text under the title. It is hidden by default, and
a new board opens straight onto its columns.

To show it, hover over the area above the title and press **"Show
description"** (the button with an eye). A field opens under the title. While
the description is visible the same button reads **"Hide description"**. Hiding
does not delete the text — it stays on the board.

## The board menu: «⋯» in the list on the left

Hover over a board in the list on the left — «⋯» appears on the right. In the
menu:

- **"Export board archive"** — export this board as one file. Exporting all
  boards at once, and import, are in [Moving](./transfer.md);
- **"New template from board"** — turn the board into a template, so the next
  one starts with the same columns and routes; see
  ["Columns and routes"](./automation/index.md);
- **"Duplicate board"**, **"Hide board"**, **"Delete board"** and
  **"Move To..."** — move the board to another category of the list.

The board's behaviour — columns, routes, project folders, agent instructions —
is configured elsewhere: in the **"Columns and routes"** window, opened from
the «⋯» menu in the board header, right of the view tabs. See
["Columns and routes"](./automation/index.md).

## Views

Under the board on the left are its views: kanban, table, calendar, gallery.

Everything that arrives from outside goes not to a working board but to a
board of its own, **Входящие** — see [the Inbox section](./inbox/index.md). (The
app names that board in Russian, and the name is not translated.)

The view header has the view's name, **"Route"**, **"Filter"**, **"Sort"** and
search. These are settings of the view, not of the board: each view has its own.
When they do not fit the window the bar scrolls sideways. The ⋯ menu belongs to
the view too; on the Входящие board it holds **"Sources…"** and nothing else.

Clicking the view's name opens a menu: the board's views, **"Rename view"**,
**"Duplicate view"**, **"Delete view"** and **"Add view"**. Renaming opens a
field in the header itself — Enter saves it, Esc cancels.

## Showing one route

The kanban's view header has **"Route"**. It starts on "All
routes": the board shows every column and every card. Pick a route and the
board keeps only that route's stages and only the cards travelling it.

The button is there on any board with a route at all, even a single one:
narrowing a board of nine columns to the three its route goes through is worth
it too.

The board's first column is separated from the rest by a vertical line —
always, not only when a route is picked. What arrives is to the left of it,
what the board does with it is to the right. A card arrives in that first
column before anything has decided which route it takes, so it stays there
whatever the route is: that column belongs to every route at once. A card already travelling
another route disappears from it; a card that has chosen no route is shown
under every one.

Which route a card is on is decided exactly as the automation decides it: the
card stands on one of the route's stages, or it names the route with an option
of that name — usually in the «Workflow» field. See
["How a card gets onto a route"](./automation/index.md).

The pick is kept in the view itself: it is the same after a reload and on
another screen. For a view per route, add a view and pick the route in it — the
other views are untouched.
