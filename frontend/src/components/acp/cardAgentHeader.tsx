import {Show} from 'solid-js'

import {useIntl} from '../../intl'
import type {SlotProps} from '../../extensions'

import CaseStamp, {isCaseStampAvailable} from './caseStamp'
import AttentionAnswers from './attentionAnswers'
import {attentionHeading, questionText, useCardAttention} from './attention'

import './cardAgentHeader.scss'

// What the agents add under a card's title in its dialog (the cardDetailTop
// slot): where the work on the card lives, and what an agent is asking.
const CardAgentHeader = (props: SlotProps['cardDetailTop']) => {
    const intl = useIntl()
    const asking = useCardAttention(() => props.card?.id || '')

    return (
        <>
            <Show when={isCaseStampAvailable()}>
                <CaseStamp cardId={props.card.id}/>
            </Show>

            {/* Where the card's own amber badge promises it will be. The
                notification says the same and is answered the same way, but it
                is dismissible and this is not: a question outlives being read. */}
            <Show when={asking()}>
                {(wait) => (
                    <div class='CardDetail__asking'>
                        <span class='CardDetail__askingHeading'>{attentionHeading(intl, wait())}</span>
                        <Show when={wait().text}>
                            <span class='CardDetail__askingText'>{questionText(intl, wait())}</span>
                        </Show>
                        <AttentionAnswers target={wait()}/>
                    </div>
                )}
            </Show>
        </>
    )
}

export default CardAgentHeader
