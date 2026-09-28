# Folders and branches

An agent always works in some folder on your machine. A card picks one folder
with its «Folder» field — one, because a card has one working copy and one
branch.

## The folder on the card

A folder is chosen on the card itself: click the value of the «Folder» field. The
list has this board's folders, **"Add a folder…"** under them and, when a folder
is already chosen, **"No folder"**.

**"Add a folder…"** opens a folder picker. The folder you pick is added to the
board under its own name and put on the card at once. From then on it is in the
list on every card of the board — there is no need to add it again. If the
folder is already added on another board, the app does not add it twice but
makes it shared. If the name is already taken by another folder, a number is
added to it: `app 2`.

On a board that has no «Folder» field yet, the card still has a «Folder» row
reading **"Not chosen"**; the first folder added makes the field.

The full list of folders, renaming them and their settings are in the board's
menu → **"Folders…"**.

When the board has only one folder, a new card is given it straight away: there
is nothing to choose between, and the field is filled in where you can see it. A
board with several folders leaves the field empty — that choice is yours. The
switch is in Settings → "Other".

A card shows its folder on its face on the board, as a line with a folder icon
under the title. An empty field draws no line. «The board’s drafts» never appears
there: that is where a card that has not named a folder is worked, not an answer
the card gives.

**The folder cannot be changed once work has started.** As soon as a working
copy or a branch has been taken for the card, the field stops being editable and
says which folder the card works in. Changing folders means making a new card:
the work stayed in the copy it was done in, and moving the field would not move
it there. This holds on another machine too: the card carries its branch in the
«Branch» field, so the work is known about there even when the copy itself is
not.

**Without a folder the agent still runs** — in the board's drafts, in a directory
of the card's own. The directory stays after the run: what the agent wrote is
there. It has no code and no branch, so for a card that needs code, pick a
folder. If the field names a folder that has since been taken off the list, the
agent does not start and the card says why: pick another folder or **"No
folder"**.

"Folders…" is in the menu of every board agents work on; "Where to deploy…" is
on a board that asked for it.

Every folder in the list is a card of its own: the name, the path under it,
and — for a repository — its two settings.

A folder is one of two kinds:

- **an ordinary folder** — notes, texts, lists. The agent works in it as it
  stands, one card at a time; in the list it carries nothing but its name;
- **a repository** — a directory under git, marked **"repository"**. Here every
  card gets a branch of its own, and the card shows it in its «Branch» field.

The kind is not recorded once and for all: the app asks git every time it lists
them. Run `git init` in a folder you added a month ago and it is a repository —
nothing to reconfigure.

A folder is added with one button, **Add a folder…**, and repositories stand in
the same list as plain folders — including where the agent asks "Which folder
will you work in?".

## The base branch

A repository has a **"Work branches from"** field. Work on a card starts from
that branch, and a finished branch is merged back into it.

The value is filled in when the folder is added: the app asks git which branch
the repository treats as its main one. Change it if you work off another —
`develop`, say.

## How an agent works in a repository

This is a choice about **that folder on this board** — the two buttons under
**"The agent works"** on the repository's card in "Folders…". A folder belongs to one board anyway, so for it
this reads as "how this folder works"; a folder marked «On every board» can be
used differently by two boards.

**"in a copy of its own"** (the default) — the card gets its own copy of the
repository and its own branch in it.

- several cards of one repository go at once;
- your own working directory is left alone: the agent never switches branches
  under you;
- the copy lives in the app's data and is not underfoot.

**"in the folder itself"** — the card's branch is made right in your folder.

- you see the agent's work in your editor as it happens, without going to
  another directory;
- one card at a time: until the first card's branch is merged into the main
  one (locally or on the server, through a pull request) or deleted, the next
  one waits and says on the card what it is waiting for. It does not start by
  itself afterwards — once the branch is merged, move it into the column again;
- if the folder has **uncommitted changes to tracked files**, the agent will not
  start: the app will not switch a branch under your unsaved work. Commit or
  stash it. Untracked files — a build directory, an `.env`, a scratch clone —
  do not get in the way: they survive a branch switch untouched.

There is no third answer for a repository: "work on whatever is checked out" is
what an ordinary folder already does.

A card that has already started stays where it started: its work is in that copy
or on that branch. The new answer applies to the cards that come after.

## The card's branch

One card, one branch — however many times an agent runs on it. The branch reads
like the task: a Russian title is transliterated, so «Почини логин» becomes
`pochini-login-1a2b`.

For names by meaning rather than by spelling — Settings → This machine → Other →
**"The agent names each card's branch"**: before the card's first branch the
agent runs briefly and answers with one name ("fix-sso-login"); a slow or odd
answer falls back to the card's title.

The branch is written into the card's «Branch» field, so it travels with the card
to another board and is visible on another machine. The copy's directory is not
written to the card — a path means nothing on another machine; it is shown on
the card's stamp.

When the work is finished and everything is committed, the copy's directory is
put away and the branch stays: the next run on that card remakes the copy from
the branch. A copy with uncommitted changes — or one a CLI is still running in —
is left exactly where it is.

## Where the agent works

An agent works on the card's own branch; a deploy stands in the folder itself,
because it publishes a branch that already exists and needs no copy of its own.
A conversation with no card («Talk it over with an agent») stands in the folder too and
changes nothing in it.
