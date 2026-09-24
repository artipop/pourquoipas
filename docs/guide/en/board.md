# The board: title, description, menu

This page is about the board itself: the header, the menu, the views. How an
agent takes a card is covered in ["The agent on a card"](./agent.md).

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

The description is also asked when a board is made, on the **"Name"** step, in
the **"Description"** field. The agent that sorts the inbox reads it to decide
which cards to move to this board — see [Sorting the inbox](./inbox/sorting.md).
The field may be left empty.

## The board menu: «⋯» in the list on the left

Hover over a board in the list on the left — «⋯» appears on the right. In the
menu:

- **"Export board archive"** — export this board as one file. Exporting all
  boards at once, and import, are in [Moving](./transfer.md);
- **"New template from board"** — turn the board into a template, so the next
  one starts with the same columns;
- **"Duplicate board"**, **"Hide board"**, **"Delete board"** and
  **"Move To..."** — move the board to another category of the list.

## The «⋯» menu in the board header

This is where the board answers the questions about this machine, one menu item
per question:

- **«Системный промпт доски…»** — what this board's agents are told before
  anything else (["The board's system prompt"](./automation/prompt.md));
- **«Папки…»** — the folders its agents work in
  (["Folders and branches"](./folders.md));
- **«Агенты…»** and **«Куда деплоить…»** — registries of the machine, opened
  from where they are used;
- **«Пройти настройку заново…»** — the same wizard a new board opens;
- **«Сохранить как шаблон…»** and the exports, at the bottom under a rule.

Which items a board has is the board's own answer: one that publishes nowhere is
not asked where to deploy.

## Views

Under the board on the left are its views: kanban, table, calendar, gallery.

Everything that arrives from outside goes not to a working board but to a
board of its own, **Входящие** — see [the Inbox section](./inbox/index.md). (The
app names that board in Russian, and the name is not translated.)

The view header has the view's name, **"Filter"**, **"Sort"** and search. These are settings of the view, not of the board: each view has its own.
When they do not fit the window the bar scrolls sideways. The ⋯ menu belongs to
the view too; on the Входящие board it holds **"Sources…"** and nothing else.

Clicking the view's name opens a menu: the board's views, **"Rename view"**,
**"Duplicate view"**, **"Delete view"** and **"Add view"**. Renaming opens a
field in the header itself — Enter saves it, Esc cancels.

## Columns

A column is an ordinary option of the property the kanban groups by («Статус» in
the shipped templates). Adding, renaming and reordering them is done on the
board itself; columns have no editor of their own, and the board knows nothing
about them but their names.

The name matters, though, because the agent reads it. A card is moved into a
column, the agent is given that column's name along with the names of the
board's other columns, and it decides from them where to put the card when it is
finished. «Готово», «На ревью», «Заблокировано» are sentences, not settings.

The board's first column is separated from the rest by a vertical rule. What
arrives is to the left of it; what the board does with it is to the right.
