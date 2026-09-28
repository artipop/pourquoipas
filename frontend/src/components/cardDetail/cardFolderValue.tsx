import {For, Show, createSignal} from 'solid-js'

import {useIntl} from '../../intl'

import {Board, IPropertyOption, IPropertyTemplate} from '../../blocks/board'
import {Card} from '../../blocks/card'
import mutator from '../../mutator'
import {goError} from '../../goMessages'
import Label from '../../widgets/label'
import Menu from '../../widgets/menu'
import MenuWrapper from '../../widgets/menuWrapper'
import {sendFlashMessage} from '../flashMessages'

import {canAddFolder, pickFolderForBoard} from '../acp/cardFolder'
import {fieldNames} from '../acp/workdirSync'

type Props = {
    board: Board
    card: Card

    // The board's folder field; absent on a board that has not got one yet,
    // which the first folder added from here gives it.
    property?: IPropertyTemplate
    readOnly: boolean
}

// The card's folder, chosen from the folders the board already offers or
// added from here. It replaces the ordinary select on this one row because
// typing a new option is not a folder: an option with no registry entry behind
// it is a folder the card names and no agent can find.
const CardFolderValue = (props: Props) => {
    const intl = useIntl()
    const [busy, setBusy] = createSignal(false)

    const options = (): IPropertyOption[] => props.property?.options || []
    const chosen = () => {
        const id = props.property ? props.card.fields.properties[props.property.id] : undefined
        return options().find((o) => o.id === id)
    }

    const choose = (optionId: string) => {
        if (props.property) {
            mutator.changePropertyValue(props.board.id, props.card, props.property.id, optionId)
        }
    }

    const add = async () => {
        setBusy(true)
        try {
            const added = await pickFolderForBoard(
                props.board,
                intl.formatMessage({id: 'CardFolder.pick-title', defaultMessage: 'Choose a folder for the card'}),
                fieldNames(intl),
            )
            if (added) {
                await mutator.changePropertyValue(props.board.id, props.card, added.propertyId, added.optionId)
            }
        } catch (e) {
            sendFlashMessage({content: goError(intl, e), severity: 'high'})
        } finally {
            setBusy(false)
        }
    }

    const label = () => chosen()?.value || intl.formatMessage({id: 'CardFolder.none', defaultMessage: 'Not chosen'})

    return (
        <Show
            when={!props.readOnly}
            fallback={
                <div class='octo-propertyvalue octo-propertyvalue--readonly'>
                    <Label color={chosen() ? (chosen()!.color || '') : 'empty'}>
                        <span class='Label-text'>{label()}</span>
                    </Label>
                </div>
            }
        >
            <MenuWrapper
                disabled={busy()}
                label={intl.formatMessage({id: 'CardFolder.menu', defaultMessage: 'Card folder'})}
                menu={
                    <Menu>
                        <For each={options()}>
                            {(option) => (
                                <Menu.Text
                                    id={option.id}
                                    name={option.value}
                                    check={option.id === chosen()?.id}
                                    onClick={() => choose(option.id)}
                                />
                            )}
                        </For>
                        <Show when={options().length > 0 && canAddFolder()}>
                            <Menu.Separator/>
                        </Show>
                        <Show when={canAddFolder()}>
                            <Menu.Text
                                id='add-folder'
                                name={intl.formatMessage({id: 'CardFolder.add', defaultMessage: 'Add a folder…'})}
                                onClick={add}
                            />
                        </Show>
                        <Show when={chosen()}>
                            <Menu.Text
                                id='clear-folder'
                                name={intl.formatMessage({id: 'CardFolder.clear', defaultMessage: 'No folder'})}
                                onClick={() => choose('')}
                            />
                        </Show>
                    </Menu>
                }
            >
                <div class='octo-propertyvalue'>
                    <Label color={chosen() ? (chosen()!.color || '') : 'empty'}>
                        <span class='Label-text'>{label()}</span>
                    </Label>
                </div>
            </MenuWrapper>
        </Show>
    )
}

export default CardFolderValue
