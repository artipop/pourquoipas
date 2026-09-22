import {For, Show, createEffect, createSignal, onSettled} from 'solid-js'
import type {Component} from 'solid-js'
import {useSearchParams} from '@solidjs/router'

import {FormattedMessage, useIntl} from '../../intl'
import {BindingBoard, listBoards} from '../../bindings/boards'
import {createLoader} from '../../hooks/loader'

import {closeShare, shareItem} from './shareBindings'
import './sharePage.scss'

// The dialog the system's the share dialog opens.
//
// Everything it is given arrives in the URL — the share extension has no way to
// hand over anything else, and a page that reads its input from the address is
// a page a phone, a browser and the app's own little window can all open. What
// it asks is one question: which board. The title and the note are there
// because a link is easier to find again when it says what it was for, and both
// have sensible answers already filled in.

// The inbox first, then the rest in the order Go lists them: collect first and
// decide later is the point of an inbox, so a link sent to oneself goes there
// unless the person already knows where it belongs.
export function inboxFirst(boards: BindingBoard[]): BindingBoard[] {
    return [...boards.filter((b) => b.inbox), ...boards.filter((b) => !b.inbox)]
}

const SharePage: Component = () => {
    const intl = useIntl()
    const [params] = useSearchParams()

    const [boards] = createLoader(async () => inboxFirst(await listBoards()), undefined as BindingBoard[] | undefined)
    const [target, setTarget] = createSignal('')
    const [title, setTitle] = createSignal('')
    const [note, setNote] = createSignal('')
    const [busy, setBusy] = createSignal(false)
    const [error, setError] = createSignal('')
    const [done, setDone] = createSignal<'created' | 'already' | ''>('')

    const url = () => String(params.url || '')

    // The title is what the sharing app called the page, and the note starts
    // with anything it sent as text — a selection, usually, which is exactly
    // what somebody sharing a quote wants to keep.
    onSettled(() => {
        setTitle(String(params.title || ''))
        setNote(String(params.text || ''))
    })

    // Preselected once the boards are known: the first, which is the inbox. A
    // dialog that opens with nothing selected asks two questions instead of
    // one. Not the board picked last time: that would pull every later link
    // away from the inbox after a single exception.
    createEffect(() => [target(), boards()] as const, ([chosen, available]) => {
        if (chosen || !available?.length) {
            return
        }
        setTarget(available[0].id)
    })
    const toInbox = () => Boolean(boards()?.find((board) => board.id === target())?.inbox)

    const send = async () => {
        const boardId = target()
        if (!boardId || busy()) {
            return
        }
        setBusy(true)
        setError('')
        try {
            const result = await shareItem(boardId, title(), url(), note())
            if (result.failed) {
                // The pipeline counts a refusal rather than throwing it, so a
                // batch of one that failed would otherwise look like success.
                setError(intl.formatMessage({id: 'Share.failed', defaultMessage: 'Could not file it. Try again.'}))
                return
            }
            setDone(result.created ? 'created' : 'already')

            // Left on screen for a moment: the window closing is the only
            // acknowledgement there is, and one that happens instantly reads as
            // the button not having worked.
            setTimeout(() => {
                closeShare()
            }, 900)
        } catch (e: any) {
            setError(String(e?.message || e))
        } finally {
            setBusy(false)
        }
    }

    return (
        <div class='SharePage'>
            <Show
                when={!done()}
                fallback={
                    <div class='SharePage__done'>
                        <Show
                            when={toInbox()}
                            fallback={
                                <Show
                                    when={done() === 'created'}
                                    fallback={
                                        <FormattedMessage
                                            id='Share.already-board'
                                            defaultMessage='It is already on the board.'
                                        />
                                    }
                                >
                                    <FormattedMessage
                                        id='Share.created-board'
                                        defaultMessage='Filed on the board.'
                                    />
                                </Show>
                            }
                        >
                            <Show
                                when={done() === 'created'}
                                fallback={
                                    <FormattedMessage
                                        id='Share.already'
                                        defaultMessage='It is already in the inbox.'
                                    />
                                }
                            >
                                <FormattedMessage
                                    id='Share.created'
                                    defaultMessage='Filed in the inbox.'
                                />
                            </Show>
                        </Show>
                    </div>
                }
            >
                <h1 class='SharePage__heading'>
                    <FormattedMessage
                        id='Share.title'
                        defaultMessage='Save to a board'
                    />
                </h1>

                <input
                    class='SharePage__title'
                    type='text'
                    value={title()}
                    placeholder={intl.formatMessage({id: 'Share.untitled', defaultMessage: 'Title'})}
                    onInput={(e) => setTitle(e.currentTarget.value)}
                />
                <Show when={url()}>
                    <p class='SharePage__url'>{url()}</p>
                </Show>
                <textarea
                    class='SharePage__note'
                    rows='2'
                    value={note()}
                    placeholder={intl.formatMessage({id: 'Share.note', defaultMessage: 'A note, if you want one'})}
                    onInput={(e) => setNote(e.currentTarget.value)}
                />

                <h2 class='SharePage__label'>
                    <FormattedMessage
                        id='Share.board'
                        defaultMessage='Which board?'
                    />
                </h2>
                <Show when={boards()}>
                    <Show
                        when={(boards() || []).length > 0}
                        fallback={
                            <p class='SharePage__empty'>
                                <FormattedMessage
                                    id='Share.no-boards'
                                    defaultMessage='There are no boards yet.'
                                />
                            </p>
                        }
                    >
                        <div class='SharePage__boards'>
                            <For each={boards() || []}>
                                {(board) => (
                                    <button
                                        type='button'
                                        class={['SharePage__board', {'SharePage__board--picked': target() === board.id}]}
                                        aria-pressed={String(target() === board.id)}
                                        onClick={() => setTarget(board.id)}
                                    >
                                        <span class='SharePage__icon'>{board.icon}</span>
                                        {board.title}
                                    </button>
                                )}
                            </For>
                        </div>
                    </Show>
                </Show>

                <Show when={error()}>
                    <p class='SharePage__error'>{error()}</p>
                </Show>

                <div class='SharePage__actions'>
                    <button
                        type='button'
                        class='SharePage__cancel'
                        onClick={() => {
                            closeShare()
                        }}
                    >
                        <FormattedMessage
                            id='Share.cancel'
                            defaultMessage='Cancel'
                        />
                    </button>
                    <button
                        type='button'
                        class='SharePage__send'
                        disabled={busy() || !target()}
                        onClick={() => {
                            send()
                        }}
                    >
                        <FormattedMessage
                            id='Share.send'
                            defaultMessage='Save'
                        />
                    </button>
                </div>
            </Show>
        </div>
    )
}

export default SharePage
