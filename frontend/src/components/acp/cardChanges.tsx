import {Show, onSettled} from 'solid-js'

import {useIntl} from '../../intl'
import Button from '../../widgets/buttons/button'
import CompassIcon from '../../widgets/icons/compassIcon'
import type {CardSidePanel} from '../../extensions'
import HintTip from '../onboardingHints/hintTip'
import {HINT_CHANGES} from '../onboardingHints/hints'

import CardDiffPanel, {isCardDiffAvailable} from './cardDiff'
import {cardAgentState, refreshCardAgent} from './cardAgentState'
import {onAgentEvent} from './agentEvents'

// «Changes» in a card's dialog: what the agent changed in the card's folder,
// read from git (cardDiff.tsx). Plugged in as a side panel (extensions.tsx).

const ChangesButton: CardSidePanel['Button'] = (props) => {
    const intl = useIntl()

    // How much the work amounts to, which the button is drawn on.
    const agentState = cardAgentState(props.card.id)
    onSettled(() => {
        if (!isCardDiffAvailable()) {
            return undefined
        }
        refreshCardAgent(props.card.id)

        // The work moves while the dialog is open — an agent commits, a stage
        // ends — and a session event is the cheapest thing that means "look
        // again".
        return onAgentEvent('acp:session', () => refreshCardAgent(props.card.id))
    })

    // Drawn only where there is something to read. What decides is the count
    // the Go side keeps as the work moves (GetCardAgent's `changes`), not a
    // live look: by review the card's working copy has been folded away, and
    // asking git per card is a process per card.
    const hasChanges = () => {
        const changes = agentState().changes
        return Boolean(changes && (changes.files > 0 || changes.dirty > 0))
    }

    return (
        <Show when={isCardDiffAvailable() && hasChanges()}>
            <Button
                icon={<CompassIcon icon='file-patch-outline'/>}
                class='cardFollowBtn cardFollowBtn--attach'
                emphasis='gray'
                size='medium'
                active={props.active}
                onClick={props.toggle}
            >
                {intl.formatMessage({id: 'CardDiff.title', defaultMessage: 'Changes'})}
            </Button>
            <span class='cardDialog__tourAnchor'>
                <HintTip
                    id={HINT_CHANGES}
                    wanted={true}
                    class='HintTip--button'
                    placement='bottom-end'
                    title={intl.formatMessage({id: 'Hint.changes.title', defaultMessage: 'What the agent changed'})}
                    body={intl.formatMessage({id: 'Hint.changes.body', defaultMessage: 'The changes the agent made in the folder, read from git: what is committed on the card’s branch and what is not committed yet.'})}
                />
            </span>
        </Show>
    )
}

export const cardChangesPanel: CardSidePanel = {
    id: 'changes',
    Button: ChangesButton,
    Panel: (props) => (
        <CardDiffPanel
            cardId={props.card.id}
            onClose={props.onClose}
        />
    ),
}
