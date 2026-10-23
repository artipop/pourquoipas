import {Show} from 'solid-js'

import {useIntl} from '../../intl'
import CompassIcon from '../../widgets/icons/compassIcon'
import {aiEnabled} from '../../aiSwitch'
import {useAppSelector} from '../../store/hooks'
import {getBoardUsers} from '../../store/users'
import {createLoader} from '../../hooks/loader'
import type {SlotProps} from '../../extensions'
import HintTip from '../onboardingHints/hintTip'
import {HINT_AGENT_MOVE, HINT_QUESTION, cardMovedFrom} from '../onboardingHints/hints'

import {attentionHeading, useCardAttention} from './attention'
import {findWorkdirProperty} from './workdirSync'
import {loadAgentUsers} from './agentAssignees'

import './cardAgentBadges.scss'

// What the agents add to a card on a board (the cardBadges slot): the question
// an agent is waiting on, the folder the card's work is in, and the hints about
// both. Nothing at all with the agents switched off.
const CardAgentBadges = (props: SlotProps['cardBadges']) => {
    const intl = useIntl()

    // Where this card's work lives, on the card's face: on a board of several
    // folders it is most of what the board is scanned for. The card's own field
    // only, and not when the view already lists it.
    const folderChip = () => {
        if (!aiEnabled()) {
            return ''
        }
        const property = findWorkdirProperty(props.board, props.board.cardProperties)
        if (!property || props.visiblePropertyIds.includes(property.id)) {
            return ''
        }
        const value = props.card.fields.properties[property.id]
        const optionId = Array.isArray(value) ? value[0] : value
        return property.options.find((o) => o.id === optionId)?.value || ''
    }

    // An agent that has stopped to ask something is the one thing about a card
    // a person has to notice without opening it.
    const attention = useCardAttention(() => props.card.id)
    const agentTitle = () => (attention() ? attentionHeading(intl, attention()!) : '')
    const asking = () => aiEnabled() && Boolean(attention())

    // An agent's move is told apart from a person's by who last wrote the card:
    // an agent's writes are signed with its own account.
    const boardUsers = useAppSelector(getBoardUsers)
    const [agentUsers] = createLoader(() => loadAgentUsers(), [] as string[])
    const movedFrom = cardMovedFrom(props.card.id, props.groupId)
    const movedByAgent = () => aiEnabled() && Boolean(movedFrom) &&
        agentUsers().includes(boardUsers()[props.card.modifiedBy]?.username || '')

    const openCard = (e: MouseEvent) => {
        e.stopPropagation()
        props.showCard(props.card.id)
    }

    return (
        <>
            {/* From the card's bottom-right corner: amber, because the machine
                needs a person here, and a click away from where it is
                answered. */}
            <Show when={asking()}>
                <Show
                    when={!props.readonly}
                    fallback={
                        <span
                            class='KanbanCard__asking'
                            role='status'
                            title={agentTitle()}
                            aria-label={agentTitle()}
                        >
                            <CompassIcon icon='help-circle-outline'/>
                        </span>
                    }
                >
                    <button
                        type='button'
                        class='KanbanCard__asking'
                        title={agentTitle()}
                        aria-label={agentTitle()}
                        onClick={openCard}
                    >
                        <CompassIcon icon='help-circle-outline'/>
                    </button>
                </Show>
            </Show>
            <Show when={folderChip()}>
                <div class='KanbanCard__folder'>
                    <CompassIcon icon='folder-outline'/>
                    <span>{folderChip()}</span>
                </div>
            </Show>
            <HintTip
                id={HINT_QUESTION}
                wanted={asking()}
                class='HintTip--card'
                placement='right-end'
                title={intl.formatMessage({id: 'Hint.question.title', defaultMessage: 'The agent is asking'})}
                body={intl.formatMessage({id: 'Hint.question.body', defaultMessage: 'The agent working this card has stopped to ask something, and waits until somebody answers. Open the card: the question is under its title, and choosing one of the options is the answer. The notification about it answers the same way.'})}
            />
            <HintTip
                id={HINT_AGENT_MOVE}
                wanted={movedByAgent()}
                class='HintTip--card'
                placement='right-end'
                title={intl.formatMessage({id: 'Hint.agent-move.title', defaultMessage: 'The agent moved the card'})}
                body={intl.formatMessage({id: 'Hint.agent-move.body', defaultMessage: 'It chose this column by its name, as the one that says what happened. Renaming the columns is how you tell agents what each of them means.'})}
            />
        </>
    )
}

export default CardAgentBadges
