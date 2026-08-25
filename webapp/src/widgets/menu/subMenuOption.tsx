import {Show, createSignal} from 'solid-js'
import type {JSX} from 'solid-js'

import {useIntl} from '../../intl'
import CompassIcon from '../../widgets/icons/compassIcon'

import TextOption from './textOption'

import './subMenuOption.scss'

type SubMenuOptionProps = {
    id: string
    name: string
    position?: 'bottom' | 'top' | 'left' | 'left-bottom' | 'auto'
    icon?: JSX.Element
    children: JSX.Element
    class?: string
}

function SubMenuOption(props: SubMenuOptionProps): JSX.Element {
    const intl = useIntl()
    const [isOpen, setIsOpen] = createSignal(false)

    const openLeftClass = () => (props.position === 'left' || props.position === 'left-bottom' ? ' open-left' : '')

    let ref: HTMLDivElement | undefined

    const [style, setStyle] = createSignal<JSX.CSSProperties>({})

    // A submenu is drawn inside the menu and flush against its option, so it
    // is not placed against the viewport the way the menu itself is — all it
    // asks is which end of the option to hang from, on both axes, and the
    // answer is the side of the screen with more room. Measured when it opens
    // rather than when the option mounts: the option mounts with the menu,
    // and the menu is placed by floating-ui a beat later — a rect read at
    // mount is where the menu was going to be laid out from, not where it is,
    // and the export submenu on the board's own ⋯ menu answered "plenty of
    // room to the right" from a corner it never stood in.
    const open = () => {
        const newStyle: JSX.CSSProperties = {}
        if (props.position === 'auto' && ref) {
            const box = ref.getBoundingClientRect()
            const above = box.top
            const below = window.innerHeight - box.bottom
            newStyle[above > below ? 'bottom' : 'top'] = '0'

            // The list may be longer than the room on the chosen side — the
            // property-type list is — and an absolutely positioned submenu
            // otherwise runs off the screen with no way to scroll to the rest.
            // The stylesheet gives .SubMenu overflow-y for exactly this cap.
            newStyle['max-height'] = `${Math.max(above, below, 160) - 12}px`

            // The same choice sideways: the ⋯ menu of the board's header
            // stands at the right edge of the window, and a submenu that only
            // ever hangs right is a submenu that opens clipped there.
            if (window.innerWidth - box.right < box.left) {
                newStyle.left = 'auto'
                newStyle.right = '100%'
            }
        }

        setStyle(newStyle)
        setIsOpen(true)
    }

    return (
        <div
            id={props.id}
            class={`MenuOption SubMenuOption menu-option${openLeftClass()}${isOpen() ? ' menu-option-active' : ''}${props.class ? ' ' + props.class : ''}`}

            // The submenu is drawn inside this option and flush against it, so
            // the pointer never leaves on its way in: entering opens it, and
            // leaving — for the option below, or off the menu — closes it.
            onMouseEnter={open}
            onMouseLeave={() => setIsOpen(false)}

            // A tap opens it too, because a phone has no pointer to hover with
            // and this menu is on the phone as well. It opens rather than
            // toggles: with a pointer the entering already opened it, and a
            // click that closed what hovering had just opened is what a person
            // does the moment they decide to click the thing they are pointing
            // at. Closing is leaving it, or closing the menu around it.
            onClick={(e: MouseEvent) => {
                e.preventDefault()
                e.stopPropagation()
                open()
            }}
            ref={ref}
        >
            <Show
                when={props.icon}
                fallback={<div class='noicon'/>}
            >
                <div class='menu-option__icon'>{props.icon}</div>
            </Show>
            <div class='menu-name'>{props.name}</div>
            <CompassIcon icon='chevron-right'/>
            <Show when={isOpen()}>
                <div
                    class={'SubMenu Menu noselect ' + (props.position || 'bottom')}
                    style={style()}
                >
                    <div class='menu-contents'>
                        <div class='menu-options'>
                            {props.children}
                        </div>
                        <div class='menu-spacer hideOnWidescreen'/>

                        <div class='menu-options hideOnWidescreen'>
                            <TextOption
                                id='menu-cancel'
                                name={intl.formatMessage({id: 'Menu.cancel', defaultMessage: 'Cancel'})}
                                class='menu-cancel'
                                onClick={() => undefined}
                            />
                        </div>
                    </div>

                </div>
            </Show>
        </div>
    )
}

export default SubMenuOption
