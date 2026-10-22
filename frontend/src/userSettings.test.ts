// A person's settings are kept with the person (/users/me/config), the
// machine's switches with the install (Go). Both are held in memory and read
// before the first render; nothing of either is left in the browser.

import octoClient from './octoClient'
import {
    UserSettingKey,
    UserSettings,
    exportUserSettingsBlob,
    hydrateUserSettings,
    importUserSettingsBlob,
} from './userSettings'
import {fakeGoSide} from './test/goSide'

vi.mock('./octoClient')
const client = vi.mocked(octoClient)

const pref = (name: string, value: string) => ({user_id: 'me', category: 'focalboard', name, value})

describe('userSettings', () => {
    beforeEach(() => {
        client.patchUserConfig.mockResolvedValue([])
    })

    afterEach(async () => {
        fakeGoSide()
        client.getMyConfig.mockResolvedValue([])
        await hydrateUserSettings()
        localStorage.clear()
        vi.clearAllMocks()
    })

    test('the person\'s settings come from their config', async () => {
        client.getMyConfig.mockResolvedValue([pref('language', 'de'), pref('theme', 'paper')])

        await hydrateUserSettings()

        expect(UserSettings.language).toBe('de')
        expect(UserSettings.theme).toBe('paper')
    })

    // The config also holds what the store reads for itself — the tour, the
    // hidden boards — and none of it is a UI setting.
    test('only the keys that are UI settings are taken', async () => {
        client.getMyConfig.mockResolvedValue([pref('hiddenBoardIDs', '[]'), pref('aiEnabled', 'false')])

        await hydrateUserSettings()

        expect(UserSettings.get('hiddenBoardIDs' as UserSettingKey)).toBeNull()
        expect(UserSettings.aiEnabled).toBe(true)
    })

    test('a change is kept with the person', () => {
        UserSettings.language = 'ru'
        UserSettings.theme = null

        expect(client.patchUserConfig).toHaveBeenCalledWith('me', {updatedFields: {language: 'ru'}})
        expect(client.patchUserConfig).toHaveBeenCalledWith('me', {deletedFields: ['theme']})
    })

    // A desktop attached to a server must not switch off somebody else's
    // agents: the machine's switches are the install's, not the person's.
    test('the machine\'s switches are the install\'s', async () => {
        const SetMachineSetting = vi.fn().mockResolvedValue(undefined)
        fakeGoSide({
            GetMachineSettings: vi.fn().mockResolvedValue({aiEnabled: false, agentNotifications: true, prefillCardFolder: true}),
            SetMachineSetting,
        })

        await hydrateUserSettings()
        expect(UserSettings.aiEnabled).toBe(false)

        UserSettings.prefillCardFolder = false
        expect(SetMachineSetting).toHaveBeenCalledWith(UserSettingKey.PrefillCardFolder, false)
        expect(client.patchUserConfig).not.toHaveBeenCalled()
    })

    // wails:// storage outlives the app and its database: a board remembered
    // there opened a fresh install on "board not found".
    test('nothing kept for the person or the machine is read from the browser', async () => {
        localStorage.setItem(UserSettingKey.LastBoardId, 'gone')
        localStorage.setItem(UserSettingKey.AIEnabled, 'false')

        await hydrateUserSettings()

        expect(UserSettings.get(UserSettingKey.LastBoardId)).toBeNull()
        expect(UserSettings.aiEnabled).toBe(true)
    })

    test('what is only the browser\'s own stays in the browser', () => {
        UserSettings.set(UserSettingKey.RandomIcons, 'true')

        expect(localStorage.getItem(UserSettingKey.RandomIcons)).toBe('true')
        expect(client.patchUserConfig).not.toHaveBeenCalled()
    })

    // A server's login page has no session: the defaults stand, and the page
    // still draws.
    test('a refused read leaves the defaults', async () => {
        client.getMyConfig.mockRejectedValue(new Error('401'))
        fakeGoSide({GetMachineSettings: vi.fn().mockRejectedValue(new Error('not logged in'))})

        await hydrateUserSettings()

        expect(UserSettings.language).toBeNull()
        expect(UserSettings.aiEnabled).toBe(true)
    })

    // The blob is base64 of the settings, and it is built on every write —
    // before `notifySettingsChanged` even checks whether a native host is
    // listening. `btoa` takes one byte per character, so until a theme could
    // be given a name nothing here had ever held text above U+00FF, and the
    // first one that did threw on each keystroke.
    test('carries a setting written in a language btoa cannot spell', () => {
        UserSettings.set(UserSettingKey.CustomThemeName, 'Ночная смена')

        const blob = exportUserSettingsBlob()
        UserSettings.set(UserSettingKey.CustomThemeName, null)
        importUserSettingsBlob(blob)

        expect(UserSettings.get(UserSettingKey.CustomThemeName)).toBe('Ночная смена')
    })

    // A write runs inside whatever the person is doing — typing a name, ticking
    // a switch. A server that refuses must not take that down with it.
    test('keeps the setting when the write is refused', () => {
        client.patchUserConfig.mockImplementation(() => {
            throw new Error('no server')
        })

        expect(() => UserSettings.set(UserSettingKey.CustomThemeName, 'Тёмная ночь')).not.toThrow()
        expect(UserSettings.get(UserSettingKey.CustomThemeName)).toBe('Тёмная ночь')
    })
})
