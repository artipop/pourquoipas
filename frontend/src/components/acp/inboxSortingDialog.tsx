// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {For, Show, createSignal, onSettled} from 'solid-js'

import {useIntl} from '../../intl'

import Button from '../../widgets/buttons/button'
import Dialog from '../dialog'

import {goApp} from '../../goBridge'
import {GoMessage, goMessage} from '../../goMessages'

import './inboxSortingDialog.scss'

// Sorting the inbox: an agent carries what arrives on to the board it belongs
// on (internal/inboxsort). Opened from the inbox's own menu, beside the
// sources, because it is about what the sources bring.
//
// What the agent decides from is on this screen too — each board's
// description, which a person writes, and its summary, which the agent writes
// from the board's cards — so somebody whose card went to the wrong board can
// see why and say it better.

export type SortingBoard = {
    id: string
    title: string
    icon?: string
    description: string
    summary: string
    summaryAt?: string
    cardCount: number
}

export type SortingDecision = {
    id: string
    cardId?: string
    cardTitle?: string
    boardId?: string
    outcome: 'moved' | 'kept' | 'failed'
    reason?: GoMessage
    at: string
}

export type SortingState = {
    enabled: boolean
    agent?: string
    available: boolean
    boards: SortingBoard[]
    decisions: SortingDecision[]
}

export function isInboxSortingAvailable(): boolean {
    return Boolean(goApp()?.GetInboxSorting)
}

type Props = {
    onClose: () => void
}

const InboxSortingDialog = (props: Props) => {
    const intl = useIntl()
    const bindings = goApp()

    const [state, setState] = createSignal<SortingState | null>(null)
    const [agents, setAgents] = createSignal<string[]>([])
    const [error, setError] = createSignal('')
    const [notice, setNotice] = createSignal('')
    const [busy, setBusy] = createSignal(false)

    const refresh = async () => {
        if (!bindings?.GetInboxSorting) {
            return
        }
        try {
            setState(JSON.parse(await bindings.GetInboxSorting()))
            if (bindings.ListAgents) {
                const listed: Array<{name: string}> = JSON.parse(await bindings.ListAgents()) || []
                setAgents(listed.map((a) => a.name))
            }
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    // Not awaited: onSettled reads a returned promise as a cleanup value.
    onSettled(() => {
        void refresh()
    })

    const run = async (work: () => Promise<void>) => {
        setBusy(true)
        setError('')
        setNotice('')
        try {
            await work()
            await refresh()
        } catch (e: any) {
            setError(String(e?.message || e))
        } finally {
            setBusy(false)
        }
    }

    const save = (enabled: boolean, agent: string) => run(async () => {
        await bindings!.SetInboxSorting(JSON.stringify({enabled, agent}))
    })

    const sortNow = () => run(async () => {
        const res = JSON.parse(await bindings!.SortInboxNow())
        setNotice(intl.formatMessage(
            {id: 'InboxSorting.sorted', defaultMessage: 'Moved: {moved}. Left in the inbox: {kept}. Failed: {failed}.'},
            {moved: res.moved || 0, kept: res.kept || 0, failed: res.failed || 0},
        ))
    })

    const refreshSummaries = () => run(async () => {
        await bindings!.RefreshBoardSummaries()
    })

    const saveDescription = (board: SortingBoard, text: string) => {
        if (text.trim() === (board.description || '').trim()) {
            return
        }
        run(async () => {
            await bindings!.SetBoardDescription(board.id, text)
        })
    }

    const agent = () => state()?.agent || ''
    const boardTitle = (id?: string) => state()?.boards.find((b) => b.id === id)?.title || '—'

    const outcomeText = (d: SortingDecision) => {
        switch (d.outcome) {
        case 'moved':
            return intl.formatMessage({id: 'InboxSorting.outcome-moved', defaultMessage: 'moved to «{board}»'}, {board: boardTitle(d.boardId)})
        case 'kept':
            return intl.formatMessage({id: 'InboxSorting.outcome-kept', defaultMessage: 'left in the inbox'})
        default:
            return intl.formatMessage({id: 'InboxSorting.outcome-failed', defaultMessage: 'not sorted'})
        }
    }

    return (
        <Dialog
            class='InboxSortingDialog'
            title={<span>{intl.formatMessage({id: 'InboxSorting.title', defaultMessage: 'Sorting the inbox'})}</span>}
            subtitle={
                <span>
                    {intl.formatMessage({
                        id: 'InboxSorting.subtitle',
                        defaultMessage: 'An agent reads each card in the inbox and moves it to the board it belongs on. It decides by the board’s description and by a summary of the board’s cards. When no board fits, the card stays in the inbox.',
                    })}
                </span>
            }
            onClose={props.onClose}
        >
            <div class='InboxSortingDialog__content'>
                <Show when={state() && !state()!.available}>
                    <p class='InboxSortingDialog__hint'>
                        {intl.formatMessage({id: 'InboxSorting.unavailable', defaultMessage: 'Agents are turned off in this app, so the inbox is not sorted. Turn them on in the settings.'})}
                    </p>
                </Show>

                <section class='InboxSortingDialog__section'>
                    <label class='InboxSortingDialog__field'>
                        {intl.formatMessage({id: 'InboxSorting.agent', defaultMessage: 'Agent that sorts'})}
                        <select
                            value={agent()}
                            disabled={busy()}
                            onChange={(e) => save(Boolean(state()?.enabled && e.currentTarget.value), e.currentTarget.value)}
                        >
                            <option value=''>{intl.formatMessage({id: 'InboxSorting.agent-none', defaultMessage: 'Not chosen'})}</option>
                            <For each={agents()}>
                                {(name) => <option value={name}>{name}</option>}
                            </For>
                        </select>
                    </label>
                    <label class='InboxSortingDialog__check'>
                        <input
                            type='checkbox'
                            checked={Boolean(state()?.enabled)}
                            disabled={busy() || !agent()}
                            onChange={(e) => save(e.currentTarget.checked, agent())}
                        />
                        {intl.formatMessage({id: 'InboxSorting.enabled', defaultMessage: 'Sort cards from sources as they arrive'})}
                    </label>
                    <p class='InboxSortingDialog__hint'>
                        {intl.formatMessage({id: 'InboxSorting.enabled-hint', defaultMessage: 'When this is off, cards stay in the inbox until you move them or press «Sort now». A card moved by the agent stands in the first column of its board and does not start that column’s automation.'})}
                    </p>
                    <div class='InboxSortingDialog__actions'>
                        <Button
                            emphasis='primary'
                            disabled={busy() || !agent()}
                            onClick={sortNow}
                        >
                            {intl.formatMessage({id: 'InboxSorting.sort-now', defaultMessage: 'Sort now'})}
                        </Button>
                        <Button
                            disabled={busy() || !agent()}
                            onClick={refreshSummaries}
                        >
                            {intl.formatMessage({id: 'InboxSorting.refresh-summaries', defaultMessage: 'Update summaries'})}
                        </Button>
                        <Show when={busy()}>
                            <span class='InboxSortingDialog__hint'>
                                {intl.formatMessage({id: 'InboxSorting.busy', defaultMessage: 'The agent is working. This takes up to a few minutes.'})}
                            </span>
                        </Show>
                    </div>
                    <Show when={notice()}>
                        <p class='InboxSortingDialog__notice'>{notice()}</p>
                    </Show>
                    <Show when={error()}>
                        <p class='InboxSortingDialog__error'>{error()}</p>
                    </Show>
                </section>

                <section class='InboxSortingDialog__section'>
                    <h3>{intl.formatMessage({id: 'InboxSorting.boards', defaultMessage: 'Boards'})}</h3>
                    <Show when={(state()?.boards.length || 0) === 0}>
                        <p class='InboxSortingDialog__hint'>
                            {intl.formatMessage({id: 'InboxSorting.no-boards', defaultMessage: 'There are no boards besides the inbox yet.'})}
                        </p>
                    </Show>
                    <For each={state()?.boards || []}>
                        {(board) => (
                            <div class='InboxSortingDialog__board'>
                                <span class='InboxSortingDialog__boardTitle'>
                                    {board.icon ? `${board.icon} ` : ''}{board.title}
                                </span>
                                <label class='InboxSortingDialog__field'>
                                    {intl.formatMessage({id: 'InboxSorting.description', defaultMessage: 'Description'})}
                                    <textarea
                                        rows={2}
                                        value={board.description || ''}
                                        disabled={busy()}
                                        placeholder={intl.formatMessage({id: 'InboxSorting.description-placeholder', defaultMessage: 'What cards belong on this board'})}
                                        onBlur={(e) => saveDescription(board, e.currentTarget.value)}
                                    />
                                </label>
                                <div class='InboxSortingDialog__summary'>
                                    <span class='InboxSortingDialog__label'>
                                        {intl.formatMessage({id: 'InboxSorting.summary', defaultMessage: 'Summary of the cards'})}
                                    </span>
                                    <span>
                                        {board.summary || intl.formatMessage({id: 'InboxSorting.no-summary', defaultMessage: 'None yet. It is written at the next sort, or by «Update summaries».'})}
                                    </span>
                                </div>
                            </div>
                        )}
                    </For>
                </section>

                <section class='InboxSortingDialog__section'>
                    <h3>{intl.formatMessage({id: 'InboxSorting.log', defaultMessage: 'Recent decisions'})}</h3>
                    <Show when={(state()?.decisions.length || 0) === 0}>
                        <p class='InboxSortingDialog__hint'>
                            {intl.formatMessage({id: 'InboxSorting.log-empty', defaultMessage: 'The agent has not sorted anything yet.'})}
                        </p>
                    </Show>
                    <ul class='InboxSortingDialog__log'>
                        <For each={state()?.decisions || []}>
                            {(d) => (
                                <li class={`InboxSortingDialog__decision InboxSortingDialog__decision--${d.outcome}`}>
                                    <span class='InboxSortingDialog__decisionCard'>{d.cardTitle || '—'}</span>
                                    {' — '}
                                    {outcomeText(d)}
                                    <Show when={d.reason}>
                                        <span class='InboxSortingDialog__reason'>{goMessage(intl, d.reason)}</span>
                                    </Show>
                                </li>
                            )}
                        </For>
                    </ul>
                </section>
            </div>
        </Dialog>
    )
}

export default InboxSortingDialog
