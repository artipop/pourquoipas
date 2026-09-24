# Onboarding: a first run that works

What a person should understand in their first ten minutes, and how to get
them there. Written after an attempt that failed, so it starts with why.

## What went wrong, and the rules that come out of it

Focalboard's tour was resurrected: a `/welcome` screen offering to show
somebody around, and nine tips over the board. Driven through a browser
against a real first launch, the button led to nothing — no tip appeared,
and what opened on the new board was `boardSetupWizard`, asking for the
project, the agent, the deploy target and the QA server. A full-screen
greeting had been put in front of a working wizard.

Three things were wrong, and each is a rule for the next attempt.

**The tour was about another product.** Views, links, sharing a board,
sidebar categories: kanban furniture anyone who has seen a board already
knows, and none of it is why this app exists. What is worth explaining is
the one thing no other board does — a card lands in a column and an agent
does the work.

**It described instead of doing.** A tip saying "open a card to explore the
powerful ways this can help you organise your work" teaches nothing. The
moment a person understands this product is the moment they watch an agent
finish something they asked for.

**It was not run before it was called done.** The entry point was tested;
what was behind it was not. Onboarding work is finished when a clean data
directory has been driven end to end in a browser, not when tests pass.

## What already exists

Most of the first run is built. None of it needs replacing.

- **`boardSetupWizard.tsx` + `internal/acp/setup.go`** — the real first run.
  Four questions — name, folder, agent, done — and a board may declare its
  own (`impliedSetup`, `BoardSetupPlan`; the Development template adds a
  deploy host). The board opens it itself. This is the spine; everything
  below hangs off it.
- **The templates carry content.** Every one opens with a card called «Where
  to start» («С чего начать») whose body is prose about how that board works
  — and whose steps 3 and 4 still describe the terminal and the routes
  (`docs/todo.md`, «Шаблоны объясняют доску, которой больше нет»). Explanation as data, in the
  board: it survives translation, it is editable, and it costs no UI.
- **`tutorial_tour_tip/`** — the drawing half of a coach mark: a positioned
  bubble and a hole punched over a real element, measured from a selector.
  Mechanically fine. It is the linear nine-step *script* around it that is
  not.
- **`components/acp/attention.ts` and the event socket** — how the page
  already learns that something happened to a card, without polling.
- **`docs/guide/`** — the manual. Onboarding points into it, never
  duplicates it.

## What success is

A person who has just installed this, with an agent already on their
machine, should within ten minutes have **watched an agent finish a task
they typed**, and be able to say where the result went and what decides
where the card goes next.

That is the measure — not "saw nine tips". A change that does not move
somebody closer to that moment is not onboarding.

## The plan

Six pieces, each shippable on its own, in dependency order.

### 1. Delete the Focalboard tour — half done

Gone already (the fork shrink, `docs/fork-shrink-removed.md`): `pages/welcome/`,
`POST /teams/{teamID}/onboard` with `server/app/onboarding.go`, and
`static/boards-welcome*.png` with the other unreferenced upstream images.

Left, all of it page code, and nothing blocks it any more — the Solid 2
migration it waited for is done: `frontend/src/components/onboardingTour/` (twelve steps and their tests), the
gifs they import from `src/assets/`, and the old preferences
(`onboardingTourStarted`, `onboardingTourStep`, `tourCategory`), which
`centerPanel.tsx`, `boardsSwitcher.tsx` and `boardTemplateSelector.tsx` still
read; step 4 says what replaces them.

Nothing routes any of it. It has now cost two rounds of work to rediscover
that; leaving it in the tree guarantees a third. The `tutorial_tour_tip/`
drawing code stays — step 4 uses it.

### 2. End the wizard with a first task

The wizard's last step is `done` and only says so. Instead it should offer
the one action that explains the product: **«Поставить первую задачу»** —
one field, one sentence from the person, and the app makes a card assigned to
the agent the wizard just registered, puts it in a column and opens it.

- There is no trigger column any more: a card starts when it is **moved** into
  a column and its assignee is an agent (`trigger.go`). So the first task is
  a card made in the first column and then moved into the next, or a card made
  straight into a column if creating one there counts as a move — which it has
  to be checked whether it does. No new configuration either way.
- The offer appears **only if the agent step passed**. The wizard already
  knows; an offer that fails because nothing is installed is worse than no
  offer at all.
- **It spends the person's own quota**, so it asks first and says so.
- A board whose wizard registered no agent gets no offer. It ends by pointing
  at its own «Where to start» card instead.

This is the highest-value piece and could ship alone.

### 3. Say what is happening while the first card runs

Somebody watching a card they just created does not yet know that the agent
answers with a comment on the card, asks its questions on the card itself, and
moves the card on when it is done.

One dismissible strip above the board while **their first card** is running:
that the agent is working, that its result arrives as a comment and its
questions under the case stamp, and that it will move the card to the column
whose name says what happened. It disappears when the card leaves the column,
and never comes back.

Driven by the events already on the socket — the same ones
`components/acp/attention.ts` subscribes to. No polling, no new API.

### 4. Contextual hints, one at a time, never a tour

Everything else worth learning happens at a moment that arrives on its own.
Tie each hint to its moment rather than to a position in a script:

| Moment | What the hint says |
|---|---|
| An agent asks its first question | the amber dot, and that the options *are* the answer — on the card or from the notification |
| The agent moves a card for the first time | it picked the column by its name; renaming columns is how a person tells agents what the columns mean |
| A card first arrives in «Входящие» | something outside brought this in; it waits here until you move it to the board it belongs on |
| «Changes» first appears on a card | what the agent changed in the folder, read from git |

Each is independent, shown once, drawn with `tutorial_tour_tip/`.

**Store them as a set, not a counter.** The old design had `tourCategory`
plus `onboardingTourStep`, which forced a total order and made "show me that
one again" meaningless. A `shownHints` preference holding ids lets hints be
added, removed and reordered without a migration, and lets one be cleared
alone.

One trap to remember, because it will bite again: `patchProps` used
`setState('users', 'myConfig', obj)`, and `setState` with a plain object
*merges* — a preference deleted server-side lived on in the page for ever.
It reconciles now, and `appStore.test.ts` holds the line.

### 5. Make «Where to start» the reading surface — done

Every template opens with it now, written for that board's own work: the
developer one got the card it never had, and each board ships two example
tasks beside it rather than five placeholders. What is left of this item is
the pointing: step 2's fallback and step 3's strip should lead to that card.

Content in a template is data: no code path, no message id, editable by
whoever edits the template, and it arrives with the board rather than being
layered over it. Prose belongs there; the app itself stays short.

### 6. Only then, a settings entry

Once hints exist as a set, Settings → «The app itself» can carry a row
that clears `shownHints`, and it will mean something. Adding it before there
was anything to replay is what produced a button that cleared four
preferences and navigated to a route that no longer existed.

## How each piece gets verified

- **A browser against a real build, from an empty data directory.** A build
  without the `production` tag uses `XCIII-dev`, so this never touches a
  real install: `wails3 task build:server DEV=true`, or `go build` with
  `server,frontend`. A throwaway cypress project pointed at
  `http://localhost:8080` drives it; the session token is in the bootstrap
  script the front door injects into the page.
- **The whole path, not the entry point.** Template → wizard → first task →
  what the agent left on the card and its branch. Every failure described at
  the top would
  have been caught by walking one step further than the change itself.
- **`docs/guide/`** gets its page in the same change, per the repository's
  rule, and every string it quotes is checked against `frontend/i18n/ru.json`.

## Open questions

- **What does the first task say?** A prefilled suggestion per template
  would be concrete, but one that does not fit the person's project is worse
  than an empty field. Probably an empty field with the template's own
  placeholder.
- **No agent installed at all.** Steps 2 and 3 need one. The wizard already
  offers to install an adapter through npx; whether onboarding waits for
  that or teaches the board without an agent is a product decision.
- **What carries to the landing.** `site/` has no mention of «Входящие» —
  mail, a tracker, share-from-a-phone, webhooks — which is half the product.
  The first-task flow, once it exists, is also the most demonstrable thing
  here.
