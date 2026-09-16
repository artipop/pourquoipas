import {Loading, Show, lazy} from 'solid-js'
import type {Component} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {useIntl} from '../intl'

import RandomIcon from '../widgets/icons/random'
import DeleteIcon from '../widgets/icons/delete'
import EmojiIcon from '../widgets/icons/emoji'
import Menu from '../widgets/menu'
import MenuWrapper from '../widgets/menuWrapper'
import './iconSelector.scss'

// emoji-mart and its data set arrive with the picker, which opens from a
// submenu and not before.
const EmojiPicker = lazy(() => import('../widgets/emojiPicker'))

type Props = {
    readonly?: boolean
    iconElement: JSX.Element
    onAddRandomIcon: () => void
    onSelectEmoji: (emoji: string) => void
    onRemoveIcon: () => void
}

const IconSelector: Component<Props> = (props) => {
    const intl = useIntl()

    return (
        <div class='IconSelector'>
            <Show
                when={!props.readonly}
                fallback={props.iconElement}
            >
                <MenuWrapper
                    menu={
                        <Menu>
                            <Menu.Text
                                id='random'
                                icon={<RandomIcon/>}
                                name={intl.formatMessage({id: 'ViewTitle.random-icon', defaultMessage: 'Random'})}
                                onClick={props.onAddRandomIcon}
                            />
                            <Menu.SubMenu
                                id='pick'
                                icon={<EmojiIcon/>}
                                name={intl.formatMessage({id: 'ViewTitle.pick-icon', defaultMessage: 'Pick icon'})}
                            >
                                <Loading fallback={null}>
                                    <EmojiPicker onSelect={props.onSelectEmoji}/>
                                </Loading>
                            </Menu.SubMenu>
                            <Menu.Text
                                id='remove'
                                icon={<DeleteIcon/>}
                                name={intl.formatMessage({id: 'ViewTitle.remove-icon', defaultMessage: 'Remove icon'})}
                                onClick={props.onRemoveIcon}
                            />
                        </Menu>
                    }
                >
                    {props.iconElement}
                </MenuWrapper>
            </Show>
        </div>
    )
}

export default IconSelector
