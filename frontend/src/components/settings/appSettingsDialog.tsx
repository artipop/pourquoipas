import {For, Show, createSignal} from 'solid-js'
import {Dynamic} from '@solidjs/web'
import type {Component} from 'solid-js'

import {useIntl} from '../../intl'
import {useAppSelector} from '../../store/hooks'
import {getMe} from '../../store/users'
import {aiEnabled} from '../../aiSwitch'

import Dialog from '../dialog'

import ServerPanel, {isServerAttachAvailable} from '../acp/serverPanel'
import MachineMiscPanel from '../acp/machineMiscPanel'

import AIPanel, {isAIPanelAvailable} from './aiPanel'
import AppPanel from './appPanel'
import DataPanel from './dataPanel'
import PeoplePanel from './peoplePanel'
import UpdatesPanel, {isUpdatesAvailable} from './updatesPanel'

import './appSettingsDialog.scss'

// Everything this app is asked about itself rather than about a board: how it
// looks and what language it speaks, whose board it works, what
// comes in and goes out as an archive, and what it is allowed to interrupt
// with.
//
// Agents are the person's, not a board's, so they are connected here and
// nowhere else: the AI section has the switch, the agents of both kinds and the
// proxies a program may be started behind.
//
// All of it used to be reached through a board's ⋯ menu, which put settings
// behind a board and made them unreachable when none was open; then through the
// sidebar's own menu, where a submenu of a submenu was where a person went
// looking for an import. One dialog with a list down the side is what keeps
// them one thing rather than a menu that grows an entry per feature — and what
// emptied the corner of the board, where the theme, the language and the way to
// the manual had each become an icon standing in for a word.

// The name is an accessor, not a string, and that is the whole of why the
// language of this dialog follows the one picked inside it. A Solid component
// body runs once, so a name formatted there is a name in whatever language was
// current when the dialog opened — and the panel that changes the language is
// two clicks away, inside this very dialog. The nav and the panel heading
// stayed Russian while everything drawn inside JSX turned English.
type Section = {
    id: string
    name: () => string
    when: () => boolean

    // A component, not rendered JSX: switching sections has to build the panel
    // again, because a panel reads its registry when it mounts and that is how
    // it picks up what another section just changed.
    body: Component<{onClose?: () => void}>
}

type Props = {
    onClose: () => void

    // The section to open on, when another screen sent the person here.
    section?: string
}

const AppSettingsDialog = (props: Props) => {
    const intl = useIntl()

    // Administering the install is a thing only a hosted server has: a desktop
    // asks nobody who they are, so nobody there is an admin and the section is
    // not drawn. The answer comes from the account rather than from the mode,
    // because a member of a team is not an admin either.
    const me = useAppSelector(getMe)
    const administersInstall = () => Boolean(me()?.permissions?.includes('manage_system'))

    const sections: Section[] = [
        {

            // First, and therefore what opens: it is the one section every
            // install has something to say in, agents or no agents.
            id: 'app',
            name: () => intl.formatMessage({id: 'Settings.section-app', defaultMessage: 'The app itself'}),
            when: () => true,
            body: AppPanel,
        },
        {

            // The AI's own section: the switch over all of it, and the agents.
            id: 'ai',
            name: () => intl.formatMessage({id: 'Settings.section-ai', defaultMessage: 'AI'}),
            when: isAIPanelAvailable,
            body: AIPanel,
        },
        {

            // Directly after the app itself, because it is the app talking
            // about the app — and because it is the one section that sometimes
            // has something to say before anybody comes looking.
            id: 'updates',
            name: () => intl.formatMessage({id: 'Updates.title', defaultMessage: 'Updates'}),
            when: isUpdatesAvailable,
            body: UpdatesPanel,
        },
        {
            id: 'people',
            name: () => intl.formatMessage({id: 'People.title', defaultMessage: 'People'}),
            when: administersInstall,
            body: PeoplePanel,
        },
        {
            id: 'server',
            name: () => intl.formatMessage({id: 'Server.title', defaultMessage: 'Server'}),
            when: isServerAttachAvailable,
            body: ServerPanel,
        },
        {
            id: 'data',
            name: () => intl.formatMessage({id: 'Settings.section-data', defaultMessage: 'Import and export'}),
            when: () => true,
            body: DataPanel,
        },
        {

            // Every setting in it is about agents, so with the AI switched off
            // it is not drawn at all.
            id: 'misc',
            name: () => intl.formatMessage({id: 'Machine.section-misc', defaultMessage: 'Other'}),
            when: aiEnabled,
            body: MachineMiscPanel,
        },
    ]

    const offered = () => sections.filter((s) => s.when())
    const [current, setCurrent] = createSignal(props.section || offered()[0]?.id || 'app')
    const section = () => offered().find((s) => s.id === current()) || offered()[0]

    return (
        <Dialog
            class='AppSettingsDialog'
            title={<span>{intl.formatMessage({id: 'Settings.title', defaultMessage: 'Settings'})}</span>}
            onClose={props.onClose}
        >
            <div class='AppSettingsDialog__body'>
                <nav class='AppSettingsDialog__nav'>
                    <For each={offered()}>
                        {(entry) => (
                            <button
                                type='button'
                                class={`AppSettingsDialog__navItem${entry.id === current() ? ' AppSettingsDialog__navItem--current' : ''}`}
                                onClick={() => setCurrent(entry.id)}
                            >
                                {entry.name()}
                            </button>
                        )}
                    </For>
                </nav>
                <div class='AppSettingsDialog__panel'>
                    <Show when={section()}>
                        {(entry) => (
                            <>
                                <h3 class='AppSettingsDialog__panelTitle'>{entry().name()}</h3>
                                <Dynamic
                                    component={entry().body}
                                    onClose={props.onClose}
                                />
                            </>
                        )}
                    </Show>
                </div>
            </div>
        </Dialog>
    )
}

export default AppSettingsDialog
