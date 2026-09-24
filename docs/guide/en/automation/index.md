# How the board starts an agent

There is one rule, and it fits in a sentence:

> **Move a card into a column while an agent is assigned to it, and that agent
> starts working on it.**

Columns have no editor and no settings. A column is an ordinary option of the
property the kanban groups by, so it means whatever the people using the board
mean by it. That is what the agent is given: it is told which column the card
was dropped into, and what the board's other columns are called.

## What has to be true

1. **An agent is assigned to the card.** The field is «Who is on it» (on the
   «Development» board it is «Assignee»). A card
   assigned to a person, or to nobody, starts nothing.
2. **Agents are switched on.** Settings → «Agents work on the cards».

With one agent registered, assigning it is one click on the card; the field
offers every agent on the machine. Registering one is covered in
["Agents, npx and Node.js"](../settings/agents.md).

## What the agent is given

In this order:

1. **the board's system prompt** — ["The board's system prompt"](./prompt.md);
2. **the agent's own prompt**, from the registry, which holds on every board;
3. **the card**: its title and its description. There is no separate "task"
   field — the description is the task;
4. **where the card stands**: the name of the column it was just moved into, and
   the names of every column the board has;
5. **what the branch already holds**, when there is one: its name, what it was
   cut from, the commits on it, the changed files.

## What the agent does at the end

It is written into the task itself, because a CLI does not always load the
board's tools before it starts working:

1. **a comment** — one, with the result: what was done or decided and what a
   person has to know. Not a log of its steps.
2. **card fields** — if the task asked for anything to be recorded.
3. **moving the card** — into the column whose name says what has happened. If
   none of them does, the card stays where it is.
4. **"the work is finished"** — this is what ends the run: a session's turn
   ending is not the same as the work being over.

A move the agent makes itself starts nothing again.

If the agent finishes without writing a comment, the app writes one for it —
there is always something on the card. If the work was cut off (the app was
closed, the CLI died), the card keeps its place and the reason is written on it.

## Nothing is happening — what to look at

- **nobody is assigned** — assign an agent;
- **a person is assigned** — that is what "I am doing this myself" means;
- **agents are switched off** — Settings → «Agents work on the cards»;
- **the board has no folder** — the agent still runs, in a scratch directory of
  its own. See ["Folders and branches"](../folders.md);
- **anything else** — open the card: the reason is written on it.

## What used to be here

The board used to carry a registry of columns (an action, a crew, a limit) and
routes with arrows and conditions, and every column had an editor of its own.
All of it is gone: what the graph was for, the agent does, and a column means
its name again. The outcome field, column limits, queues and conditions on
transitions went with the routes.
