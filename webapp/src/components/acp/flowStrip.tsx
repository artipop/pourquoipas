// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {For, Show, createSignal, onSettled} from 'solid-js'

import {useIntl} from '../../intl'
import CompassIcon from '../../widgets/icons/compassIcon'

import {createLoader} from '../../hooks/loader'

import {agentBindings} from './bindings'
import {onAgentEvent} from './agentEvents'

import './flowStrip.scss'

// Where the card is on its route, on the card itself. Until now this could only
// be reconstructed by reading the comments backwards, which is a poor way to
// answer the one question a stalled card raises: what is it waiting for?

export type CardFlowStage = {
    nodeId: string
    column: string
    action: string
    crew?: string[]
    current: boolean
    done: boolean
}

// CardFlowMark is a move a person can make from where the card stands: the
// value they put in the outcome field, and the stage it leads to. Two at most,
// and they are the whole of what a wait stage asks.
export type CardFlowMark = {
    value: string
    column: string
    forward: boolean
}

// CardFlowAsk is the question a transition is holding the card at: one at a
// time, which is the shape every other pick in this app has — the folder, the
// agent, the two outcome buttons. The card goes nowhere until it is answered.
export type CardFlowAsk = {
    questionId: string
    text: string
    options: Array<{value: string, label: string}>

    // Where the card goes once every question is answered.
    column: string

    // How many questions are still unanswered, this one included.
    left?: number
}

export type CardFlow = {
    flow: string
    stages: CardFlowStage[]
    currentNodeId: string
    since?: string
    branch?: string
    waitingFor?: string[]
    queued?: boolean
    running?: boolean

    // Why nothing is happening, when the machinery knows — a stage that would
    // not start, an event with no edge. This used to be a comment on the card.
    stalled?: string

    // What a person can say from here — see CardFlowMark. Absent wherever the
    // route decides for itself.
    marks?: CardFlowMark[]

    // The question holding the card at a transition, when there is one.
    ask?: CardFlowAsk
}

export function isFlowStripAvailable(): boolean {
    return Boolean(agentBindings()?.GetCardFlow)
}

// waited says how long the card has stood where it stands, in the units a
// person would use out loud. The number and the unit are kept apart so the
// wording stays translatable.
export function waited(since: string | undefined, now: number): {value: number, unit: string} | null {
    if (!since) {
        return null
    }
    const started = Date.parse(since)
    if (isNaN(started)) {
        return null
    }

    // Go's zero time is a valid date — year one — and a count in the hundred
    // thousands of days is what it
    // reads as after the subtraction. A card whose stage entry was never
    // stamped has no answer to "how long", not a nine-century one.
    if (started <= 0) {
        return null
    }
    const minutes = Math.max(0, Math.floor((now - started) / 60000))
    if (minutes < 60) {
        return {value: minutes, unit: 'minutes'}
    }
    const hours = Math.floor(minutes / 60)
    if (hours < 24) {
        return {value: hours, unit: 'hours'}
    }
    return {value: Math.floor(hours / 24), unit: 'days'}
}

type Props = {
    cardId: string
}

const FlowStrip = (props: Props) => {
    const intl = useIntl()
    const bindings = agentBindings()
    const [flow, reloadFlow] = createLoader(async (cardId: string): Promise<CardFlow | null> => {
        if (!bindings?.GetCardFlow) {
            return null
        }
        try {
            return JSON.parse(await bindings.GetCardFlow(cardId))
        } catch {
            return null
        }
    }, null as CardFlow | null, () => props.cardId)

    onSettled(() => {
        // A session starting, finishing or moving the card changes the answer.
        const off = onAgentEvent('acp:session', () => reloadFlow())

        // So does a transition stopping to ask, which is announced as the wait
        // it is rather than as a session — nothing is running.
        const offWait = onAgentEvent('acp:attention', () => reloadFlow())
        return () => {
            off?.()
            offWait?.()
        }
    })

    // Marking is a write to the board, and the board tells the route, and the
    // route moves the card — so what comes back is a card standing somewhere
    // else. Refreshed rather than assumed: the mark may lead nowhere if
    // somebody edited the route while this card sat here.
    const [marking, setMarking] = createSignal(false)
    const answer = async (choice: CardFlowMark) => {
        if (!bindings?.MarkCardOutcome || marking()) {
            return
        }
        setMarking(true)
        try {
            await bindings.MarkCardOutcome(props.cardId, choice.value)
        } finally {
            setMarking(false)
        }
        reloadFlow()
    }

    // Answering releases the card: the last answer finishes the move that was
    // interrupted, so what comes back is a card standing somewhere else. The
    // same "ask, then look again" the marks take, for the same reason.
    const [answering, setAnswering] = createSignal(false)
    const answerAsk = async (option: {value: string}) => {
        const ask = flow()?.ask
        if (!bindings?.AnswerCardAsk || !ask || answering()) {
            return
        }
        setAnswering(true)
        try {
            await bindings.AnswerCardAsk(props.cardId, ask.questionId, option.value)
        } finally {
            setAnswering(false)
        }
        reloadFlow()
    }

    const since = () => {
        const age = waited(flow()?.since, Date.now())
        return age === null ? '' : sinceLabel(intl as Parameters<typeof sinceLabel>[0], age)
    }
    const status = () => {
        const ask = flow()?.ask
        if (ask) {
            return intl.formatMessage(
                {id: 'FlowStrip.asking', defaultMessage: 'waiting for an answer before “{column}”'},
                {column: ask.column},
            )
        }
        if (flow()?.running) {
            return intl.formatMessage({id: 'FlowStrip.running', defaultMessage: 'working now'})
        }
        if (flow()?.queued) {
            return intl.formatMessage({id: 'FlowStrip.queued', defaultMessage: 'waiting for a free place in the column'})
        }

        // The reason arrives as a whole sentence from the Go side, in the
        // language the machinery failed in; it carries its own verb.
        if (flow()?.stalled) {
            return flow()!.stalled!
        }
        const waitingFor = flow()?.waitingFor
        if (waitingFor && waitingFor.length > 0) {
            return intl.formatMessage({id: 'FlowStrip.waiting', defaultMessage: 'waiting: {events}'}, {events: waitingFor.join(', ')})
        }
        return intl.formatMessage({id: 'FlowStrip.idle', defaultMessage: 'nothing to wait for here — the card moves on by hand'})
    }

    return (
        <Show when={flow() && flow()!.stages.length > 0}>
            <div class='FlowStrip'>
                <div class='FlowStrip__head'>
                    <span class='FlowStrip__name'>{flow()!.flow}</span>
                    <Show when={flow()!.branch}>
                        <span class='FlowStrip__branch'>{flow()!.branch}</span>
                    </Show>
                </div>
                <div class='FlowStrip__stages'>
                    <For each={flow()!.stages}>
                        {(stage) => (
                            <span
                                class={`FlowStrip__stage${stage.current ? ' FlowStrip__stage--current' : ''}${stage.done ? ' FlowStrip__stage--done' : ''}`}
                                title={stage.crew && stage.crew.length > 0 ? stage.crew.join(', ') : ''}
                            >{stage.column}</span>
                        )}
                    </For>
                </div>
                {/* A stage where nobody works and nothing runs waits for a
                    person to say how it went, and saying so used to mean
                    finding a field on the card and picking a value out of it.
                    There are two answers and the route knows where each leads,
                    so they are two buttons naming the stage they lead to. */}
                {/* A transition that asks holds the card on this side of it.
                    The question is drawn where the two outcome buttons are,
                    and for the same reason: the answer belongs on the card's
                    face rather than three clicks inside a field. */}
                <Show when={flow()!.ask}>
                    {(ask) => (
                        <div class='FlowStrip__ask'>
                            <div class='FlowStrip__askText'>{ask().text}</div>
                            <div class='FlowStrip__marks'>
                                <For each={ask().options}>
                                    {(option) => (
                                        <button
                                            type='button'
                                            class='FlowStrip__mark FlowStrip__mark--forward'
                                            disabled={answering()}
                                            onClick={() => answerAsk(option)}
                                        >
                                            {option.label}
                                        </button>
                                    )}
                                </For>
                            </div>
                            <Show when={(ask().left || 0) > 1}>
                                <div class='FlowStrip__askLeft'>
                                    {intl.formatMessage(
                                        {id: 'FlowStrip.ask-left', defaultMessage: '{count, plural, one {# question} few {# questions} other {# questions}} left'},
                                        {count: ask().left},
                                    )}
                                </div>
                            </Show>
                        </div>
                    )}
                </Show>

                <Show when={(flow()!.marks || []).length > 0 && !flow()!.running}>
                    <div class='FlowStrip__marks'>
                        <For each={flow()!.marks}>
                            {(mark) => (
                                <button
                                    type='button'
                                    class={`FlowStrip__mark${mark.forward ? ' FlowStrip__mark--forward' : ' FlowStrip__mark--back'}`}
                                    disabled={marking()}
                                    title={intl.formatMessage(
                                        {id: 'FlowStrip.mark-title', defaultMessage: 'Mark “{value}” — the card goes to “{column}”'},
                                        {value: mark.value, column: mark.column},
                                    )}
                                    onClick={() => answer(mark)}
                                >
                                    <Show when={!mark.forward}><CompassIcon icon='chevron-left'/></Show>
                                    <span>{mark.column}</span>
                                    <Show when={mark.forward}><CompassIcon icon='chevron-right'/></Show>
                                </button>
                            )}
                        </For>
                    </div>
                </Show>

                <div
                    class={['FlowStrip__status', {'FlowStrip__status--stalled': Boolean(flow()?.stalled) && !flow()?.running && !flow()?.queued}]}
                >
                    {status()}
                    {since() && ` · ${since()}`}
                </div>
            </div>
        </Show>
    )
}

// sinceLabel is "for 20 minutes" in the reader's language.
function sinceLabel(
    intl: {formatMessage: (d: {id: string, defaultMessage: string}, v?: Record<string, unknown>) => string},
    age: {value: number, unit: string},
): string {
    switch (age.unit) {
    case 'hours':
        return intl.formatMessage({id: 'FlowStrip.hours', defaultMessage: '{value} h'}, {value: age.value})
    case 'days':
        return intl.formatMessage({id: 'FlowStrip.days', defaultMessage: '{value} d'}, {value: age.value})
    default:
        return intl.formatMessage({id: 'FlowStrip.minutes', defaultMessage: '{value} min'}, {value: age.value})
    }
}

export default FlowStrip
