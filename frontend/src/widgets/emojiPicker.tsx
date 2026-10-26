import {onSettled} from 'solid-js'
import type {Component} from 'solid-js'
import {Picker} from 'emoji-mart'

import {useIntl} from '../intl'

import {loadEmojiLocale} from './emojiLocale'

import './emojiPicker.scss'

type Props = {
    onSelect: (emoji: string) => void
}

type PickedEmoji = {
    native?: string
}

// emoji-mart 5 is not a framework component: Picker is a custom element that
// mounts itself into a host node, so the wrapper stays this small. The data set
// it needs is fetched on the way in (emojiData.ts) rather than at startup.
const EmojiPicker: Component<Props> = (props) => {
    const intl = useIntl()
    let host: HTMLDivElement | undefined

    onSettled(() => {
        const node = host
        if (!node) {
            return undefined
        }

        // v5 draws native glyphs instead of the sprite sheet v3 needed, so the
        // 3.7 MB emoji_spirit.png is no longer shipped. It also renders into a
        // shadow root, which is why the .emoji-mart-* overrides that used to sit
        // in emojiPicker.scss cannot reach it any more.
        const locale = intl.locale
        const picker = new Picker({
            parent: node,
            previewPosition: 'none',
            i18n: () => loadEmojiLocale(locale),
            onEmojiSelect: (emoji: PickedEmoji) => {
                if (emoji.native) {
                    // props.onSelect is read at pick time, so a new handler
                    // never remounts the picker and never loses its state.
                    props.onSelect(emoji.native)
                }
            },
        }) as unknown as {remove?: () => void}

        return () => {
            picker.remove?.()
            node.replaceChildren()
        }
    })

    return (
        <div
            class='EmojiPicker'
            onClick={(e) => e.stopPropagation()}
        >
            <div ref={host}/>
        </div>
    )
}

export default EmojiPicker
