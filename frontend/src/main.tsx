import {render} from '@solidjs/web'

import App from './app'
import {installPolyfills} from './polyfills'
import {initThemes} from './theme'
import {importNativeAppSettings} from './nativeApp'
import {hydrateUserSettings, reportLanguage} from './userSettings'
import {reportWords} from './goWords'
import {getCurrentLanguage, loadMessages} from './i18n'

import {IUser} from './user'
import {getMe} from './store/users'
import {useAppSelector} from './store/hooks'
import {AppStoreProvider, createAppStore} from './store'
import mutator from './mutator'
import {installGoBridge} from './goBridge'

import '@mattermost/compass-icons/css/compass-icons.css'

// The product's two typefaces, self-hosted: the desktop build has no network,
// and the page must look the same on every platform. Both are variable, so each
// carries every weight the design uses in one file per subset, and the browser
// fetches only the subsets the page needs.
//
// Cyrillic is what narrowed the field to these two. The interface is Russian,
// and most condensed grotesques on offer — IBM Plex Sans Condensed among them —
// ship only `cyrillic-ext`, which does not cover the basic Cyrillic letters.
import '@fontsource/fira-sans-condensed/500.css'
import '@fontsource/fira-sans-condensed/600.css'
import '@fontsource-variable/inter'
import '@fontsource-variable/playfair-display/wght-italic.css'
import '@fontsource-variable/jetbrains-mono'

import './styles/variables.scss'
import './styles/main.scss'
import './styles/labels.scss'
import './styles/_markdown.scss'

import WithWebSockets from './components/withWebSockets'

// Before anything else: what a library calls unconditionally and this webview
// does not have (polyfills.ts says which, and what it costs when it is missing).
installPolyfills()

importNativeAppSettings()

const MainApp = () => {
    const me = useAppSelector<IUser|null>(getMe)

    return (
        <WithWebSockets userId={me()?.id}>
            <App/>
        </WithWebSockets>
    )
}

// The first paint waits for the install's own memory: the desktop window's
// localStorage is empty on every launch (a loopback origin with a random
// port), and the theme and the language have to be right when the page
// appears, not corrected after it. One binding call over loopback; a build
// with no Go side resolves it at once.
async function boot() {
    installGoBridge()

    await hydrateUserSettings()
    initThemes()

    // Which language this page settled on — the pick, else the webview's own
    // locale. The Go side cannot read the second one, and it installs the board
    // templates in a language (internal/boardadapter/templatetext.go). Awaited,
    // because it can mean those boards are replaced and everything read before
    // that is stale.
    await Promise.all([reportLanguage(getCurrentLanguage()), loadMessages(getCurrentLanguage())])

    // And the words Go draws for us — the menu bar, notifications, window
    // titles — in the same language. Not awaited: nothing on the page waits
    // for them.
    reportWords(getCurrentLanguage())

    // The store is created here, not in a module singleton, and everything
    // that writes to it from outside the component tree — the Mutator — gets
    // its actions handed over before the first render.
    const store = createAppStore()
    mutator.setActions(store.actions)

    render(() => (
        <AppStoreProvider store={store}>
            <MainApp/>
        </AppStoreProvider>
    ), document.getElementById('app-root')!)
}

boot()
