import {UserSettings} from '../userSettings'

import {ThemeConfig} from './config'

// The themes a person brings.
//
// There was one slot for a pasted config, and "pasting a second replaces the
// first" was a simplification that stopped being one the moment somebody
// wanted two — a theme you wrote is a theme, and the list holds themes.
//
// A theme is kept as the **text** of its two halves, not as parsed configs:
// the field edits text, and a config that came back reindented or reordered is
// not the one that was pasted. What it costs is that a half is parsed on every
// paint; what it buys is that the field and the store hold the same thing.

export type CustomTheme = {
    id: string
    name: string
    light: string
    dark: string
}

// The id the single slot answered to. The first theme keeps it, so an install
// whose `theme` setting names it needs no migrating.
export const firstCustomId = 'custom-theme'

const PREFIX = 'custom:'

function read(): CustomTheme[] {
    const raw = UserSettings.customThemes
    if (!raw) {
        return []
    }
    try {
        const parsed = JSON.parse(raw)
        if (!Array.isArray(parsed)) {
            return []
        }
        return parsed.filter(looksLikeTheme)
    } catch {
        // A store that will not parse is a store that is not there. It is
        // rewritten by the next edit rather than repaired: there is nothing in
        // half a JSON array worth keeping.
        return []
    }
}

function looksLikeTheme(value: unknown): value is CustomTheme {
    const theme = value as CustomTheme
    return Boolean(theme) && typeof theme === 'object' &&
        typeof theme.id === 'string' && typeof theme.name === 'string' &&
        typeof theme.light === 'string' && typeof theme.dark === 'string'
}

function write(themes: CustomTheme[]): void {
    UserSettings.customThemes = themes.length ? JSON.stringify(themes) : null
}

export function customThemes(): CustomTheme[] {
    return read()
}

export function customTheme(id: string): CustomTheme | undefined {
    return read().find((theme) => theme.id === id)
}

export function isCustomThemeId(id: string): boolean {
    return id === firstCustomId || id.startsWith(PREFIX)
}

// Ids are minted rather than counted from the length: a list two of whose
// themes have been deleted would otherwise mint an id one of the survivors
// already holds.
function nextId(themes: CustomTheme[]): string {
    if (!themes.some((theme) => theme.id === firstCustomId)) {
        return firstCustomId
    }
    const taken = new Set(themes.map((theme) => theme.id))
    for (let n = 2; ; n++) {
        const id = `${PREFIX}${n}`
        if (!taken.has(id)) {
            return id
        }
    }
}

export function addCustomTheme(seed: {light: ThemeConfig, dark: ThemeConfig}, format: (config: ThemeConfig) => string): CustomTheme {
    const themes = read()
    const id = nextId(themes)
    const theme: CustomTheme = {
        id,
        name: '',
        light: format({...seed.light, id, name: undefined}),
        dark: format({...seed.dark, id, name: undefined}),
    }
    write([...themes, theme])

    return theme
}

export function updateCustomTheme(id: string, patch: Partial<Omit<CustomTheme, 'id'>>): void {
    write(read().map((theme) => (theme.id === id ? {...theme, ...patch} : theme)))
}

export function removeCustomTheme(id: string): void {
    write(read().filter((theme) => theme.id !== id))
}

// What the single slot left behind, folded into the list once. Both spellings
// of it: the pair it ended as, and the one config it began as. The old keys are
// not cleared — an install that rolls back to a build before this still finds
// its theme where it left it.
export function adoptSingleSlot(isDark: (source: string) => boolean): void {
    if (read().length > 0) {
        return
    }
    const light = UserSettings.customThemeLight ?? ''
    const dark = UserSettings.customThemeDark ?? ''
    const single = UserSettings.customTheme ?? ''
    if (!light && !dark && !single) {
        return
    }
    const theme: CustomTheme = {
        id: firstCustomId,
        name: UserSettings.customThemeName ?? '',
        light,
        dark,
    }
    if (single && !light && !dark) {
        theme[isDark(single) ? 'dark' : 'light'] = single
    }
    write([theme])
}
