# Your own server

The app on your computer belongs to one person: it never asks who you are, and
whoever opens the board sees everything. Working as a team is **a separate
install, on a server**: there are always accounts there, everybody logs in under
their own name, and the first person to register becomes the administrator.

There is no switch in the settings any more. An install used to be moved into
team mode and back; now it is decided by which build you installed, and no
restart changes it.

## What a server gives you

- everybody who opens the board logs in under their own name and password;
- cards are assigned to people by name, in the same field agents are assigned in;
- comments appear on cards: there is somebody to address them to;
- a waiting agent gets an addressee — whoever the card is assigned to.

This says **who** may open the board, not where it is visible from. A second
device still comes in through the tailnet («Доступ с телефона»), and the two are
unrelated.

## The first login

1. Install the server build and open its address in a browser.
2. While nobody is on the server, it opens on a registration page. Choose a name
   and a password — that account becomes the administrator of the install.
3. After that the server opens on a login page, and registering is by invitation
   only.

## Inviting the rest

**Настройки → «Люди»** — the section is shown to the administrator alone. In it:

- **«Отправьте это тому, кто присоединяется»** — a link of the form
  `…/register?t=…`. **«Скопировать»** puts it on the clipboard; pass it on any
  way you like. They open the link, choose a name and a password, and land on
  the boards you add them to.
- **«Новая ссылка»** issues another one and retires the previous. That is how a
  link sent to the wrong person is taken back.

Without such a link nobody can register.

A new person sees no boards at all by default: access to a board is membership
in it, and membership is granted on the board itself.

## The list of people

Below the link are all the accounts there are. Under each name it says what kind
of account it is: **администратор**, **агент** or **источник**. The app makes
the agent and source accounts itself — a card names its agent by being assigned
to it, and that name needs an account.

For everybody but yourself there are two buttons:

- **«Сделать администратором»** and **«Снять администрирование»** — who else
  runs the install;
- **«Выключить»** and **«Включить»** — somebody switched off cannot log in, and
  their open sessions end immediately.

There are no buttons against your own account. And the last administrator can
neither step down nor be switched off: an install nobody can administer cannot
be repaired from inside the app.

## Connecting your own app to the server

You can work the server's board from the app on your own computer as well as
from a browser — and then the cards live on the server while the agents, the
terminals and the folders stay on your machine.

1. **Настройки → «Сервер»**.
2. Type the address (a bare `board.example.com` will do), your name and your
   password.
3. **«Подключиться»**, then **restart the app**: which board it works is decided
   when it starts.

The password is not kept — what is stored is the session it buys. **«Работать
доской этой машины»** disconnects the app from the server; everything made on
the server stays there.

This is not the same as «Доступ с телефона»: the tailnet answers where the board
can be reached *from*, and «Сервер» answers *whose* it is.

## Switching between your own board and the server's

Once connected, a switcher appears in the sidebar above the list of boards, with
two entries: **«Эта машина»** and the server's address.

It decides only **what the window shows**. The agents work both boards: a stage
running on the server's card keeps going while you look at your own. The page
reloads when you switch — everything open on it was fetched against the board it
came from.

## A forgotten password

A password is changed by whoever is logged in, on the change-password page. If
logging in is no longer possible, the administrator switches that account off
and invites the person again with a new link. The app knows no email addresses
and sends no mail.
