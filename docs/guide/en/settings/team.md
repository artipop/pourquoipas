# Your own server

The app on your computer belongs to one person: it never asks who you are, and
whoever opens the board sees everything. A server is **a separate install**,
reached from a browser and from the phone: there are always accounts there,
everybody logs in under their own name and works with their own boards, and
the first person to register becomes the administrator. Which of the two you
get is decided by the build you installed.

The app offers no shared work on one board: a board has one owner.

## What a server gives you

- boards open in a browser and on the phone, under your own name and password;
- the agents on a server are models by API key: everybody has their own key and
  their own agents, and a server has no folders and runs no programs, see [AI
  and models by API key](./ai.md);
- the comments agents leave on cards are shown.

## The first login

1. Install the server build and open its address in a browser.
2. While nobody is on the server, it opens on a registration page. Choose a name
   and a password — that account becomes the administrator of the install.
3. After that the server opens on a login page, and registering is by invitation
   only.

## Inviting the rest

**Settings → «People»** — the section is shown to the administrator alone. In it:

- **«Send this to whoever is joining»** — a link of the form
  `…/register?t=…`. **«Copy»** puts it on the clipboard; pass it on any
  way you like. They open the link, choose a name and a password, and make
  their own boards.
- **«New link»** issues another one and retires the previous. That is how a
  link sent to the wrong person is taken back.

Without such a link nobody can register.

A new person starts with an empty list of boards: they do not see anybody
else's.

## The list of people

Below the link are all the accounts there are. Under each name it says what kind
of account it is: **administrator**, **agent** or **source**. The app makes
the agent and source accounts itself — a card names its agent by being assigned
to it, and that name needs an account.

For everybody but yourself there are two buttons:

- **«Make an administrator»** and **«Take away administration»** — who else
  runs the install;
- **«Switch off»** and **«Switch on»** — somebody switched off cannot log in, and
  their open sessions end immediately.

There are no buttons against your own account. And the last administrator can
neither step down nor be switched off: an install nobody can administer cannot
be repaired from inside the app.

## Connecting your own app to the server

You can work the server's board from the app on your own computer as well as
from a browser — and then the cards live on the server while the agents, the
and the folders stay on your machine.

1. **Settings → «Server»**.
2. Type the address (a bare `board.example.com` will do), your name and your
   password.
3. **«Connect»**, then **restart the app**: which board it works is decided
   when it starts.

The password is not kept — what is stored is the session it buys. **«Work this
machine's own board»** disconnects the app from the server; everything made on
the server stays there.

## Switching between your own board and the server's

Once connected, a switcher appears in the sidebar above the list of boards, with
two entries: **"This machine"** and the server's address.

It decides only **what the window shows**. The agents work both boards: an
agent running on the server's card keeps going while you look at your own. The page
reloads when you switch — everything open on it was fetched against the board it
came from.

## A forgotten password

A password is changed by whoever is logged in, on the change-password page. If
logging in is no longer possible, the administrator switches that account off
and invites the person again with a new link. The app knows no email addresses
and sends no mail.
