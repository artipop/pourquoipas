# Onboarding: a first run that works

What a person should understand in their first ten minutes, and how the app
gets them there. It replaces `onboarding-plan.md`, whose six pieces are built;
the reasoning that shaped them is kept here.

## What success is

A person who has just installed this, with an agent already on their machine,
should within ten minutes have **watched an agent finish a task they typed**,
and be able to say where the result went and what decides where the card goes
next. A change that does not move somebody closer to that moment is not
onboarding.

## The rules, from an attempt that failed

Focalboard's tour was once resurrected as a `/welcome` screen and nine tips.
Driven through a browser against a real first launch, the button led to
nothing: no tip appeared, and a full-screen greeting stood in front of a working
setup wizard. Three rules came out of it.

- **Say this product.** Views, links and sidebar categories are kanban
  furniture anybody who has seen a board knows. What is worth explaining is the
  one thing no other board does — a card lands in a column and an agent does the
  work.
- **Do instead of describing.** The moment a person understands this product is
  the moment they watch an agent finish something they asked for.
- **Run it before calling it done.** Onboarding work is finished when a clean
  data directory has been driven end to end in a browser, not when tests pass.

## Where it starts

Nothing is put in front of the board. The setup wizard that was the first run
(name → agent → «Поставить первую задачу») was removed: it asked what the board
itself answers (its name, edited on the board) and what is not a board's
question at all (the agent, which is the person's and is connected in Settings
→ AI). A board made from a template opens as it is, as in Focalboard, and the
way in is three things that are always there:

- **Settings → AI → «Подключить агента…»** — the one place an agent is
  connected (`docs/architecture.md`, «Agents are connected in one place»);
- **«С чего начать»** — every template's first card, saying how that board
  works and how to start the first task: connect an agent, assign the card to
  it, move it into the second column;
- **the tour**, from Settings → «Приложение» → «Тур по доске».

## The tour

Focalboard's, rebuilt in place (`components/onboardingTour/`, drawn by
`tutorial_tour_tip/`): four categories walked in order, a few tips each, progress
in user preferences — `onboardingTourStarted`, `tourCategory`,
`onboardingTourStep` and **`tourBoard`**, the board it runs on, by id. It used to
look for a board called «Welcome to Boards!» and a card called «Create a new
card»; no template was called that, so nobody ever saw it.

| Category | Steps | Drawn in |
|---|---|---|
| base | open a card — over the board's first card | `kanbanCard.tsx` (`onboarding` prop, set by `kanban.tsx` on the first card of the first column) |
| card | who works on the card · the folder · the description is the task · comments | the property rows (`cardDetailProperties.tsx`), `cardDetailContents.tsx`, the Comments button (`cardDialog.tsx`) |
| board | the columns · «Создать» → «Обсудить с агентом…» · the board's ⋯ menu · add a view · copy a link · share (team) | `kanban.tsx`, `viewHeader.tsx`, `kanbanCard.tsx`, `centerPanel.tsx` |
| sidebar | «Входящие» · the board list · personal categories (team) · search | `sidebarInboxItem.tsx`, `sidebarCategory.tsx`, `boardsSwitcher.tsx` |

**Steps are named and depend on the switches** (`tourScript` in `index.ts`):
with the agents off the steps about agents are left out, and a Focalboard feature
that needs a second person (sharing, personal categories) is shown only in team
mode; comments are toured everywhere, being the agent's result. A step's saved number is
its place in the list for the current switches, so the manager's «Next» and
«Done» and the dots read the same answer.

A tip is drawn where its element is, so it has to be somewhere always drawn: the
comments tip used to live inside the comments panel, which is closed until
somebody opens it, and held the tour at a step nobody could see.

The tour is started from the settings only, now that no wizard offers it. The
categories move on by themselves: an open card on the tour's board starts
the card's part (`useStartCardTour`); the card's part finished and the card
closed starts the board's (`centerPanel.tsx`); the board's finished starts the
sidebar's (`inbox.tsx`).

The tips carry no pictures: the gifs they had showed Focalboard's interface.

## While the first card runs

`acp/firstRunStrip.tsx` (the `boardHeader` slot): one strip under the board's header while
the first card an agent works is running — that its questions appear on the card
and in a notification, where its result is (a comment, and on a desktop also
«Изменения» in the folder; `isHosted` picks the text), and that it
moves the card into the column whose name says what happened. It goes when the
card leaves the column it was running in, the run ends or it is closed, and it
never comes back. Driven by `acp:session` on the event socket; nothing is
polled.

## Hints

`onboardingHints/`: what arrives at a moment of its own, drawn with the tour's
dot and box and one «Понятно».

| Hint | Moment | Drawn in |
|---|---|---|
| `question` | a card shows the amber badge | `kanbanCard.tsx` |
| `agentMove` | a card was moved to another column by an agent's account | `kanbanCard.tsx` (`cardMovedFrom`) |
| `inbox` | the inbox is open and has a card | `kanbanCard.tsx` |
| `changes` | «Изменения» is drawn on a card | `cardDialog.tsx` |

**They are a set, not a counter**: `shownHints` holds the ids already shown, so a
hint can be added, removed or shown again without a migration. The strip is in
the same set (`firstRun`). **One is drawn at a time**: the first place that
wants a hint holds the slot (`useHintSlot`) — every card with a question wants
the same one.

An agent's move is told apart from a person's by who last wrote the card: an
agent's writes are signed with its own account (`internal/author`), and the page
knows which usernames are agents (`loadAgentUsers`). A moved card is drawn anew
by its new column, so where it was is kept outside it.

## «С чего начать»

Every template opens with a card that says how that board works, written for the
board's own work. It is marked `readFirst` in the template's data rather than
found by its title, and `createCard` keeps the mark, because the page rewrites a
card's fields wholesale on every edit. The strip leads to it.

## Settings

Settings → «Приложение» → «Тур по доске»: clears `shownHints` and starts the
tour on the board that is open. It exists because there is now a tour to replay
and a set to clear; the button it replaces cleared four preferences and went to
a route that no longer existed.

## How it is verified

A browser against a real build from an empty data directory. A build without the
`production` tag uses `pourquoipas-dev`, so this never touches a real install:
`go build -tags server,frontend`, run with a data directory of its own, and
drive `http://localhost:8080`. The whole path, not the entry point: connect an
agent → template → «С чего начать» → a card moved → what the agent left on the
card and its branch.

## Open questions

- **Nothing starts the tour by itself.** The wizard offered it; without the
  wizard it is in the settings only, and a newcomer finds «С чего начать» on the
  board instead. Whether the first board a person opens should start the tour,
  as Focalboard's welcome board did, is open.
- **No agent connected at all.** «С чего начать» says to connect one; the form
  offers to install an adapter through npx. Whether onboarding should teach the
  board without an agent is a product decision.
