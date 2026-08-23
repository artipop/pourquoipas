// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {For, Show, createSignal, onCleanup, onMount} from 'solid-js'

import {useIntl} from '../../intl'

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
    const [flow, setFlow] = createSignal<CardFlow | null>(null)

    const refresh = async () => {
        if (!bindings?.GetCardFlow) {
            return
        }
        try {
            setFlow(JSON.parse(await bindings.GetCardFlow(props.cardId)))
        } catch (e) {
            setFlow(null)
        }
    }

    onMount(() => {
        refresh()

        // A session starting, finishing or moving the card changes the answer.
        const off = onAgentEvent('acp:session', () => refresh())
        onCleanup(() => off?.())
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
        await refresh()
    }

    const since = () => {
        const age = waited(flow()?.since, Date.now())
        return age === null ? '' : sinceLabel(intl as Parameters<typeof sinceLabel>[0], age)
    }
    const status = () => {
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
                                    {mark.forward ? `${mark.column} →` : `← ${mark.column}`}
                                </button>
                            )}
                        </For>
                    </div>
                </Show>

                <div
                    class='FlowStrip__status'
                    classList={{'FlowStrip__status--stalled': Boolean(flow()?.stalled) && !flow()?.running && !flow()?.queued}}
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
