# Settings

App settings live in one window: at the bottom of the left-hand list, under
"Add board", the **"Settings"** button with a cog. They apply to all boards.
Sections:

- **"The app itself"** — theme, language, help and feedback;
- **"AI"** — the "Agents work on the cards" switch, the agents and the
  proxies, see [Connecting agents](./ai.md);
- **"Updates"** — the app's version and moving to a newer one, see
  [Updates](./updates.md);
- **«People»** — who is on the server and who runs it; the section is shown to
  the administrator alone, see ["Your own server"](./team.md);
- **"Import and export"** — board archive export and import, see
  [Moving](../transfer.md);
- **"Other"** — the instruction for conversations without a card, "Notify me
  when an agent is waiting", "Fill a new card's folder in when the board has
  only one", ["Access for other applications"](#access-for-other-applications) and install details. The section is gone while the AI is off
  (see [AI agents](#ai-agents)).

Sections this build does not support are not shown: on a machine without
agents, "The app itself", "Updates", "Import and export" and "Other" remain.

Agents of both kinds — programs on this computer and models by key — are
connected in **"AI"**, where their list and the proxies are too: see
[Connecting agents](./ai.md) and [Program agents, npx and Node.js](./agents.md).

## The app itself: theme, language, help

- **Light or dark** — "Light", "Dark", "As the system is". The app has one look;
  it applies immediately; in full, [Light and dark](./themes.md).
- **Language** — the list of app languages. When none was ever picked, the
  system language applies. The board templates are shown in the chosen
  language. A board made from a template gets its column and field names in
  the language chosen when it was made, and changing the language does not
  change it — it is your data.
- **Help** — "The guide", with an "Open" button: it opens this guide. Below
  it — "Give feedback", with a "Write" button: it opens your mail program
  addressed to `hello@deffun.com`. The address is also printed next to the
  button, to copy if the mail program does not open.

## AI agents

The **"Agents work on the cards"** switch in "AI" turns everything
to do with agents on and off. It is on by default.

When it is off, the app is a plain kanban board:

- a board's ⋯ menu has no "Folders…" or the board's system prompt, and "AI"
  has no list of agents;
- moving a card into a column starts nothing;
- a card has no "Changes" and none of the agents' fields — the folder and the
  branch; the assignee field offers people only;
- there are no agent notifications and no dot on the icon next to the clock;
- the settings have no "Other" section, and the phone has no "Waiting" tab.

Nothing is deleted: agents, folders and the values on the cards stay, and come
back as soon as the switch is on again. An agent already working when the switch
is turned off finishes what it was doing.

The switch is shown only in a build that has agents at all.

All of this is available before the first board is created. The picked theme
and language are kept by the install itself and survive a restart.

## Agent notifications

The **"Notify me when an agent is waiting"** switch is in the "Other" section.
A notification carries the question itself and its buttons: it can be answered
from there without opening the card.

When the app is minimised or behind another window, the notification comes from
the system instead: it names the agent that is asking and the card it is about.
Clicking it opens the card, and that counts the same way as knowing about the
question.

When off, the switch removes only the notifications. The amber badge on the card
stays, and so does the dot on the app's icon next to the clock: they
say an agent is waiting, and they interrupt nobody.

On macOS the system asks for notification permission once — the first time an
agent is actually waiting. Without it you keep the notifications in the board's
window and the icon next to the clock.

## The icon next to the clock

The app keeps an icon in the menu bar (the system tray on Windows and Linux). An
amber dot appears on it while at least one agent is waiting for a person.

Clicking it opens a menu:

- **"Open"** — bring the board's window back, even from minimised or closed.
- The waiting agents, one per line: the agent's name and the card. Clicking one
  opens that card. If more than six are waiting, the last line is "…and N more": how many
  agents did not fit.
- **"Quit"** — close the application entirely.

The icon cannot be turned off: it interrupts nobody, and it is there for exactly
the time when nobody is looking at the board.

## Closing the window is not quitting

The board's window can be closed: the application keeps running, and so do the
agents. An agent sees its work through, sources bring cards in,
notifications arrive. What stays is the icon next to the clock — it says the
application is running, and it brings the window back.

The same window comes back with the same board, not a page loaded from scratch.
On macOS the Dock icon does the same thing.

To quit for real, use **"Quit"** in the icon's menu (or ⌘Q while the window is
open). Quitting stops the agents that are working: if one never said how it
ended, its card stays in its column with the reason written on it.

## The agent's instructions

The "Other" section holds the text an agent is given in a conversation with no
card. Like the rest of the application's instructions it is written in English:
the app is used in more than one language, and this text is read by a model.
Yours can be in any language — like the board's system prompt and the cards
themselves it is passed through as it is, and the agent answers in the language
of what it was given.

## Access for other applications

Another application can take the cards of one column as its own tasks — XXVI
does: a column of a board becomes a source of its inbox. It needs a token for
that.

1. Open the "Other" section and, in **"Access for other applications"**, press
   **"Issue a token"**.
2. The token appears below. It is shown once: only its hash is kept. Press
   **"Copy"** and paste the token into the other application.
3. In the other application, name the board and the column as they are written
   on the board.

An application with the token can only read the cards of a column: the title,
the column, the values of the fields and when the card last changed. It cannot
move or change cards.

- **"New token"** issues the token again; the previous one stops working at
  once.
- **"Revoke"** closes access to everything that had the token.

The cards are served by the app itself, so it has to be running on this
machine. When Pourquoi Pas? is closed, the other application says that Pourquoi Pas? is not
running.

## The settings file

Some agent settings are not in the window: they are changed rarely, and they
are edited by hand in a file. It sits in the install's own directory:

- macOS — `~/Library/Application Support/pourquoipas/acp/config.json`;
- Windows — `%AppData%\pourquoipas\acp\config.json`;
- Linux — `~/.config/pourquoipas/acp/config.json` (or `$XDG_CONFIG_HOME`).

Close the app before editing the file: it reads the file at startup.

| Key | What it sets |
|---|---|
| `maxConcurrent` | how many sessions run at once on this machine (3 by default) |
| `sessionTimeoutMinutes` | how long one turn of an agent may take (15) |
| `testTimeoutMinutes` | how long one test run may take (30) |
| `sessionIdleMinutes` | how long a session waits between turns (30) |
| `autoAllowTools` | which tools an agent may use without asking. A session started by a card has nobody to ask, so anything not on the list is refused for it |
| `worktreeMode` | what to do in a repository on a board that was never asked: `always` — a copy of its own per card, `never` — a branch in the folder itself. The folder's own answer on this board (board menu → "Folders…") wins — see [Folders and branches](../folders.md) |
| `artifactsDir` | where screenshots and reports of test runs are kept |

Columns are not in this file: they belong to the board itself and travel with it
into an export and into a template.

## What is not here

Settings of a specific board — project folders and agent instructions — are
asked from the board's own ⋯ menu, one item per question.
