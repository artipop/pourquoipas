// A theme is a config, and the config is Mattermost's.
//
// Its keys are the ones documented at
// https://docs.mattermost.com/end-user-guide/preferences/customize-your-theme.html
// so a theme somebody found for another product can be pasted into the
// settings and work here. That is the whole reason for borrowing a format
// rather than inventing one: themes for it already exist, and nobody is going
// to hand-write twenty-six colours for a board.
//
// The two products do not draw the same things, so the mapping is stated once
// here and nowhere else:
//
//   used            centerChannelBg, centerChannelColor, sidebarBg,
//                   sidebarText, buttonBg, buttonColor, linkColor,
//                   sidebarTextActiveBorder, errorTextColor, dndIndicator,
//                   awayIndicator, onlineIndicator, mentionHighlightLink
//
//   used only when  sidebarTeamBarBg, sidebarHeaderBg — the sidebar is a plane
//   the sidebar is  of its own here, so when a theme paints the channel list
//   not a plane     the colour of the message pane these are where it said
//                   where its edge is (`tokens.ts`, `distinctSidebar`)
//
//   accepted, and   sidebarUnreadText, sidebarTextHoverBg,
//   nothing drawn   sidebarTextActiveColor, sidebarHeaderTextColor, mentionBg,
//   from them       mentionBj, mentionColor, mentionHighlightBg,
//                   newMessageSeparator, codeTheme
//
// The last group names parts of a chat client this board has not got — an
// unread channel, a mention jewel, a syntax theme for code blocks. They are
// kept in the object rather than stripped so that a theme exported from here
// can be pasted back into Mattermost unchanged; every sidebar state in this
// product is drawn as an alpha over `sidebarText`, which is why the hover and
// active keys have no counterpart to point at.
//
// What Mattermost has no word for at all — a raised surface, the ten label
// tints, the shadow — is derived in `tokens.ts` and can be overridden by the
// extension keys below, which the shipped themes use to say exactly what they
// mean rather than accept a derivation.

import {parseColor, RGB} from './color'

// The Mattermost keys, in the order that page lists them.
export const MATTERMOST_KEYS = [
    'sidebarBg',
    'sidebarText',
    'sidebarUnreadText',
    'sidebarTextHoverBg',
    'sidebarTextActiveBorder',
    'sidebarTextActiveColor',
    'sidebarHeaderBg',
    'sidebarTeamBarBg',
    'sidebarHeaderTextColor',
    'onlineIndicator',
    'awayIndicator',
    'dndIndicator',
    'mentionBg',
    'mentionBj',
    'mentionColor',
    'centerChannelBg',
    'centerChannelColor',
    'newMessageSeparator',
    'linkColor',
    'buttonBg',
    'buttonColor',
    'errorTextColor',
    'mentionHighlightBg',
    'mentionHighlightLink',
] as const

// Ours. Flat and unprefixed like the rest, because the file is one object a
// person edits by hand; `codeTheme` is the precedent for a key that is not a
// colour.
export const LABEL_KEYS = [
    'labelDefault',
    'labelGray',
    'labelBrown',
    'labelOrange',
    'labelYellow',
    'labelGreen',
    'labelBlue',
    'labelPurple',
    'labelPink',
    'labelRed',
] as const

export const XCIII_KEYS = [
    'surfaceBg',
    'visitedLinkColor',
    'shadowColor',
    ...LABEL_KEYS,
] as const

export type MattermostKey = typeof MATTERMOST_KEYS[number]
export type LabelKey = typeof LABEL_KEYS[number]
export type XciiiKey = typeof XCIII_KEYS[number]

export type ThemeConfig = {

    // The identity a person picks it by. `id` is what the setting stores;
    // `name` is what the dropdown draws. A pasted config carries neither and
    // becomes the one custom slot.
    id?: string
    name?: string

    // Which side of the line the theme is on, which decides `color-scheme`,
    // the shadows, and which way the derivations move. Inferred from
    // `centerChannelBg` when absent, since a config from elsewhere never says.
    //
    // Typed `string` rather than the two literals because these arrive as
    // imported JSON, where TypeScript can only see `string`. What the literals
    // would have caught is caught at parse instead, and better: a `type` that
    // is neither is refused out loud rather than read as light.
    type?: string

    codeTheme?: string
} & Partial<Record<MattermostKey | XciiiKey, string>>

export type ParsedTheme = {
    id: string
    name: string
    dark: boolean
    config: ThemeConfig
    colors: Partial<Record<MattermostKey | XciiiKey, RGB>>
}

// The four a theme cannot be built without. Everything else has somewhere
// sensible to fall back to; these are the canvas, the ink on it, the sidebar
// and the button, and a config missing one of them is not a theme.
export const REQUIRED_KEYS: MattermostKey[] = ['centerChannelBg', 'centerChannelColor', 'sidebarBg', 'buttonBg']

export class ThemeConfigError extends Error {}

// parseTheme turns whatever was pasted into something `tokens.ts` can read, or
// throws with the reason in the message — the field is the only place a person
// finds out, so "не похоже на JSON" and "нет centerChannelBg" have to be
// different sentences.
export function parseTheme(input: unknown, fallbackId = 'custom'): ParsedTheme {
    let raw = input
    if (typeof raw === 'string') {
        const text = raw.trim()
        if (!text) {
            throw new ThemeConfigError('empty')
        }
        try {
            raw = JSON.parse(text)
        } catch {
            throw new ThemeConfigError('json')
        }
    }
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
        throw new ThemeConfigError('object')
    }

    const config = raw as ThemeConfig
    const colors: Partial<Record<MattermostKey | XciiiKey, RGB>> = {}
    const bad: string[] = []

    for (const key of [...MATTERMOST_KEYS, ...XCIII_KEYS]) {
        const value = config[key]
        if (value === undefined || value === null || value === '') {
            continue
        }
        const rgb = parseColor(value)
        if (!rgb) {
            bad.push(key)
            continue
        }
        colors[key] = rgb
    }

    if (bad.length > 0) {
        throw new ThemeConfigError(`color:${bad.join(', ')}`)
    }

    if (config.type !== undefined && config.type !== 'light' && config.type !== 'dark') {
        throw new ThemeConfigError('type')
    }

    const missing = REQUIRED_KEYS.filter((key) => !colors[key])
    if (missing.length > 0) {
        throw new ThemeConfigError(`missing:${missing.join(', ')}`)
    }

    return {
        id: typeof config.id === 'string' && config.id ? config.id : fallbackId,
        name: typeof config.name === 'string' && config.name ? config.name : fallbackId,
        dark: config.type ? config.type === 'dark' : isDarkGround(colors.centerChannelBg!),
        config,
        colors,
    }
}

function isDarkGround(bg: RGB): boolean {
    const [r, g, b] = bg
    return ((0.299 * r) + (0.587 * g) + (0.114 * b)) < 128
}
