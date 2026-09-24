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
  Mechanically fine, and so is the tour built on it (`onboardingTour/`);
  what is wrong is where the tour runs and what it says — step 1.
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

### 1. Rebuild the Focalboard tour in place

The first version of this plan deleted the tour. It stays instead, in
Focalboard's shape, and is taught about this product: the tree is kept close
to upstream in structure and main features, and the tour's machinery is
sound — what is wrong with it is where it runs and what it says.

**What is there.** `components/onboardingTour/` — twelve tips in four
categories, `TOUR_BASE` (open a card) → `TOUR_CARD` (properties, comments,
description) → `TOUR_BOARD` (add a view, copy a link, share the board) →
`TOUR_SIDEBAR` (the sidebar, categories, search), ordered by `TOUR_ORDER` and
numbered by `TourCategoriesMapToSteps`. Each tip is a `TourTipRenderer` over
`tutorial_tour_tip/` (the bubble, the punched-out hole, the manager's
next/skip/finish), mounted where its element is: `cardDetailProperties`,
`commentsList`, `cardDetailContents`, `viewHeader`, `centerPanel`,
`sidebarCategory`, `boardsSwitcher`. Progress is three user preferences —
`onboardingTourStarted`, `tourCategory`, `onboardingTourStep` — and the tip
texts were already rewritten for this product in `frontend/i18n`.

**Why nobody has ever seen it.** Every tip asks whether the board is called
«Welcome to Boards!» and the card «Create a new card» (`OnboardingBoardTitle`,
`OnboardingCardTitle` in `cardDetail.tsx`, and a literal copy in
`centerPanel.tsx`), and the tour is started only by picking a template of that
name (`boardTemplateSelector.tsx`). No template is called that. Finding a
board by its title is also exactly what this repository does not do.

The work, in the order it becomes visible:

1. **Run on a board by id, not by title.** A fourth preference,
   `tourBoard`, written when the tour starts; every `isOnboardingBoard` asks
   for it. The card tour starts on the first card opened on that board,
   whatever it is called — «Where to start» is what the base tip points at,
   but nothing checks its name. `OnboardingBoardTitle` and
   `OnboardingCardTitle` go, and with them the `requireCard` title check.
2. **Start it where a person already is.** Not from the template picker and
   not from a `/welcome` screen: the last page of the setup wizard offers it
   («Показать, как устроена доска»), on the board just set up. Step 6's
   settings row restarts it (Settings → «The app itself»), as Focalboard's
   own menu once did.
3. **Say this product in the steps.** The categories stay; what is in them
   changes, so the tour is about the one thing no other board does:

   | Category | Steps |
   |---|---|
   | base | open a card — the tip over «Where to start» |
   | card | the assignee field: an agent there is what starts work · the folder field · the description is the task · comments are where the agent answers |
   | board | the columns: moving a card into one starts its agent, and the agent moves it on · «New» → «Talk it over with an agent…» · the board's ⋯ menu: prompt, folders, agents · add a view |
   | sidebar | the inbox pinned on top, and what arrives there · categories · search |

   «Copy the link» and «Share the board» stay as Focalboard features but only
   where they work: the share step only in team mode (`teamMode`), where the
   button is drawn at all.
4. **Steps depend on what is switched on.** With agents off
   (`aiEnabled`) the card and board steps about agents are left out, the way
   every `is…Available()` already leaves them out of the page. So
   `TourCategoriesMapToSteps` becomes a function of the switch and the mode,
   and the manager's "last step" logic reads it instead of a constant map.
5. **Images.** The five gifs in `src/assets/` show Focalboard's UI. Tips go
   without images until they can be recorded from this app; `changeViews.gif`
   may survive for «add a view» if it still matches.
6. **Tests and the guide.** The tip tests exist per step and move with them
   (`onboardingTour/*/…test.tsx`, plus `cardDetail`, `centerPanel` and the
   template selector, which read the old titles). `docs/guide/` gets a line
   saying where the tour starts and how to see it again.

Verified the way this document says everything here is: a clean data
directory, a browser, the whole path — template → wizard → tour → a card
moved into a column with an agent on it.

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

### 4. Contextual hints, one at a time, after the tour

Everything else worth learning happens at a moment that arrives on its own.
Tie each hint to its moment rather than to a position in a script:

| Moment | What the hint says |
|---|---|
| An agent asks its first question | the amber dot, and that the options *are* the answer — on the card or from the notification |
| The agent moves a card for the first time | it picked the column by its name; renaming columns is how a person tells agents what the columns mean |
| A card first arrives in «Входящие» | something outside brought this in; it waits here until you move it to the board it belongs on |
| «Changes» first appears on a card | what the agent changed in the folder, read from git |

Each is independent, shown once, drawn with `tutorial_tour_tip/` — the same
machinery as the tour, which walks a person through the board once; these
arrive later, at moments the tour cannot schedule.

**Store them as a set, not a counter.** The tour keeps Focalboard's
`tourCategory` plus `onboardingTourStep`, because a tour is a script and its
order is the point; hints are not, and a counter would force a total order on
them and make "show me that one again" meaningless. A `shownHints` preference holding ids lets hints be
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

Once the tour runs (step 1) and hints exist as a set, Settings → «The app
itself» can carry a row that starts the tour again and clears `shownHints`,
and it will mean something. Adding it before there was anything to replay is
what produced a button that cleared four preferences and navigated to a route
that no longer existed.

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
