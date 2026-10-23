// The Wails-generated Go bindings are PascalCase methods, not constructors.

import {For, Show, createEffect, createSignal, onSettled} from 'solid-js'

import {useIntl} from '../../intl'
import type {IntlShape} from '../../intl'
import {onAgentEvent} from '../../goEvents'
import {attentionHeading, questionText, useAttention} from '../../components/acp/attention'
import AttentionAnswers from '../../components/acp/attentionAnswers'
import {aiEnabled} from '../../aiSwitch'

import {BindingBoard, BindingCard, listBoards, listInbox} from '../../bindings/boards'
import {createLoader} from '../../hooks/loader'

import MobileInbox from './mobileInbox'
import MobileCards from './mobileCards'

import './mobilePage.scss'

// The board on a phone, and deliberately not the board.
//
// A phone is not where columns are dragged or a description is written. It is
// where you find out that something arrived or that an agent is stuck, and do
// the one thing that settles it. So the page is four screens and a row of
// buttons at the bottom, which is where a thumb reaches:
//
//   the inbox view  — what a source left and nobody has looked at, moved onto a
//                 board from here, which is the whole point of an inbox;
//   the cards screen  — one board's cards as a list, to find out where something got
//                 to without walking to the desk;
//   the waiting screen      — what is asking for a person, answered here: the
//                 agent is holding its turn open on the question, and a phone
//                 is where a person is when they are not at the desk.
//
// the waiting screen is what opens, because it is the only one of the three that cannot
// wait; the others say how many are there on their own button.
//
// It needs nothing from the board's own API: everything on it comes from the
// bindings, which the front door serves to a phone exactly as it serves them to
// the window (main.App.* over /wails/runtime) — including the board itself, see
// bindings/boards.ts. What it does need besides is the event socket, since the
// Wails bus does not leave the machine — see components/acp/agentEvents.

type Tab = 'inbox' | 'cards' | 'waiting'

// The tab that is about agents, and is not drawn with the AI switched off.
const agentTabs: Tab[] = ['waiting']

const MobilePage = () => {
    const intl = useIntl()
    const waiting = useAttention()
    const [picked, setTab] = createSignal<Tab>(aiEnabled() ? 'waiting' : 'inbox')

    // With the AI switched off the phone is the inbox and the cards: the two
    // agent tabs are not drawn, and one that was open gives way to the inbox.
    const tab = (): Tab => (aiEnabled() || !agentTabs.includes(picked()) ? picked() : 'inbox')

    // The inbox and the boards are read here rather than inside the tab that
    // shows them, because the number on a tab has to be right before anybody
    // opens it — a tab that only counts once you are looking at it counts
    // nothing. The boards are read once and lent to both tabs that need them.
    const [inbox, reloadInbox] = createLoader(() => listInbox(), [] as BindingCard[])
    const [boards] = createLoader(() => listBoards(), [] as BindingBoard[])

    // The phone app shows one machine per tab, each tab a frame holding that
    // machine's board (mobile/frontend/index.html), and puts the number waiting
    // on the tab — so which desktop needs a person is visible without opening
    // its tab. The frame is cross-origin, so the number is posted out rather
    // than read in. The target origin is '*' because the app's own page is not
    // served by us and its origin is the platform's business; a count of open
    // questions is nothing to protect, and the frame's own origin is what the
    // app checks on the way in.
    createEffect(() => waiting().length, (count) => {
        if (window.parent === window) {
            return
        }
        try {
            window.parent.postMessage({type: 'pourquoipas:waiting', count}, '*')
        } catch {
            // Whatever is holding this page is not listening; nothing here
            // depends on it.
        }
    })

    // A card being written is a card that may have arrived, so the event that
    // redraws a board also asks the inbox to look again.
    onSettled(() => onAgentEvent('acp:session', () => reloadInbox()))

    return (
        <div class='MobilePage'>
            <header class='MobilePage__header'>
                <span class='MobilePage__wordmark'>{'Pourquoi Pas?'}</span>
                <span class='MobilePage__subtitle'>
                    {intl.formatMessage({id: 'Mobile.subtitle', defaultMessage: 'What needs you'})}
                </span>
            </header>

            <Show when={tab() === 'inbox'}>
                <MobileInbox
                    cards={inbox()}
                    boards={boards()}
                    onMoved={reloadInbox}
                />
            </Show>

            <Show when={tab() === 'cards'}>
                <MobileCards boards={boards()}/>
            </Show>

            <Show when={tab() === 'waiting'}>
                <section class='MobilePage__section'>
                    <h2 class='MobilePage__heading'>
                        {intl.formatMessage({id: 'Mobile.waiting', defaultMessage: 'Waiting for an answer'})}
                    </h2>
                    <Show
                        when={waiting().length > 0}
                        fallback={
                            <p class='MobilePage__empty'>
                                {intl.formatMessage({id: 'Mobile.nothing-waiting', defaultMessage: 'Nothing is waiting. The agents are working.'})}
                            </p>
                        }
                    >
                        <For each={waiting()}>
                            {(target) => (
                                <article class='MobilePage__item'>
                                    <span class='MobilePage__who'>{attentionHeading(intl, target)}</span>
                                    <span class='MobilePage__card'>
                                        {target.title || intl.formatMessage({id: 'Attention.untitled', defaultMessage: 'Untitled card'})}
                                    </span>

                                    <Show when={target.text}>
                                        <p class='MobilePage__question'>{questionText(intl, target)}</p>
                                    </Show>

                                    {/* Answered here, which on a phone is the
                                        whole point: the agent is waiting on it
                                        and the person is not at the desk
                                        (attentionAnswers). */}
                                    <AttentionAnswers target={target}/>
                                </article>
                            )}
                        </For>
                    </Show>
                </section>
            </Show>

            {/* The row of buttons is at the bottom because that is where a
                thumb is, and it is fixed because the lists above it scroll. */}
            {/* By position, not by entry: the entries are rebuilt whenever a
                count changes, and keyed rows would replace every button then —
                a tap landing on one that was just replaced is a tap lost. */}
            <nav class='MobilePage__tabs'>
                <For
                    each={tabs(intl, inbox().length, waiting().length)}
                    keyed={false}
                >
                    {(entry) => (
                        <button
                            type='button'
                            class={`MobilePage__tab${tab() === entry().id ? ' MobilePage__tab--on' : ''}`}
                            aria-current={tab() === entry().id ? 'page' : undefined}
                            onClick={() => setTab(entry().id)}
                        >
                            <span class='MobilePage__tab-name'>{entry().name}</span>
                            <Show when={entry().count > 0}>
                                <span class='MobilePage__tab-count'>{entry().count}</span>
                            </Show>
                        </button>
                    )}
                </For>
            </nav>
        </div>
    )
}

// tabs is the bar, in the order things happen: something arrives, it becomes a
// card, an agent asks about it. Each carries how many are behind it, so a tab
// that is not open can still say that it wants attention.
function tabs(intl: IntlShape, inbox: number, waiting: number) {
    const all = [
        {id: 'inbox' as Tab, name: intl.formatMessage({id: 'Mobile.inbox', defaultMessage: 'Inbox'}), count: inbox},
        {id: 'cards' as Tab, name: intl.formatMessage({id: 'Mobile.cards', defaultMessage: 'Cards'}), count: 0},
        {id: 'waiting' as Tab, name: intl.formatMessage({id: 'Mobile.waiting-tab', defaultMessage: 'Waiting'}), count: waiting},
    ]
    return aiEnabled() ? all : all.filter((entry) => !agentTabs.includes(entry.id))
}

export default MobilePage
