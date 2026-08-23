// The install keeps its own UI preferences (ui-settings.json on the Go side):
// the desktop window opens on a loopback origin with a random port, so
// localStorage alone forgot everything on every launch. localStorage is now a
// boot-time cache, hydrated before the first render and written through on
// every change.

import {
    UserSettingKey,
    UserSettings,
    exportUserSettingsBlob,
    hydrateUserSettings,
    importUserSettingsBlob,
} from './userSettings'

const anyWindow = window as any

describe('userSettings and the install’s own memory', () => {
    afterEach(() => {
        delete anyWindow.go
        localStorage.clear()
        vi.clearAllMocks()
    })

    test('hydration fills localStorage with what the install remembers', async () => {
        anyWindow.go = {main: {App: {
            GetUIPreferences: vi.fn().mockResolvedValue(JSON.stringify({language: 'de', theme: 'paper'})),
        }}}

        await hydrateUserSettings()

        expect(UserSettings.language).toBe('de')
        expect(UserSettings.theme).toBe('paper')
    })

    // The board's session token lives in localStorage too, and it must never
    // wander into a settings file: only the keys the install deliberately
    // keeps come through.
    test('hydration takes only the keys the install keeps', async () => {
        anyWindow.go = {main: {App: {
            GetUIPreferences: vi.fn().mockResolvedValue(JSON.stringify({boardSessionId: 'stolen', language: 'ru'})),
        }}}

        await hydrateUserSettings()

        expect(localStorage.getItem('boardSessionId')).toBeNull()
        expect(UserSettings.language).toBe('ru')
    })

    test('a change is written through to the install', () => {
        const set = vi.fn().mockResolvedValue(undefined)
        anyWindow.go = {main: {App: {SetUIPreference: set}}}

        UserSettings.language = 'ru'
        UserSettings.theme = 'screen'

        expect(set).toHaveBeenCalledWith('language', 'ru')
        expect(set).toHaveBeenCalledWith('theme', 'screen')
    })

    test('forgetting a setting forgets it in the install too', () => {
        const set = vi.fn().mockResolvedValue(undefined)
        anyWindow.go = {main: {App: {SetUIPreference: set}}}

        UserSettings.language = null

        expect(set).toHaveBeenCalledWith('language', '')
    })

    // welcomePageViewed lives in the user's server-side config, emoji-mart's
    // keys are the library's own: neither is the install's business.
    test('keys the install does not keep are not sent', () => {
        const set = vi.fn().mockResolvedValue(undefined)
        anyWindow.go = {main: {App: {SetUIPreference: set}}}

        UserSettings.set(UserSettingKey.WelcomePageViewed, '1')

        expect(set).not.toHaveBeenCalled()
    })

    // The same bundle runs in a plain browser,
    // where there is no Go side at all: localStorage stays the whole memory.
    test('without bindings, localStorage is the whole memory', async () => {
        await hydrateUserSettings()
        UserSettings.language = 'sv'

        expect(UserSettings.language).toBe('sv')
    })

    // The blob is base64 of the settings, and it is built on every write —
    // before `notifySettingsChanged` even checks whether a native host is
    // listening. `btoa` takes one byte per character, so until a theme could
    // be given a name nothing here had ever held text above U+00FF, and the
    // first one that did threw on each keystroke.
    test('carries a setting written in a language btoa cannot spell', () => {
        UserSettings.set(UserSettingKey.CustomThemeName, 'Ночная смена')

        const blob = exportUserSettingsBlob()
        localStorage.clear()
        importUserSettingsBlob(blob)

        expect(UserSettings.get(UserSettingKey.CustomThemeName)).toBe('Ночная смена')
    })
})
