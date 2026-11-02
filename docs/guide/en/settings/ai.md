---
title: "Connecting agents"
---

Agents are connected in one place — **"Settings" → "AI"** — and they are yours,
not a board's. Any of your agents can be made the assignee of a card on any
board: it is in the assignee field ("Assignee", "Who does it" and the like)
next to the people. There is nothing to add to a board.

An agent is one of two kinds:

- **"Program on this computer"** — Claude Code, Codex, GitHub Copilot,
  JetBrains Junie, Antigravity or another ACP agent. The app starts the
  program; it works under your own account with its provider and in the card's
  folder. Only on a computer.
- **"Model by API key"** — a model from Anthropic, OpenAI or any service that
  speaks the OpenAI API. The app calls it itself, on your key. Nothing to
  install. On a computer and on a server alike.

Both are assigned, started and asked the same way: questions, the comment,
moving the card and notifications are the same for both.

## The "AI" section

At the top is the **"Agents work on the cards"** switch. It turns everything
about agents on and off. Off, it takes the agents out of the assignee field and
their fields off the cards; the agents connected are kept. What else goes is in
[Settings](./index.md#ai-agents).

Under it is **"Notify me when an agent is waiting"**: a notification with the
agent's question, answered right in it (see [Agent
notifications](./index.md#agent-notifications)).

Below that is **"Agents"**: the list of your agents. A row says what the agent
is: "Claude · program on this computer" or "Anthropic (Claude) ·
claude-opus-5-5 · Model by API key". Each has **"Edit"** and **"Remove"**.

Further down, on a computer only, are **"Proxy configurations"**: network
settings a program agent can refer to by name.

## Connecting a program

1. **"Connect an agent…"**.
2. **"Program on this computer"**.
3. **"Program"** — Claude, Codex, Antigravity, GitHub Copilot, JetBrains Junie
   or "ACP (other)". For "ACP (other)" fill in **"Command that starts it over
   ACP"**, for instance `gemini --acp`.
4. If the program cannot be started, the line under the field says why, with
   **"Install adapter"** beside it — see [Program agents, npx and
   Node.js](./agents.md).
5. **"Name"** is filled in with the program's name; it is the name the agent is
   picked by on a card, and can be changed before connecting.
6. **"Connect"**.

## Connecting a model by key

1. **"Connect an agent…"**.
2. **"Model by API key"**. On a server there is nothing to choose: it is the
   only kind there.
3. **"Provider"** — Anthropic (Claude) or OpenAI. OpenAI also stands for any
   service with the same API: OpenRouter, a local model; for those fill in the
   **"Address (optional)"**, for instance `https://openrouter.ai/api/v1`.
4. **"API key"** — paste the key. When you already have a key for this
   provider there is no field: it says **"Key is set"** and the key's last
   characters instead.
5. **"Model"** is filled in with the default; the field suggests the models the
   provider opens to your key.
6. **"Name"** — "Claude" or "OpenAI", can be changed.
7. **"Connect"**. While the key is checked the button says "Checking the key…":
   the app asks the provider for its list of models. If the provider refuses the
   key, its answer is under the form, and neither the key nor the agent is
   saved.

The key is the provider's, not the agent's: every agent of yours on that
provider runs on it, and a second agent on the same provider does not ask for
it. To change it — **"Edit"** on any such agent → **"Replace the key"**;
**"Remove the key"** removes the key and the address, and that provider's
agents do not start until a key is there again.

The key is kept in the system keychain (on a server, in a file only the app may
read) and is never shown again. A key set in the environment the app was
started in (`PP_AI_ANTHROPIC`, `PP_AI_OPENAI`, the address `PP_AI_OPENAI_URL`)
is changed there, and the form says so.

A card assigned to a model with no key does not start and says on it "There is
no API key for …: add one in Settings → AI".

## "More settings"

Everything under **"More settings"** has a working default, and none of it is
needed for a first card.

- **"Agent system prompt"** — what the agent hears at the start of every
  session, after the board's ([The board's system prompt](../automation/prompt.md)).
- **"MCP servers"** — the JSON any MCP client takes. For a model, and on a
  server, by address only: `{"docs": {"url": "https://…/mcp"}}`.
- A program also has **"Model (optional)"**, **"Binary path"**, **"Launch
  command (argv)"**, and **"Agent settings"** — what the program reports about
  itself (Fast mode or effort, say); to learn them the app starts the program
  when you open "More settings". Then **"Proxy configuration"**,
  **"Environment"** and **"Extra CLI args"**; Claude also has **"The CLI behind
  the adapter"** with Remote control.

## What a model can do

**On a computer** a model works in the card's folder, like a program: in the
same working copy and on the same branch. It reads files, searches them, writes
and edits them and runs commands. A command not on the allowed list arrives as
a question on the card — "Run the command: …?" — with "Allow", "Allow always"
and "Refuse". With no folder on the card the model works in the card's drafts.

**On a server** a model has no files and no commands: its work is what it
writes on the card. The comment, field values, the move to a column, new cards.

## Whose agents and keys

Yours. On a computer, one set per install. On a server everyone signed in has
their own agents and keys: you see nobody else's agents, in the list or in the
assignee field, and your agent runs on your key.
