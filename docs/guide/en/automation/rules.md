# Rules: conditions on arrows

Usually an event is one arrow: the step passed, the card moved. Conditions are
for when that is not enough. There are three kinds, by who makes the decision:
the board, the agent or a person.

## By a card property

Example: "The step passed → to Deploy, but only if Priority = High; the rest —
to Review".

Under the arrow in the stage panel there is **"+ branch on a condition"**: a
second arrow appears on the same event. The condition is a board property and
its value, both picked from lists. The first arrow whose condition holds fires;
an arrow without a condition is the fallback. The condition is written on the
arrow itself ("if Priority = High").

If the event arrives, no condition matches and there is no fallback arrow, the
card stays put, and the reason is written in amber on the route strip. So add
the fallback arrow right away.

## By what the agent wrote

Example: "The step passed, and the agent's answer contains 'READY TO DEPLOY' →
to Deploy".

The condition "only if the agent wrote…" checks the agent's closing words —
what it said when it finished the stage. Ask
the agent in the task text to end its report with an agreed phrase — and the
branch with this condition fires only on its decision. The condition is
available only on outcomes of steps where an agent worked.

## By a mark on the card

A stage can wait for a mark on the card instead of a repository event: a person
sets "Approved = Yes" — and the card moves on immediately, without polling. In
the arrow's event list this is "the card was ticked"; which option — is its
condition (required).

The stage reacts only to the named property: ticking "Priority" will not fire
an arrow waiting for "Approved". If a session was running on the card at that
moment, it is cancelled — the person takes priority.

A card on such a stage shows in its route strip what it is waiting for,
condition included.

## A question before the card moves on

Some decisions are neither the board's nor the agent's but yours, and they are
made once, on one card: how the agent is to work with the folder here. An arrow
can ask about that before it lets the card through.

The question is written on the arrow — the transition panel has **"Ask before
the card moves on"**. Pick what to ask about:

- **which folder the agent works in** — the board's own folders. The answer is
  written into the card's folder field, as if you had picked it by hand.
- **how the agent works with the folder** — a copy of the folder per card, or a
  branch in the folder itself. The answer goes into the card's workspace: it is
  what the copy or the branch is made from, and every later stage — the review,
  the deploy — takes its changes from there.
- **any board property with a set of values** — the answer is written into that
  property of the card. From there it is read by later stages and can be
  branched on like any other value.

The wording can be your own: the field under the question shows what will be
written on the card if you type nothing.

Until somebody answers, the card stays where it was and nothing starts. The
question is on its route strip — with the name of the stage the card is going
to, and the answers as buttons; "Ждут" and the notification say so too. An
answer is one click; several questions are asked one at a time, in order, and
each is applied as it is given — until a folder is chosen there is nothing for
"how should it be worked" to be about, which is why the way into work asks the
folder first and the second question only then.

The question is about this arrow, not about the stage. The arrow "Not started →
In progress" asks; the way back, "In review → In progress", does not — there it
is all settled already.

### The way onto the route

A route's first stage has no incoming arrow you can see: the card arrives there
from "Not started", from a source, or simply by hand — from somewhere the route
does not pass through. That way in exists all the same, and it is a transition
too: it can ask.

In the «Development» template the way onto each route already asks how the agent
is to work with the folder. So a card you drag from "Not started" into "In progress"
asks first and starts work afterwards, in the workspace you chose.

It is asked once. A card coming back into "In progress" from a review arrived along
an ordinary arrow of the route, and is asked nothing.

The canvas does not draw the way in yet, so the question on it cannot be changed
in the editor. Editing the route does not erase it either.

If there is nothing to answer — the property was deleted from the board, or the
chosen folder is not a repository — that question is skipped and the card moves
on. The
route carried cards before anybody wrote a question on the arrow, and a
question nobody can answer must not be what stops it.
