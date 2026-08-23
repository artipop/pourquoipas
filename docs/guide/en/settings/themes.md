# Themes

The theme is picked in "Settings → The app → Theme". It is a dropdown, and the
choice applies at once — the board repaints under it.

## What is in the list

- **"System theme"** — the light or the dark XCIII theme, whichever the system
  is set to. It follows the system, with no restart.
- **XCIII** — «XCIII — бумага» and «XCIII — экран», the two themes the
  product itself is drawn in.
- **Ported** — themes from Mattermost's community list: Windows Dark, One Dark,
  Gruvbox Dark, Night Owl, Discord Dark, Solarized Dark, GitHub, Monokai.
- **Acid** — «Кислота: синий по жёлтому», «Кислота: жёлтый по синему»,
  «Эйсид-техно: лайм», «Эйсид-техно: фосфор».
- **"Custom config"** — a theme you pasted in yourself, below.

The picked theme is kept by the install itself: it survives a restart and
applies on a phone through
["Access from a phone"](./index.md#access-from-a-phone).

## A config of your own

A theme is a JSON object in Mattermost's format. Pick "Custom config" in the
list and a field appears under it. It opens on a copy of the theme that was on
screen, so the easiest way to write a new one is to edit an old one. Paste your
config and press **"Apply"**.

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
