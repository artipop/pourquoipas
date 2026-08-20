# Columns and routes

The board's behaviour is configured in the **"Columns and routes"** window:
what each column does and where a card moves next. Two ways to open it:

- the board menu (⋯ in the header) → **"Columns and routes"**;
- any column's menu → **"What happens in this column…"** — the same window with
  that column already selected.

There are no scripts or formulas: everything is assembled from ready-made
actions.

## Basic terms

- **A column** — where a card stands. A column has an action: nothing, an
  agent, a deploy, a test. The action applies regardless of the route.
- **A route** — arrows between columns: where a card moves after a step passes
  or fails. There can be several routes; a card picks its own.
- **A stage** — a column within one route. Exceptions are configured here: "on
  this route this column behaves differently".

Configure in this order: column actions first, then arrows — an arrow fires on
the outcome of an action, so while a column does nothing, the arrow has nothing
to fire on.

## The editor window

Tabs on top: **"Columns"** — all the board's columns and their actions; a route
tab — the same boxes with that route's arrows (columns outside the route are
faded and join it on click); **"+ route"** — a new route.

**"Add a column"** — the row under the tabs: "Agent", "Deploy", "Test",
"Column". Click a kind and a new column with that action appears on the board;
drag the block onto the canvas instead if where it stands matters. There is no
separate "create a stage" step: a route stage is always a board column.

**The canvas** — the board as a diagram, each column a box.

**The panel** — on the right, shows what is selected. What it asks about
depends on the tab: on "Columns" it is about the column (true on every route),
on a route tab about that stage of the route.

## Column settings

The **"Columns"** tab, select a column — in the panel:

- **name** — edited here; routes are tied to the column, not its name, so
  renaming breaks nothing;
- **"When a card lands here"** — the action: nothing, an agent works, the
  branch is deployed, the preview is tested;
- **"Worked by"** — which agents take this column's cards. If none is ticked,
  the card determines the agent;
- **"At once"** — how many cards the column runs at a time; the rest queue and
  start as slots free up;
- **deploy target** — on a deploy column;
- **"What the agent is told here"** — the column's instructions. The agent gets
  them in front of the card's task, and they open a conversation a person
  starts in this column too: "You are the reviewer — find defects, fix
  nothing" on «Ревью» makes the same agent a reviewer. Written in any language —
  passed through as it is;
- **"Writes onto the card"** — the properties a stage leaves on the card. On an
  agent column it is a contract: the agent passes the values in `finish_work`,
  and a "required" one is refused without a value — the stage cannot end until
  the property is written. A deploy column picks one property for its preview
  address; a test column one for its verdict (`pass`/`fail`/`blocked`). Values
  land on the card **before** it travels on, so a conditional arrow can rely on
  them.

  The list offers not every board property but the ones a stage can actually
  write to: text, a link, a number, an email, a phone and one of a set. A date,
  a person and «Автор»/«Создана» are left out — there is nothing to write there.

  When the board has no property for the value, the last item in the list is
  **"+ a new property…"**. The form asks for a name and for what the property
  holds: text, a link, or one of a set of values. The property appears on the
  board at once — it shows on every card — and is declared as this stage's
  output in the same breath. A deploy column and a test column are not asked
  what it holds: there it is a link and the set `pass`/`fail`/`blocked`, and the
  form asks only for the name.

  What it holds decides whether an arrow can branch on it. An arrow asks "only
  if the card has this value", which is a question to a set: a property with a
  set of values has something to pick from, a text one has not. A text property
  travels to the stages further on and is read there.

  One property the app writes itself, and it is not declared: **«Исход»** — how
  the stage ended («прошло», «не прошло», «заблокировано»). Every stage writes
  it, which is why it is not in the writes list; an arrow can still branch on
  it, like any property with a set of values. Because of it a board no longer
  needs columns like «Проверено» or «Не прошло»: when a stage has no transition
  for its outcome the card stays where it is, the outcome is on it, and the
  route strip carries the reason in amber.

  A board made from the «Разработка» template has this set up already: the
  «Деплой» column writes «Превью» (a link), and it shows on that box on the
  canvas. How a stage ended goes into «Исход».

  The declared writes reach the agent in its brief: what it has to pass in
  `finish_work`, and what each value may be — for a property with a set, the
  values themselves. To check while working, the agent has the
  `list_card_fields` tool: every card property of this board, which of them can
  be written to at all, and which this stage has to report;
- **"Gets from the card"** — the properties the stage reads: their values open
  its brief ("From the card: Превью: …"), so the agent does not have to think
  of asking `get_card`.

  They do not always have to be ticked. When a route stage's list is empty, it
  gets whatever the stages before it write — the panel shows those in a faint
  row naming the stage that produces each: «Превью · пишет «Деплой»». Declare a
  write once and the value travels on by itself. Naming anything in the list
  replaces that set whole;
- **"MCP servers of this column"** — the tools the agent working here is given.
  The same JSON any MCP client takes, so it can be copied from a server's own
  README:

  ```json
  {"mcpServers": {"playwright": {"command": "npx", "args": ["-y", "@playwright/mcp@latest"]}}}
  ```

  A column's servers are added to the agent's own rather than replacing them:
  the same agent needs a browser on «QA» and does not need one on «В работе»,
  and there is no second agent to register for it. Their tools run without
  confirmation prompts. The name `board` is taken — that is how the board's own
  tools reach the column, the ones an agent says it has finished with.

These settings apply on every route. An exception for one route is set in the
stage panel — select the stage on the route's tab.

## Outputs on the box, and a fork drawn from one

On a route tab a stage's box carries a third line: what the stage writes onto
the card. It is not a caption but a point to pull from — drag from a property
that has a set of values to another stage and you get a transition "the card
goes here if this property equals…". The value is chosen in the panel; until
then the arrow is captioned «Вердикт = ?» in amber, because the transition
exists and the answer does not yet.

A property with no set of values (text, a link) is drawn faint and cannot be
pulled from: an arrow asks "is the value this one", and text has no set. Such a
property travels on down the route and is read by the stages further on.

A long arrow — one that skips several stages or sends the card back — runs in a
corridor of its own under the graph, so that two of them do not travel side by
side and read as one.

An arrow you have already drawn asks about properties in the same order: first
what the stage it leaves writes, then what the stages before it write — with the
writing stage named beside each — and only then the rest of the board's
properties.

### A column that tests

Testing is an agent clicking through a browser, and the browser is an MCP
server — from "MCP servers of this column", or from the agent's own settings.
With one nowhere, the panel says so and the test stage does not start: a card
that arrives waits for a person to check it. That is a working state and not a
breakage — checking by hand is always allowed.

The board's setup wizard asks this on its **QA** step: pick an agent (one can be
added there) and the browser is set up for the column, with that agent as its
crew. For the column rather than for the agent, because it is the testing that
needs a browser and not the tester: an agent given one here would carry it onto
every other board.

The QA step appears once the board has a column or a stage that tests — even if
the column was added a month after the board was made. Deploy works the same
way: add a column that deploys and "Where to deploy…" turns up in the ⋯ menu;
take it away and the question goes with it.

## Stage settings

A route tab, select a box — in the panel:

- **"What happens at this stage"** — the action on this route alone. While it
  reads "— as the column: … —" the column's action runs, and the line names it;
- **"Worked here by"** — this stage's agents. While none is ticked, the line
  under the boxes says who works it instead — the column's crew. This is how
  one route runs "In progress" with one agent and another with a different one;
- **"The stage works"** — on the card's own branch, or in the folder itself.
  Asked of a deploy and a test, where it decides something: checking before the
  branch is merged happens on the card's branch, checking what is already
  published happens in the folder. An agent stage works on the card's branch,
  and the control only stands there once an answer has been given;
- **deploy target** — on a deploy stage;
- **"What the agent is told at this stage"** — instructions for this route
  alone. While the field is empty the column's apply, and the placeholder names
  them;
- **"Writes onto the card" / "Gets from the card"** — the same as the column's,
  for this route alone; an empty list inherits;
- **"MCP servers at this stage"** — the tools for this route alone. While the
  field is empty the column's servers apply and the heading names them; filling
  it in replaces the column's whole set;
- **"From here the card goes"** — how many transitions the stage has, and the
  button to add one; an arrow itself is edited by clicking it on the canvas,
  which makes the panel about that arrow;
- **"Settings of the column itself…"** — over to the "Columns" tab, to what is
  true of the column on any route.

## Arrows

An arrow is drawn by dragging from the edge of a box. The point you drag from
sets the event:

- **upper right point** — "when the step passed" (green);
- **lower right point** — "when the step failed" (red);
- **bottom point** — "wait for an event": branch merged, pull request merged,
  checks passed (grey, dashed).

Clicking an arrow opens it in the panel: the event, the destination, the
[condition](./rules.md). Delete or Backspace removes the selection.

## How a card gets onto a route

Click an empty spot on a route tab — the panel shows the route itself: name,
project (optional) and the list of columns.

A card joins a route by picking the option with the route's name — usually in
the "Route" field. If the board has no such option, the editor warns you and
offers a button to add it.

A card without a route is still served by the columns, but does not move on its
own.

## What is set elsewhere

The board's other questions are their own items in the board menu (⋯ in the
header), and which of them a board has depends on the template it was made
from:

- **"The board's system prompt…"** — what is said first to every agent of this
  board, and what is said to one of them in particular (see
  [The board's system prompt](./prompt.md));
- **"Folders…"** — folders on your machine where this board's agents work, and
  how one that is a repository is worked in (see [Folders and branches](../folders.md));
- **"Where to deploy…"** — deploy targets: the Dokku host, the SSH user and
  key, the preview domain. The item is only on a board that asks about
  deploying — of the shipped templates that is «Разработка». The targets
  themselves are shared by the whole machine: one added here is visible to any
  other deploying board;
- **"Walk the setup again…"** — the same questions the board asked when it was
  first opened. The folder can be put off with "Skip": without one a card can
  still be discussed with an agent, but on a route it will wait at the stage
  that works it. On the "Agent" step the wizard asks who works this board's
  cards: the agent names are chips, clicking one turns it on and off, and
  several or none may be chosen. Whoever is chosen is written into "Worked by"
  on the column an agent works in.

## Saving

- changes to the board itself — a new column, a rename — apply
  immediately;
- column actions and routes are written with the **"Save"** button. Everything
  is checked before writing: a transition to nowhere, two stages on one column,
  an empty condition or an unknown agent is rejected with an explanation.

"Close" without saving discards unsaved changes (except changes to the board
itself — those already applied).

## Templates

A board is born from a template: "+ Add board" → a template in the list on the
left → "Use this template". The template brings the columns, what they do, the
routes, and the **questions** the setup wizard asks when a board is created
from it — project folder, agent, deploy target, browser for tests.

Your own template is edited with the pencil beside it in that same list: the
name, the icon, what it is for, and those questions. Columns and routes are
only listed there — they are changed on a board, in "Columns and routes…".
