// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Show, createSignal} from 'solid-js'

import {useIntl} from '../../intl'

import {Board} from '../../blocks/board'
import Button from '../../widgets/buttons/button'
import Dialog from '../dialog'

import {createLoader} from '../../hooks/loader'

import {goApp} from '../../goBridge'
import {aiEnabled} from '../../aiSwitch'

import './boardPromptsDialog.scss'

// What this board's agents are told before anything else — the system prompt of
// the board, on its own screen.
//
// It was a fold at the bottom of the automation dialog, under the route
// canvas, which is the wrong place twice over: what an agent is told is not a
// question about columns and arrows, and a fold under a canvas is somewhere
// nobody scrolls to. The board's ⋯ menu asks each of the board's questions in
// its own item (the folders dialog, the deploy-target dialog), and this is one of them.
//
// There are exactly two prompts and the screen says so. This one is the
// board's; the other is the agent's own, in the agents panel in the settings, and it holds
// on every board this machine has. A third — a text per (board, agent) — was
// built and taken out again: it is the only one of them that belongs to no
// single thing a person can point at, and what a folder wants said is already
// said in the folder, by the AGENTS.md its CLI reads.

export function isBoardPromptsAvailable(): boolean {
    return aiEnabled() && Boolean(goApp()?.GetBoardPrompt)
}

type Props = {
    board: Board
    onClose: () => void
}

const BoardPromptsDialog = (props: Props) => {
    const intl = useIntl()
    const bindings = goApp()

    const [stored] = createLoader(async (): Promise<{text: string, error: string}> => {
        try {
            return {text: bindings?.GetBoardPrompt ? await bindings.GetBoardPrompt(props.board.id) : '', error: ''}
        } catch (e) {
            return {text: '', error: String(e)}
        }
    }, undefined as {text: string, error: string} | undefined)

    // The form waits for the stored text: an empty box that took typing before
    // the answer arrived had what was typed written over by it.
    const [prompt, setPrompt] = createSignal(() => stored()?.text ?? '')
    const [saveError, setSaveError] = createSignal('')
    const error = () => saveError() || stored()?.error || ''
    const [saving, setSaving] = createSignal(false)

    const save = async () => {
        if (!bindings?.SetBoardPrompt) {
            return
        }
        setSaveError('')
        setSaving(true)
        try {
            await bindings.SetBoardPrompt(props.board.id, prompt())
            props.onClose()
        } catch (e) {
            setSaveError(String(e))
        } finally {
            setSaving(false)
        }
    }

    return (
        <Dialog
            class='BoardPromptsDialog'
            title={<span>{intl.formatMessage({id: 'BoardPrompts.title', defaultMessage: 'The board’s system prompt'})}</span>}
            subtitle={<span>{intl.formatMessage({id: 'BoardPrompts.subtitle', defaultMessage: 'Said first to every agent that works on this board — in a session a column starts, and in a conversation somebody opens.'})}</span>}
            onClose={props.onClose}
        >
            <Show when={stored()}>
                <div class='BoardPromptsDialog__content'>
                    <textarea
                        rows={10}
                        value={prompt()}
                        aria-label={intl.formatMessage({id: 'BoardPrompts.title', defaultMessage: 'The board’s system prompt'})}
                        placeholder={intl.formatMessage({id: 'BoardPrompts.placeholder', defaultMessage: 'What this board is about, and how work is done on it'})}
                        onInput={(e) => setPrompt(e.currentTarget.value)}
                    />

                    {/* The order, printed. An agent is given three texts and a
                        person can only reason about that if the screen says which
                        ones and in what order — the whole reason the third prompt
                        this dialog once had is not here. */}
                    <p class='BoardPromptsDialog__hint'>
                        {intl.formatMessage({id: 'BoardPrompts.order', defaultMessage: 'The agent is given this text, then its own prompt from "Settings → Agents" — which holds on every board — and then the card’s task.'})}
                    </p>

                    <div class='BoardPromptsDialog__actions'>
                        <Button
                            emphasis='primary'
                            onClick={save}
                            disabled={saving()}
                        >
                            {intl.formatMessage({id: 'BoardPrompts.save', defaultMessage: 'Save'})}
                        </Button>
                        <Button onClick={props.onClose}>
                            {intl.formatMessage({id: 'BoardPrompts.cancel', defaultMessage: 'Cancel'})}
                        </Button>
                    </div>

                    <Show when={error()}>
                        <div class='BoardPromptsDialog__error'>{error()}</div>
                    </Show>
                </div>
            </Show>
        </Dialog>
    )
}

export default BoardPromptsDialog
