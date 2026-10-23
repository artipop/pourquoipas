# Folders and branches

An agent always works in some folder on your machine. A card picks one folder
with its «Folder» field.

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

The full list of folders and renaming them are in the board's
menu → **"Folders…"**.

When the board has only one folder, a new card is given it straight away: there
is nothing to choose between, and the field is filled in where you can see it. A
board with several folders leaves the field empty — that choice is yours.

A card shows its folder on its face on the board, as a line with a folder icon
under the title. An empty field draws no line. «The board’s drafts» never appears
there: that is where a card that has not named a folder is worked, not an answer
the card gives.

**The folder cannot be changed once work has started.** As soon as an agent
has started on the card, the field stops being editable and says which folder
the card works in. Changing folders means making a new card: the work stayed
where it began, and moving the field would not move it there.

**Without a folder the agent still runs** — in the board's drafts, in a directory
of the card's own. The directory stays after the run: what the agent wrote is
there. It has no code and no branch, so for a card that needs code, pick a
folder. If the field names a folder that has since been taken off the list, the
agent does not start and the card says why: pick another folder or **"No
folder"**.

"Folders…" is in the menu of every board agents work on.

Every folder in the list is a card of its own: the name, the path under it,
and — for a repository — its two settings.

A folder is one of two kinds:

- **an ordinary folder** — notes, texts, lists. The agent works in it as it
  stands, one card at a time; in the list it carries nothing but its name;
- **a repository** — a directory under git, marked **"repository"**. Here the
  agent works in a copy of the repository of its own, on the card's branch:
  several cards run at once, and your own working copy is left alone.

The kind is not recorded once and for all: the app asks git every time it lists
them. Run `git init` in a folder you added a month ago and it is a repository —
nothing to reconfigure.

A folder is added with one button, **Add a folder…**, and repositories stand in
the same list as plain folders — including where the agent asks "Which folder
will you work in?".

## The card's branch

One card, one branch — however many times an agent runs on it. The branch starts
from the repository's main branch, and the copy is kept in the app's data. When
the work is finished and everything is committed, the copy is removed and the
branch stays: the next run on this card restores the copy from it. A copy with
uncommitted changes is never touched.

## Where the agent works

An agent works on the card's own branch. A conversation with no card («Talk it over with an agent») stands in the folder too and
changes nothing in it.
