import {For, Show} from 'solid-js'
import type {JSX} from 'solid-js'

import Menu from './menu'
import MenuWrapper from './menuWrapper'
import CheckIcon from './icons/check'
import DropdownIcon from './icons/dropdown'

import './select.scss'

// Choosing one of a list, in the app's own hand.
//
// A native <select> is drawn by the platform and not by us: it keeps the
// system's white on the dark board, the system's font instead of the two this
// product sets, and it drops a list no stylesheet can reach. Beside the board's
// own menus — which is where all of these stand — it reads as a control from a
// different program. This is that menu (widgets/menu) behind a control shaped
// like the inputs it sits next to, so a form has one voice throughout.
//
// The options are given rather than nested as children, because every caller
// has a list already and none of them wanted a menu — they wanted a field.

export type SelectOption = {
    value: string
    label: string
    disabled?: boolean

    // A line drawn above this option. What it separates is a caller's own
    // business — the theme list uses it to set the slot for a config you paste
    // apart from the themes that came with the app — so it is a flag on the
    // option and not a shape the list has to be built in.
    separator?: boolean
}

type Props = {
    value: string
    options: SelectOption[]
    onChange: (value: string) => void

    // What the control says when the value matches no option — the empty
    // choice of a field where "nothing" is an answer. An option for it belongs
    // in `options` too, or there is no way back to nothing.
    placeholder?: string

    // The field's name, for anyone who cannot see which label it stands under.
    label?: string

    class?: string
    disabled?: boolean
}

const Select = (props: Props): JSX.Element => {
    const current = () => props.options.find((o) => o.value === props.value)
    const text = () => current()?.label ?? props.placeholder ?? ''

    return (
        <MenuWrapper
            class={`Select${props.class ? ' ' + props.class : ''}`}
            label={props.label}
            disabled={props.disabled}

            // The trigger is a <button>, so anything that acts on a click of its
            // own further up — a row, a card, a menu — would act on opening the
            // menu too. **A <label> must not be one of those ancestors**: a
            // label counts a button as its control and forwards a click on the
            // caption to it, which opened the menu from the caption; that is
            // not fixable from here, and every call site puts the caption in a
            // plain element beside the field instead.
            stopPropagationOnToggle={true}
            menu={
                <Menu
                    position='bottom'
                    matchAnchorWidth={true}
                >
                    <For each={props.options}>
                        {(option) => (
                            <>
                                <Show when={option.separator}>
                                    <Menu.Separator/>
                                </Show>
                                <Menu.Text
                                    id={option.value}
                                    name={option.label}
                                    disabled={option.disabled}
                                    check={true}
                                    icon={option.value === props.value ? <CheckIcon/> : <div class='empty-icon'/>}
                                    onClick={() => {
                                        if (!option.disabled) {
                                            props.onChange(option.value)
                                        }
                                    }}
                                />
                            </>
                        )}
                    </For>
                </Menu>
            }
        >
            {/* The field's name is on the wrapper, which is the thing that
                opens the menu; the button inside is named by what it shows.
                Stated rather than left to the text, because these stand inside
                a <label> and a button in a label takes the label's name — both
                halves would then answer to "Kind" and neither could be asked
                for on its own. */}
            <button
                type='button'
                class='Select__trigger'
                disabled={props.disabled}
                aria-label={text()}
            >
                <Show
                    when={current()}
                    fallback={<span class='Select__value Select__value--empty'>{text()}</span>}
                >
                    <span class='Select__value'>{text()}</span>
                </Show>
                <DropdownIcon/>
            </button>
        </MenuWrapper>
    )
}

export default Select
