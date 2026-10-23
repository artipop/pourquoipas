import {Show, createEffect, createSignal, onCleanup} from 'solid-js'

import {useIntl} from '../../intl'

import {Card} from '../../blocks/card'
import {aiEnabled} from '../../aiSwitch'
import {isHosted} from '../../buildKind'
import Button from '../../widgets/buttons/button'
import IconButton from '../../widgets/buttons/iconButton'
import CloseIcon from '../../widgets/icons/close'

import {HINT_FIRST_RUN, useHints} from '../onboardingHints/hints'

import {onAgentEvent} from './agentEvents'

import './firstRunStrip.scss'

type Props = {
    cards: Card[]

    // The property the board's columns are, to tell when the card has left
    // the one it was running in.
    columnPropertyId?: string

    // The board's «Where to start» card, when it has one.
    readFirst?: {title: string, open: () => void}
}

type SessionEvent = {
    cardId?: string
    status?: string
}

const ENDED = ['done', 'failed', 'cancelled']

// FirstRunStrip is what somebody watching their first card run does not know
// yet: where the agent's answer and its questions will be, and that it moves
// the card on itself. Shown over the board while the first card an agent works
// is running, and never again once the card has left its column, the run has
// ended or the strip has been closed. Driven by the session events already on
// the socket — nothing is polled.
const FirstRunStrip = (props: Props) => {
    const intl = useIntl()
    const hints = useHints()

    // A server has no folders, so its agents' results are the comment alone.
    const hosted = isHosted()

    // The card, and the column it was in when its run was first heard of.
    const [watched, setWatched] = createSignal<{cardId: string, column: string} | undefined>(undefined)

    const columnOf = (cardId: string): string => {
        const card = props.cards.find((c) => c.id === cardId)
        const value = card && props.columnPropertyId ? card.fields.properties[props.columnPropertyId] : ''
        return Array.isArray(value) ? value[0] || '' : value || ''
    }

    const done = () => {
        setWatched(undefined)
        hints.markShown(HINT_FIRST_RUN)
    }

    const off = onAgentEvent('acp:session', (payload?: SessionEvent) => {
        if (!payload?.cardId || hints.shown(HINT_FIRST_RUN)) {
            return
        }
        const current = watched()
        if (!current && payload.status === 'running' && props.cards.some((c) => c.id === payload.cardId)) {
            setWatched({cardId: payload.cardId, column: columnOf(payload.cardId)})
        } else if (current?.cardId === payload.cardId && ENDED.includes(payload.status || '')) {
            done()
        }
    })
    onCleanup(off)

    // The agent moves the card on when it is done, and a person may move it
    // first: either way the card is no longer where it was running.
    createEffect(() => {
        const current = watched()
        return current ? columnOf(current.cardId) !== current.column : false
    }, (left) => {
        if (left) {
            done()
        }
    })

    const card = () => props.cards.find((c) => c.id === watched()?.cardId)

    return (
        <Show when={aiEnabled() && card() && !hints.shown(HINT_FIRST_RUN)}>
            <div
                class='FirstRunStrip'
                role='status'
            >
                <div class='FirstRunStrip__text'>
                    <b>{intl.formatMessage({id: 'FirstRun.working', defaultMessage: 'The agent is working on “{title}”.'}, {title: card()!.title})}</b>
                    {' '}
                    <Show
                        when={hosted}
                        fallback={intl.formatMessage({id: 'FirstRun.how-desktop', defaultMessage: 'If it has a question, the question appears on the card and in a notification. Its result arrives as a comment on the card, and what it changes in the folder is under “Changes”. When it is done, it moves the card into the column whose name says what happened.'})}
                    >
                        {intl.formatMessage({id: 'FirstRun.how-server', defaultMessage: 'If it has a question, the question appears on the card and in a notification. Its result arrives as a comment on the card. When it is done, it moves the card into the column whose name says what happened.'})}
                    </Show>
                </div>
                <Show when={props.readFirst}>
                    {(guide) => (
                        <Button onClick={() => guide().open()}>
                            {intl.formatMessage({id: 'FirstRun.read-first', defaultMessage: 'How the board works'})}
                        </Button>
                    )}
                </Show>
                <IconButton
                    icon={<CloseIcon/>}
                    title={intl.formatMessage({id: 'FirstRun.close', defaultMessage: 'Close'})}
                    onClick={done}
                />
            </div>
        </Show>
    )
}

export default FirstRunStrip
