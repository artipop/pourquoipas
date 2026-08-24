// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Accessor, For, Show, createEffect, createSignal, onCleanup} from 'solid-js'

import {useIntl} from '../../intl'

import Dialog from '../dialog'

import {agentBindings} from './bindings'
import {Attention, keyOf, useAttention} from './attention'
import {wasMovedHere} from './movedHere'

import './askDialog.scss'

// The question, put in front of the person who caused it.
//
// A transition can hold a card until somebody says how the work is to be
// arranged, and that wait is drawn in two places already: on the card's own
// route strip, and — because a wait usually happens with nobody watching — as a
// notification. Neither is right for the case the question was built for. A
// person dragging a card from the backlog into work is looking at the board
// with their hand on the card, and answering meant finding that card again and
// reading a strip on it.
//
// So the wait asks here instead, and only when this window is the one that
// moved the card (movedHere.ts). Everything else about it is unchanged: the
// engine holds the card the same way, and closing this without answering
// leaves the question exactly where it was, on the strip and in the
// notification. This is a second door onto one question, not a second question.
//
// Nothing is drawn on a terminal screen, for the reason the notification stack
// is not: that window is one terminal and a box of ours over it covers what it
// is about.
const terminalScreen = (): boolean => (/^\/(acp|m)\/terminal\//).test(window.location.pathname)

// asked is the wait this dialog is for: a card the route is holding, with
// answers to offer. A handover with no options is a card parked on a stage
// where nobody works — there is nothing to answer, and being told is the whole
// of it.
const asked = (a: Attention): boolean => a.reason === 'card' && a.awaiting && (a.options || []).length > 0

// asking is the wait the dialog currently has on screen, by its key, or ''.
//
// The notification stack reads it and leaves that wait alone: the question is
// already in front of the person, and a box in the corner announcing the box
// they are looking at is the noise this dialog was meant to remove. It is a
// module signal rather than a prop because the two are mounted side by side in
// app.tsx and neither owns the other.
const [asking, setAsking] = createSignal('')

export const askedInDialog: Accessor<string> = asking

const AskDialog = () => {
    const intl = useIntl()
    const waiting = useAttention()
    const claimed = asking
    const setClaimed = setAsking
    const [busy, setBusy] = createSignal(false)

    // The signal outlives the component, so a window torn down mid-question
    // would leave the notification stack hiding a wait nobody is being asked
    // about any more.
    onCleanup(() => setAsking(''))

    // Claiming is separate from drawing because wasMovedHere answers once: it
    // takes the record with it, so asking it on every render would open the
    // dialog and then immediately stop matching. What is kept is the wait's own
    // key, and the dialog stands until it is answered or closed.
    createEffect(() => {
        if (terminalScreen() || claimed()) {
            return
        }
        const mine = waiting().find((a) => asked(a) && a.cardId && wasMovedHere(a.cardId))
        if (mine) {
            setClaimed(keyOf(mine))
        }
    })

    const target = () => waiting().find((a) => keyOf(a) === claimed() && asked(a))

    // A wait that ends while the dialog is open — answered on the card, in
    // another window, or by the route giving up on it — takes the dialog with
    // it. What it must not do is leave a box standing over a question nobody is
    // being asked any more.
    createEffect(() => {
        if (claimed() && !target()) {
            setClaimed('')
        }
    })

    const answer = async (value: string) => {
        const wait = target()
        const bindings = agentBindings()
        if (!wait?.cardId || !bindings?.AnswerCardAsk || busy()) {
            return
        }
        setBusy(true)
        try {
            // No question named: the wait is keyed by the card rather than by
            // the question, and the Go side reads an empty id as "the one this
            // card is being asked now" — which is the only one it can be, since
            // they are asked one at a time.
            await bindings.AnswerCardAsk(wait.cardId, '', value)
            setClaimed('')
        } finally {
            setBusy(false)
        }
    }

    return (
        <Show when={target()}>
            {(wait) => (
                <Dialog
                    size='small'
                    class='AskDialog'
                    onClose={() => setClaimed('')}
                >
                    <div class='AskDialog__box'>
                        <Show when={wait().title}>
                            <div class='AskDialog__card'>{wait().title}</div>
                        </Show>
                        <h3 class='AskDialog__text'>{wait().text}</h3>
                        <div class='AskDialog__options'>
                            <For each={wait().options}>
                                {(option) => (
                                    <button
                                        type='button'
                                        class='AskDialog__option'
                                        disabled={busy()}
                                        onClick={() => answer(option.id)}
                                    >
                                        {option.label}
                                    </button>
                                )}
                            </For>
                        </div>

                        {/* Closing is not an answer, and says so: the card goes
                            on waiting where it is, which is what the strip on
                            it shows. */}
                        <button
                            type='button'
                            class='AskDialog__later'
                            disabled={busy()}
                            onClick={() => setClaimed('')}
                        >
                            {intl.formatMessage({id: 'AskDialog.later', defaultMessage: 'Answer on the card'})}
                        </button>
                    </div>
                </Dialog>
            )}
        </Show>
    )
}

export default AskDialog
