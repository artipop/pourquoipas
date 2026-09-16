import {createEffect, createSignal} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {SearchIndex} from 'emoji-mart'

import {$createTextNode, $getSelection, $isRangeSelection, LexicalEditor, TextNode} from 'lexical'

import {ensureEmojiData} from '../../../widgets/emojiData'

import {TypeaheadMenu, basicTypeaheadTriggerMatch} from './typeahead'

type EmojiResult = {
    id: string
    name: string
    colons: string
    native: string
}

const MAX_EMOJI_SUGGESTIONS = 10

// `:`-triggered emoji autocomplete backed by emoji-mart's search index. Inserts
// the picked emoji's native glyph, replacing the `:query` trigger text.
type Props = {
    editor: LexicalEditor
}

const EmojiPlugin = (props: Props): JSX.Element => {
    const [query, setQuery] = createSignal<string | null>(null)
    const [options, setOptions] = createSignal<EmojiResult[]>([])

    const triggerFn = basicTypeaheadTriggerMatch(':', {minLength: 1})

    // emoji-mart 5 searches asynchronously and puts the glyph on the emoji's
    // first skin rather than on the emoji itself.
    createEffect(query, (q) => {
        if (!q) {
            setOptions([])
            return undefined
        }

        // The cleanup is what drops a search the next keystroke has overtaken:
        // Solid 2 runs it before the next apply, so `cancelled` is read by
        // exactly the run that was superseded.
        let cancelled = false

        // The data set is no longer registered at startup, so the first `:`
        // waits for it; every search after that is the same call it was.
        ensureEmojiData().
            then(() => SearchIndex.search(q)).
            then((results: Array<{id: string, name: string, skins?: Array<{native?: string}>}>) => {
                if (cancelled) {
                    return
                }
                const found = (results || []).
                    slice(0, MAX_EMOJI_SUGGESTIONS).
                    map((e): EmojiResult => ({id: e.id, name: e.name, colons: `:${e.id}:`, native: e.skins?.[0]?.native || ''})).
                    filter((e) => Boolean(e.native))
                setOptions(found)
            })
        return () => {
            cancelled = true
        }
    })

    const onSelectOption = (
        selected: EmojiResult,
        nodeToReplace: TextNode | null,
        closeMenu: () => void,
    ) => {
        // Already inside editor.update() — the menu runs the selection there.
        const emojiText = `${selected.native} `
        const newNode = $createTextNode(emojiText)
        if (nodeToReplace && nodeToReplace.isAttached()) {
            nodeToReplace.replace(newNode)
            newNode.selectNext(0, 0)
        } else {
            const selection = $getSelection()
            if ($isRangeSelection(selection)) {
                selection.insertText(emojiText)
            }
        }
        closeMenu()
    }

    return (
        <TypeaheadMenu<EmojiResult>
            editor={props.editor}
            options={options()}
            triggerFn={triggerFn}
            class='MarkdownEditorInput--emojis'
            onQueryChange={setQuery}
            onSelectOption={onSelectOption}
            itemRender={(emoji, isSelected, select, highlight) => (
                <div
                    role='option'
                    aria-selected={String(isSelected())}
                    class={`EmojiEntry ${isSelected() ? 'EmojiEntry--selected' : ''}`}
                    onMouseDown={(e) => e.preventDefault()}
                    onMouseEnter={highlight}
                    onClick={select}
                >
                    <span class='EmojiEntry__native'>{emoji.native}</span>
                    <span class='EmojiEntry__colons'>{emoji.colons}</span>
                </div>
            )}
        />
    )
}

export default EmojiPlugin
