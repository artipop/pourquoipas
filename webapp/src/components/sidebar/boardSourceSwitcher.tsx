// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {For, Show, createMemo} from 'solid-js'

import {useIntl} from '../../intl'
import Menu from '../../widgets/menu'
import MenuWrapper from '../../widgets/menuWrapper'
import CompassIcon from '../../widgets/icons/compassIcon'
import {agentBindings} from '../acp/bindings'

import './boardSourceSwitcher.scss'

// Which board this window shows: the one on this machine, or the one on the
// server this desktop is attached to.
//
// It is not a setting and it is not about the agents. Both boards are worked
// whichever one is on screen — a stage on the server's card goes on running
// while somebody reads their own board — so this is only about what the window
// is looking at, which is why it lives here and not in the settings dialog.
//
// Nothing is drawn until there is a second board to go to: a switcher with one
// entry is a label pretending to be a control.

type BoardChoice = {
    id: string
    title: string
}

type AttachState = {
    selected: string
    sources: BoardChoice[]
}

const BoardSourceSwitcher = () => {
    const intl = useIntl()
    const bindings = agentBindings()
    const state = createMemo(async (): Promise<AttachState | null> => {
        if (!bindings?.GetServerAttachment) {
            return null
        }
        try {
            return JSON.parse(await bindings.GetServerAttachment()) as AttachState
        } catch {
            // A desktop that cannot answer has one board, which is what the
            // absence of this control already says.
            return null
        }
    }, {loadingValue: null})

    const sources = () => state()?.sources ?? []
    const current = () => sources().find((s) => s.id === (state()?.selected ?? '')) ?? sources()[0]

    const choose = async (id: string) => {
        if (!bindings?.SelectBoardSource || id === state()?.selected) {
            return
        }
        await bindings.SelectBoardSource(id)

        // The page holds one board's cards, members and both sockets, all of
        // them fetched against the board it loaded from. Reloading is the
        // honest way to change that; absorbing it would be a screen showing one
        // board's cards under another's name.
        window.location.reload()
    }

    return (
        <Show when={sources().length > 1}>
            <div class='BoardSourceSwitcher'>
                <MenuWrapper>
                    <div class='BoardSourceSwitcher__current'>
                        <span class='BoardSourceSwitcher__title'>{current()?.title}</span>
                        <CompassIcon icon='chevron-down'/>
                    </div>
                    <Menu position='bottom'>
                        <Menu.Label>
                            {intl.formatMessage({id: 'BoardSource.title', defaultMessage: 'Which board this window shows'})}
                        </Menu.Label>
                        <For each={sources()}>
                            {(source) => (
                                <Menu.Text
                                    id={source.id || 'this-machine'}
                                    name={source.title}
                                    check={source.id === (state()?.selected ?? '')}
                                    onClick={() => choose(source.id)}
                                />
                            )}
                        </For>
                    </Menu>
                </MenuWrapper>
                <div class='BoardSourceSwitcher__hint'>
                    {intl.formatMessage({
                        id: 'BoardSource.hint',
                        defaultMessage: 'Agents keep working both boards.',
                    })}
                </div>
            </div>
        </Show>
    )
}

export default BoardSourceSwitcher
