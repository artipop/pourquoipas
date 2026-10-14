import {createSignal, onSettled} from 'solid-js'
import type {JSX} from '@solidjs/web'

type TextInputOptionProps = {
    initialValue: string
    onConfirmValue: (value: string) => void
    onValueChanged: (value: string) => void
}

function TextInputOption(props: TextInputOptionProps): JSX.Element {
    let nameTextbox: HTMLInputElement | undefined
    const [value, setValue] = createSignal(props.initialValue)

    // Enter confirms and closes the menu, and Chrome blurs a focused input as
    // it is removed, so the same value would be confirmed a second time on the
    // way out — a property renamed twice.
    let confirmed: string | undefined
    const confirm = () => {
        if (value() !== confirmed) {
            confirmed = value()
            props.onConfirmValue(value())
        }
    }

    onSettled(() => {
        nameTextbox?.focus()
        nameTextbox?.setSelectionRange(0, value().length)
    })

    return (
        <input
            ref={nameTextbox}
            type='text'
            class='PropertyMenu menu-textbox menu-option'
            onClick={(e) => e.stopPropagation()}
            onInput={(e) => {
                // The React version reported the value captured before this
                // keystroke — one behind — and something may lean on that.
                const previous = value()
                setValue((e.target as HTMLInputElement).value)
                props.onValueChanged(previous)
            }}
            value={value()}
            title={value()}
            onBlur={confirm}
            onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === 'Escape') {
                    confirm()
                    e.stopPropagation()
                    if (e.key === 'Enter') {
                        (e.target as HTMLElement).dispatchEvent(new Event('menuItemClicked'))
                    }
                }
            }}
            spellcheck={true}
        />
    )
}

export default TextInputOption
