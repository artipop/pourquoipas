// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Show, createSignal, onSettled} from 'solid-js'

import {useIntl} from '../../intl'

import {UserSettings} from '../../userSettings'
import Button from '../../widgets/buttons/button'
import {sendFlashMessage} from '../flashMessages'

import {goApp} from '../../goBridge'

import {isAgentsAvailable} from './agentsPanel'
import PromptField from './promptField'

import './machineMiscPanel.scss'

// The switch for agent-named branches is parked, not deleted: git is kept out
// of what a person is asked about, and a branch named after the card is good
// enough. Flip this back to offer it again; the Go side still reads the stored
// value (GetAgentNamedBranches).
const OFFER_NAMED_BRANCHES = false

// The planning prompt — what an agent is told when a conversation opens with no
// card — is parked too: it is an instruction to the agent with a working
// default (DefaultPlanningPrompt), not something a person needs to see.
const OFFER_PLANNING_PROMPT = false

// Filling a new card's folder in on a board with one folder is on by default
// and is not offered as a switch: it writes an ordinary value the card shows
// and that can be changed, so there is nothing to decide in advance.
const OFFER_PREFILL_FOLDER = false

// The settings that belong to the install rather than to any board, and that
// have nowhere better to be: what a planning terminal opens saying, and the
// couple of facts a person needs before opening config.json by hand.
//
// The planning prompt lived in the planning dialog, beside the project and the
// agent, back when that dialog was reached from a board's menu. It is a setting
// of this machine and it is edited once, so it belongs with the other settings
// of this machine — and the dialog it left is now what it always should have
// been: a place to open a terminal.

const MachineMiscPanel = () => {
    const intl = useIntl()
    const bindings = goApp()

    const [prompt, setPrompt] = createSignal('')
    const [savedPrompt, setSavedPrompt] = createSignal('')
    const [namedBranches, setNamedBranches] = createSignal(false)
    const [error, setError] = createSignal('')

    // Access for other applications (feedaccess.go). The token is shown once,
    // right after it is issued: only its hash is kept.
    const [feedIssued, setFeedIssued] = createSignal(false)
    const [feedToken, setFeedToken] = createSignal('')

    // The machine's, though only the page reads it: the card is made here.
    const [prefillFolder, setPrefillFolder] = createSignal(UserSettings.prefillCardFolder)

    const togglePrefillFolder = (on: boolean) => {
        setPrefillFolder(on)
        UserSettings.prefillCardFolder = on
    }

    onSettled(() => {
        // Not awaited: onSettled reads a returned promise as a cleanup value.
        void (async () => {
            try {
                if (bindings?.GetPlanningPrompt) {
                    const stored = await bindings.GetPlanningPrompt()
                    setPrompt(stored)
                    setSavedPrompt(stored)
                }
                if (bindings?.GetAgentNamedBranches) {
                    setNamedBranches(await bindings.GetAgentNamedBranches())
                }
                if (bindings?.FeedTokenIssued) {
                    setFeedIssued(await bindings.FeedTokenIssued())
                }
            } catch (e: any) {
                setError(String(e?.message || e))
            }
        })()
    })

    // Saved on the click: it is one switch, and a Save button beside a
    // checkbox is a step for nothing.
    const toggleNamedBranches = async (on: boolean) => {
        setNamedBranches(on)
        try {
            await bindings?.SetAgentNamedBranches?.(on)
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    const issueFeedToken = async () => {
        try {
            setFeedToken(await bindings!.IssueFeedToken())
            setFeedIssued(true)
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    const revokeFeedToken = async () => {
        try {
            await bindings!.RevokeFeedToken()
            setFeedToken('')
            setFeedIssued(false)
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    const savePrompt = async () => {
        if (!bindings?.SetPlanningPrompt) {
            return
        }
        setError('')
        try {
            await bindings.SetPlanningPrompt(prompt())
            setSavedPrompt(prompt())
            sendFlashMessage({
                content: intl.formatMessage({id: 'Machine.planning-prompt-saved', defaultMessage: 'Saved'}),
                severity: 'normal',
            })
        } catch (e: any) {
            setError(String(e?.message || e))
        }
    }

    return (
        <div class='MachineMiscPanel'>
            <div class='MachineMiscPanel__subtitle'>
                {intl.formatMessage({id: 'Machine.subtitle', defaultMessage: 'Settings of this install: they apply to every board on it.'})}
            </div>
            <div class='MachineMiscPanel__content'>
                <Show when={OFFER_PLANNING_PROMPT && Boolean(bindings?.GetPlanningPrompt)}>
                    <PromptField
                        label={intl.formatMessage({
                            id: 'Machine.planning-prompt',
                            defaultMessage: 'What an agent is told when a conversation is opened without a card (the board\'s own instructions and the agent\'s come before it, the project after)',
                        })}
                        value={prompt()}

                        // Ten rows: the default instructions are eight lines,
                        // and a box that cuts off its own default reads as a
                        // bug rather than as a setting.
                        rows={10}
                        onInput={setPrompt}
                    >
                        <Show when={prompt() !== savedPrompt()}>
                            <Button onClick={savePrompt}>
                                {intl.formatMessage({id: 'Machine.save-planning-prompt', defaultMessage: 'Save the instructions'})}
                            </Button>
                        </Show>
                    </PromptField>
                </Show>

                {/* Named by the agent, spelled by the machine otherwise: the
                    switch is here because it is about how this machine spends
                    agent runs, not about any board. */}
                <Show when={OFFER_NAMED_BRANCHES}>
                    <label class='MachineMiscPanel__toggle'>
                        <input
                            type='checkbox'
                            checked={namedBranches()}
                            onChange={(e) => toggleNamedBranches(e.currentTarget.checked)}
                        />
                        {intl.formatMessage({id: 'Machine.named-branches', defaultMessage: 'The agent names each card’s branch'})}
                    </label>
                    <p class='MachineMiscPanel__hint'>
                        {intl.formatMessage({id: 'Machine.named-branches-hint', defaultMessage: 'A short agent run before the card’s first branch; a slow or odd answer falls back to the card’s title.'})}
                    </p>
                </Show>

                {/* A default, not a rule: it writes an ordinary value onto the
                    card, which is visible and can be changed or cleared. Only
                    ever when the board offers exactly one folder — with two
                    there is a question, and nothing here is going to guess it.
                    Offered where the other agent settings are, since a board
                    with no agent has no folder field to fill. */}
                <Show when={OFFER_PREFILL_FOLDER && isAgentsAvailable()}>
                    <label class='MachineMiscPanel__toggle'>
                        <input
                            type='checkbox'
                            checked={prefillFolder()}
                            onChange={(e) => togglePrefillFolder(e.currentTarget.checked)}
                        />
                        {intl.formatMessage({id: 'Machine.prefill-folder', defaultMessage: 'Fill a new card’s folder in when the board has only one'})}
                    </label>
                    <p class='MachineMiscPanel__hint'>
                        {intl.formatMessage({id: 'Machine.prefill-folder-hint', defaultMessage: 'The card gets the folder the moment it is made, and it can be changed or cleared like any other field. A board that offers several folders is never filled in — that is a question only you can answer.'})}
                    </p>
                </Show>

                {/* Another application reading a column of a board as its own
                    inbox — XXVI does. A setting of the install, because the
                    token opens every board on it. */}
                <Show when={Boolean(bindings?.IssueFeedToken)}>
                    <div class='MachineMiscPanel__setting'>
                        <div class='MachineMiscPanel__fact'>
                            <span class='MachineMiscPanel__factName'>
                                {intl.formatMessage({id: 'Machine.feed', defaultMessage: 'Access for other applications'})}
                            </span>
                            <span class='MachineMiscPanel__factValue'>
                                {feedIssued() ? intl.formatMessage({id: 'Machine.feed-on', defaultMessage: 'A token is issued: an application that has it can read the cards of a column. Read only — it cannot move or change them.'}) : intl.formatMessage({id: 'Machine.feed-off', defaultMessage: 'Other applications — XXVI, for one — can take the cards of a column as tasks. They need a token to do it.'})}
                            </span>
                        </div>
                        <Show when={feedIssued()}>
                            <Button onClick={revokeFeedToken}>
                                {intl.formatMessage({id: 'Machine.feed-revoke', defaultMessage: 'Revoke'})}
                            </Button>
                        </Show>
                        <Button onClick={issueFeedToken}>
                            {feedIssued() ? intl.formatMessage({id: 'Machine.feed-reissue', defaultMessage: 'New token'}) : intl.formatMessage({id: 'Machine.feed-issue', defaultMessage: 'Issue a token'})}
                        </Button>
                    </div>
                    <Show when={feedToken()}>
                        <div class='MachineMiscPanel__token'>
                            <span class='MachineMiscPanel__hint'>
                                {intl.formatMessage({id: 'Machine.feed-token', defaultMessage: 'The token, shown once — paste it into the other application now. The previous one no longer works.'})}
                            </span>
                            <code>{feedToken()}</code>
                            <Button onClick={() => navigator.clipboard?.writeText(feedToken())}>
                                {intl.formatMessage({id: 'Sources.copy', defaultMessage: 'Copy'})}
                            </Button>
                        </div>
                    </Show>
                </Show>

                {/* The guide, not docs/flows.md: this line used to name a file
                    of the source tree, which a person reading it off the
                    settings panel has no way to open. */}
                <p class='MachineMiscPanel__hint'>
                    {intl.formatMessage({
                        id: 'Machine.hint-config',
                        defaultMessage: 'How many sessions run at once, how long a turn may take, which tools an agent may use without asking — those are edited by hand in the app\'s config.json. The guide says where the file is and what is in it.',
                    })}
                </p>

                <Show when={error()}>
                    <div class='MachineMiscPanel__error'>{error()}</div>
                </Show>
            </div>
        </div>
    )
}

export default MachineMiscPanel
