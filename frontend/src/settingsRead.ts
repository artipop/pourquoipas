import {reportWords} from './goWords'
import {getCurrentLanguage, loadMessages} from './i18n'
import {initThemes} from './theme'
import {hydrateUserSettings} from './userSettings'
import {loadAppWords} from './appWords'

// readSettings reads whose settings the page shows and puts them on screen:
// before the first render, and again when somebody logs in or out on a server,
// where the login page was drawn with nobody's.
export async function readSettings(): Promise<void> {
    await hydrateUserSettings()
    initThemes()

    // Which language this page settled on — the pick, else the webview's own
    // locale — and the words of the boards the app made, which arrive
    // translated into it (appWords.ts). Awaited, because a board read before
    // the words are here is drawn in English.
    const lang = getCurrentLanguage()
    await Promise.all([loadAppWords(lang), loadMessages(lang)])

    // And the words Go draws for us — the menu bar, notifications, window
    // titles — in the same language. Not awaited: nothing on the page waits
    // for them.
    reportWords(lang)
}
