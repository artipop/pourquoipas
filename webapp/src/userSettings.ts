import {notifySettingsChanged} from './nativeApp'
import {Utils} from './utils'

export enum UserSettingKey {
    Language = 'language',
    Theme = 'theme',

    // Light, dark or whichever the system is set to. Beside `Theme` rather
    // than folded into it, because which design and which polarity are two
    // questions: following the system used to mean giving up every design but
    // the product's own.
    ThemeMode = 'themeMode',

    // The themes a person brought, as a JSON array: id, name and the text of
    // both halves. One key rather than three per theme, because the install
    // reads its settings as a flat map and a list is one value.
    CustomThemes = 'customThemes',

    // What one slot held before the list did. All three are read once at
    // startup and never written again: an install that rolls back to a build
    // before the list still finds its theme where it left it.
    CustomThemeLight = 'customThemeLight',
    CustomThemeDark = 'customThemeDark',
    CustomThemeName = 'customThemeName',
    CustomTheme = 'customTheme',
    LastTeamId = 'lastTeamId',
    LastBoardId = 'lastBoardId',
    LastViewId = 'lastViewId',
    EmojiMartSkin = 'emoji-mart.skin',
    EmojiMartLast = 'emoji-mart.last',
    EmojiMartFrequently = 'emoji-mart.frequently',
    RandomIcons = 'randomIcons',
    MobileWarningClosed = 'mobileWarningClosed',
    WelcomePageViewed = 'welcomePageViewed',
    NameFormat = 'nameFormat',

    // Whether a card whose agent is waiting for an answer says so out loud.
    AgentNotifications = 'agentNotifications',

    // Whether a new card is given the board's folder when the board offers
    // exactly one.
    PrefillCardFolder = 'prefillCardFolder'
}

// The keys the install keeps for itself, in <dataDir>/ui-settings.json on the
// Go side. localStorage is only a boot-time cache for these: the desktop
// window opens on a loopback origin with a random port, and everything the
// browser stored under the previous one is unreachable by the next launch —
// docs/deferred.md, moving off localStorage. The rest of the enum stays
// browser-only on purpose: the emoji-mart keys are the library's own, and
// welcomePageViewed lives in the user's server-side config.
const installKept = new Set<string>([
    UserSettingKey.Language,
    UserSettingKey.Theme,
    UserSettingKey.ThemeMode,
    UserSettingKey.CustomThemes,
    UserSettingKey.CustomThemeLight,
    UserSettingKey.CustomThemeDark,
    UserSettingKey.CustomThemeName,
    UserSettingKey.CustomTheme,
    UserSettingKey.NameFormat,
    UserSettingKey.LastTeamId,
    UserSettingKey.LastBoardId,
    UserSettingKey.LastViewId,
    UserSettingKey.MobileWarningClosed,
    UserSettingKey.AgentNotifications,
    UserSettingKey.PrefillCardFolder,
])

// The Wails bindings, when this bundle runs in the desktop app — the same
// bundle also runs in a plain browser, where localStorage is the whole memory
// and these are absent.
const preferenceBindings = () => (window as any).go?.main?.App

// hydrateUserSettings fills localStorage with what the install remembers,
// before the first render (main.tsx awaits it): the theme and the language
// have to be right on the first paint, not a correction after it.
export async function hydrateUserSettings(): Promise<void> {
    const bindings = preferenceBindings()
    if (!bindings?.GetUIPreferences) {
        return
    }
    try {
        // The Wails-generated Go bindings are PascalCase methods.
        // eslint-disable-next-line new-cap
        const prefs = JSON.parse((await bindings.GetUIPreferences()) || '{}')
        for (const [key, value] of Object.entries(prefs)) {
            if (installKept.has(key) && typeof value === 'string') {
                localStorage.setItem(key, value)
            }
        }
    } catch (e) {
        // A refused read leaves the boot-time guesses in place.
    }
}

export class UserSettings {
    static get(key: UserSettingKey): string | null {
        return localStorage.getItem(key)
    }

    static set(key: UserSettingKey, value: string | null): void {
        if (!Object.values(UserSettingKey).includes(key)) {
            return
        }
        if (value === null) {
            localStorage.removeItem(key)
        } else {
            localStorage.setItem(key, value)
        }

        // Write-through: the localStorage write above answers this session,
        // the install remembers for the next one. Fire and forget — a
        // refused write must not block the switch being made.
        if (installKept.has(key)) {
            try {
                // eslint-disable-next-line new-cap
                preferenceBindings()?.SetUIPreference?.(key, value ?? '')?.catch?.(() => undefined)
            } catch (e) {
                // A binding that throws on the way out rather than in its
                // promise: the install goes without, this session keeps it.
            }
        }
        notifySettingsChanged(key)
    }

    static get language(): string | null {
        return UserSettings.get(UserSettingKey.Language)
    }

    static set language(newValue: string | null) {
        UserSettings.set(UserSettingKey.Language, newValue)
    }

    static get theme(): string | null {
        return UserSettings.get(UserSettingKey.Theme)
    }

    static set theme(newValue: string | null) {
        UserSettings.set(UserSettingKey.Theme, newValue)
    }

    static get themeMode(): string | null {
        return UserSettings.get(UserSettingKey.ThemeMode)
    }

    static set themeMode(newValue: string | null) {
        UserSettings.set(UserSettingKey.ThemeMode, newValue)
    }

    static get customTheme(): string | null {
        return UserSettings.get(UserSettingKey.CustomTheme)
    }

    static set customTheme(newValue: string | null) {
        UserSettings.set(UserSettingKey.CustomTheme, newValue)
    }

    static get customThemes(): string | null {
        return UserSettings.get(UserSettingKey.CustomThemes)
    }

    static set customThemes(newValue: string | null) {
        UserSettings.set(UserSettingKey.CustomThemes, newValue)
    }

    static get customThemeLight(): string | null {
        return UserSettings.get(UserSettingKey.CustomThemeLight)
    }

    static set customThemeLight(newValue: string | null) {
        UserSettings.set(UserSettingKey.CustomThemeLight, newValue)
    }

    static get customThemeDark(): string | null {
        return UserSettings.get(UserSettingKey.CustomThemeDark)
    }

    static set customThemeDark(newValue: string | null) {
        UserSettings.set(UserSettingKey.CustomThemeDark, newValue)
    }

    static get customThemeName(): string | null {
        return UserSettings.get(UserSettingKey.CustomThemeName)
    }

    static set customThemeName(newValue: string | null) {
        UserSettings.set(UserSettingKey.CustomThemeName, newValue)
    }

    static get lastTeamId(): string | null {
        return UserSettings.get(UserSettingKey.LastTeamId)
    }

    static set lastTeamId(newValue: string | null) {
        UserSettings.set(UserSettingKey.LastTeamId, newValue)
    }

    // maps last board ID for each team
    // maps teamID -> board ID
    static get lastBoardId(): {[key: string]: string} {
        let rawData = UserSettings.get(UserSettingKey.LastBoardId) || '{}'
        if (rawData[0] !== '{') {
            rawData = '{}'
        }

        let mapping: {[key: string]: string}
        try {
            mapping = JSON.parse(rawData)
        } catch {
            // revert to empty data if JSON conversion fails.
            // This will happen when users run the new code for the first time
            mapping = {}
        }

        return mapping
    }

    static setLastTeamID(teamID: string | null): void {
        UserSettings.set(UserSettingKey.LastTeamId, teamID)
    }

    static setLastBoardID(teamID: string, boardID: string | null): void {
        const data = this.lastBoardId
        if (boardID === null) {
            delete data[teamID]
        } else {
            data[teamID] = boardID
        }
        UserSettings.set(UserSettingKey.LastBoardId, JSON.stringify(data))
    }

    static get lastViewId(): {[key: string]: string} {
        const rawData = UserSettings.get(UserSettingKey.LastViewId) || '{}'
        let mapping: {[key: string]: string}
        try {
            mapping = JSON.parse(rawData)
        } catch {
            // revert to empty data if JSON conversion fails.
            // This will happen when users run the new code for the first time
            mapping = {}
        }

        return mapping
    }

    static setLastViewId(boardID: string, viewID: string | null): void {
        const data = this.lastViewId
        if (viewID === null) {
            delete data[boardID]
        } else {
            data[boardID] = viewID
        }
        UserSettings.set(UserSettingKey.LastViewId, JSON.stringify(data))
    }

    // On unless it has been turned off: an agent that has stopped to ask
    // something is worth interrupting for, and somebody who disagrees says so
    // once.
    static get agentNotifications(): boolean {
        return UserSettings.get(UserSettingKey.AgentNotifications) !== 'false'
    }

    static set agentNotifications(newValue: boolean) {
        UserSettings.set(UserSettingKey.AgentNotifications, JSON.stringify(newValue))
    }

    // On unless it has been turned off: a board with one folder has one answer
    // to where to work, and making a person give it on every card is asking a
    // question that has no second option. What is written is an ordinary value
    // on the card, so it is visible and can be changed or cleared — which is
    // why it is safe to write it without being asked.
    static get prefillCardFolder(): boolean {
        return UserSettings.get(UserSettingKey.PrefillCardFolder) !== 'false'
    }

    static set prefillCardFolder(newValue: boolean) {
        UserSettings.set(UserSettingKey.PrefillCardFolder, JSON.stringify(newValue))
    }

    // Off unless somebody turns it on by hand. A card gets an emoji nobody
    // chose, and on a board of case files it reads as noise: the icon is a
    // thing a person picks when it means something. Nothing in the interface
    // switches it any more — the icon picker still has «Random» for one card —
    // so this is read, and never written.
    static get prefillRandomIcons(): boolean {
        return UserSettings.get(UserSettingKey.RandomIcons) === 'true'
    }

    static getEmojiMartSetting(key: string): any {
        const prefixed = `emoji-mart.${key}`
        Utils.assert((Object as any).values(UserSettingKey).includes(prefixed))
        const json = UserSettings.get(prefixed as UserSettingKey)
        return json ? JSON.parse(json) : null
    }

    static setEmojiMartSetting(key: string, value: any): void {
        const prefixed = `emoji-mart.${key}`
        Utils.assert((Object as any).values(UserSettingKey).includes(prefixed))
        UserSettings.set(prefixed as UserSettingKey, JSON.stringify(value))
    }

    static get mobileWarningClosed(): boolean {
        return UserSettings.get(UserSettingKey.MobileWarningClosed) === 'true'
    }

    static set mobileWarningClosed(newValue: boolean) {
        UserSettings.set(UserSettingKey.MobileWarningClosed, String(newValue))
    }

    static get nameFormat(): string | null {
        return UserSettings.get(UserSettingKey.NameFormat)
    }

    static set nameFormat(newValue: string | null) {
        UserSettings.set(UserSettingKey.NameFormat, newValue)
    }
}

// base64 of the settings as UTF-8 bytes, and not of the string itself: `btoa`
// takes one byte per character and throws `InvalidCharacterError` on anything
// above U+00FF. Nothing kept here had held free text until a theme could be
// given a name, and `notifySettingsChanged` builds this blob on *every* write,
// before it checks whether there is a native host to send it to — so naming a
// theme «Ночная смена» threw on each keystroke. ASCII encodes identically
// either way, so a host that was reading these before reads the same bytes.
export function exportUserSettingsBlob(): string {
    const bytes = new TextEncoder().encode(exportUserSettings())
    let binary = ''
    for (const byte of bytes) {
        binary += String.fromCharCode(byte)
    }
    return window.btoa(binary)
}

function exportUserSettings(): string {
    const keys = Object.values(UserSettingKey)
    const settings = Object.fromEntries(keys.map((key) => [key, localStorage.getItem(key)]))
    settings.timestamp = `${Date.now()}`
    return JSON.stringify(settings)
}

export function importUserSettingsBlob(blob: string): string[] {
    const binary = window.atob(blob)
    const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0))

    return importUserSettings(new TextDecoder().decode(bytes))
}

function importUserSettings(json: string): string[] {
    const settings = parseUserSettings(json)
    if (!settings) {
        return []
    }
    const timestamp = settings.timestamp
    const lastTimestamp = localStorage.getItem('timestamp')
    if (!timestamp || (lastTimestamp && Number(timestamp) <= Number(lastTimestamp))) {
        return []
    }
    const importedKeys = []
    for (const [key, value] of Object.entries(settings)) {
        if (Object.values(UserSettingKey).includes(key as UserSettingKey)) {
            if (value) {
                localStorage.setItem(key, value as string)
            } else {
                localStorage.removeItem(key)
            }
            importedKeys.push(key)
        }
    }
    return importedKeys
}

function parseUserSettings(json: string): any {
    try {
        return JSON.parse(json)
    } catch (e) {
        return undefined
    }
}
