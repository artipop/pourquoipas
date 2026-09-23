// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {For, Show, createEffect, createMemo, createSignal, onSettled} from 'solid-js'

import {useIntl} from '../../intl'

import CompassIcon from '../../widgets/icons/compassIcon'

import {createLoader} from '../../hooks/loader'

import {goApp} from '../../goBridge'
import {aiEnabled} from '../../aiSwitch'

import {onAgentEvent} from './agentEvents'

import './cardDiff.scss'

// A card's work read by eye. A route can park a card on a stage where nothing
// runs and a person decides — «На ревью» is exactly that — and until this
// existed there was nowhere to decide *from*: the branch was a label on the
// card and the changes themselves lived in a directory somebody had to go and
// find.
//
// Deliberately a reader and nothing else. There is no staging, no comment on a
// line and no approve button here: how a stage ended is one field on the card
// («Исход»), answered by the two buttons the route strip already draws, and a
// second way to say the same thing would be a second answer to one question.

export type DiffFile = {
    path: string
    oldPath?: string
    added: number
    removed: number
    binary?: boolean
    untracked?: boolean
    patch?: string
    truncated?: boolean
}

export type CardDiffData = {
    cardId: string
    branch?: string
    base?: string
    dir?: string
    mode?: string
    committed: DiffFile[]
    uncommitted: DiffFile[]
    reason?: string
}

export function isCardDiffAvailable(): boolean {
    return aiEnabled() && Boolean(goApp()?.GetCardDiff)
}

// A patch line, classified by the one character git puts in front of it. The
// file header (`diff --git`, `index`, `---`, `+++`) is dropped: the name is the
// row above, and repeating it inside the box is the noise every diff viewer
// starts with.
type PatchLine = {kind: 'hunk' | 'add' | 'del' | 'ctx', text: string}

// The sign in the gutter. A column of its own rather than the first character
// of the line, so a patch can be copied out of the page as code.
const gutterMark = (kind: PatchLine['kind']): string => {
    if (kind === 'add') {
        return '+'
    }
    return kind === 'del' ? '−' : ''
}

export const patchLines = (patch: string): PatchLine[] => {
    const out: PatchLine[] = []
    let started = false
    for (const line of patch.split('\n')) {
        if (line.startsWith('@@')) {
            started = true
            out.push({kind: 'hunk', text: line})
            continue
        }
        if (!started || line === '') {
            // Before the first hunk it is git's own header; an empty string is
            // the trailing newline of the patch, never a line of the file — a
            // blank line inside a hunk still carries its leading space.
            continue
        }
        if (line.startsWith('+')) {
            out.push({kind: 'add', text: line.slice(1)})
        } else if (line.startsWith('-')) {
            out.push({kind: 'del', text: line.slice(1)})
        } else if (line.startsWith('\\')) {
            // "\ No newline at end of file" — git's own note, not a line of code.
            out.push({kind: 'ctx', text: line})
        } else {
            out.push({kind: 'ctx', text: line.slice(1)})
        }
    }
    return out
}

// How many lines make a file one nobody opened the panel to read. A lock file,
// a generated bundle, a vendored blob: it is a real change and belongs in the
// list, but expanded it buries the four lines somebody came for. Folded, it is
// one row with its own counts and a chevron.
//
// A number rather than a guess at the path, because "generated" has no spelling
// anybody can agree on — package-lock.json, go.sum, a migration, a snapshot —
// and the thing that actually makes a file unreadable here is its size.
const foldedFromLines = 200

const FileBlock = (props: {file: DiffFile, foldAll?: number, unfoldAll?: number}) => {
    const intl = useIntl()
    const lines = createMemo(() => patchLines(props.file.patch || ''))
    const [open, setOpen] = createSignal(lines().length <= foldedFromLines)

    // The two buttons in the summary say "all of them", so they have to reach a
    // file whose own chevron somebody has already clicked. They are counters
    // rather than a shared open/closed state, because a person folding one file
    // afterwards must not be undone by a signal that still says "all open".
    createEffect(() => props.foldAll, (fold) => {
        if (fold) {
            setOpen(false)
        }
    })
    createEffect(() => props.unfoldAll, (unfold) => {
        if (unfold) {
            setOpen(true)
        }
    })

    // A file with nothing to unfold — a new file git is not tracking yet, a
    // binary one — is a row and not a control: a chevron that opens nothing is
    // a promise the row cannot keep.
    const foldable = () => lines().length > 0

    return (
        <div class='CardDiff__file'>
            <button
                type='button'
                class='CardDiff__fileHead'
                disabled={!foldable()}
                onClick={() => setOpen(!open())}
            >
                <Show when={foldable()}>
                    <CompassIcon icon={open() ? 'chevron-down' : 'chevron-right'}/>
                </Show>
                <span class='CardDiff__path'>
                    <Show when={props.file.oldPath}>
                        <span class='CardDiff__oldPath'>{props.file.oldPath + ' → '}</span>
                    </Show>
                    {props.file.path}
                </span>
                <Show when={props.file.untracked}>
                    <span class='CardDiff__tag'>
                        {intl.formatMessage({id: 'CardDiff.untracked', defaultMessage: 'new file'})}
                    </span>
                </Show>
                <Show when={props.file.binary}>
                    <span class='CardDiff__tag'>
                        {intl.formatMessage({id: 'CardDiff.binary', defaultMessage: 'binary'})}
                    </span>
                </Show>
                <Show when={props.file.added > 0}>
                    <span class='CardDiff__added'>{'+' + props.file.added}</span>
                </Show>
                <Show when={props.file.removed > 0}>
                    <span class='CardDiff__removed'>{'−' + props.file.removed}</span>
                </Show>
            </button>
            <Show when={open() && lines().length > 0}>
                <div class='CardDiff__patch'>
                    <For each={lines()}>
                        {(line) => (
                            <div class={'CardDiff__line CardDiff__line--' + line.kind}>
                                <span class='CardDiff__gutter'>{gutterMark(line.kind)}</span>
                                <span class='CardDiff__text'>{line.text}</span>
                            </div>
                        )}
                    </For>
                    <Show when={props.file.truncated}>
                        <div class='CardDiff__cut'>
                            {intl.formatMessage({id: 'CardDiff.truncated', defaultMessage: 'The rest is too large to show — read it in the folder.'})}
                        </div>
                    </Show>
                </div>
            </Show>
        </div>
    )
}

// CardDiffView is the reading itself, shared by the panel beside the card and
// the window: one component, because the two are the same screen at two sizes
// and keeping two of them is how they drift.
export const CardDiffView = (props: {cardId: string}) => {
    const intl = useIntl()
    const [reading, reread] = createLoader(async (cardId: string): Promise<{diff?: CardDiffData, error: string}> => {
        const bindings = goApp()
        if (!bindings?.GetCardDiff) {
            return {error: ''}
        }
        try {
            return {diff: JSON.parse(await bindings.GetCardDiff(cardId)) as CardDiffData, error: ''}
        } catch (e) {
            // The diff already on screen stays under the error: a folder that
            // failed to read once has not stopped holding what it held.
            return {diff: reading()?.diff, error: String((e as Error)?.message || e)}
        }
    }, undefined as {diff?: CardDiffData, error: string} | undefined, () => props.cardId)
    const diff = () => reading()?.diff
    const error = () => reading()?.error ?? ''

    onSettled(() => {
        // The work moves while this is open — the agent commits, a stage ends —
        // and a diff that answers for the moment it was opened is a diff that
        // quietly lies. A session event is the cheapest thing that means "the
        // work changed"; the button beside it is for everything else.
        return onAgentEvent('acp:session', () => reread())
    })

    const total = createMemo(() => {
        const d = diff()
        if (!d) {
            return {files: 0, added: 0, removed: 0}
        }
        const all = [...d.committed, ...d.uncommitted]
        return {
            files: all.length,
            added: all.reduce((sum, f) => sum + f.added, 0),
            removed: all.reduce((sum, f) => sum + f.removed, 0),
        }
    })

    // Counters rather than one open/closed flag: see FileBlock. Bumping one is
    // "do this to all of them now", and what somebody does to a single file
    // afterwards stands.
    const [foldAll, setFoldAll] = createSignal(0)
    const [unfoldAll, setUnfoldAll] = createSignal(0)

    return (
        <Show
            when={reading()}
            fallback={
                <div class='CardDiff'>
                    <div class='CardDiff__empty'>
                        {intl.formatMessage({id: 'CardDiff.loading', defaultMessage: 'Reading the folder…'})}
                    </div>
                </div>
            }
        >
            <div class='CardDiff'>
                <div class='CardDiff__summary'>
                    <Show when={diff()?.branch}>
                        <span class='CardDiff__branch'>
                            <CompassIcon icon='source-branch'/>
                            {diff()!.branch}
                        </span>
                    </Show>
                    <Show when={diff()?.base}>
                        <span class='CardDiff__base'>
                            {intl.formatMessage({id: 'CardDiff.base', defaultMessage: 'against {base}'}, {base: diff()!.base})}
                        </span>
                    </Show>
                    <Show when={total().files > 0}>
                        <span class='CardDiff__counts'>
                            {intl.formatMessage(
                                {id: 'CardDiff.counts', defaultMessage: '{files, plural, one {# file} other {# files}}'},
                                {files: total().files},
                            )}
                            <span class='CardDiff__added'>{'+' + total().added}</span>
                            <span class='CardDiff__removed'>{'−' + total().removed}</span>
                        </span>
                    </Show>
                    <Show when={total().files > 1}>
                        <button
                            type='button'
                            class='CardDiff__button'
                            title={intl.formatMessage({id: 'CardDiff.fold-all', defaultMessage: 'Collapse every file'})}
                            aria-label={intl.formatMessage({id: 'CardDiff.fold-all', defaultMessage: 'Collapse every file'})}
                            onClick={() => setFoldAll(foldAll() + 1)}
                        >
                            <CompassIcon icon='arrow-collapse'/>
                        </button>
                        <button
                            type='button'
                            class='CardDiff__button'
                            title={intl.formatMessage({id: 'CardDiff.unfold-all', defaultMessage: 'Expand every file'})}
                            aria-label={intl.formatMessage({id: 'CardDiff.unfold-all', defaultMessage: 'Expand every file'})}
                            onClick={() => setUnfoldAll(unfoldAll() + 1)}
                        >
                            <CompassIcon icon='arrow-expand'/>
                        </button>
                    </Show>
                    <button
                        type='button'
                        class='CardDiff__button'
                        title={intl.formatMessage({id: 'CardDiff.refresh', defaultMessage: 'Read the folder again'})}
                        aria-label={intl.formatMessage({id: 'CardDiff.refresh', defaultMessage: 'Read the folder again'})}
                        onClick={() => reread()}
                    >
                        <CompassIcon icon='refresh'/>
                    </button>
                </div>

                <Show when={error()}>
                    <div class='CardDiff__error'>{error()}</div>
                </Show>

                {/* "Nothing to show" is an ordinary state of a card and says which
                    one it is — not started, started elsewhere, folder gone. It is
                    drawn as a note and never as an error. */}
                <Show when={!error() && diff()?.reason}>
                    <div class='CardDiff__empty'>{diff()!.reason}</div>
                </Show>

                <Show when={(diff()?.committed.length || 0) > 0}>
                    <div class='CardDiff__section'>
                        {intl.formatMessage({id: 'CardDiff.committed', defaultMessage: 'On the branch'})}
                    </div>
                    <For each={diff()!.committed}>
                        {(file) => (
                            <FileBlock
                                file={file}
                                foldAll={foldAll()}
                                unfoldAll={unfoldAll()}
                            />
                        )}
                    </For>
                </Show>

                {/* Uncommitted work is its own section rather than a mark on a row:
                    a stage working in the person's own folder is *told* to leave
                    its changes like this (composePrompt), so for that card this is
                    the whole of the work and not a leftover. */}
                <Show when={(diff()?.uncommitted.length || 0) > 0}>
                    <div class='CardDiff__section'>
                        {intl.formatMessage({id: 'CardDiff.uncommitted', defaultMessage: 'Not committed'})}
                    </div>
                    <For each={diff()!.uncommitted}>
                        {(file) => (
                            <FileBlock
                                file={file}
                                foldAll={foldAll()}
                                unfoldAll={unfoldAll()}
                            />
                        )}
                    </For>
                </Show>
            </div>
        </Show>
    )
}

type Props = {
    cardId: string
    onClose: () => void
}

// The panel beside the card: the same reading with a head over it, and a window
// to move it to — which is what a diff mostly wants, the panel being the
// narrowest place in the app to read code in.
const CardDiffPanel = (props: Props) => {
    const intl = useIntl()

    const inWindow = async () => {
        const bindings = goApp()
        if (!bindings?.OpenCardDiff) {
            return
        }
        await bindings.OpenCardDiff(props.cardId)
    }

    return (
        <div class='CardDiffPanel'>
            <div class='CardDiffPanel__head'>
                {/* The same glyph the toolbar button is drawn with, so the
                    panel and the button that opened it are one thing. A word on
                    its own says «Изменения» about a card that has three other
                    panels; the icon says which. */}
                <CompassIcon icon='file-patch-outline'/>
                <span class='CardDiffPanel__name'>
                    {intl.formatMessage({id: 'CardDiff.title', defaultMessage: 'Changes'})}
                </span>
                <Show when={Boolean(goApp()?.OpenCardDiff)}>
                    <button
                        type='button'
                        class='CardDiff__button'
                        title={intl.formatMessage({id: 'CardDiff.window', defaultMessage: 'Open in a separate window'})}
                        aria-label={intl.formatMessage({id: 'CardDiff.window', defaultMessage: 'Open in a separate window'})}
                        onClick={inWindow}
                    >
                        <CompassIcon icon='open-in-new'/>
                    </button>
                </Show>
                <button
                    type='button'
                    class='CardDiff__button'
                    title={intl.formatMessage({id: 'CardDiff.collapse', defaultMessage: 'Put the changes away'})}
                    aria-label={intl.formatMessage({id: 'CardDiff.collapse', defaultMessage: 'Put the changes away'})}
                    onClick={props.onClose}
                >
                    <CompassIcon icon='close'/>
                </button>
            </div>
            <div class='CardDiffPanel__body'>
                <CardDiffView cardId={props.cardId}/>
            </div>
        </div>
    )
}

export default CardDiffPanel
