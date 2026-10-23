# How a card gets worked on

What happens between dropping a card into a column and getting the work back:
which agent takes it, where it writes, which branch it commits to, and what
moves the card on. Written for a person using the board; the code is in
`internal/acp`.

## The whole rule

**A card moved into a column, assigned to an agent, is worked by that agent.**

That is all of it. There is no editor behind it, nothing to configure per
column, and no route to draw. A column is an ordinary option of the board's
status field and means whatever the people using the board mean by it; dragging
a card into one is a sentence, and the agent reads it along with the card.

Two things have to be true:

- the card names an agent in its assignee field («Кто занимается» on the shipped
  boards) — a card assigned to a person, or to nobody, starts nothing;
- the agents are switched on (Settings → «Агенты работают с карточками»).

A card dragged out of a column while its agent is working it takes the work
back: the run is cancelled, the working copy is released, and the new column
starts a fresh one.

## What the agent is told

Before the task it gets, in this order:

1. **the board's prompt** — ⋯ menu → «Системный промпт доски…», what is true of
   every card here;
2. **the agent's own prompt** — Settings → AI → the agent, what is true of that agent
   on every board;
3. **the card**: its title and its description. There is no separate "task"
   field — the description *is* the task;
4. **where the card stands**: the column it was just moved into, and the names of
   every column the board has, read off the board at that moment;
5. **what the branch already holds**, when there is one: the branch name, what it
   was cut from, the commits on it, the changed files. A first run on a clean
   branch gets nothing here.

## What the agent does at the end

The task says so in as many words, because a CLI does not always load its tools
before it starts working:

1. **`comment_card`** — one comment with the result: what was done or decided,
   and what a person has to know or do next. Not a log of its steps.
2. **`update_card`** — values into the card's own fields, if the task asked for
   any. `list_card_fields` says what each field accepts.
3. **`move_card`** — into the column whose name says what has happened. If none
   of them does, the card stays where it is.
4. **`finish_work`** — done or not done, and one line. This is what ends the run.

A move the agent makes itself does not start it again.

If the agent finishes without writing a comment, the app writes one for it, so
there is always something on the card. If it stops without calling
`finish_work` — the app was closed, the CLI died — the card keeps its place and
the reason is recorded on it.

## Where the work happens

A card names a folder in its «Папка» field, and the folder is a registry entry
on this machine (⋯ menu → «Папки…»). What the agent gets depends on the folder:

- **a git repository** — a workspace of the card's own: a worktree, or the folder
  itself on a branch, depending on how the folder is set up for this board. The
  branch is written onto the card and is what survives; the working copy is
  folded away when the run ends and remade from the branch next time.
- **an ordinary folder** — the folder itself, as it is.
- **no folder at all** — the card's own drafts, a directory under the board's
  drafts that is kept after the run, and the agent is told nothing about a
  branch. A folder the card names and the board no longer has is not this: the
  card stalls and says so.

A card with work started cannot change its folder: the field goes read-only.

## Questions

An agent asks through the protocol: permission for a tool it was not given, or a
choice it needs made. The question appears in three places and any of them
answers it:

- **on the card** — an amber badge in its corner, and the question itself with
  its answers when the card is open;
- **in the notification** in the corner of the board window, unless notifications
  are switched off in the settings;
- **on the phone**, under «Ждут».

The first answer wins; the others stop showing the question. Dismissing a
notification means "I know", not "answered" — the card keeps the question until
somebody answers it.

## Talking before there is a card

The arrow beside «Создать» in the board header opens **«Обсудить с агентом…»** —
a conversation with no card behind it. The agent reads the folder you choose and
changes nothing in it; the cards you agree on it creates itself, on that board.

## What a board is asked before any of this works

Nothing. A board made from a template opens as it is; its name and description
are edited on the board. The agent is connected once, in Settings → AI, and is
offered on every board; the folder is chosen on the card, in its «Папка» field.
Every question an agent's work raises may be left unanswered: the board still
works, on a default said out loud where the question is asked.

## What is not here any more

Routes, column actions, crews, per-column limits, stage property contracts and
the «Исход» field were all real and are gone. `docs/decisions.md` records what
they cost. Anything a board used to get from them it now gets from the agent:
the agent is told where the card is and what the columns are called, and it
moves the card itself.
