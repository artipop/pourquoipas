# Sorting the inbox

An agent can move cards from the inbox to the boards they belong on. It decides
by the board's title, its description and a summary of its cards. When no board
fits or the agent is not sure, the card stays in the inbox.

Sorting is off until you turn it on.

## Turn it on

1. Open the inbox board, the **⋯** menu → **"Sorting the inbox…"**.
2. In **"Agent that sorts"** pick an agent. The list is your agents of both
   kinds: programs on the computer and models by key ([Agent](../agent.md)).
   When the list is empty, connect an agent in «Settings → AI» → «Connect an
   agent…» ([Connecting agents](../settings/ai.md)).
3. Tick **"Sort cards from sources as they arrive"**.

From then on, every card a source brings moves to a matching board a few seconds
after it arrives, or stays in the inbox.

The **"Sort now"** button sorts everything in the inbox, your own cards
included. It works with the box unticked as well. The agent takes up to a few
minutes.

## Where the card lands

A moved card stands in the board's **first column** and **does not start** that
column's automation: the agent picks the board, and what happens to the card
next is up to you.

## What the agent decides by

- **The board's description.** It is asked when a board is made, on the step
  with the name. In the sorting dialog each board's description can be edited:
  write which cards belong on it — topics, projects, places.
- **The summary of the cards.** A few sentences about what the board's cards
  are about. The agent writes it itself and updates it when the board's cards
  change. **"Update summaries"** rewrites it for every board at once.
- **The board's recent cards** — their titles.

If a card went to the wrong board, add to the description of the board it
should have gone to, or of the one it went to by mistake.

## The log

At the bottom of the dialog, **"Recent decisions"**: which card was moved where
and why, which stayed in the inbox, which could not be sorted.

## If it does not work

| What you see | Why |
|---|---|
| "Agents are turned off in this app…" | Agents are turned off in the app's settings |
| The box and the buttons are disabled | No agent is chosen |
| "not sorted" and an error text in the log | The agent did not start or did not answer as asked; the error is in the log |
| A card stayed in the inbox | The agent found no matching board; the reason is in the log |
| A card made with "New" was not sorted by itself | Your own cards are sorted only by "Sort now" |
