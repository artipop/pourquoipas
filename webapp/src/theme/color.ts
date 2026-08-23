// Colour arithmetic for the theme configs: parsing what somebody pasted, and
// the mixing the derivation in `tokens.ts` does with it.
//
// It is a module of its own so the contrast test can use exactly the same
// arithmetic the product does. A test that computes contrast its own way is a
// second implementation, and the second one goes wrong.

export type RGB = [number, number, number]

const HEX = /^#?([\da-f]{3}|[\da-f]{4}|[\da-f]{6}|[\da-f]{8})$/i

// Mattermost writes `#rrggbb`, and a theme somebody hand-edited may be short
// form or carry an alpha this product has nowhere to put. Alpha is dropped
// rather than refused: a config is somebody's paste, and refusing the whole of
// it over two extra characters helps nobody.
export function parseColor(value: unknown): RGB | null {
    if (typeof value !== 'string') {
        return null
    }
    const match = HEX.exec(value.trim())
    if (!match) {
        return null
    }
    let digits = match[1]
    if (digits.length <= 4) {
        digits = digits.split('').map((c) => c + c).join('')
    }
    return [0, 2, 4].map((i) => parseInt(digits.slice(i, i + 2), 16)) as RGB
}

export function toHex([r, g, b]: RGB): string {
    return `#${[r, g, b].map((n) => Math.round(clamp(n)).toString(16).padStart(2, '0')).join('')}`
}

// The tokens the stylesheets read are bare `r, g, b` triples, never colours:
// almost every rule in the product wants `rgba(var(--x), α)`, and a triple is
// the only form that can be given an alpha at the call site.
export function toTriple([r, g, b]: RGB): string {
    return [r, g, b].map((n) => Math.round(clamp(n))).join(', ')
}

function clamp(n: number): number {
    return Math.min(255, Math.max(0, n))
}

// `amount` is the share of `a`, so mix(hue, surface, 0.2) reads as "a fifth of
// the hue over the surface" — which is what a label tint is.
//
// The result is rounded, and that is not tidiness: everything here ends up as
// an integer triple in a custom property, so a search that measures the
// unrounded value settles one level below the ratio it was looking for and
// ships a colour that misses it.
export function mix(a: RGB, b: RGB, amount: number): RGB {
    return [0, 1, 2].map((i) => Math.round(clamp((a[i] * amount) + (b[i] * (1 - amount))))) as RGB
}

export function luminance([r, g, b]: RGB): number {
    const channel = (v: number) => {
        const s = v / 255
        return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
    }
    return (0.2126 * channel(r)) + (0.7152 * channel(g)) + (0.0722 * channel(b))
}

export function contrast(a: RGB, b: RGB): number {
    const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
    return (hi + 0.05) / (lo + 0.05)
}

// What a colour composites to when it is drawn at less than full opacity. The
// product does that everywhere — a label's own text sits at 0.8 — and a
// contrast check against the colour rather than against what it composites to
// measures something nobody sees.
export function over(fg: RGB, bg: RGB, alpha: number): RGB {
    return mix(fg, bg, alpha)
}

export const BLACK: RGB = [0, 0, 0]
export const WHITE: RGB = [255, 255, 255]

// Which end to walk toward to get away from a colour. Asked as a contrast
// question rather than answered from a luminance threshold: relative luminance
// 0.5 is already a bright colour, so `#abb2bf` — a light grey to any eye — sits
// at 0.44 and a threshold sends everything that has to escape it toward white.
export function awayFrom(color: RGB): RGB {
    return contrast(WHITE, color) >= contrast(BLACK, color) ? WHITE : BLACK
}

export function isDark(rgb: RGB): boolean {
    return luminance(rgb) < 0.5
}

export const AA = 4.5

// Ink that can actually be read on `bg`. The preferred value wins whenever it
// reaches AA, so a config's own answer is honoured; when it does not, the
// candidates are tried in turn and the best contrast is the last resort.
//
// This exists because a pasted theme is somebody else's product: Mattermost's
// button is a small pill against a light chat pane, and the same colour under
// ours is a page-wide surface. Solarized's `#fdf6e3` on its own `#859900` is
// 3.4:1 there and 3.4:1 here — the difference is that here it is the primary
// action on every dialog in the app.
export function readableInk(bg: RGB, preferred: RGB | null, candidates: RGB[] = []): RGB {
    const tried = [preferred, ...candidates, WHITE, BLACK].filter((c): c is RGB => c !== null)
    for (const ink of tried) {
        if (contrast(ink, bg) >= AA) {
            return ink
        }
    }
    return tried.reduce((best, ink) => (contrast(ink, bg) > contrast(best, bg) ? ink : best), tried[0])
}

// A colour that can be seen against `bg`, moved as little as possible and
// never off its own hue: it is lightened or darkened, whichever direction
// leads away from the background.
//
// This is the other half of the bargain the config format makes. Mattermost's
// `awayIndicator` is an eight-pixel dot beside a name; here the same value is
// the word «ждёт» on a card, and `#d4b579` on white is 1.97:1 — a colour that
// is fine as a dot and absent as text. Adjusting it keeps the theme's decision
// (this is the yellow one) while making the product readable, which is the
// only way a theme written for another product can be worn by this one.
export function ensureReadable(color: RGB, bg: RGB, ratio = AA): RGB {
    if (contrast(color, bg) >= ratio) {
        return color
    }
    const target = awayFrom(bg)
    if (contrast(target, bg) < ratio) {
        return target
    }

    // Contrast rises monotonically as the colour moves toward the far end, so
    // twelve halvings put it within a level of the least adjustment that works.
    let lo = 0
    let hi = 1
    for (let i = 0; i < 12; i++) {
        const mid = (lo + hi) / 2
        if (contrast(mix(target, color, mid), bg) >= ratio) {
            hi = mid
        } else {
            lo = mid
        }
    }
    return mix(target, color, hi)
}

// What the product asks of a colour that is a mark rather than a word — the
// bar beside the open board in the sidebar, a drop line. WCAG's own figure for
// a non-text element, and the reason the stamp is not held to AA: a 3px rule
// at 4.5:1 against a near-black sidebar would have to be nearly white, which
// is exactly the colour it must not be.
export const AA_MARK = 3

// A background a given ink can be read on, moved away from that ink and
// nowhere else. The counterpart of `ensureReadable`, for the one case where it
// is the ground that may move: a label chip carries its own text, so a tint
// with no room deepens rather than the ten hues all collapsing to the surface.
export function ensureTextReadable(bg: RGB, fg: RGB, alpha = 1, ratio = AA): RGB {
    const reads = (candidate: RGB) => contrast(over(fg, candidate, alpha), candidate) >= ratio
    if (reads(bg)) {
        return bg
    }
    const target = awayFrom(fg)
    let lo = 0
    let hi = 1
    if (!reads(mix(target, bg, hi))) {
        return mix(target, bg, hi)
    }
    for (let i = 0; i < 12; i++) {
        const mid = (lo + hi) / 2
        if (reads(mix(target, bg, mid))) {
            hi = mid
        } else {
            lo = mid
        }
    }
    return mix(target, bg, hi)
}
