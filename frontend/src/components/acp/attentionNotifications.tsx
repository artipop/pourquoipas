import {For, Show, createMemo} from 'solid-js'

import {useIntl} from '../../intl'

import CompassIcon from '../../widgets/icons/compassIcon'

import {useAppSelector} from '../../store/hooks'
import {getMe} from '../../store/users'
import {getTeamMode} from '../../store/clientConfig'
import {getUserBlockSubscriptions} from '../../store/initialLoad'
import {getCards} from '../../store/cards'
import {getBoards} from '../../store/boards'
import {IUser} from '../../user'
import {Subscription} from '../../wsclient'

import {
    Attention,
    ackWait,
    agentNotificationsOn,
    attentionHeading,
    keyOf,
    useAttention,
} from './attention'
import AttentionAnswers from './attentionAnswers'
import {askedInDialog} from './askDialog'

import {waitAudience, waitIsMine} from './attentionAudience'

import './attentionNotifications.scss'

// Being told, rather than having to look.
//
// An agent works on a card in a window nobody is watching, and now and then it
// needs a person: a decision only somebody with the product in their head can
// make, or permission for something it was not given. The card grows an amber
// button for that, but a button is only seen by somebody already looking at the
// board — so the wait says itself, here.
//
// It carries the question and its answers: the agent's turn is open until
// somebody answers, and the person being interrupted is the person who can
// (attentionAnswers.tsx).
//
// It is a setting because it interrupts: a person who would rather find out by
// looking turns it off, and the card keeps its button.

// stackLimit is how many waits are worth showing at once. Beyond that they stop
// being a notification and become a wall, so the rest are counted instead.
const stackLimit = 3

// Being told is a thing that happens once, and what remembers that is the Go
// side (attention.ts, internal/acp/attentionack.go) — not a list kept here.
//
// A list kept in one page was wrong twice over: the board's window, a second
// window and the phone each drew their own copy, and a reload brought back
// everything a person had already dealt with.

// phoneWaits is the phone's own waiting screen, which lists the same waits with
// the same answers: the stack over it would say everything twice.
//
// Read once rather than watched: a window opened there stays there.
const phoneWaits = (): boolean => (/^\/m\/waiting/).test(window.location.pathname)

const AttentionNotifications = () => {
    const intl = useIntl()
    const waiting = useAttention()

    // Whose wait this is. On an install of one person the question does not
    // arise; in a team a box saying "an agent is waiting" about somebody else's
    // card is the noise this is meant to be the opposite of
    // (attentionAudience.ts, docs/teamwork.md).
    const teamMode = useAppSelector<boolean>(getTeamMode)
    const me = useAppSelector<IUser|null>(getMe)
    const cards = useAppSelector(getCards)
    const boards = useAppSelector(getBoards)
    const following = useAppSelector<Subscription[]>(getUserBlockSubscriptions)

    const mine = (a: Attention): boolean => waitIsMine({
        teamMode: teamMode(),
        myId: me()?.id,
        audience: waitAudience(a.cardId ? cards()[a.cardId] : undefined, a.boardId ? boards()[a.boardId] : undefined),
        following: Boolean(a.cardId) && following().some((s) => s.blockId === a.cardId),
    })

    // Deduped by what identifies a wait, not by the object: the list is rebuilt
    // from events and from a full reload when the socket reconnects, and one
    // wait drawn twice is the notification a person cannot tell from a second
    // agent asking the same thing.
    const pending = createMemo(() => {
        if (!agentNotificationsOn() || phoneWaits()) {
            return []
        }
        const byKey = new Map<string, Attention>()
        for (const a of waiting()) {
            // The wait the dialog is asking about is not announced: it is on
            // screen already, in front of the person who caused it, and a box
            // saying there is a question over the box asking it is exactly the
            // noise the dialog exists to remove (askDialog.tsx).
            if (!a.acked && mine(a) && keyOf(a) !== askedInDialog()) {
                byKey.set(keyOf(a), a)
            }
        }
        return [...byKey.values()]
    })
    const shown = createMemo(() => pending().slice(0, stackLimit))
    const hidden = createMemo(() => Math.max(0, pending().length - stackLimit))

    return (
        <Show when={shown().length > 0}>
            <div class='AttentionNotifications'>
                {/* Said once, above the stack, as soon as there is more than
                    one: two cards waiting used to be two cards' worth of
                    notification with nothing saying they were two. */}
                <Show when={pending().length > 1}>
                    <div class='AttentionNotifications__count'>
                        {intl.formatMessage({id: 'Attention.waiting-count', defaultMessage: '{count, plural, one {# agent is waiting} other {# agents are waiting}}'}, {count: pending().length})}
                    </div>
                </Show>
                {/* The waits scroll inside the stack rather than the stack
                    growing past the top of the window: a question carries its
                    own answers, so two or three of them are already taller than
                    a laptop screen, and what ran off the top was the oldest
                    wait — the one that has gone unanswered longest. */}
                <div class='AttentionNotifications__list'>
                    <For each={shown()}>
                        {(target) => (
                            <div
                                class='AttentionNotifications__item'
                                role='alert'
                            >
                                <div class='AttentionNotifications__body'>
                                    <span class='AttentionNotifications__heading'>
                                        {attentionHeading(intl, target)}
                                    </span>
                                    <span class='AttentionNotifications__card'>
                                        {target.title || intl.formatMessage({id: 'Attention.untitled', defaultMessage: 'Untitled card'})}
                                    </span>

                                    <Show when={target.text}>
                                        <span class='AttentionNotifications__question'>{target.text}</span>
                                    </Show>

                                    {/* The answer belongs where the question is
                                        read: the agent is holding its turn open
                                        on it (attentionAnswers.tsx). A card the
                                        route handed to a person carries no
                                        answers — its ✕ is the whole of it
                                        (internal/acp/cardwait.go). */}
                                    <AttentionAnswers target={target}/>
                                </div>
                                <button
                                    type='button'
                                    class='AttentionNotifications__close'
                                    title={intl.formatMessage({id: 'Attention.dismiss', defaultMessage: 'Dismiss'})}
                                    aria-label={intl.formatMessage({id: 'Attention.dismiss', defaultMessage: 'Dismiss'})}
                                    onClick={() => ackWait(target)}
                                >
                                    <CompassIcon icon='close'/>
                                </button>
                            </div>
                        )}
                    </For>
                </div>
                <Show when={hidden() > 0}>
                    <div class='AttentionNotifications__more'>
                        {intl.formatMessage({id: 'Attention.more', defaultMessage: '…and {count} more waiting'}, {count: hidden()})}
                    </div>
                </Show>
            </div>
        </Show>
    )
}

export default AttentionNotifications
