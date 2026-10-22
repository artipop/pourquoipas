# First run

This page is about what the app shows on a new board: the last step of the
setup, the first task, the tour of the board and the hints.

## The last step of the setup

A new board opens with the setup wizard: name and agent. The wizard does not ask
for a folder — it is chosen on the card, in its «Folder» field (see [Folders and
branches](./folders.md)). Its last step, **"Ready"**, has three actions.

- **"Set the first task"**. Write what needs doing in the **"Your first task, in
  one sentence"** field and press the button. The app makes a card in the first
  column, assigns it to the agent, moves it into the second column and opens
  it. The move is what starts the agent — just as if you had dragged the card
  yourself (see [How the board starts an agent](./automation/index.md)). With
  several agents, a choice of agent is under the field. The agent runs under
  your own account with its provider and uses that account's quota.

  The field is there only when an agent is registered on the machine, AI is on,
  and the board has two columns and a field for who works on a card. Otherwise
  there is an **"Open “Where to start”"** button instead: every board made from
  a template has that card, and it explains how to work with the board.
- **"Show how the board works"** closes the wizard and starts the tour of this
  board.
- **"Done"** just closes the wizard.

To walk the wizard again, use the board's menu: **"Walk the setup again…"**.

## While the agent works on the first card

While an agent works on the first card, a strip is shown under the board's
header: where the agent's questions will appear, where to look for the result,
and that the agent moves the card itself into the column whose name says what
happened. On a computer the result is a comment on the card, and the changes in
the folder are under **"Changes"**; on a server it is the comment. The
strip goes when the card leaves its column or the agent finishes, and does not
come back. To put it away sooner, press **"Close"**.

## The tour of the board

The tour is a few tips over the board's real elements, in order:

1. a card — open any one;
2. in the card — the field for who works on it, the folder, the description
   (which is the task itself), the comments;
3. on the board — the columns, **"New"** → **"Talk it over with an agent…"**,
   the board's **"⋯"** menu, views and a link to a card; on a server,
   the "Share" button too;
4. in the left-hand list — **"Inbox"**, the list of boards, search.

**"Next"** goes on, **"Done"** closes a part of the tour, and the cross skips
it. When AI is off, the tour has no tips about agents.

## Hints

Some things the tour cannot show, because they happen later. The app tells you
about each of them once, when it happens:

- an agent asks its first question — a hint by the amber badge on the card;
- an agent moves a card itself for the first time — a hint on that card;
- the inbox has a card for the first time — a hint on it;
- **"Changes"** appears on a card for the first time — a hint by the button.

One hint is shown at a time. **"Got it"** puts it away for good.

## Showing the tour again

**"Settings"** → **"The app itself"** → **"The tour"** → **"Show"**. The tour
starts on the board that is open, and the hints that were put away come back
when what they are about happens. With no board open the button is disabled.
