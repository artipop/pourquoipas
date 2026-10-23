# Templates: a board that has not been made yet

A template is where a board comes from, and what matters on it is rarely the
cards. It is the columns the board arrives with, the fields a card names its
folder and its branch in, what the board tells its agents, and what the app has
to be told about **this machine** before any of it is useful.

The last of those is not visible on a board, so a template is not edited by
opening it and moving cards about. *«Шаблон»* is the window that shows it,
reached from:

- the template picker — the pencil on any template of yours;
- the banner on a template you already have open — *«Колонки и настройка…»*;
- the ⋯ menu of an ordinary board — *«Сохранить как шаблон…»*, which copies the
  board, under the board's own name, and opens the copy. **That door is parked**
  (`OFFER_SAVE_AS_TEMPLATE` in `viewHeaderActionsMenu.tsx`): everything behind
  it works and is tested, it simply has no reason to stand in the menu of every
  board, where making a template is the rarest thing anybody does and it sat one
  slot from the things a board is set up with.

**The columns are shown there and not edited there.** A template is made by
building a board and saving it, so what it carries is what already worked; the
way to change it is to change the board and save it as a template again. That
window once held a whole route canvas inside a scrolling dialog, which put a
graph editor between somebody and the two fields they came to fill in — and
gave one set of routes two places to be edited, the second of them a copy
nobody was looking at.

## What a template carries

| | Where it comes from | What it does when a board is made |
|---|---|---|
| **Columns** | the board it was saved from | the new board's kanban is grouped the same way, with the same column names — which is what an agent reads |
| **The board's prompt** | *«Системный промпт доски…»* on that board | every agent working the new board is told it first |

They live in the template board's own properties, so they travel with it and
belong to the new board from the moment it is made: editing them there does not
touch the template, and editing the template does not reach back into boards
already made.

**`projectProperty`** is the id of the card
property that holds the projects. Making a board from a template duplicates it
without renumbering the card properties, so the id the template writes is the id
the new board has, and nothing has to recognise the field by what it is called —
the person who owns the board may rename it to anything. A board that has not
got the key has not got the field; the first project registered on it makes one
and writes the id down. Adding a *new* property to a template is still
hand-editing JSONL, which is what «[Свойства в шаблонах](deferred.md)» in the deferred work
is about.

## No questions

A board made from a template asks nothing: it opens with its columns, its
prompt and its «С чего начать» card. Templates used to declare the steps of a
setup wizard (`setup`); the wizard was removed, and a `setup` a template still
carries is ignored. Board names are unique (`boardTitle.ts`), and a second
board made from «Разработка» is renamed on the board itself.

## Saving a working board as a template

Everything a template carries is on the board already, so the copy is an
ordinary duplicate; *«Сохранить как шаблон…»* adds two things Focalboard's own
duplicate loses — the board's own name, and the questions it declares. What it
deliberately does not carry is the machine: the agents it named live in *«Настройки → Эта машина…»* and are this install's, the folders
belong to the board they were added on, and a board made from the template asks
for them again.

A copy also carries **the marker** (`template`), and that one is not the
copy's to keep. A board made from «Разработка» carries the marker, a copy takes
the board's properties with it, and the copy then claims to *be* «Разработка»:
three templates of that name stood in the picker, and the importer maintained
whichever came last while the rest stayed for ever. Both doors do it — this one
and *«Новый шаблон из доски»* in the board's own ⋯ menu, which is a plain
duplicate and does not come through here at all.

So it is taken off where it can be taken off from either: at the next launch,
by the importer, which owns the marker (`disownTemplate`). Disowned rather than
removed — the copy is somebody's own board, and all that was wrong with it is a
word it inherited; without the marker it is listed among your own templates,
where it can be deleted.

## What is offered in the picker

The templates the install ships — Development, Content and Personal, which a
Russian install reads as «Разработка», «Контент» and «Личное» — and every
template you have made. Which is which is read off
**who created it** and not off its version stamp: the importer files its own
under the system user, while a version is a thing a template can inherit — a
board carries the version of the template it came from, and a template saved
from that board carried it too, so a saved copy counted as shipped and appeared
in neither list. The rest of Focalboard's
own defaults are hidden: they carry none of the fields a card names a folder or
a branch in, so a board made from one has nothing to set up.

**They are read in each person's language.** The files are written once, in
English, installed as they are, and translated where they are shown, from a
catalogue of words beside them (`internal/boardadapter/templates/i18n/*.json`,
`templatetext.go`, `appWords.ts`, and [i18n.md](i18n.md) for how to add one):
the catalogue is keyed by the very string the file carries, so a template is
one board rather than a copy per language, and a string nobody has translated
stays as the file wrote it. On a server two people see the same template in
two languages. A board made from one is translated once, as it is copied, into
the language of the person who made it, and is theirs from then on: switching
the language changes nothing on it. The templates the install ships cannot be
edited — they are replaced on an upgrade — and a template of your own starts as
a board made from one. Nothing is exempt from the
translation: the three strings that used to be («Исход» and its two values) went
with the field the engine wrote them into.

No template carries an inbox: everything a source brings lands on the one
inbox board the app makes itself (`EnsureInboxBoard`), not on a board made from
a template.

Content is the smallest of them: its columns are a brief, a draft and
proofreading, and it publishes nowhere.
