import {Show} from 'solid-js'

import {useIntl} from '../../intl'
import {Board, IPropertyTemplate} from '../../blocks/board'
import {Card} from '../../blocks/card'
import {Permission} from '../../constants'
import {useHasCurrentBoardPermissions} from '../../hooks/permissions'
import {aiEnabled} from '../../aiSwitch'
import type {CardPropertyRow, SlotProps} from '../../extensions'

import AssignAgentTourStep from '../onboardingTour/assignAgent/assign_agent'
import PickFolderTourStep from '../onboardingTour/pickFolder/pick_folder'

import CardFolderValue from './cardFolderValue'

import {boardBranchProperty} from './automation'
import {cardAgentState} from './cardAgentState'
import {fieldNames, findWorkdirProperty} from './workdirSync'
import {canAddFolder} from './cardFolder'
import {personProperty} from './attentionAudience'

// The agents' fields in a card's dialog, answered through extensions.tsx: who
// works the card (where the tour points), the folder its work is in, and the
// branch that work is on.

const isWorkdir = (board: Board, t: IPropertyTemplate) => t.id !== '' && t.id === findWorkdirProperty(board, board.cardProperties)?.id
const isBranch = (board: Board, t: IPropertyTemplate) => t.id !== '' && t.id === boardBranchProperty(board)

// The folder stops being a choice once the card has a workspace: the work was
// claimed under the folder the card named then, and moving the card to another
// folder afterwards would make it describe somewhere the work is not.
const workdirLocked = (card: Card) => Boolean(cardAgentState(card.id)().work?.started)

const folderName = (board: Board, card: Card) => {
    const property = findWorkdirProperty(board, board.cardProperties)
    const value = property && card.fields.properties[property.id]
    const optionId = Array.isArray(value) ? value[0] : value
    return property?.options.find((o) => o.id === optionId)?.value || ''
}

export const assigneeRow: CardPropertyRow = {
    claims: (board, t) => t.id === personProperty(board)?.id,
    rowClass: 'octo-propertyrow--assignee',
    InRow: () => <AssignAgentTourStep/>,
}

// The branch is the machine's record of where the card's work lives. A copy
// of the folder (worktree) is named so, because the stamp under the title
// already says which kind of place it is.
const BranchName: CardPropertyRow['Name'] = (props) => {
    const intl = useIntl()
    return (
        <Show
            when={cardAgentState(props.card.id)().work?.mode === 'worktree'}
            fallback={props.template.name}
        >
            {intl.formatMessage({id: 'CardDetail.worktree-property', defaultMessage: 'Worktree'})}
        </Show>
    )
}

export const branchRow: CardPropertyRow = {
    claims: isBranch,
    Name: BranchName,
    readonly: () => true,
}

const FolderLockHint = (props: {board: Board, card: Card}) => {
    const intl = useIntl()
    const work = () => cardAgentState(props.card.id)().work

    // Started, and not on this machine: a card travels and the workspace does
    // not, so this says where the work is and lets the person get there.
    return (
        <Show when={workdirLocked(props.card)}>
            <div class='octo-propertyhint'>
                <Show
                    when={work()?.started && !work()?.here}
                    fallback={intl.formatMessage({
                        id: 'CardDetail.folder-locked',
                        defaultMessage: 'The card is already working in “{folder}”, where its branch is. For another folder, make a new card.',
                    }, {folder: folderName(props.board, props.card)})}
                >
                    {intl.formatMessage({
                        id: 'CardDetail.folder-locked-elsewhere',
                        defaultMessage: 'Work on this card has started on another machine, in “{folder}”, branch “{branch}”. To carry on here, that folder with that branch has to be on this machine.',
                    }, {folder: folderName(props.board, props.card), branch: work()?.branch || '—'})}
                </Show>
            </div>
        </Show>
    )
}

export const folderRow: CardPropertyRow = {
    claims: isWorkdir,
    rowClass: 'octo-propertyrow--folder',
    readonly: (_board, card) => workdirLocked(card),
    Value: (props) => (
        <CardFolderValue
            board={props.board}
            card={props.card}
            property={props.template}
            readOnly={props.readonly}
        />
    ),
    InRow: () => <PickFolderTourStep/>,
    Below: (props) => (
        <FolderLockHint
            board={props.board}
            card={props.card}
        />
    ),
}

// A board that has never had a folder has no field for one, and the card is
// where a folder is asked: this row stands in for the field until the first
// folder added from it makes one.
export const FolderRowToAdd = (props: SlotProps['cardPropertiesEnd']) => {
    const intl = useIntl()
    const canEditBoardProperties = useHasCurrentBoardPermissions([Permission.ManageBoardProperties])
    const canEditBoardCards = useHasCurrentBoardPermissions([Permission.ManageBoardCards])
    const offered = () => aiEnabled() && canAddFolder() && !findWorkdirProperty(props.board, props.board.cardProperties) &&
        !props.readonly && canEditBoardProperties() && canEditBoardCards()
    return (
        <Show when={offered()}>
            <div class='octo-propertyrow octo-propertyrow--folder'>
                <div class='octo-propertyname octo-propertyname--readonly'>{fieldNames(intl).folder}</div>
                <CardFolderValue
                    board={props.board}
                    card={props.card}
                    readOnly={false}
                />
                <PickFolderTourStep/>
            </div>
        </Show>
    )
}
