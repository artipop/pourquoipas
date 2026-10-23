import {Show} from 'solid-js'

import {useIntl} from '../../intl'
import Menu from '../../widgets/menu'
import {aiEnabled} from '../../aiSwitch'
import {openExtensionDialog} from '../../extensions'
import type {SlotProps} from '../../extensions'
import RootPortal from '../rootPortal'

import PlanningDialog, {isPlanningAvailable} from './planningDialog'
import BoardPromptsDialog, {isBoardPromptsAvailable} from './boardPromptsDialog'
import WorkdirsDialog from './workdirsDialog'
import {isWorkdirsAvailable} from './workdirsPanel'
import InboxSortingDialog, {isInboxSortingAvailable} from './inboxSortingDialog'

// The agents' items in the board's menus (extensions.tsx). Each opens its
// dialog through the extension host, because a menu takes what it drew with
// it when it closes.

// «New» → talk a task over with an agent: a way of making cards, so it is on
// the button that makes them. What comes out of the conversation are cards on
// this board.
export const PlanningMenuItem = (props: SlotProps['newCardMenu']) => {
    const intl = useIntl()
    return (
        <Show when={isPlanningAvailable()}>
            <Menu.Separator/>
            <Menu.Text
                id='planTask'
                name={intl.formatMessage({id: 'ViewHeader.plan-task', defaultMessage: 'Talk it over with an agent…'})}
                onClick={() => openExtensionDialog((close) => (
                    <RootPortal>
                        <PlanningDialog
                            board={props.board}
                            onClose={close}
                        />
                    </RootPortal>
                ))}
            />
        </Show>
    )
}

// The inbox's ⋯ menu: where what the sources bring goes next, decided by an
// agent — the other half of the inbox, so beside the sources.
export const InboxSortingMenuItem = (props: SlotProps['inboxMenu']) => {
    const intl = useIntl()
    return (
        <Show when={isInboxSortingAvailable() && Boolean(props.board)}>
            <Menu.Text
                id='inboxSorting'
                name={intl.formatMessage({id: 'ViewHeader.inbox-sorting', defaultMessage: 'Sorting the inbox…'})}
                onClick={() => openExtensionDialog((close) => <InboxSortingDialog onClose={close}/>)}
            />
        </Show>
    )
}

// The board's ⋯ menu: what this board tells its agents, and its folders. A
// template is a board being written, not one agents work on; a server has no
// folders (docs/modes.md).
export const BoardAgentMenuItems = (props: SlotProps['boardMenu']) => {
    const intl = useIntl()
    const agentsWork = () => aiEnabled() && !props.board.isTemplate
    return (
        <>
            <Show when={isBoardPromptsAvailable() && agentsWork()}>
                <Menu.Text
                    id='boardPrompts'
                    name={intl.formatMessage({id: 'ViewHeader.board-prompts', defaultMessage: 'The board’s system prompt…'})}
                    onClick={() => openExtensionDialog((close) => (
                        <BoardPromptsDialog
                            board={props.board}
                            onClose={close}
                        />
                    ))}
                />
            </Show>
            <Show when={agentsWork() && isWorkdirsAvailable()}>
                <Menu.Text
                    id='workdirs'
                    name={intl.formatMessage({id: 'ViewHeader.workdirs', defaultMessage: 'Folders…'})}
                    onClick={() => openExtensionDialog((close) => (
                        <WorkdirsDialog
                            board={props.board}
                            onClose={close}
                        />
                    ))}
                />
            </Show>
        </>
    )
}
