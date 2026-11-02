---
title: "Computer and server: what works where"
---

Pourquoi Pas? is installed in one of two ways:

- **on a computer** — an app with a window. One person works in it, and there
  is no logging in;
- **on a server** — opened in a browser. Everybody has their own login and
  their own boards, and the first person to register is the administrator
  ([Your own server](./settings/team.md)).

The app on a computer can also be **attached to a server**: the boards are on
the server then, and the agents and folders stay on the computer. A phone opens
the server.

There is no shared work on one board: a board has one owner.

## Agents

| | Computer | Server in a browser |
|---|---|---|
| An agent that is a program (Claude, Codex and others over ACP) | yes | no |
| A model by API key (Anthropic, OpenAI and compatible) | yes | yes |
| Folders and the «Folder» field | yes | no |
| Files and commands for a model | in the card's copy | no: the model writes its result on the card |
| «Talk it over with an agent» | yes | yes |
| Inbox sorting by an agent | yes | yes |
| Proxies for agents | yes | no |

Agents are connected in **«Settings → AI»** ([Connecting
agents](./settings/ai.md)); in a browser on a server the form offers models by
key only. Keys and agents are yours: on a server other people do not see them,
and any agent of yours can be assigned on any board of yours.

A computer attached to a server works the server's cards with its own agents:
the programs, models and folders of that computer.

## Agent questions and notifications

| | Computer | Server in a browser | Phone |
|---|---|---|---|
| The question on the card, the amber badge | yes | yes | — |
| Notifications in the board window | yes | yes | — |
| System notifications, the icon next to the clock | yes | no | — |
| The «Waiting» screen | — | — | yes |

An answer given anywhere closes the question everywhere.

## Everything else

| | Computer | Server in a browser |
|---|---|---|
| Logging in with a name and a password | no | yes |
| «Share» | no | yes |
| «Inbox» | your own | one for everybody logged in |
| Inbox sources | yes | yes |
| Sharing from the system menu (macOS) | yes | no |
| «Settings → Updates» | yes | no |
| «Settings → Server» (attaching) | yes | no |
| «Settings → People» | no | the administrator alone |

Theme, language and the last open board are yours: on a server everybody has
their own, and they follow you to another device. The «Agents work on the
cards» switch is one per install: on a computer you change it, on a server the
administrator does.

There is no deploying in either.
