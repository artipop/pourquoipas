# Inbox

Cards can arrive by themselves: a task from Kaiten, a link from the browser, a
notification from a phone. Everything that arrives is filed in one place — the
inbox board. How to connect sources is on [the next page](./sources.md).

## Where it is

The inbox is a **board of its own**, always first in the sidebar, above the
other boards. The app creates it at the first launch, and there is one for all
sources: no source puts cards straight on a working board. (The app names the
board **Входящие**; the name is not translated.)

You can rename the board — the app still recognises it.

The board has one column per source and one for your own cards. Cards cannot
be dragged between the columns: a column shows who brought the card.

There is no separate "source" field: the source is recorded as the card's
author, and the board groups by "created by". If what arrived has an address,
it is saved on the card in a link field.

## A task of your own

The **"New"** button on the inbox creates a card of your own. It stands in the
**Мои задачи** ("my tasks") column — your own column among the source columns.
It is the place for what came to mind and has not been given a place yet.

## Sorting it out

Sorting a card out means deciding where it belongs and taking it out of the
inbox.

- **On a computer**: card menu → **"Move to a board…"**. The card moves whole,
  with its history. Delete a card you do not need.
- **On a phone**: `/m`, the Inbox tab. A card is moved to a board and a column
  in two taps.
- **By an agent**: sorting can be left to an agent — [Sorting the inbox](./sorting.md).

## If nothing arrives

| What you see | Why |
|---|---|
| No "Sources…" in a board's menu | Sources are set up only in the menu of the inbox board |
| "needs a token" next to a source | The token was not saved or was revoked — "Remove" and set the source up again |
| "failed" and an error text | Usually a wrong service address or token; the error is quoted from the service |
| "dropped" in the log | "A stream of notifications: keep only what a rule asks for" is on, and there are no rules |
| A shared link made no card | The same link was already sent to this board — the card is already there |
| No inbox board in the sidebar | It was deleted. Restart the app — it creates a new, empty one |
