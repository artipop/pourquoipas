# The board's system prompt

The board menu (⋯ in the header) → **"The board's system prompt…"**. This is
what an agent is told before anything else: before the card's task, before the
deploy or test instructions, before the question you type in a terminal.

It is added to:

- a session a column starts (agent, deploy, test);
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
only when there is something in the copy to show — the first stage of a route,
on a clean branch, gets none of it.

It is there for the second stage and the ones after it. A review that runs an
agent used to get exactly the text the “In progress” stage got: the card's task, in
a folder full of somebody else's commits — and the agent did the task again
instead of checking it. What the stage is actually for is still written in its
own instructions: “What the agent is told here” on the column, or “What the
agent is told at this stage” on a route's stage, in
[“Колонки и маршруты…”](./index.md).

## Where each of them is kept

The board's prompt lives **on the board**: it travels with it into an archive
export and onto another machine. The agent's prompt lives in the machine's
registry — it is about the agent rather than about the board, and it does not
travel.
