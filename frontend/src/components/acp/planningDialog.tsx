// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {For, Show, createMemo, createSignal, onSettled} from 'solid-js'

import {useIntl} from '../../intl'

import {Board} from '../../blocks/board'
import CompassIcon from '../../widgets/icons/compassIcon'
import Dialog from '../dialog'

import {goApp} from '../../goBridge'
import {aiEnabled} from '../../aiSwitch'
import {isHosted} from '../../buildKind'
import {onBoardUsersChanged} from '../../extensions'
import {openSettings} from '../settings/openSettings'

import {onAgentEvent} from './agentEvents'
import FolderChoices from './folderChoices'
import AttentionAnswers from './attentionAnswers'
import {useAttention} from './attention'

import './planningDialog.scss'

// Talking a task through before it exists: a conversation with the agent, in a
// folder, that ends in cards the agent creates on this board.
//
// The conversation is held here rather than in the agent's own CLI: the same
// ACP session a stage runs on (internal/acp/planning.go), so what the agent
// says arrives as text this page can draw, its questions are answered where
// they are read, and the cards it agrees to create it creates through the board
// tools. The dialog is the whole interface — a transcript, a box to type in,
// and the two questions a new conversation needs: which agent, which folder.
//
// A conversation outlives the dialog. Closing the window leaves the agent
// where it is, and opening the dialog again lists what is open with everything
// said in it, so coming back is continuing rather than starting again.

type NamedEntry = {name: string}

type PlanningMessage = {
    role: 'person' | 'agent'
    text: string
}

type PlanningChat = {
    id: string
    boardId: string
    agent: string
    folder?: string
    busy: boolean
    error?: string
    messages: PlanningMessage[]
}

export function isPlanningAvailable(): boolean {
    return aiEnabled() && Boolean(goApp()?.StartPlanning)
}

type Props = {

    // The board the dialog was opened from. Planning has no card, but it may
    // leave cards — and this is the only board it may leave them on.
    board: Board
    onClose: () => void
}

const PlanningDialog = (props: Props) => {
    const intl = useIntl()
    const bindings = goApp()
    const waiting = useAttention()

    const [agents, setAgents] = createSignal<NamedEntry[]>([])
    const [agentName, setAgentName] = createSignal('')
    const [chats, setChats] = createSignal<PlanningChat[]>([])
    const [openID, setOpenID] = createSignal('')
    const [draft, setDraft] = createSignal('')
    const [busy, setBusy] = createSignal(false)
    const [error, setError] = createSignal('')

    // The conversation being read, taken out of the list rather than kept
    // beside it: the list is what Go says, and a copy of one row would go stale
    // the moment a turn ended.
    const open = createMemo(() => chats().find((c) => c.id === openID()))

    // Which agent first, which folder second: the agent question is open while
    // nobody is named, and a single registered agent answers it without being
    // asked.
    const askingAgent = () => !agentName()

    const refreshChats = async () => {
        if (!bindings?.ListPlanning) {
            return
        }
        try {
            const all: PlanningChat[] = JSON.parse(await bindings.ListPlanning(props.board.id)) || []
            setChats(all)
            if (openID() && !all.some((c) => c.id === openID())) {
                setOpenID('')
            }
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    const refreshAgents = async () => {
        if (!bindings?.ListAgents) {
            return
        }
        try {
            const parsed: NamedEntry[] = JSON.parse(await bindings.ListAgents()) || []
            setAgents(parsed)
            if (parsed.length === 1) {
                setAgentName(parsed[0].name)
            }
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    // What the agent is writing right now, while it writes it. The turn's text
    // lands in the conversation when the turn ends (acp:planning); until then
    // this is the only thing that says anything is happening at all.
    const [streaming, setStreaming] = createSignal('')

    onSettled(() => {
        refreshChats()
        refreshAgents()

        const offPlanning = onAgentEvent('acp:planning', () => {
            setStreaming('')
            refreshChats()
        })
        const offChunk = onAgentEvent('acp:chunk', (payload?: {sessionId?: string, text?: string, thought?: boolean}) => {
            if (!payload?.text || payload.thought || payload.sessionId !== openID()) {
                return
            }
            setStreaming((current) => current + payload.text)
        })

        // An agent connected in the settings meanwhile is offered at once.
        const offAgents = onBoardUsersChanged(() => refreshAgents())
        return () => {
            offPlanning()
            offChunk()
            offAgents()
        }
    })

    // Answering the folder question is what starts: a workdir by name, or '' for
    // the board's own drafts folder, resolved on the Go side.
    const start = async (workdirName: string) => {
        if (!bindings?.StartPlanning) {
            return
        }
        setError('')
        setBusy(true)
        try {
            const chat: PlanningChat = JSON.parse(await bindings.StartPlanning(props.board.id, workdirName, agentName()))
            setOpenID(chat.id)
            await refreshChats()
        } catch (e: any) {
            setError(String(e?.message || e))
        } finally {
            setBusy(false)
        }
    }

    const say = async () => {
        const text = draft().trim()
        const chat = open()
        if (!text || !chat || !bindings?.SayToPlanning) {
            return
        }
        setError('')
        setDraft('')
        setStreaming('')
        try {
            await bindings.SayToPlanning(chat.id, text)
            await refreshChats()
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    const stop = async () => {
        const chat = open()
        if (!chat || !bindings?.StopPlanningReply) {
            return
        }
        try {
            await bindings.StopPlanningReply(chat.id)
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    // Ending a conversation ends the agent with it, which is why it is asked
    // about first.
    const [ending, setEnding] = createSignal('')
    const end = async (id: string) => {
        if (!bindings?.EndPlanning) {
            return
        }
        setEnding('')
        try {
            await bindings.EndPlanning(id)
        } catch (e: any) {
            setError(String(e?.message || e))
        }
        if (openID() === id) {
            setOpenID('')
        }
        await refreshChats()
    }

    // The agent's own questions, drawn in the conversation they were asked in:
    // it is holding its turn open on them, and this is the screen the person
    // who caused it is looking at.
    const questions = createMemo(() => waiting().filter((a) => a.sessionId && a.sessionId === openID()))

    const chatWhere = (chat: PlanningChat) => chat.folder ||
        intl.formatMessage({id: 'Planning.board-drafts', defaultMessage: 'the board’s drafts'})

    const onKeyDown = (e: KeyboardEvent) => {
        // Enter sends, Shift+Enter is a new line: this is a conversation, and
        // most of what is typed in it is one line.
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault()
            say()
        }
    }

    return (
        <Dialog
            onClose={props.onClose}
            class='PlanningDialog'
            title={<div>{intl.formatMessage({id: 'Planning.title', defaultMessage: 'Talk it over with an agent'})}</div>}
        >
            <div class='PlanningDialog__body'>
                <Show
                    when={open()}
                    fallback={
                        <>
                            <p class='PlanningDialog__hint'>
                                {intl.formatMessage({
                                    id: 'Planning.hint',
                                    defaultMessage: 'The agent reads the folder you choose and changes nothing in it. When you have agreed what to do, it creates the cards on this board itself.',
                                })}
                            </p>

                            {/* The conversations already open, first: continuing
                                one is shorter than starting another, and two
                                conversations about one thing is what a list
                                nobody reads leads to. */}
                            <Show when={chats().length > 0}>
                                <section class='PlanningDialog__section'>
                                    <h4 class='PlanningDialog__sectionTitle'>
                                        {intl.formatMessage({id: 'Planning.open-conversations', defaultMessage: 'Open conversations'})}
                                    </h4>
                                    <ul class='PlanningDialog__chats'>
                                        <For each={chats()}>
                                            {(chat) => (
                                                <li class='PlanningDialog__chat'>
                                                    <button
                                                        type='button'
                                                        class='PlanningDialog__chatName'
                                                        onClick={() => setOpenID(chat.id)}
                                                    >
                                                        {`${chat.agent} · ${chatWhere(chat)}`}
                                                    </button>
                                                    <Show when={chat.error}>
                                                        <span class='PlanningDialog__error'>{chat.error}</span>
                                                    </Show>
                                                    <Show
                                                        when={ending() === chat.id}
                                                        fallback={
                                                            <button
                                                                type='button'
                                                                class='PlanningDialog__iconButton'
                                                                title={intl.formatMessage({id: 'Planning.end', defaultMessage: 'End the conversation'})}
                                                                aria-label={intl.formatMessage({id: 'Planning.end', defaultMessage: 'End the conversation'})}
                                                                onClick={() => setEnding(chat.id)}
                                                            >
                                                                <CompassIcon icon='close'/>
                                                            </button>
                                                        }
                                                    >
                                                        <span class='PlanningDialog__confirm'>
                                                            {intl.formatMessage({id: 'Planning.end-ask', defaultMessage: 'End this conversation?'})}
                                                            <button
                                                                type='button'
                                                                class='PlanningDialog__confirmYes'
                                                                onClick={() => end(chat.id)}
                                                            >
                                                                {intl.formatMessage({id: 'Planning.end-yes', defaultMessage: 'End'})}
                                                            </button>
                                                            <button
                                                                type='button'
                                                                class='PlanningDialog__confirmNo'
                                                                onClick={() => setEnding('')}
                                                            >
                                                                {intl.formatMessage({id: 'Planning.end-no', defaultMessage: 'Cancel'})}
                                                            </button>
                                                        </span>
                                                    </Show>
                                                </li>
                                            )}
                                        </For>
                                    </ul>
                                </section>
                            </Show>

                            <section class='PlanningDialog__section'>
                                <h4 class='PlanningDialog__sectionTitle'>
                                    {intl.formatMessage({id: 'Planning.new-conversation', defaultMessage: 'A new conversation'})}
                                </h4>

                                <Show
                                    when={askingAgent()}
                                    fallback={
                                        <>
                                            {/* A server has no folders (docs/modes.md):
                                                the conversation is held in the
                                                board's drafts without asking. */}
                                            <div class='PlanningDialog__ask'>
                                                {isHosted() ? intl.formatMessage({id: 'Planning.ask-start', defaultMessage: 'Start the conversation'}) : intl.formatMessage({id: 'Planning.ask-folder', defaultMessage: 'Which folder will the agent read?'})}
                                            </div>
                                            <div class='PlanningDialog__picker'>
                                                {/* The answered question, kept in
                                                    sight: the name is the way back
                                                    to it. */}
                                                <Show when={agents().length > 1}>
                                                    <button
                                                        type='button'
                                                        class='PlanningDialog__pickBack'
                                                        title={intl.formatMessage({id: 'Planning.change-agent', defaultMessage: 'Change the agent'})}
                                                        onClick={() => setAgentName('')}
                                                    >
                                                        <CompassIcon icon='account-outline'/>
                                                        {agentName()}
                                                    </button>
                                                </Show>

                                                <Show
                                                    when={!isHosted()}
                                                    fallback={
                                                        <button
                                                            type='button'
                                                            class='PlanningDialog__agentChoice'
                                                            disabled={busy()}
                                                            onClick={() => start('')}
                                                        >
                                                            {intl.formatMessage({id: 'Planning.start', defaultMessage: 'Start'})}
                                                        </button>
                                                    }
                                                >
                                                    <FolderChoices
                                                        board={props.board}
                                                        disabled={busy()}
                                                        onPick={start}
                                                    />
                                                </Show>
                                            </div>
                                        </>
                                    }
                                >
                                    <div class='PlanningDialog__ask'>
                                        {intl.formatMessage({id: 'Planning.ask-agent', defaultMessage: 'Choosing an agent'})}
                                    </div>
                                    <div class='PlanningDialog__picker'>
                                        <div class='PlanningDialog__agentChoices'>
                                            <For each={agents()}>
                                                {(a) => (
                                                    <button
                                                        type='button'
                                                        class='PlanningDialog__agentChoice'
                                                        disabled={busy()}
                                                        onClick={() => setAgentName(a.name)}
                                                    >
                                                        {a.name}
                                                    </button>
                                                )}
                                            </For>
                                            {/* An agent is connected in one place,
                                                and the list here follows it. */}
                                            <button
                                                type='button'
                                                class='PlanningDialog__pickAdd'
                                                onClick={() => openSettings('ai')}
                                            >
                                                {intl.formatMessage({id: 'Planning.add-agent', defaultMessage: 'Connect an agent in Settings → AI…'})}
                                            </button>
                                        </div>
                                    </div>
                                </Show>
                            </section>
                        </>
                    }
                >
                    <div class='PlanningDialog__conversation'>
                        <div class='PlanningDialog__conversationHead'>
                            <button
                                type='button'
                                class='PlanningDialog__pickBack'
                                onClick={() => setOpenID('')}
                            >
                                <CompassIcon icon='chevron-left'/>
                                {intl.formatMessage({id: 'Planning.all-conversations', defaultMessage: 'All conversations'})}
                            </button>
                            <span class='PlanningDialog__meta'>
                                {`${open()!.agent} · ${chatWhere(open()!)}`}
                            </span>
                        </div>

                        <div class='PlanningDialog__transcript'>
                            <For each={open()!.messages}>
                                {(message) => (
                                    <div class={`PlanningDialog__message PlanningDialog__message--${message.role}`}>
                                        {message.text}
                                    </div>
                                )}
                            </For>
                            <Show when={streaming()}>
                                <div class='PlanningDialog__message PlanningDialog__message--agent'>
                                    {streaming()}
                                </div>
                            </Show>
                            <Show when={open()!.busy && !streaming()}>
                                <div class='PlanningDialog__thinking'>
                                    {intl.formatMessage({id: 'Planning.thinking', defaultMessage: 'The agent is reading…'})}
                                </div>
                            </Show>
                        </div>

                        <For each={questions()}>
                            {(target) => (
                                <div class='PlanningDialog__question'>
                                    <span>{target.text}</span>
                                    <AttentionAnswers target={target}/>
                                </div>
                            )}
                        </For>

                        <Show when={open()!.error}>
                            <div class='PlanningDialog__error'>{open()!.error}</div>
                        </Show>

                        <div class='PlanningDialog__compose'>
                            <textarea
                                class='PlanningDialog__input'
                                rows={3}
                                value={draft()}
                                placeholder={intl.formatMessage({id: 'Planning.placeholder', defaultMessage: 'What has to be done?'})}
                                onInput={(e) => setDraft(e.currentTarget.value)}
                                onKeyDown={onKeyDown}
                            />
                            <Show
                                when={open()!.busy}
                                fallback={
                                    <button
                                        type='button'
                                        class='PlanningDialog__send'
                                        disabled={!draft().trim()}
                                        onClick={say}
                                    >
                                        {intl.formatMessage({id: 'Planning.send', defaultMessage: 'Send'})}
                                    </button>
                                }
                            >
                                <button
                                    type='button'
                                    class='PlanningDialog__send'
                                    onClick={stop}
                                >
                                    {intl.formatMessage({id: 'Planning.stop', defaultMessage: 'Stop'})}
                                </button>
                            </Show>
                        </div>
                    </div>
                </Show>

                <Show when={error()}>
                    <div class='PlanningDialog__error'>{error()}</div>
                </Show>
            </div>
        </Dialog>
    )
}

export default PlanningDialog
