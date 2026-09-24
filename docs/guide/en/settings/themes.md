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
- **Acid: blue and yellow** and **Acid techno** — the acid ones, and they
  are meant to hurt.
- **Your own themes** — the ones you wrote, as many as you like; below.
- **"+ New theme"** — the last row of the list; it is not a theme, it makes one.

The picked theme and polarity are kept by the install itself: they survive a
restart and apply on a phone through
["Access from a phone"](./index.md#access-from-a-phone).

## Themes of your own

A theme is a JSON object in Mattermost's format. Pick **"+ New theme"** in the
list and an editor opens below it, in a frame of its own. It opens on a copy of
the theme that was on screen, so the easiest way to write a new one is to edit
an old one. Paste your config and press **"Apply"**.

You can have as many as you like: each takes a row of its own in the list,
below the line, under the ones that ship with the app. To come back to one,
pick it in the list and the editor opens on it.

**"Delete theme"** removes the one you are on and puts the XCIII theme back:
the one you were wearing has just stopped existing. It cannot be undone, and a
config is ordinary text, so copy it first if it is still worth anything.

A theme of your own has two halves, like every other one, and "Light or dark"
works there as it does everywhere, "As the system is" included.

**"Which half to edit"** is a separate choice inside "Custom config", with two
answers: there is no such thing as an "as the system is" config, so the field
asks about the half itself. It opens on whichever half is on screen and then
stays where you put it: what you are editing and what the board is wearing are
two different questions.

A half you have not written is taken from the one you have, so a single config
is enough.

The first time you pick "Custom config", both halves are filled with a copy of
the theme that was on screen: what you get is a fork of it to edit rather than
an empty field.

**"Name"** — what this theme is called in the list. While it is empty the list
says "Custom config".

**"Apply"** writes what is in the field and repaints the board. **"Cancel"**
puts back into the field whatever is applied right now — the only thing an edit
here can undo; to leave for another theme, pick it in the list. Both buttons are
pale until something in the field has changed.

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
