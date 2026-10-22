# Moving a board, and backups

A board is exported and imported as one `.boardarchive` file. The same file is
the backup.

- **one board** — the board's ⋯ menu → "Export board archive";
- **all at once** — "Settings" → "Import and export" → "Export";
- **loading** — "Import", in the same place. Boards from an archive are added
  to the existing ones, nothing is replaced; the same archive imported twice
  gives two boards.

Below in the same section — **Trello, Notion, Todoist**. These are not
migration buttons: the export is done on the service's side and turned into an
archive, which is then imported here; each card opens the instructions.

## What moves with a board

Everything configured on the board itself: its columns; the agent instruction;
the setup wizard's questions; cards, attachments, members and views. The inbox
is this machine's board; on the new machine the app creates one.

A board opened on the new machine looks and behaves the same.

## What needs to be set up again

Everything that belongs to the machine rather than the board: folder paths,
SSH keys, tokens and environment variables are not valid on another computer.

| What | Where to set it up after the move |
|---|---|
| Agents — model, environment, MCP servers, proxies | the board's ⋯ menu → [Agents…](./settings/agents.md) |
| Proxies | the same window as the agents, under the list |
| Project folders | the board's ⋯ menu → "Folders…" |
| Light or dark, language | "Settings" → "The app itself" |
| Sources — the token is issued anew | [Sources…](./inbox/sources.md) in the inbox board's menu |

The setup wizard asks only for what is missing; it can be walked again any
time: ⋯ → «Walk the setup again…».

Agent names are remembered: the assignment field travels with the card, so
registering an agent under the same name is enough. A column that refers to an
unregistered agent is kept and waits for the registration.

## What does not move

- **Sources and their rules** — set up again. There will be no duplicates:
  cards remember what they were created from, and a reconnected source will
  not create them a second time.
- **The project name.** You point at the folder yourself; name the project as
  before — cards refer to it by name.
- **Run history.** An agent's sessions belong to this
  machine and do not move; on the new machine a card starts a fresh one.
