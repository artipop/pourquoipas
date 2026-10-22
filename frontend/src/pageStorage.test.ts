import {sessionToken} from './session'
import {UserSettingKey, UserSettings} from './userSettings'
import type {IAppWindow} from './types'

describe('pageStorage', () => {
    afterEach(() => {
        delete (window as unknown as IAppWindow).pageOnWails
        localStorage.clear()
    })

    // A fresh install opened on "board not found": the browser still held the
    // last board of a database that was gone.
    test('a desktop window does not take its board or session from the browser', () => {
        localStorage.setItem(UserSettingKey.LastBoardId, 'gone')
        localStorage.setItem('boardSessionId', 'stale')
        const w = window as unknown as IAppWindow
        w.pageOnWails = true
        w.seededSession = 'fresh'

        expect(UserSettings.get(UserSettingKey.LastBoardId)).toBeNull()
        expect(sessionToken()).toBe('fresh')

        UserSettings.set(UserSettingKey.LastBoardId, 'inbox')
        expect(UserSettings.get(UserSettingKey.LastBoardId)).toBe('inbox')
        expect(localStorage.getItem(UserSettingKey.LastBoardId)).toBe('gone')
    })

    test('what is only the page\'s own stays in the browser', () => {
        const w = window as unknown as IAppWindow
        w.pageOnWails = true
        UserSettings.set(UserSettingKey.RandomIcons, 'true')
        expect(localStorage.getItem(UserSettingKey.RandomIcons)).toBe('true')
    })
})
