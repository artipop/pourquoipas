import {render} from '@solidjs/web'

import App from './app'
import {installPolyfills} from './polyfills'
import {importNativeAppSettings} from './nativeApp'
import {readSettings} from './settingsRead'

import {IUser} from './user'
import {getMe} from './store/users'
import {useAppSelector} from './store/hooks'
import {AppStoreProvider, createAppStore} from './store'
import mutator from './mutator'
import {installGoBridge} from './goBridge'
import {onBoardUsersChanged} from './extensions'
import {installAgentFeatures} from './components/acp/install'

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

// The first paint waits for the settings: the theme and the language have to
// be right when the page appears.
async function boot() {
    installGoBridge()

    await readSettings()

    // The store is created here, not in a module singleton, and everything
    // that writes to it from outside the component tree — the Mutator — gets
    // its actions handed over before the first render.
    const store = createAppStore()
    mutator.setActions(store.actions)

    installAgentFeatures()
    onBoardUsersChanged(() => store.actions.boards.fetchExtraBoardUsers())

    render(() => (
        <AppStoreProvider store={store}>
            <MainApp/>
        </AppStoreProvider>
    ), document.getElementById('app-root')!)
}

boot()
