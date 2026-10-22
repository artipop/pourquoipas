# The board's system prompt

The board menu (⋯ in the header) → **"The board's system prompt…"**. This is
what an agent is told before anything else: before the card's task and before
anything you type in a conversation.

It is added to:

- a session a card being moved into a column starts;
- a conversation opened by "Talk it over with an agent".

Write here what is true of the whole board: "answer in Russian", "these are
household chores, not code", "the stack is Go and SolidJS, tests are required".
The field may be left empty — the board still works, the agent simply gets the
card's task and nothing else.

## There are exactly two prompts

| Prompt | Where it is set | Where it holds |
|---|---|---|
| The board's | the board menu → "The board's system prompt…" | every agent of this board |
| The agent's | the board's ⋯ menu → "Agents…" | that agent, on **every** board |

The agent is given them in that order — the board's prompt, then the agent's
own, then the card's task. The order is printed in the window too, under the
box.

There is no third one, on purpose. What needs saying about a **folder** is
already said in the folder: the `AGENTS.md` (or `CLAUDE.md`) in its root, which
the CLI reads by itself. What needs saying about one card goes in the card.

## What the agent is told about the branch

Besides the prompts and the task, the agent is handed what is already in the
working copy: the branch, what it was cut from, the commits on it, the changed
files with their line counts, and the command that reads the whole diff. If
anything is uncommitted, that is said too.

Nothing has to be written for this. It is assembled on its own, and it appears
only when there is something in the copy to show — the first run, on a clean
branch, gets none of it.

It is there for the second run on the same card and the ones after it. An agent
called in to review used to get exactly the text it got the first time: the
card's task, in a folder full of commits already made — and it did the task
again instead of checking it. What is wanted this time is written on the card
itself, or in the board's prompt.

## Where each of them is kept

The board's prompt lives **on the board**: it travels with it into an archive
export and onto another machine. The agent's prompt lives in the machine's
registry — it is about the agent rather than about the board, and it does not
travel.
