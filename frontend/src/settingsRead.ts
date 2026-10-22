import {reportWords} from './goWords'
import {getCurrentLanguage, loadMessages} from './i18n'
import {initThemes} from './theme'
import {hydrateUserSettings, reportLanguage} from './userSettings'

// readSettings reads whose settings the page shows and puts them on screen:
// before the first render, and again when somebody logs in or out on a server,
// where the login page was drawn with nobody's.
export async function readSettings(): Promise<void> {
    await hydrateUserSettings()
    initThemes()

    // Which language this page settled on — the pick, else the webview's own
    // locale. The Go side cannot read the second one, and it installs the board
    // templates in a language (internal/boardadapter/templatetext.go). Awaited,
    // because it can mean those boards are replaced and everything read before
    // that is stale.
    const lang = getCurrentLanguage()
    await Promise.all([reportLanguage(lang), loadMessages(lang)])

    // And the words Go draws for us — the menu bar, notifications, window
    // titles — in the same language. Not awaited: nothing on the page waits
    // for them.
    reportWords(lang)
}
