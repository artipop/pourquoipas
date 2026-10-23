import {Component, onSettled} from 'solid-js'

import {getMessages} from './i18n'
import {IntlProvider} from './intl'
import {SortableProvider} from './hooks/sortable'
import {FlashMessages} from './components/flashMessages'
import MoveCardToBoard from './components/moveCardToBoard'
import {ExtensionDialogs, ExtensionSlot} from './extensions'
import {getLanguage} from './store/language'
import {useAppSelector, useAppStore} from './store/hooks'
import AppRouter from './router'

const App: Component = () => {
    const {actions} = useAppStore()
    const language = useAppSelector<string>(getLanguage)

    onSettled(() => {
        actions.language.fetchLanguage()
        actions.users.fetchMe()
        actions.clientConfig.fetchClientConfig()
    })

    return (
        <IntlProvider
            locale={language().split(/[_]/)[0]}
            messages={getMessages(language())}
        >
            <SortableProvider>
                <FlashMessages milliseconds={2000}/>

                {/* What features draw over the whole window, and the dialogs
                    their menu items open (extensions.tsx). */}
                <ExtensionSlot name='app'/>
                <ExtensionDialogs/>

                {/* Outside the router: the card menu that asks for it unmounts
                    as soon as it is clicked, so the dialog cannot live inside
                    the menu. */}
                <MoveCardToBoard/>
                <div id='frame'>
                    <div id='main'>
                        <AppRouter/>
                    </div>
                </div>
            </SortableProvider>
        </IntlProvider>
    )
}

export default App
