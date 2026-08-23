import {UserSettings} from '../userSettings'

import {parseTheme, ParsedTheme, ThemeConfig, ThemeConfigError} from './config'
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

// A pasted config has no family and there is one slot for it: pasting a second
// replaces the first, the way the field it is pasted into does. It is one
// config rather than a pair, so it brings its own polarity and the switch does
// not apply to it.
export const customThemeName = 'custom-theme'

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

// The custom slot's own config, as text, because that is what a person edits.
// It is kept whether or not it is the theme in use, so switching to a shipped
// theme and back does not lose what was pasted.
export function getCustomThemeSource(): string {
    return UserSettings.customTheme ?? ''
}

export function setCustomThemeSource(source: string): void {
    UserSettings.customTheme = source || null
}

function wantsDark(mode: ThemeMode): boolean {
    // Asked only when it is the answer: `matchMedia` is one of the things a
    // jsdom without it does not have, and a stated polarity has no business
    // consulting the machine's preference.
    return mode === 'dark' || (mode === 'system' && prefersDark())
}

// resolveTheme answers with the config that is actually going to be drawn.
function resolveTheme(familyId: ThemeName, mode: ThemeMode): ParsedTheme {
    if (familyId === customThemeName) {
        return parseTheme(getCustomThemeSource(), customThemeName)
    }
    const family = themeFamily(familyId) ?? themeFamily(defaultFamilyId)!
    const config = wantsDark(mode) ? family.dark : family.light

    return parseTheme(config, config.id ?? family.id)
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

    // The custom slot is one config carrying its own polarity, so there is
    // nothing to redraw — and redrawing it would mean parsing a config that
    // may not parse, over a question that was not about it.
    if (activeFamilyId !== customThemeName) {
        apply(activeFamilyId, activeMode)
    }
    return mode
}

export function getActiveThemeName(): ThemeName {
    return activeFamilyId
}

export function getActiveThemeMode(): ThemeMode {
    return activeMode
}

// The config the product is drawing right now, formatted the way the field
// wants it back. This is what the custom slot opens on, and it is the reason a
// new theme is written by editing an old one rather than from nothing.
export function activeThemeConfig(): ThemeConfig {
    return resolveTheme(activeFamilyId, activeMode).config
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

    if (themeFamily(stored)) {
        activeFamilyId = stored
    } else if (stored === customThemeName && getCustomThemeSource()) {
        activeFamilyId = customThemeName
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
