import {createSignal} from 'solid-js'

import {goApp} from './goBridge'
import {notifySettingsChanged} from './nativeApp'
import octoClient from './octoClient'
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

    // Whether the app runs agents at all. Off, it is a kanban board and
    // nothing on screen mentions an agent (aiSwitch.ts); the Go side reads the
    // same key to stop starting stages (machinesettings.go).
    AIEnabled = 'aiEnabled',

    // Whether a card whose agent is waiting for an answer says so out loud.
    AgentNotifications = 'agentNotifications',

    // Whether a new card is given the board's folder when the board offers
    // exactly one.
    PrefillCardFolder = 'prefillCardFolder'
}

// A person's settings are theirs, kept in the board's preferences
// (/users/me/config): on a server everybody has their own and takes them from
// device to device, and on a desktop they sit in the same database as the
// boards the last-board keys point at. The machine's switches are the
// install's (machinesettings.go): a desktop attached to a server must not turn
// off somebody else's agents. Both are held here in memory, filled before the
// first render; the rest of the enum is the browser's own and stays in
// localStorage — the emoji-mart keys are the library's, welcomePageViewed is
// read off the user's config by the store.
const personal = new Set<string>([
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
])

const ofTheMachine = new Set<string>([
    UserSettingKey.AIEnabled,
    UserSettingKey.AgentNotifications,
    UserSettingKey.PrefillCardFolder,
])

const kept = new Map<string, string>()

// Ticks when the settings are read afresh — after a login or a logout — so a
// tracked reader asks again.
const [hydrated, setHydrated] = createSignal(0)

function keptHere(key: string): boolean {
    return personal.has(key) || ofTheMachine.has(key)
}

function read(key: string): string | null {
    if (keptHere(key)) {
        hydrated()
        return kept.get(key) ?? null
    }
    return localStorage.getItem(key)
}

function put(key: string, value: string | null): void {
    if (!keptHere(key)) {
        if (value === null) {
            localStorage.removeItem(key)
        } else {
            localStorage.setItem(key, value)
        }
    } else if (value === null) {
        kept.delete(key)
    } else {
        kept.set(key, value)
    }
}

// hydrateUserSettings reads both halves before the first render (main.tsx
// awaits it): the theme and the language have to be right on the first paint.
// Without a session — a server's login page — the person's half is empty and
// the defaults stand until the login reads it again.
export async function hydrateUserSettings(): Promise<void> {
    const [mine, machine] = await Promise.all([
        octoClient.getMyConfig().catch(() => undefined),
        loadMachineSettings(),
    ])
    kept.clear()
    for (const pref of mine ?? []) {
        if (personal.has(pref.name) && typeof pref.value === 'string') {
            kept.set(pref.name, pref.value)
        }
    }
    if (machine) {
        kept.set(UserSettingKey.AIEnabled, String(machine.aiEnabled))
        kept.set(UserSettingKey.AgentNotifications, String(machine.agentNotifications))
        kept.set(UserSettingKey.PrefillCardFolder, String(machine.prefillCardFolder))
    }
    setHydrated((n) => n + 1)
}

async function loadMachineSettings() {
    try {
        // eslint-disable-next-line new-cap
        return await goApp()?.GetMachineSettings?.()
    } catch (e) {
        // Refused — a server's guard before the login — or no Go side: the
        // switches are on, which is what they are until turned off.
        return undefined
    }
}

// Fire and forget: the switch on screen is made by the write to memory, and a
// refused write must not undo it.
function keep(key: string, value: string | null): void {
    try {
        if (personal.has(key)) {
            const patch = value === null ? {deletedFields: [key]} : {updatedFields: {[key]: value}}
            octoClient.patchUserConfig('me', patch).catch(() => undefined)
        } else if (ofTheMachine.has(key)) {
            // eslint-disable-next-line new-cap
            goApp()?.SetMachineSetting?.(key, value !== 'false')?.catch?.(() => undefined)
        }
    } catch (e) {
        // A client that throws on the way out rather than in its promise.
    }
}

// reportLanguage tells the install which language the page is showing, picked
// or not: the shipped board templates are boards rather than catalogue
// strings, so the Go side installs them in a language — and the webview's own
// locale, which is the answer when nobody picked, is something only this side
// can read. It stores nothing; a resolved default is not a choice.
//
// Awaited before the first render, because the answer can mean five boards are
// replaced: a page that has already read the old ones holds boards that no
// longer exist, and the template picker drew both languages at once. It resolves
// at once where there is no Go side, and where nothing changed.
export async function reportLanguage(lang: string): Promise<void> {
    try {
        // eslint-disable-next-line new-cap
        await goApp()?.UseLanguage?.(lang)
    } catch (e) {
        // No Go side, or a binding that threw: the templates stay in the
        // language they were installed in.
    }
}

export class UserSettings {
    static get(key: UserSettingKey): string | null {
        return read(key)
    }

    static set(key: UserSettingKey, value: string | null): void {
        if (!Object.values(UserSettingKey).includes(key)) {
            return
        }
        put(key, value)
        keep(key, value)
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

    // On unless it has been turned off, the rule aiEnabled in
    // machinesettings.go states on the other side: an install that has always run agents keeps
    // running them until somebody says otherwise.
    static get aiEnabled(): boolean {
        return UserSettings.get(UserSettingKey.AIEnabled) !== 'false'
    }

    static set aiEnabled(newValue: boolean) {
        UserSettings.set(UserSettingKey.AIEnabled, JSON.stringify(newValue))
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
    const settings = Object.fromEntries(keys.map((key) => [key, read(key)]))
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
            put(key, value ? value as string : null)
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
