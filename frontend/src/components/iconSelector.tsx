import {Show, lazy} from 'solid-js'
import type {Component} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {useIntl} from '../intl'
import {ensureEmojiData} from '../widgets/emojiData'
import {loadEmojiLocale} from '../widgets/emojiLocale'

import RandomIcon from '../widgets/icons/random'
import DeleteIcon from '../widgets/icons/delete'
import EmojiIcon from '../widgets/icons/emoji'
import Menu from '../widgets/menu'
import MenuWrapper from '../widgets/menuWrapper'
import './iconSelector.scss'

import {AsyncContent} from './asyncContent'

type Props = {
    readonly?: boolean
    iconElement: JSX.Element
    onAddRandomIcon: () => void
    onSelectEmoji: (emoji: string) => void
    onRemoveIcon: () => void
}

const IconSelector: Component<Props> = (props) => {
    const intl = useIntl()

    // All three start on first use, rather than fetching data only after the
    // wrapper mounts. The boundary owns the whole wait and any failed import.
    const EmojiPicker = lazy(async () => {
        const [module] = await Promise.all([
            import('../widgets/emojiPicker'),
            ensureEmojiData(),
            loadEmojiLocale(intl.locale),
        ])
        return module
    })

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
                                <AsyncContent>
                                    <EmojiPicker onSelect={props.onSelectEmoji}/>
                                </AsyncContent>
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
