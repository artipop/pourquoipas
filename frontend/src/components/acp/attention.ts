// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Accessor, createMemo, createSignal, onSettled} from 'solid-js'

import {UserSettings} from '../../userSettings'

import {goApp} from '../../goBridge'
import {aiEnabled} from '../../aiSwitch'

import {onAgentEvent} from './agentEvents'

// Which cards are waiting for a person.
//
// A card whose agent is waiting looks exactly like a card whose agent is busy,
// and the answer waits until somebody happens to look. This keeps what the Go
// side says in one place, so every card on the board shares one subscription
// and one initial load rather than asking for itself.

// The one way a card wants a person: the agent is asking. It wanted a tool it
// was not given, or sent an elicitation, and its turn stays open until somebody
// answers — which is done where the question is read (attentionAnswers.tsx).
export type AttentionReason = 'question'

export type QuestionOption = {
    id: string
    label: string
    description?: string
    kind?: string
}

export type Attention = {
    key: string

    // Who is asking, for a wait with no card behind it: the planning dialog
    // finds its own agent's questions by this.
    sessionId?: string
    cardId?: string
    boardId?: string
    title?: string
    agent?: string
    reason: AttentionReason
    tool?: string

    // A question carries itself, so it can be answered where it is read.
    questionId?: string
    text?: string
    options?: QuestionOption[]
    freeText?: boolean
    awaiting: boolean
    since?: string

    // Whether a person has already been told about this one. Kept by the Go
    // side, because the same wait is drawn in every window and on the phone and
    // "do not tell me again" means all of them (internal/acp/attentionack.go).
    // It hides the notification and nothing else: the card's own button is part
    // of the card and stays.
    acked?: boolean
}

// keyOf is what identifies one wait — the Go side fills it in, and this is the
// same rule for anything that arrives without one. A question is keyed by its
// own id, because an agent can have two open on one card and answering one must
// not take the other off the screen.
export const keyOf = (a: Attention): string => {
    if (a.key) {
        return a.key
    }
    return a.questionId ? `q:${a.questionId}` : `card:${a.cardId}`
}

const [waiting, setWaiting] = createSignal<Attention[]>([])

// Consumers are counted rather than assumed: the subscription is worth holding
// only while something displays it, and a test that unmounts everything gets a
// clean module back.
let consumers = 0
let unsubscribe: (() => void) | undefined

function apply(payload: Attention): void {
    if (!payload?.cardId && !payload?.sessionId) {
        return
    }
    setWaiting((current) => {
        const rest = current.filter((a) => keyOf(a) !== keyOf(payload))
        if (!payload.awaiting) {
            return rest
        }
        return [...rest, payload].sort((a, b) => (a.since || '').localeCompare(b.since || ''))
    })
}

async function reload(): Promise<void> {
    const bindings = goApp()
    if (!bindings?.ListAttention) {
        return
    }
    try {
        setWaiting(JSON.parse(await bindings.ListAttention()) || [])
    } catch {
        // Nothing waiting is the right answer when nothing can be asked.
        setWaiting([])
    }
}

function subscribe(): () => void {
    consumers++
    if (consumers === 1) {
        // No payload means the socket has just (re)connected and nobody knows
        // what was missed while it was down — which for a question waiting to
        // be answered is exactly the wrong thing to be wrong about, so the
        // whole list is fetched again.
        unsubscribe = onAgentEvent('acp:attention', (payload?: Attention) => (payload ? apply(payload) : reload()))
        reload()
    }
    return () => {
        consumers--
        if (consumers === 0) {
            unsubscribe?.()
            unsubscribe = undefined

            // After the disposal rather than during it: this runs as a
            // component's cleanup, and Solid 2 refuses a write there. Checked
            // again, because a component mounted in between wants the list.
            queueMicrotask(() => {
                if (consumers === 0) {
                    setWaiting([])
                }
            })
        }
    }
}

// With the AI switched off nothing waits for a person as far as the screen is
// concerned: no dot on a card, no notification, no question. A stage already
// running when the switch was turned goes on in Go, and its wait is back the
// moment the switch is.
const shown: Accessor<Attention[]> = () => (aiEnabled() ? waiting() : [])

// useAttention gives a component everything waiting for a person, oldest wait
// first, and keeps it current for as long as the component lives.
export function useAttention(): Accessor<Attention[]> {
    onSettled(() => subscribe())
    return shown
}

// useCardAttention is the same thing asked about one card — what a card on the
// board shows an indicator for.
export function useCardAttention(cardId: Accessor<string>): Accessor<Attention | undefined> {
    onSettled(() => subscribe())

    return createMemo(() => shown().find((a) => a.cardId === cardId()))
}

// ackWait says a person has seen this wait — waved it away, or gone to look at
// it. Both are the same answer to the same question, which is whether anybody
// still needs telling.
//
// It goes to the Go side rather than being remembered here, so that one click
// takes the notification off every window at once and a reload does not bring
// it back. What brings it back is the agent doing something and stopping again.
export async function ackWait(target: Attention): Promise<void> {
    const key = keyOf(target)

    // Marked here first: a click has to take the box away now, not when the
    // round trip lands — and in a browser or plugin deployment, where there is
    // no Go side, this is the whole of it.
    setWaiting((current) => current.map((a) => (keyOf(a) === key ? {...a, acked: true} : a)))
    try {
        await goApp()?.AckAttention?.(key)
    } catch {
        // Nothing to say and nothing to undo: the notification is already gone
        // in this window, and the worst a refused ack costs is that the next
        // window to open draws it once.
    }
}

type Formatter = {
    formatMessage: (descriptor: {id: string, defaultMessage: string}, values?: Record<string, string>) => string
}

// attentionHeading is the one sentence a wait is worth, wherever it is shown —
// the dot's tooltip, the notification, the card.
export function attentionHeading(intl: Formatter, target: Attention): string {
    const agent = target.agent || ''
    return agent ? intl.formatMessage({id: 'Attention.asking-agent', defaultMessage: '{agent} is asking'}, {agent}) : intl.formatMessage({id: 'Attention.asking', defaultMessage: 'The agent is asking'})
}

// Whether being told about it at all is wanted. The indicator on a card is
// always there — it is part of the card — but a notification interrupts, so it
// is a setting, and one the notifications have to see change without a reload.
const [notificationsOn, setNotificationsOn] = createSignal(UserSettings.agentNotifications)

export const agentNotificationsOn: Accessor<boolean> = notificationsOn

export function setAgentNotifications(on: boolean): void {
    UserSettings.agentNotifications = on
    setNotificationsOn(on)
}
