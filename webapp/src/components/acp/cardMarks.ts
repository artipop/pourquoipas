// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Accessor, createMemo, createSignal, onCleanup, onMount} from 'solid-js'

import {agentBindings} from './bindings'
import {onAgentEvent} from './agentEvents'

// The two answers a card can be given from where it stands — «← В работе»,
// «Деплой →» — for every card of the board at once.
//
// One call for the page, indexed by card, for the reason liveTerminals is: the
// board needs this per card and has as many cards as it likes, so asking
// GetCardFlow for each would be a round trip to Go for every card drawn.
//
// Only cards on a stage that waits for a person appear here at all. A stage
// that decides for itself answers for itself, and the Go side leaves those out
// rather than making every reader remember to.

export type CardMark = {
    value: string
    column: string
    forward: boolean
}

const [byCard, setByCard] = createSignal<Record<string, CardMark[]>>({})

// Counted rather than assumed, as in liveTerminals and attention: the
// subscription is worth holding only while a board is drawing them.
let consumers = 0
let unsubscribe: (() => void) | undefined
let board = ''

async function reload(): Promise<void> {
    const bindings = agentBindings()
    if (!bindings?.ListCardMarks || !board) {
        setByCard({})
        return
    }
    try {
        setByCard(JSON.parse(await bindings.ListCardMarks(board)) || {})
    } catch (e) {
        setByCard({})
    }
}

function subscribe(boardId: string): () => void {
    board = boardId
    consumers++
    if (consumers === 1) {
        // A card moving, a stage starting or finishing — each changes which
        // answers a card has, and all of them arrive as this one event.
        unsubscribe = onAgentEvent('acp:session', () => reload())
    }
    reload()
    return () => {
        consumers--
        if (consumers === 0) {
            unsubscribe?.()
            unsubscribe = undefined
            setByCard({})
        }
    }
}

// useCardMarks is what one card can be answered with, for as long as the
// component asking lives.
export function useCardMarks(boardId: Accessor<string>, cardId: Accessor<string>): Accessor<CardMark[]> {
    onMount(() => onCleanup(subscribe(boardId())))
    return createMemo(() => byCard()[cardId()] || [])
}

// markCard writes the answer and asks again: the board tells the route, the
// route moves the card, and what comes back is a card standing somewhere else
// with different answers — or none.
export async function markCard(cardId: string, value: string): Promise<void> {
    const bindings = agentBindings()
    if (!bindings?.MarkCardOutcome) {
        return
    }
    await bindings.MarkCardOutcome(cardId, value)
    await reload()
}
