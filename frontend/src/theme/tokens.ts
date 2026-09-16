// A parsed config becomes the raw tokens `styles/_tokens.scss` declares at the
// top of its two theme blocks. Everything below the "no colour" line in that
// file is written in terms of these, so setting them is the whole of applying
// a theme — there is no second list of properties to keep in step.
//
// Two things happen here that a straight rename would not.
//
// **What a config does not say is derived.** A pasted Mattermost theme names
// thirteen things this product draws with, and has no word for a raised
// surface, for the ten label tints or for a shadow. A theme that arrived with
// holes in it would be a theme that looks broken.
//
// **What a config says unreadably is adjusted, and only then.** The same
// value is a different size in the two products — a presence dot there, a word
// on a card here — so every derived colour is put through `ensureReadable`
// against the ground it lands on. A theme whose own answer already works is
// untouched, which is why the two shipped themes come out of this byte for
// byte as `_tokens.scss` declares them.

import {
    AA,
    AA_MARK,
    awayFrom,
    BLACK,
    contrast,
    ensureReadable,
    ensureTextReadable,
    isDark,
    mix,
    readableInk,
    RGB,
    toHex,
    toTriple,
    WHITE,
} from './color'
import {LabelKey, ParsedTheme} from './config'

// The only colours named outside `_tokens.scss`, and they are not a palette:
// they are the hue each label is *of*, used solely to tint a theme's own
// surface when a config declines to say what its labels look like. A shipped
// theme states its ten (`labelBlue`, …) and never reaches this table.
//
// Kept at full saturation because the mixing below decides how strong a tint
// is; an anchor that was already pale would tint twice.
const LABEL_ANCHORS: Record<LabelKey, RGB | null> = {
    labelDefault: null,
    labelGray: [139, 139, 139],
    labelBrown: [139, 90, 43],
    labelOrange: [224, 123, 30],
    labelYellow: [217, 180, 0],
    labelGreen: [46, 158, 91],
    labelBlue: [47, 127, 208],
    labelPurple: [122, 91, 208],
    labelPink: [208, 91, 158],
    labelRed: [208, 74, 58],
}

export const LABEL_TOKENS: Record<LabelKey, string> = {
    labelDefault: '--prop-default',
    labelGray: '--prop-gray',
    labelBrown: '--prop-brown',
    labelOrange: '--prop-orange',
    labelYellow: '--prop-yellow',
    labelGreen: '--prop-green',
    labelBlue: '--prop-blue',
    labelPurple: '--prop-purple',
    labelPink: '--prop-pink',
    labelRed: '--prop-red',
}

// How much of the anchor goes over the surface. Measured against the shipped
// light theme, whose ten tints this reproduces to within a couple of levels —
// which is the check that it is a rule and not a guess. It is a ceiling
// rather than a constant: a theme with less room backs off from it.
const LABEL_TINT = 0.2

// Label text is drawn at 0.8, so what has to be readable is what that
// composites to and not the ink itself.
const LABEL_TEXT_ALPHA = 0.8

export type ThemeTokens = Record<string, string>

export function themeTokens(theme: ParsedTheme): ThemeTokens {
    const {colors, dark} = theme
    const canvas = colors.centerChannelBg!
    const ink = colors.centerChannelColor!
    const accent = colors.buttonBg!

    // A card is lighter than the board it stands on in both themes — on paper
    // because a card is a sheet on a desk, on screen because that is the only
    // way depth exists there. The lift differs because the headroom does: half
    // the way to white barely moves an off-white and would flood a black. It
    // backs off rather than being fixed, since lifting a dark canvas walks the
    // surface toward its own ink.
    const surface = colors.surfaceBg ?? liftedSurface(canvas, ink, dark)

    // The sidebar is a plane of its own here and is not always one there: a
    // Mattermost theme may paint the channel list the same colour as the
    // message pane, and Solarized Dark does — `#073642` for both — which put
    // the board and the navigation on one flat sheet with no edge between
    // them. Such a theme has said where its edge is somewhere else, in the two
    // keys above the channel list, so they are asked in turn before this side
    // invents a shade.
    const sidebar = distinctSidebar(colors.sidebarBg!, canvas, colors.sidebarTeamBarBg, colors.sidebarHeaderBg)

    const sidebarInk = ensureReadable(colors.sidebarText ?? pickInk(sidebar, ink, canvas), sidebar)
    const link = ensureReadable(colors.linkColor ?? accent, canvas)
    const danger = ensureReadable(colors.errorTextColor ?? colors.dndIndicator ?? [226, 66, 52], canvas)

    const tokens: ThemeTokens = {
        '--canvas-rgb': toTriple(canvas),
        '--surface-rgb': toTriple(surface),
        '--ink-rgb': toTriple(ink),
        '--sidebar-rgb': toTriple(sidebar),
        '--sidebar-ink-rgb': toTriple(sidebarInk),

        '--accent-rgb': toTriple(accent),

        // `buttonColor` is a chat client's small pill; here the same pair is
        // the primary action on every dialog in the app.
        '--accent-ink-rgb': toTriple(readableInk(accent, colors.buttonColor ?? null, [canvas, ink])),

        // The stamp is a mark, not a word — the bar beside the open board.
        '--stamp-rgb': toTriple(ensureReadable(colors.sidebarTextActiveBorder ?? accent, sidebar, AA_MARK)),
        '--link-rgb': toTriple(link),
        '--visited-rgb': toTriple(ensureReadable(colors.visitedLinkColor ?? colors.mentionHighlightLink ?? link, canvas)),

        '--danger-rgb': toTriple(danger),
        '--dnd-rgb': toTriple(ensureReadable(colors.dndIndicator ?? danger, canvas)),
        '--warning-rgb': toTriple(ensureReadable(colors.awayIndicator ?? [255, 180, 76], canvas)),
        '--success-rgb': toTriple(ensureReadable(colors.onlineIndicator ?? [61, 184, 135], canvas)),
    }

    const shadow = colors.shadowColor ?? (dark ? BLACK : ink)
    tokens['--shadow-rgb'] = toTriple(shadow)
    tokens['--shadow-1'] = dark ? 'none' : '0 1px 0 rgba(var(--shadow-rgb), 0.06)'
    tokens['--shadow-2'] = dark ? '0 2px 8px -2px rgba(var(--shadow-rgb), 0.5)' : '0 2px 6px -2px rgba(var(--shadow-rgb), 0.16)'
    tokens['--shadow-3'] = dark ? '0 24px 64px -16px rgba(var(--shadow-rgb), 0.7)' : '0 16px 48px -12px rgba(var(--shadow-rgb), 0.32)'
    tokens['--backdrop'] = `rgba(${toTriple(shadow)}, ${dark ? '0.66' : '0.44'})`

    for (const key of Object.keys(LABEL_TOKENS) as LabelKey[]) {
        tokens[LABEL_TOKENS[key]] = toHex(colors[key] ?? derivedLabel(key, surface, ink, dark))
    }

    return tokens
}

// The sidebar has an ink of its own because it is the one part of the product
// that is dark in both themes on purpose. A config that names none is asked
// which of its two grounds the sidebar is more like, and given the other's.
function pickInk(sidebar: RGB, ink: RGB, canvas: RGB): RGB {
    return contrast(ink, sidebar) >= contrast(canvas, sidebar) ? ink : canvas
}

// Literally the same colour, and not "close": the shipped dark theme puts the
// sidebar four levels below the canvas on purpose — a hard edge between two
// large planes is visible at four levels — and a tolerance wide enough to
// catch that would rewrite a decision instead of repairing an absence.
function sameGround(a: RGB, b: RGB): boolean {
    return a.every((channel, i) => channel === b[i])
}

function distinctSidebar(sidebar: RGB, canvas: RGB, ...offered: Array<RGB | undefined>): RGB {
    if (!sameGround(sidebar, canvas)) {
        return sidebar
    }
    for (const candidate of offered) {
        if (candidate && !sameGround(candidate, canvas)) {
            return candidate
        }
    }

    // Nothing was offered, so the edge is made: away from the canvas, which is
    // darker on a dark theme and lighter on a light one.
    return mix(awayFrom(canvas), canvas, 0.08)
}

function liftedSurface(canvas: RGB, ink: RGB, dark: boolean): RGB {
    const start = dark ? 0.07 : 0.5
    for (let lift = start; lift > 0; lift -= 0.01) {
        const surface = mix(WHITE, canvas, lift)
        if (contrast(ink, surface) >= AA) {
            return surface
        }
    }
    return canvas
}

// A label is its hue laid thinly over the theme's own surface. Thin enough
// that the tint barely moves the luminance is what keeps the text on it
// readable — the old dark labels were alpha over a tinted ground and came out
// as mud — and where a theme has no room for even that, the tint goes to
// nothing and the chip is the surface, which is a pale label and not an
// unreadable one.
function derivedLabel(key: LabelKey, surface: RGB, ink: RGB, dark: boolean): RGB {
    const anchor = LABEL_ANCHORS[key]
    const plain = dark ? mix(ink, surface, 0.04) : mix(WHITE, surface, 0.5)
    const tinted = anchor ? mix(anchor, surface, LABEL_TINT) : plain
    return ensureTextReadable(tinted, ink, LABEL_TEXT_ALPHA)
}

export {isDark}
