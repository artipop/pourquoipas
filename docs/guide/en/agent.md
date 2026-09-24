# The agent on a card

This page covers how an agent works a card: who takes it, how to answer its
questions and what stays on the card.

## Who gets a card

A card is worked by whoever it is assigned to. The assignment field is an
ordinary board field of the "person" type; templates name it differently:
«Исполнитель» on the «Разработка» board, «Кто делает» on «Личное», «Кто ведёт»
on «Контент».

It has to be filled in by hand: the app assigns nobody by itself. A card
assigned to a person, or to nobody, is left alone — it simply stands where it
was put.

To make an agent appear in the assignment list, register it: the board's ⋯ menu
→ [Agents…](./settings/agents.md). After registration it becomes a board member
under its own name: open the field and pick it.

**The list holds every agent on the machine.** A board keeps no crew of its own:
any registered agent can be assigned on any board. The only thing that takes
them out of the list is the «Агенты работают с карточками» switch in
[the settings](./settings/index.md).

The agent reads its task from the **card's description** — there is no separate
task field. The board's instruction to all its agents is added in front of it
(["The board's system prompt"](./automation/prompt.md)), and so is the agent's
own prompt from the registry.

## What starts the work

Moving the card into another column. All that has to be true:

- an agent is assigned to the card;
- agents are switched on in the settings.

Nothing else is configured — columns have no actions and no crews. The agent is
given the name of the column the card was put into and the names of the board's
other columns; what to do, it reads from the card.

If the card is dragged out of the column while the agent is working, the work is
cancelled: a move is a new instruction, not a continuation of the old one.

## What the agent does at the end

1. writes **one comment** with the result;
2. puts values **into the card's fields**, if the task called for it;
3. **moves the card itself** into the column whose name says what has happened:
   «Готово», «На ревью», «Заблокировано». If there is no fitting column, the
   card stays where it is;
4. says the **work is finished**, which is what ends the session.

A move the agent makes itself starts no new work.

If the agent finishes without writing anything, the app writes the comment —
something always stays on the card. If the work was cut off (the app was closed,
the CLI died), the card keeps its place and the reason is written on it.

## Talking to an agent before there is a card

A task can be talked through before it becomes a card: the arrow beside
«Создать» in the board header → **«Обсудить с агентом…»**.

First it asks **which agent** and **which folder** — the board's folders, «черновики
доски» and «Добавить папку…». After that it is an ordinary conversation: you
write, the agent answers, «Остановить» interrupts it.

In such a conversation the agent changes nothing in the folder — its
instructions and the tool policy hold it to reading — and the cards you agree on
it creates on this board itself.

At the top of the dialog are the **«Открытые разговоры»**: a conversation
outlives its window. **«Завершить разговор»** stops the agent and takes the
conversation off the list.

## The agent's questions

An agent asks through the protocol: permission for a tool it was not given, or a
choice that cannot be made for it. The question appears in three places, and any
of them answers it:

- **on the card** — an amber badge in its bottom-right corner, and the question
  itself with its buttons once the card is open;
- **in the notification** in the corner of the board window;
- **on the phone**, under «Ждут».

Whichever answer arrives first counts; the others stop showing the question.

A notification can be dismissed with «Скрыть» — that means "I know", not
"answered": the question stays on the card until somebody answers it. A
dismissed notification does not come back in another window, on the phone, or
after a reload.

If the app is minimised the notification comes from the system, and an amber dot
appears on the icon beside the clock.

Notifications are switched off in
[the settings](./settings/index.md#уведомления-агента). The badge on the card
and the dot beside the clock stay either way — a notification is about the
question, they are about the agent still waiting.

## The changes, read by eye

The card's toolbar has a **«Изменения»** button. It appears on a card that has
something to show: with no work started, or nothing changed, there is no button.

It opens the diff on the right — what has been done on this card:

- **«В ветке»** — what is already committed on top of the main branch;
- **«Не закоммичено»** — what is changed but not committed, and new files git
  does not track yet.

At the top: the branch name, what it was cut from, and how many files and lines
in total. Each file folds away when its row is clicked. The arrows button
(«Перечитать папку») reads everything again; the diff itself refreshes when the
agent finishes something.

It can be read after the work is over, too. When an agent finishes, the card's
working copy is folded away and the branch stays — so the committed half is read
from the branch even in a column where the copy is gone. The uncommitted half
lives only while the copy does; a folder with unsaved work is never folded away.

The panel beside the card is narrow, so its header has **«Открыть в отдельном
окне»** — the same thing full screen.

Any card can be looked at, but there is nothing to show until somebody has
worked it. The panel says so: the work has not started, or it started on another
machine — a working copy lives on the machine the agent worked on and does not
travel with the card.

## The history: who did what to the card

The card's toolbar has a **«История»** button. It opens a list of what happened
to the card on the right, newest first:

- who made it;
- who renamed it, and to what;
- who changed a field and how: “Статус”: В работе → Готово;
- who wrote a comment — and what it said.

The names are real. Every registered agent has a board account of its own, and
its writes are signed by it rather than by you: the history shows that the agent
moved the card and that you set the date.

Every card has one, even one nobody has touched: being made is an entry too. It
lives on the board itself and travels with it into an export and onto another
machine.

## What is visible on the card

The branch and the directory are on the line under the card's title; the agent's
question is under it.

Comments appear once the install is in [team mode](./settings/team.md): a
**«Комментарии»** button joins the card's toolbar, it opens the panel on the
right, and the card's face shows their count. While one person works the board
the comments are written but not drawn: everything worth knowing is either on
the card or in the diff.

If the work could not start — no folder, or another card is holding the folder —
the reason is shown on the card and goes as soon as something happens on it
again.
