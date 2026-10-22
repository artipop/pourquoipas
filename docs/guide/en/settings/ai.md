# AI and models by API key

An agent in Pourquoi Pas? is one of two kinds:

- **A CLI on this machine** — Claude, Codex and other ACP agents. The app
  starts their program, and it works in the card's folder. How to add them is
  in [Agents, npx and Node.js](./agents.md).
- **«Model by API key»** — a model the app calls itself, with your API key.
  Nothing has to be installed; only the key is needed.

Both are added in the same window and assigned to a card the same way, with
the «Who does it» field. Questions, the comment, moving the card and the
notifications are the same too.

## The AI section of the settings

**Settings → «AI»**. At the top is the switch **«Agents work on the cards»**:
it turns everything about agents, of both kinds, on and off. What disappears
when it is off is in [Settings](./index.md#ai-agents).

Below it, under **«Model keys»**, are the providers:

- **Anthropic (Claude)**;
- **OpenAI** — and any service with the same API: OpenRouter, a local model.
  For those, fill in the address, for example `https://openrouter.ai/api/v1`.

Each provider says **«Key is set»** or **«No key»**.

## Adding a key

1. Settings → «AI».
2. Paste the key into **«API key»** in the provider's row. Leave the address
   empty when you call the provider itself.
3. **«Check and save»**. The app asks the provider for its models with this
   key. If the provider accepts it, «The provider accepted the key.» appears;
   if not, its answer does, and the key is not stored.

The key is kept in the system keychain (on a server, in a file only the app
can read) and is never shown again: the field is always empty. To change the
address and keep the key, leave the key field empty and press «Check and
save». **«Remove the key»** removes the key and the address.

A key given in the environment the app was started in
(`PP_AI_ANTHROPIC`, `PP_AI_OPENAI`, the address in `PP_AI_OPENAI_URL`) is
changed there, and the row says so instead of showing fields.

## Adding a model agent

1. The board's ⋯ menu → **«Agents…»** → **«Add agent…»**.
2. **«Kind»** — **«Model by API key»**.
3. **«Provider»** — Anthropic (Claude) or OpenAI.
4. **«Model»** — the model's name. With a key set, the field suggests the
   models the provider opens to that key. For Anthropic the default is
   `claude-opus-5-5`.
5. Under «Agent settings» you can choose **effort** — how hard the model
   thinks, from `low` to `max`. Left unchosen, it is the model's default.
6. The agent's prompt and MCP servers are as for any agent. A model's MCP
   server may be remote: `{"docs": {"url": "https://…/mcp"}}`.
7. **«Save»**.

When the provider has no key yet, the form says so. The agent can be saved
anyway and works once the key is there. A card assigned to a model with no key
does not start and says on itself «There is no API key for …: add one in
Settings → AI».

## What a model can do

**On a computer** a model works in the card's folder the way a CLI does: in
the same working copy and on the same branch. It reads files, searches them,
writes and edits them, and runs commands. A command that is not on the
agent's allowed list arrives as a question on the card — «Run the command:
…?» — with the answers «Allow», «Allow always» and «Refuse». A card with no
folder is worked in the card's drafts.

**On a server** a model has no files and no commands: its work is what it
writes on the card. The comment, field values, the move to a column, new
cards.

Either way the model may ask you something in the middle of its work — the
question appears on the card, in a notification and on the phone, as any
agent's does.

## Whose key

The key is yours. On a computer there is one per install. On a server every
registered person has their own key and their own agents: you do not see
anybody else's agents, and your agent runs on your key.
