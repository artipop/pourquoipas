import {rawThemeVars} from '../styles/tokensSource'

import {AA, AA_MARK, contrast, over, parseColor, RGB} from './color'
import {parseTheme, ThemeConfigError} from './config'
import {familyOfVariant, SHIPPED_VARIANTS, themeFamily, THEME_FAMILIES} from './themes'
import {themeTokens} from './tokens'

// A theme is a config, and these are the things that have to be true of one.
//
// It has to *say the same as the stylesheet* for the two halves of the theme
// the product was designed in — `_tokens.scss` still paints the first frame, so
// a config that disagreed with it would be a flash of one palette followed by
// another.
//
// Every variant has to be *readable*, including the ones borrowed from another
// product and the ones written as a joke. The derivation adjusts what a config
// says where it has to; this is the check that it adjusted enough.
//
// And every theme has to be a *pair*, because the picker now asks which design
// and which polarity as two questions: a family with one half would leave the
// second control pointing at nothing.

const triple = (value: string): RGB => value.split(',').map((n) => Number(n.trim())) as RGB

// The stylesheet writes `#fff` where a config writes `#ffffff` — stylelint's
// `color-hex-length` asks for the short form and `JSON.stringify` has no
// opinion — so the two are compared as colours and not as text.
const sameHex = (value: string): string => {
    const rgb = value.startsWith('#') ? parseColor(value) : null
    return rgb ? rgb.join(', ') : value
}
const colour = (value: string): RGB => (value.startsWith('#') ? parseColor(value)! : triple(value))

const LABEL_TOKENS = [
    '--prop-default', '--prop-gray', '--prop-brown', '--prop-orange', '--prop-yellow',
    '--prop-green', '--prop-blue', '--prop-purple', '--prop-pink', '--prop-red',
]

// Label text is drawn at 0.8, so what has to be readable is what that
// composites to. The same figure `tokens.ts` derives against.
const LABEL_TEXT_ALPHA = 0.8

describe('theme/themes', () => {
    it('gives every theme and every variant an id of its own', () => {
        const families = THEME_FAMILIES.map((family) => family.id)
        expect(families.filter((id) => !id)).toEqual([])
        expect(new Set(families).size).toBe(families.length)

        const variants = SHIPPED_VARIANTS.map((variant) => variant.id)
        expect(variants.filter((id) => !id)).toEqual([])
        expect(new Set(variants).size).toBe(variants.length)
    })

    it('names every theme, so the list has something to draw', () => {
        expect(THEME_FAMILIES.filter((family) => !family.name)).toEqual([])
    })

    // The load-bearing half of the split. A family whose two halves are the
    // same polarity — or the same file — is one the light/dark control cannot
    // move, and the control would say nothing while looking like it does.
    it('gives every theme a light half and a dark half', () => {
        for (const family of THEME_FAMILIES) {
            expect([family.id, family.light.type]).toEqual([family.id, 'light'])
            expect([family.id, family.dark.type]).toEqual([family.id, 'dark'])
        }
    })

    // How a setting written when a variant *was* a theme is read back:
    // `windows-dark` still names a real half of a real theme.
    it('finds the theme a variant belongs to', () => {
        expect(familyOfVariant('windows-dark')?.family.id).toBe('windows')
        expect(familyOfVariant('windows-dark')?.dark).toBe(true)
        expect(familyOfVariant('light-theme')?.family.id).toBe('xciii')
        expect(familyOfVariant('light-theme')?.dark).toBe(false)
        expect(familyOfVariant('no-such-theme')).toBeUndefined()
    })

    // The load-bearing one. `_tokens.scss` is the first paint and the config is
    // every frame after it; the two disagreeing is a palette that changes as
    // the page finishes loading.
    describe.each([['light', false], ['dark', true]] as const)('the XCIII %s half', (half, dark) => {
        const config = () => (dark ? themeFamily('xciii')!.dark : themeFamily('xciii')!.light)

        it('says exactly what _tokens.scss says', () => {
            const tokens = themeTokens(parseTheme(config(), half))
            const scss = rawThemeVars(dark)
            const raw = Object.fromEntries(
                Object.entries(scss).
                    filter(([name]) => name in tokens).
                    map(([name, value]) => [name, sameHex(value)]),
            )
            expect(raw).toEqual(Object.fromEntries(
                Object.entries(tokens).map(([name, value]) => [name, sameHex(value)]),
            ))
        })

        // And the other direction: a raw token added to the stylesheet that no
        // config can produce is a colour the theme field cannot reach.
        it('produces every raw token the stylesheet declares', () => {
            const tokens = themeTokens(parseTheme(config(), half))
            const missing = Object.keys(rawThemeVars(dark)).filter((name) => !(name in tokens))
            expect(missing).toEqual([])
        })
    })

    describe.each(SHIPPED_VARIANTS.map((variant) => [variant.name!, variant] as const))('%s', (_name, config) => {
        const tokens = () => themeTokens(parseTheme(config, config.id!))
        const token = (name: string) => colour(tokens()[name])

        it('reads on the canvas and on a raised surface', () => {
            const ink = token('--ink-rgb')
            expect(contrast(ink, token('--canvas-rgb'))).toBeGreaterThanOrEqual(AA)
            expect(contrast(ink, token('--surface-rgb'))).toBeGreaterThanOrEqual(AA)
        })

        it('reads on a button and in the sidebar', () => {
            expect(contrast(token('--accent-ink-rgb'), token('--accent-rgb'))).toBeGreaterThanOrEqual(AA)
            expect(contrast(token('--sidebar-ink-rgb'), token('--sidebar-rgb'))).toBeGreaterThanOrEqual(AA)
        })

        it('reads a link, an error and the three indicators on the canvas', () => {
            const canvas = token('--canvas-rgb')
            for (const name of ['--link-rgb', '--visited-rgb', '--danger-rgb', '--dnd-rgb', '--warning-rgb', '--success-rgb']) {
                expect([name, contrast(token(name), canvas) >= AA]).toEqual([name, true])
            }
        })

        // The stamp is a mark and not a word — the bar beside the open board —
        // so it is held to the figure for a non-text element.
        it('marks the open board in the sidebar', () => {
            expect(contrast(token('--stamp-rgb'), token('--sidebar-rgb'))).toBeGreaterThanOrEqual(AA_MARK)
        })

        it('carries label text on all ten labels', () => {
            const ink = token('--ink-rgb')
            for (const name of LABEL_TOKENS) {
                const bg = token(name)
                expect([name, contrast(over(ink, bg, LABEL_TEXT_ALPHA), bg) >= AA]).toEqual([name, true])
            }
        })

        it('leaves nothing unset', () => {
            for (const [name, value] of Object.entries(tokens())) {
                expect([name, value]).not.toEqual([name, ''])
            }
        })
    })

    // The point of borrowing the format: a theme copied off the Mattermost
    // documentation, with none of this product's own keys in it, has to work.
    describe('a theme pasted from elsewhere', () => {
        const mattermost = {
            sidebarBg: '#145dbf',
            sidebarText: '#ffffff',
            sidebarTextActiveBorder: '#579eff',
            onlineIndicator: '#06d6a0',
            awayIndicator: '#ffbc42',
            dndIndicator: '#f74343',
            centerChannelBg: '#ffffff',
            centerChannelColor: '#3d3c40',
            linkColor: '#2389d7',
            buttonBg: '#166de0',
            buttonColor: '#ffffff',
            errorTextColor: '#fd5960',
            codeTheme: 'github',
        }

        it('is read as light or dark without being told', () => {
            expect(parseTheme(mattermost).dark).toBe(false)
            expect(parseTheme({...mattermost, centerChannelBg: '#1f1f1f'}).dark).toBe(true)
        })

        // What the TypeScript literal would have caught, caught out loud: the
        // configs arrive as imported JSON, where `type` can only be `string`.
        it('refuses a polarity that is neither', () => {
            expect(() => parseTheme({...mattermost, type: 'drak'})).toThrow('type')
        })

        it('gets a surface, ten labels and a shadow it never named', () => {
            const tokens = themeTokens(parseTheme(mattermost))
            expect(tokens['--surface-rgb']).toBeTruthy()
            expect(tokens['--backdrop']).toBeTruthy()
            for (const name of LABEL_TOKENS) {
                expect(tokens[name]).toMatch(/^#[\da-f]{6}$/)
            }
        })

        it('is readable everywhere it lands', () => {
            const tokens = themeTokens(parseTheme(mattermost))
            const canvas = colour(tokens['--canvas-rgb'])
            const ink = colour(tokens['--ink-rgb'])
            expect(contrast(ink, canvas)).toBeGreaterThanOrEqual(AA)
            expect(contrast(colour(tokens['--warning-rgb']), canvas)).toBeGreaterThanOrEqual(AA)
            expect(contrast(colour(tokens['--accent-ink-rgb']), colour(tokens['--accent-rgb']))).toBeGreaterThanOrEqual(AA)
        })

        // The keys this product draws nothing from are kept, so a theme
        // exported from here goes back into Mattermost unchanged.
        it('keeps the keys this product has no counterpart for', () => {
            const {config} = parseTheme({...mattermost, mentionHighlightBg: '#ffe577'})
            expect(config.mentionHighlightBg).toBe('#ffe577')
            expect(config.codeTheme).toBe('github')
        })
    })

    // The field is the only place a person finds out, so the reasons have to be
    // different from each other.
    describe('a config that will not do', () => {
        const reason = (input: unknown) => {
            try {
                parseTheme(input)
                return 'ok'
            } catch (e) {
                return e instanceof ThemeConfigError ? e.message : 'other'
            }
        }

        it('says which way it is wrong', () => {
            expect(reason('')).toBe('empty')
            expect(reason('{oh no')).toBe('json')
            expect(reason('[]')).toBe('object')
            expect(reason('{"centerChannelBg": "#fff"}')).toBe('missing:centerChannelColor, sidebarBg, buttonBg')
            expect(reason({
                centerChannelBg: '#ffffff',
                centerChannelColor: 'rebeccapurple',
                sidebarBg: '#000000',
                buttonBg: '#000000',
            })).toBe('color:centerChannelColor')
        })

        it('takes the short forms and an alpha it has nowhere to put', () => {
            const {colors} = parseTheme({
                centerChannelBg: '#fff',
                centerChannelColor: '#000000ff',
                sidebarBg: '#123',
                buttonBg: '#112233',
            })
            expect(colors.centerChannelBg).toEqual([255, 255, 255])
            expect(colors.centerChannelColor).toEqual([0, 0, 0])
            expect(colors.sidebarBg).toEqual([17, 34, 51])
        })
    })
})
