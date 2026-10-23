# The agent on a card

This page covers how an agent works a card: who takes it, how to answer its
questions and what stays on the card.

## Who gets a card

A card is worked by whoever it is assigned to. The assignment field is an
ordinary board field of the "person" type; templates name it differently:
«Assignee» on the «Development» board, «Who does it» on «Personal», «Who leads
it» on «Content».

It has to be filled in by hand: the app assigns nobody by itself. A card
assigned to a person, or to nobody, is left alone — it simply stands where it
was put.

An agent is one of two kinds, and a card does not tell them apart:

- **a program on this computer** — Claude Code, Codex and other ACP agents;
- **a model by API key** — the app calls Anthropic or OpenAI itself, with your
  key. A server has only these, and they have no folders ([Computer and
  server](./modes.md)).

Both are connected in «Settings → AI» → «Connect an agent…» ([Connecting
agents](./settings/ai.md)). After that the agent is in the assignment field on
any board, next to the people: open the field and pick it. The agent does not
become a member of the board, and there is nothing to add to it.

**The list holds all your agents, and only yours.** A board keeps no crew of
its own; on a server nobody else's agents are in it. The only thing that takes
yours out of the list is the «Agents work on the cards» switch in
[the settings](./settings/index.md), section «AI».

The agent reads its task from the **card's description** — there is no separate
task field. The board's instruction to all its agents is added in front of it
(["The board's system prompt"](./automation/prompt.md)), and so is the agent's
own prompt («Agent system prompt» in its settings).

## What starts the work

Moving the card into another column. All that has to be true:

- an agent is assigned to the card;
- agents are switched on in the settings.

Nothing else is configured — columns have no actions and no crews. The agent is
given the name of the column the card was put into and the names of the board's
other columns; what to do, it reads from the card.

If the card is dragged out of the column while the agent is working, the work is
cancelled: a move is a new instruction, not a continuation of the old one.

## What the agent does at the end

1. writes **one comment** with the result;
2. puts values **into the card's fields**, if the task called for it;
3. **moves the card itself** into the column whose name says what has happened:
   «Done», «In review», «Blocked». If there is no fitting column, the
   card stays where it is;
4. says the **work is finished**, which is what ends the session.

A move the agent makes itself starts no new work.

If the agent finishes without writing anything, the app writes the comment —
something always stays on the card. If the work was cut off (the app was closed,
the CLI died), the card keeps its place and the reason is written on it.

## Talking to an agent before there is a card

A task can be talked through before it becomes a card: the arrow beside
«New» in the board header → **«Talk it over with an agent…»**.

First it asks **which agent** and **which folder** — the board's folders, «The board’s
drafts» and «Add a folder…»; on a server no folder is asked. After that it is
an ordinary conversation: you write, the agent answers, «Stop» interrupts it.
From the start the agent knows the board's name and its description — the
clearer the description, the less it has to ask.

In such a conversation the agent changes nothing in the folder — its
instructions and the tool policy hold it to reading — and the cards you agree on
it creates on this board itself.

At the top of the dialog are the **«Open conversations»**: a conversation
outlives its window. **«End the conversation»** stops the agent and takes the
conversation off the list.

## The agent's questions

An agent asks through the protocol: permission for a tool it was not given, or a
choice that cannot be made for it. The question appears in three places, and any
of them answers it:

- **on the card** — an amber badge in its bottom-right corner, and the question
  itself with its buttons once the card is open;
- **in the notification** in the corner of the board window;
- **on the phone**, under «Waiting».

Whichever answer arrives first counts; the others stop showing the question.

A notification can be dismissed with «Dismiss» — that means "I know", not
"answered": the question stays on the card until somebody answers it. A
dismissed notification does not come back in another window, on the phone, or
after a reload.

If the app is minimised the notification comes from the system, and an amber dot
appears on the icon beside the clock.

Notifications are switched off in
[the settings](./settings/index.md#agent-notifications). The badge on the card
and the dot beside the clock stay either way — a notification is about the
question, they are about the agent still waiting.

## The history: who did what to the card

The card's toolbar has a **«History»** button. It opens a list of what happened
to the card on the right, newest first:

- who made it;
- who renamed it, and to what;
- who changed a field and how: “Status”: In progress → Done;
- who wrote a comment — and what it said.

The names are real. Every registered agent has a board account of its own, and
its writes are signed by it rather than by you: the history shows that the agent
moved the card and that you set the date.

Every card has one, even one nobody has touched: being made is an entry too. It
lives on the board itself and travels with it into an export and onto another
machine.

## What is visible on the card

What the agent is doing is on the line under the card's title; its question is
under it.

The agent's result is a comment. The card's toolbar has a **«Comments»**
button, it opens the panel on the right, and the card's face shows their count.
It is the same on a computer and on a server.

If the work could not start — no folder, or another card is holding the folder —
the reason is shown on the card and goes as soon as something happens on it
again.
