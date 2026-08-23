import {UserSettings} from '../userSettings'

import {parseTheme, ParsedTheme, ThemeConfig, ThemeConfigError} from './config'
import {shippedTheme, SHIPPED_THEMES, THEME_GROUPS, ThemeGroup, ThemeGroupId} from './themes'
import {themeTokens} from './tokens'

// Which theme applies, and the applying of it.
//
// A theme is a config now (`config.ts`), and applying one is writing the raw
// tokens it produces onto the document element. `styles/_tokens.scss` still
// declares the two shipped palettes at the top of its file: that is what the
// first paint uses, before this module has run, and it is the copy
// `themes.test.ts` checks the two configs against — one of them being wrong is
// a test failure rather than a flash of the wrong colours.

export const lightThemeName = 'light-theme'
export const darkThemeName = 'dark-theme'
export const systemThemeName = 'system-theme'

// A pasted config has no id of its own and there is one slot for it: pasting a
// second replaces the first, the way the field it is pasted into does.
export const customThemeName = 'custom-theme'

// The board used to ship a fourth theme, `default-theme`, that differed from
// `light-theme` only in the colour of the sidebar. It is gone, and the name
// stays as an alias so a stored setting naming it still resolves.
export const defaultThemeName = lightThemeName

export type ThemeName = string

export {THEME_GROUPS, SHIPPED_THEMES, ThemeConfigError}
export type {ThemeConfig, ThemeGroup, ThemeGroupId}

let activeThemeName: ThemeName = systemThemeName

function prefersDark(): boolean {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function systemTheme(): ThemeName {
    return prefersDark() ? darkThemeName : lightThemeName
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

// resolveTheme answers with the config that is actually going to be drawn.
// `system` is not a third palette: it is whichever of the two shipped ones the
// machine is set to, resolved at paint time.
function resolveTheme(name: ThemeName): ParsedTheme {
    if (name === customThemeName) {
        return parseTheme(getCustomThemeSource(), customThemeName)
    }

    // Asked only when it is the answer: `matchMedia` is one of the things a
    // jsdom without it does not have, and a named theme has no business
    // consulting the machine's preference.
    const id = name === systemThemeName ? systemTheme() : name
    const config = shippedTheme(id) ?? shippedTheme(lightThemeName)!

    return parseTheme(config, config.id ?? lightThemeName)
}

function apply(name: ThemeName): void {
    const theme = resolveTheme(name)
    const root = document.documentElement

    // `data-theme` is what `_tokens.scss` switches on, and it stays because
    // the terminal page and a handful of rules ask which side of the line they
    // are on rather than for a colour.
    root.dataset.theme = theme.dark ? 'dark' : 'light'

    // Told, or the browser paints its own widgetry — a select's list, a
    // checkbox, a scrollbar — white on a black theme.
    root.style.colorScheme = theme.dark ? 'dark' : 'light'

    for (const [token, value] of Object.entries(themeTokens(theme))) {
        root.style.setProperty(token, value)
    }
}

export function setTheme(name: ThemeName): ThemeName {
    activeThemeName = name
    UserSettings.theme = name
    apply(name)
    return name
}

export function getActiveThemeName(): ThemeName {
    return activeThemeName
}

// The config the product is drawing right now, formatted the way the field
// wants it back. This is what the custom slot opens on, and it is the reason a
// new theme is written by editing an old one rather than from nothing.
export function activeThemeConfig(): ThemeConfig {
    return resolveTheme(activeThemeName).config
}

export function themeConfigText(config: ThemeConfig): string {
    return `${JSON.stringify(config, null, 2)}\n`
}

// What is stored is the *name* of a theme, or `custom-theme` with the config
// kept beside it. It used to be a JSON snapshot of every colour, which would
// now be a blob of dead Mattermost values laid inline over the tokens — so
// anything that is not a name this build knows is read as `system`, and so is
// a custom slot whose config has stopped parsing.
export function loadTheme(): ThemeName {
    const stored = UserSettings.theme ?? ''
    let name: ThemeName = systemThemeName
    if (stored === systemThemeName || shippedTheme(stored)) {
        name = stored
    } else if (stored === customThemeName && getCustomThemeSource()) {
        name = customThemeName
    }
    activeThemeName = name
    try {
        apply(name)
    } catch {
        // A custom config that no longer parses must not leave the page
        // unpainted; the shipped default is the answer, and the text stays in
        // the field for whoever has to fix it.
        activeThemeName = systemThemeName
        apply(systemThemeName)
    }
    return activeThemeName
}

export function initThemes(): void {
    const darkThemeMq = window.matchMedia('(prefers-color-scheme: dark)')
    const changeHandler = () => {
        if (activeThemeName === systemThemeName) {
            apply(systemThemeName)
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
