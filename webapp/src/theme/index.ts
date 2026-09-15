import {UserSettings} from '../userSettings'

import {parseTheme, ParsedTheme, ThemeConfig, ThemeConfigError} from './config'
import {
    addCustomTheme,
    adoptSingleSlot,
    CustomTheme,
    customTheme,
    customThemes,
    firstCustomId,
    isCustomThemeId,
    removeCustomTheme,
    updateCustomTheme,
} from './custom'
import {
    familyOfVariant,
    SHIPPED_VARIANTS,
    ThemeFamily,
    themeFamily,
    THEME_FAMILIES,
} from './themes'
import {themeTokens} from './tokens'

// Which theme applies, and the applying of it.
//
// **Two settings, not one.** A theme is a design — «XCIII», «Gruvbox», the
// blue-and-yellow joke — and it comes in a light half and a dark half; whether
// this machine wears the light one, the dark one or whichever the system is set
// to is a separate question with its own answer. They were one list once, and
// that list said «Gruvbox Dark» and «XCIII — бумага» in the same column as «Как
// в системе»: picking a design meant also deciding a polarity, and following
// the system meant giving up every design but ours.
//
// A theme is a config (`config.ts`), and applying one is writing the raw tokens
// it produces onto the document element. `styles/_tokens.scss` still declares
// the two shipped XCIII palettes at the top of its file: that is what the first
// paint uses, before this module has run, and it is the copy `themes.test.ts`
// checks the two configs against — one of them being wrong is a test failure
// rather than a flash of the wrong colours.

export const defaultFamilyId = 'xciii'

// The themes a person brings live in `custom.ts` and stand in the same list as
// the shipped ones. Each is a pair like every other theme — a light half and a
// dark one — so the switch beside the list means the same thing there as
// everywhere else.
export {customThemes, customTheme, isCustomThemeId, removeCustomTheme, updateCustomTheme}
export type {CustomTheme}

// The id the single slot answered to, kept as the first theme's so a stored
// setting naming it still resolves.
export const customThemeName = firstCustomId

export type ThemeHalf = 'light' | 'dark'

export type ThemeMode = 'light' | 'dark' | 'system'

export const themeModes: ThemeMode[] = ['light', 'dark', 'system']

export type ThemeName = string

export {THEME_FAMILIES, SHIPPED_VARIANTS, ThemeConfigError}
export type {ThemeConfig, ThemeFamily}

let activeFamilyId: ThemeName = defaultFamilyId
let activeMode: ThemeMode = 'system'

function prefersDark(): boolean {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
}

// One theme's half, as text, because that is what the field edits.
export function getCustomThemeSource(id: ThemeName, half: ThemeHalf): string {
    const theme = customTheme(id)

    return theme ? theme[half] : ''
}

export function setCustomThemeSource(id: ThemeName, half: ThemeHalf, source: string): void {
    updateCustomTheme(id, {[half]: source})
}

// What the list calls it. Empty is the ordinary case and the list says «Свой
// конфиг» for it; a name is what makes a theme somebody wrote their own rather
// than "the other one".
export function getCustomThemeName(id: ThemeName): string {
    return customTheme(id)?.name ?? ''
}

export function setCustomThemeName(id: ThemeName, name: string): void {
    updateCustomTheme(id, {name: name.trim()})
}

// A new theme is a fork of the one on screen, in both polarities — a person
// writing one starts from what they are looking at rather than from a format
// they have to go and look up.
export function newCustomTheme(): CustomTheme {
    return addCustomTheme(activeThemeConfigs(), themeConfigText)
}

function wantsDark(mode: ThemeMode): boolean {
    // Asked only when it is the answer: `matchMedia` is one of the things a
    // jsdom without it does not have, and a stated polarity has no business
    // consulting the machine's preference.
    return mode === 'dark' || (mode === 'system' && prefersDark())
}

// Which half is on screen. The custom slot's field opens on it, and after that
// the field has a choice of its own — «как в системе» is a real answer for the
// board and no answer at all for a config being typed.
export function activeThemeHalf(): ThemeHalf {
    return wantsDark(activeMode) ? 'dark' : 'light'
}

// resolveTheme answers with the config that is actually going to be drawn.
function resolveTheme(familyId: ThemeName, mode: ThemeMode): ParsedTheme {
    const half: ThemeHalf = wantsDark(mode) ? 'dark' : 'light'
    if (isCustomThemeId(familyId)) {
        // A half nobody has written falls back to the one they have: a theme
        // with a single config in it worked before the pair existed, and it
        // still does.
        const source = getCustomThemeSource(familyId, half) || getCustomThemeSource(familyId, other(half))

        return parseTheme(source, familyId)
    }
    const family = themeFamily(familyId) ?? themeFamily(defaultFamilyId)!
    const config = half === 'dark' ? family.dark : family.light

    return parseTheme(config, config.id ?? family.id)
}

function other(half: ThemeHalf): ThemeHalf {
    return half === 'dark' ? 'light' : 'dark'
}

function apply(familyId: ThemeName, mode: ThemeMode): void {
    const theme = resolveTheme(familyId, mode)
    const root = document.documentElement

    // `data-theme` is what `_tokens.scss` switches on, and it stays because
    // the terminal page and a handful of rules ask which side of the line they
    // are on rather than for a colour.
    root.dataset.theme = theme.dark ? 'dark' : 'light'

    // Told, or the browser paints its own widgetry — a checkbox, a scrollbar —
    // white on a black theme.
    root.style.colorScheme = theme.dark ? 'dark' : 'light'

    for (const [token, value] of Object.entries(themeTokens(theme))) {
        root.style.setProperty(token, value)
    }
}

export function setTheme(familyId: ThemeName): ThemeName {
    activeFamilyId = familyId
    UserSettings.theme = familyId
    apply(activeFamilyId, activeMode)
    return familyId
}

export function setThemeMode(mode: ThemeMode): ThemeMode {
    activeMode = mode
    UserSettings.themeMode = mode
    apply(activeFamilyId, activeMode)
    return mode
}

export function getActiveThemeName(): ThemeName {
    return activeFamilyId
}

export function getActiveThemeMode(): ThemeMode {
    return activeMode
}

// Both halves of the theme on screen, formatted the way the field wants them
// back. This is what the custom slot opens on — a fork of the theme you were
// looking at, in both polarities — and it is the reason a new theme is written
// by editing an old one rather than from nothing.
export function activeThemeConfigs(): {light: ThemeConfig, dark: ThemeConfig} {
    if (isCustomThemeId(activeFamilyId)) {
        return {
            light: resolveTheme(activeFamilyId, 'light').config,
            dark: resolveTheme(activeFamilyId, 'dark').config,
        }
    }
    const family = themeFamily(activeFamilyId) ?? themeFamily(defaultFamilyId)!

    return {light: family.light, dark: family.dark}
}

export function themeConfigText(config: ThemeConfig): string {
    return `${JSON.stringify(config, null, 2)}\n`
}

// What is stored is the *name* of a theme and the polarity beside it, or
// `custom-theme` with the config kept beside that.
//
// Three older spellings still resolve, and none of them needs a migration
// table. `light-theme` / `dark-theme` were the XCIII halves when a variant was
// a theme, and they still name real variants; so does every `windows-dark`
// this build ever wrote. `system-theme` was the third entry in that one list
// and is now the polarity's own answer. Anything else — including every
// setting written before themes were configs at all — is read as the default.
export function loadTheme(): ThemeName {
    const stored = UserSettings.theme ?? ''
    const storedMode = UserSettings.themeMode ?? ''

    activeMode = themeModes.includes(storedMode as ThemeMode) ? storedMode as ThemeMode : 'system'
    activeFamilyId = defaultFamilyId

    // Whatever the one slot left behind becomes the first theme in the list.
    adoptSingleSlot((source) => {
        try {
            return parseTheme(source).dark
        } catch {
            return false
        }
    })

    if (themeFamily(stored)) {
        activeFamilyId = stored
    } else if (customTheme(stored)) {
        activeFamilyId = stored
    } else {
        const variant = familyOfVariant(stored)
        if (variant) {
            activeFamilyId = variant.family.id

            // A stored variant states its polarity, and it is only believed
            // where nothing newer has been said: `themeMode` is written the
            // moment anybody touches the switch, so an install that has one is
            // past this.
            if (!storedMode) {
                activeMode = variant.dark ? 'dark' : 'light'
            }
        }
    }

    try {
        apply(activeFamilyId, activeMode)
    } catch {
        // A custom config that no longer parses must not leave the page
        // unpainted; the shipped default is the answer, and the text stays in
        // the field for whoever has to fix it.
        activeFamilyId = defaultFamilyId
        apply(activeFamilyId, activeMode)
    }
    return activeFamilyId
}

export function initThemes(): void {
    const darkThemeMq = window.matchMedia('(prefers-color-scheme: dark)')
    const changeHandler = () => {
        if (activeMode === 'system') {
            apply(activeFamilyId, activeMode)
        }
    }
    if (darkThemeMq.addEventListener) {
        darkThemeMq.addEventListener('change', changeHandler)
    } else if (darkThemeMq.addListener) {
        // Safari and Mac app support
        darkThemeMq.addListener(changeHandler)
    }
    loadTheme()
}
