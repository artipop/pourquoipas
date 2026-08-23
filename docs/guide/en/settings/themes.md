# Themes

The theme is picked in "Settings → The app". There are two fields there, because
there are two questions.

**"Theme"** — which design. A dropdown; the choice applies at once.

**"Light or dark"** — which side it is on: "Light", "Dark" or "As the system
is". Every theme has both halves, so you can take Gruvbox and leave "As the
system is": the board will lighten and darken with the system without ceasing
to be Gruvbox.

## What is in the list

Twelve rows, in this order:

- **XCIII** — the theme the product itself is drawn in: warm paper and a black
  screen.
- **Windows**, **One**, **Gruvbox**, **Night Owl**, **Discord**, **Solarized**,
  **GitHub**, **Monokai** — themes from Mattermost's community list. The half
  that list does not carry is the real one the theme has at home — One Light,
  Gruvbox Light, Light Owl and so on.
- **«Кислота: синий и жёлтый»** and **«Эйсид-техно»** — the acid ones, and they
  are meant to hurt.
- **"Custom config"** — a theme you pasted in yourself, below.

The picked theme and polarity are kept by the install itself: they survive a
restart and apply on a phone through
["Access from a phone"](./index.md#access-from-a-phone).

## A config of your own

A theme is a JSON object in Mattermost's format. Pick "Custom config" in the
list and a field appears under it. It opens on a copy of the theme that was on
screen, so the easiest way to write a new one is to edit an old one. Paste your
config and press **"Apply"**.

A pasted config is one half rather than a pair: it says for itself whether it is
light or dark (through the `type` key, and without it by how light the
background is). So "Light or dark" is not drawn while it is on — there is
nothing for it to switch.

Ready-made themes for this format are in Mattermost's own documentation, under
[Customize your theme](https://docs.mattermost.com/end-user-guide/preferences/customize-your-theme.html)
— they can be copied from there whole.

The smallest theme is four keys:

```json
{
  "centerChannelBg": "#1a1024",
  "centerChannelColor": "#ece0f7",
  "sidebarBg": "#2d1b4e",
  "buttonBg": "#a463f2"
}
```

Without any one of them a theme cannot be built, and the field says which key
is missing. Everything else is optional: what the config does not say, the app
derives from what it does.

## The keys

Colours are written as `#1c1a16` (the short `#abc` form is understood too).

| key | what it paints |
| --- | --- |
| `centerChannelBg` | the board's background |
| `centerChannelColor` | the text colour |
| `sidebarBg` | the left list's background |
| `sidebarText` | the text in the left list |
| `buttonBg` | buttons and accents |
| `buttonColor` | the text on a button |
| `linkColor` | links |
| `sidebarTextActiveBorder` | the bar beside the open board |
| `errorTextColor` | errors |
| `dndIndicator` | the do-not-disturb dot |
| `awayIndicator` | warnings, "the agent is waiting" |
| `onlineIndicator` | "passed", "done" |
| `mentionHighlightLink` | visited links |

The other Mattermost keys (`sidebarHeaderBg`, `mentionBg`, `codeTheme` and the
rest) can stay in the config: the app keeps them so the theme can be pasted
back into Mattermost, but paints nothing from them. The exception is
`sidebarTeamBarBg` and `sidebarHeaderBg`, which are used when `sidebarBg` is
the same colour as `centerChannelBg`, so the left list still has an edge.

Beyond Mattermost's keys, these are understood:

| key | what it paints |
| --- | --- |
| `name` | how the theme is named in the list |
| `type` | `light` or `dark`; without it, decided by how light the background is |
| `surfaceBg` | cards and dialogs |
| `visitedLinkColor` | visited links |
| `shadowColor` | shadows and the dimming under a dialog |
| `labelDefault`, `labelGray`, `labelBrown`, `labelOrange`, `labelYellow`, `labelGreen`, `labelBlue`, `labelPurple`, `labelPink`, `labelRed` | the ten label colours: columns, priorities, any select property |

## What the app fixes for you

Colours in a theme for a chat client are meant for other sizes: there
`awayIndicator` is a dot beside a name, here it is a word on a card. So a
colour that cannot be read against the board is darkened or lightened — by
exactly as much as it takes to be readable, and never off its own hue. The same
goes for the text on a button: if it cannot be read there, the app takes one
that can.

What the config leaves out is derived: the card's colour from the board's, the
ten label colours from the card's, the shadows from `shadowColor` or from the
text colour.

A theme that reads as it is is not touched at all.

## If the config is refused

The field says what is wrong with it:

- "The field is empty" — there is nothing to paste;
- "This is not JSON" — the text did not parse; usually a missing comma or quote;
- "A theme is one JSON object of colours" — a list, or something else, was pasted;
- "These keys are missing…" — which of the four required ones are absent;
- "These are not colours like #1c1a16…" — which keys hold something unreadable.

The theme is not applied, and the text stays in the field — fix it and press
"Apply" again.

## The terminal

The terminal window stays dark under every theme: a real CLI runs there, and a
console on a light background reads worse. The theme paints the chrome around
the terminal, not its own ground.
