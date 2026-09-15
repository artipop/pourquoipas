import {Show, createMemo, onSettled} from 'solid-js'
import {useParams} from '@solidjs/router'

import {useIntl} from '../../intl'

import {CardDiffView} from './cardDiff'

import './diffPage.scss'

// The page a diff window shows. It is the panel's own reader at the size the
// reading actually wants — code in a column beside a card is code read four
// words at a time — and it asks nothing of the board: the card id is in the
// address, and everything else comes from main.App.GetCardDiff, which the front
// door serves to a window, a browser and a phone alike.

type Props = {

    // Which card's work to draw. A window takes it from its own address; the
    // prop is what a test hands it.
    cardId?: string
}

const DiffPage = (props: Props = {}) => {
    const intl = useIntl()
    const params = useParams()
    const cardId = createMemo(() => props.cardId || params.cardId || '')

    onSettled(() => {
        document.title = intl.formatMessage({id: 'CardDiff.title', defaultMessage: 'Changes'})
    })

    return (
        <div class='DiffPage'>
            <Show
                when={cardId()}
                fallback={
                    <div class='DiffPage__empty'>
                        {intl.formatMessage({id: 'CardDiff.no-card', defaultMessage: 'This address names no card.'})}
                    </div>
                }
            >
                <CardDiffView cardId={cardId()}/>
            </Show>
        </div>
    )
}

export default DiffPage
