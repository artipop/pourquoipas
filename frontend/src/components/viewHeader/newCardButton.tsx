import {For, Show} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {FormattedMessage, useIntl} from '../../intl'

import {Board} from '../../blocks/board'
import ButtonWithMenu from '../../widgets/buttons/buttonWithMenu'
import AddIcon from '../../widgets/icons/add'
import Menu from '../../widgets/menu'
import {useAppSelector} from '../../store/hooks'
import {getCurrentBoardTemplates} from '../../store/cards'
import {getCurrentView} from '../../store/views'
import {ExtensionSlot} from '../../extensions'

import NewCardButtonTemplateItem from './newCardButtonTemplateItem'
import EmptyCardButton from './emptyCardButton'

type Props = {

    // The board a conversation with an agent may leave cards on. It is the only
    // reason this button knows about a board at all.
    board: Board
    addCard: () => void
    addCardFromTemplate: (cardTemplateId: string) => void
    addCardTemplate: () => void
    editCardTemplate: (cardTemplateId: string) => void
}

const NewCardButton = (props: Props): JSX.Element => {
    const cardTemplates = useAppSelector(getCurrentBoardTemplates)
    const currentView = useAppSelector(getCurrentView)
    const intl = useIntl()

    const defaultTemplateID = () => {
        const id = currentView().fields.defaultTemplateId
        return cardTemplates().some((t) => t.id === id) ? id : ''
    }

    const button = (
        <ButtonWithMenu
            onClick={() => {
                if (defaultTemplateID()) {
                    props.addCardFromTemplate(defaultTemplateID())
                } else {
                    props.addCard()
                }
            }}
            text={(
                <FormattedMessage
                    id='ViewHeader.new'
                    defaultMessage='New'
                />
            )}
        >
            <Menu position='left'>
                <Show when={cardTemplates().length > 0}>
                    <Menu.Label>
                        <b>
                            <FormattedMessage
                                id='ViewHeader.select-a-template'
                                defaultMessage='Select a template'
                            />
                        </b>
                    </Menu.Label>

                    <Menu.Separator/>
                </Show>

                <For each={cardTemplates()}>
                    {(cardTemplate) => (
                        <NewCardButtonTemplateItem
                            cardTemplate={cardTemplate}
                            addCardFromTemplate={props.addCardFromTemplate}
                            editCardTemplate={props.editCardTemplate}
                        />
                    )}
                </For>

                <EmptyCardButton
                    addCard={props.addCard}
                />

                <Menu.Text
                    icon={<AddIcon/>}
                    id='add-template'
                    name={intl.formatMessage({id: 'ViewHeader.add-template', defaultMessage: 'New template'})}
                    onClick={() => props.addCardTemplate()}
                />

                <ExtensionSlot
                    name='newCardMenu'
                    board={props.board}
                />
            </Menu>
        </ButtonWithMenu>
    )

    return button
}

export default NewCardButton
