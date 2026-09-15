import {createEffect, onSettled} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {EditableProps, useEditable} from './editable'

import './editableArea.scss'

function getBorderWidth(style: CSSStyleDeclaration): number {
    return parseInt(style.borderTopWidth || '0', 10) + parseInt(style.borderBottomWidth || '0', 10)
}

const EditableArea = (props: EditableProps): JSX.Element => {
    let elementRef: HTMLTextAreaElement | undefined
    let referenceRef: HTMLTextAreaElement | undefined
    let height = 0
    const elementProps = useEditable(props, () => elementRef)

    // The hidden reference textarea carries the same value; its scrollHeight is
    // what the visible one grows to.
    const measure = () => {
        if (!elementRef || !referenceRef) {
            return
        }

        const nextHeight = referenceRef.scrollHeight
        const textarea = elementRef

        if (nextHeight > 0 && nextHeight !== height) {
            const style = getComputedStyle(textarea)
            const borderWidth = getBorderWidth(style)

            // Directly change the height to avoid circular rerenders
            textarea.style.height = String(nextHeight + borderWidth) + 'px'

            height = nextHeight
        }
    }

    createEffect(() => props.value, () => {
        measure()
    })

    // How many lines a text takes depends on how wide it is, and the value is
    // not the only thing that changes that: opening the panel beside a card
    // narrows the column under it, so «Поднять зависимости» rewrapped to two
    // lines while the box kept the height of one — the title read as «Поднять»,
    // with the rest clipped and empty space beside it. A window resize is the
    // same bug with a different cause.
    //
    // The reference is what is watched, not the visible box: its width is the
    // column's, and its height is the answer — observing the box whose height
    // this sets would be a loop.
    onSettled(() => {
        if (typeof ResizeObserver === 'undefined' || !referenceRef) {
            return undefined
        }
        const observer = new ResizeObserver(measure)
        observer.observe(referenceRef)
        return () => observer.disconnect()
    })

    return (
        <div class={'EditableAreaWrap'}>
            <textarea
                {...elementProps}
                rows={1}
                ref={elementRef}
                class={'EditableArea ' + elementProps.class}
            />
            <div class={'EditableAreaContainer'}>
                <textarea
                    ref={referenceRef}
                    class={'EditableAreaReference ' + elementProps.class}
                    dir='auto'
                    disabled={true}
                    rows={1}
                    value={elementProps.value}
                    aria-hidden='true'
                />
            </div>
        </div>
    )
}

export default EditableArea
