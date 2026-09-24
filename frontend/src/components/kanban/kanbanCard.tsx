import {For, Show, createSignal} from 'solid-js'

import {useIntl} from '../../intl'

import {Board, IPropertyTemplate} from '../../blocks/board'
import {Card} from '../../blocks/card'
import {useListSortable} from '../../hooks/sortable'
import mutator from '../../mutator'
import {Utils} from '../../utils'
import CompassIcon from '../../widgets/icons/compassIcon'
import MenuWrapper from '../../widgets/menuWrapper'
import Tooltip from '../../widgets/tooltip'
import PropertyValueElement from '../propertyValueElement'
import ConfirmationDialogBox, {ConfirmationDialogBoxProps} from '../confirmationDialogBox'
import './kanbanCard.scss'
import CardBadges from '../cardBadges'
import CardActionsMenu from '../cardActionsMenu/cardActionsMenu'
import CardActionsMenuIcon from '../cardActionsMenu/cardActionsMenuIcon'
import {attentionHeading, useCardAttention} from '../acp/attention'
import {findWorkdirProperty} from '../acp/workdirSync'
import {aiEnabled} from '../../aiSwitch'
import {cardAsText} from '../../cardText'
import {useAppStore} from '../../store/hooks'
import {getCardContents} from '../../store/contents'
import GripIcon from '../../widgets/icons/grip'

export const OnboardingCardClassName = 'onboardingCard'

type Props = {
    card: Card
    board: Board
    visiblePropertyTemplates: IPropertyTemplate[]
    isSelected: boolean
    visibleBadges: boolean
    onClick?: (e: MouseEvent, card: Card) => void
    readonly: boolean
    onDrop: (srcCard: Card, dstCard: Card) => void
    showCard: (cardId?: string) => void
    isManualSort: boolean

    // Where the card sits, which is what a drag changes: dnd-kit needs the list
    // and the place in it to tell a move between columns from a reorder within
    // one, rather than only that something was released over something else.
    index: number
    groupId: string

    // dragDisabled is for a board whose columns are not places a card can be
    // put — the inbox view, grouped by who brought the card. It is not the same as
    // readonly: the card still opens, and its menu still works.
    dragDisabled?: boolean
}

const KanbanCard = (props: Props) => {
    const intl = useIntl()
    const [isDragging, isOver, cardRef] = useListSortable(
        'card',
        () => props.card,
        () => !props.readonly && !props.dragDisabled,
        (src, dst) => props.onDrop(src, dst),
        () => ({id: props.card.id, index: props.index, group: props.groupId}),
    )
    const visiblePropertyTemplates = () => props.visiblePropertyTemplates || []

    // Where this card's work lives, on the card's face. It is the one property
    // that decides what an agent can do here at all, and reading it meant
    // opening the card — while on a board of several folders, which folder a
    // card belongs to is most of what the board is being scanned for.
    //
    // The card's own field and nothing else: the board's drafts folder is where a
    // conversation stands when the card names no folder, not an answer the card
    // gives, and printing it here would say the card belongs to a project it
    // does not. An empty field draws nothing.
    //
    // Left out when the view already lists the property, or the same fact would
    // stand on the card twice.
    const folderChip = () => {
        if (!aiEnabled()) {
            return ''
        }
        const property = findWorkdirProperty(props.board, props.board.cardProperties)
        if (!property || visiblePropertyTemplates().some((t) => t.id === property.id)) {
            return ''
        }
        const value = props.card.fields.properties[property.id]
        const optionId = Array.isArray(value) ? value[0] : value
        return property.options.find((o) => o.id === optionId)?.value || ''
    }

    // An agent that has stopped to ask something is the one thing about a card
    // that a person has to notice without opening it: its turn stays open until
    // somebody answers, and the answer is inside the card.
    const attention = useCardAttention(() => props.card.id)
    const agentTitle = () => (attention() ? attentionHeading(intl, attention()!) : '')
    const showsAgent = () => aiEnabled() && Boolean(attention())

    const openCard = (e: MouseEvent) => {
        // The card is a click target of its own; this is the same destination
        // said twice, and the stopped propagation keeps it from being opened
        // twice over.
        e.stopPropagation()
        props.showCard(props.card.id)
    }
    const classes = () => {
        let name = props.isSelected ? 'KanbanCard selected' : 'KanbanCard'
        if (props.isManualSort && isOver()) {
            name += ' dragover'
        }
        return name
    }

    // Out of the window rather than across the board: the browser's own drag,
    // which is the only one another application can receive, from a grip of its
    // own. The card is dnd-kit's, and a natively draggable card makes the two
    // drags race (the test below the snapshot says how). The grip keeps its
    // pointerdown from the card, so dnd-kit never hears the press at all.
    //
    // Contents are read when the drag starts rather than subscribed to: every
    // card on the board would otherwise watch its description for a gesture
    // made once in a while.
    const store = useAppStore()
    const offerCard = (e: DragEvent) => {
        if (!e.dataTransfer) {
            return
        }
        const contents = getCardContents(props.card.id)(store.state)
        e.dataTransfer.effectAllowed = 'copy'
        e.dataTransfer.setData('text/plain', cardAsText(props.card, props.board, contents, intl))
        const cardElement = (e.currentTarget as HTMLElement).closest('.KanbanCard')
        if (cardElement) {
            const rect = cardElement.getBoundingClientRect()
            e.dataTransfer.setDragImage(cardElement, e.clientX - rect.left, e.clientY - rect.top)
        }
    }

    // On the element itself: Solid delegates onPointerDown to the document,
    // which is after dnd-kit's own listener on the card has taken the press.
    const keepPressFromCard = (el: HTMLElement) => el.addEventListener('pointerdown', (e) => e.stopPropagation())
    const dragOutTitle = () => intl.formatMessage({id: 'KanbanCard.dragOut', defaultMessage: 'Drag the card into another app, such as a terminal'})

    const [showConfirmationDialogBox, setShowConfirmationDialogBox] = createSignal<boolean>(false)
    const handleDeleteCard = () => {
        const card = props.card
        if (!card) {
            Utils.assertFailure()
            return
        }
        mutator.deleteBlock(card, 'delete card')
    }

    const confirmDialogProps: ConfirmationDialogBoxProps = {
        heading: intl.formatMessage({id: 'CardDialog.delete-confirmation-dialog-heading', defaultMessage: 'Confirm card delete!'}),
        confirmButtonText: intl.formatMessage({id: 'CardDialog.delete-confirmation-dialog-button-text', defaultMessage: 'Delete'}),
        onConfirm: handleDeleteCard,
        onClose: () => {
            setShowConfirmationDialogBox(false)
        },
    }

    const handleDeleteButtonOnClick = () => {
        // user trying to delete a card with blank name
        // but content present cannot be deleted without
        // confirmation dialog
        if (props.card?.title === '' && props.card?.fields?.contentOrder?.length === 0) {
            handleDeleteCard()
            return
        }
        setShowConfirmationDialogBox(true)
    }

    const handleOnClick = (e: MouseEvent) => {
        if (props.onClick) {
            props.onClick(e, props.card)
        }
    }

    return (
        <>
            <div
                ref={props.readonly || props.dragDisabled ? undefined : cardRef}
                class={[classes(), {'KanbanCard--agent': showsAgent()}]}
                style={{opacity: isDragging() ? 0.5 : 1}}
                onClick={handleOnClick}
            >
                {/* The agent's question, from the card's bottom-right
                    corner: amber, because the machine needs a person here, and
                    a click away from the card where it is answered. It stands
                    in a strip the card reserves for it (`KanbanCard--agent`),
                    so nothing on the card runs under it. */}
                <Show when={showsAgent()}>
                    <Show
                        when={!props.readonly}
                        fallback={
                            <span
                                class='KanbanCard__asking'
                                role='status'
                                title={agentTitle()}
                                aria-label={agentTitle()}
                            >
                                <CompassIcon icon='help-circle-outline'/>
                            </span>
                        }
                    >
                        <button
                            type='button'
                            class='KanbanCard__asking'
                            title={agentTitle()}
                            aria-label={agentTitle()}
                            onClick={openCard}
                        >
                            <CompassIcon icon='help-circle-outline'/>
                        </button>
                    </Show>
                </Show>

                <span
                    class='KanbanCard__dragOut'
                    draggable='true'
                    title={dragOutTitle()}
                    aria-label={dragOutTitle()}
                    ref={keepPressFromCard}
                    onClick={(e) => e.stopPropagation()}
                    onDragStart={offerCard}
                >
                    <GripIcon/>
                </span>

                <Show when={!props.readonly}>
                    <MenuWrapper
                        class={'optionsMenu'}
                        stopPropagationOnToggle={true}
                        menu={
                            <CardActionsMenu
                                cardId={props.card!.id}
                                boardId={props.card!.boardId}
                                onClickDelete={handleDeleteButtonOnClick}
                                onClickDuplicate={() => {
                                    mutator.duplicateCard(
                                        props.card.id,
                                        props.board.id,
                                        false,
                                        'duplicate card',
                                        false,
                                        {},
                                        async (newCardId) => {
                                            props.showCard(newCardId)
                                        },
                                        async () => {
                                            props.showCard(undefined)
                                        },
                                    )
                                }}
                            />
                        }
                    >
                        <CardActionsMenuIcon/>
                    </MenuWrapper>
                </Show>

                <div class='octo-icontitle'>
                    <Show when={props.card.fields.icon}>
                        <div class='octo-icon'>{props.card.fields.icon}</div>
                    </Show>
                    <div
                        class='octo-titletext'
                    >
                        {props.card.title || intl.formatMessage({id: 'KanbanCard.untitled', defaultMessage: 'Untitled'})}
                    </div>
                </div>
                <For each={visiblePropertyTemplates()}>
                    {(template) => (
                        <Tooltip
                            title={template.name}
                        >
                            <PropertyValueElement
                                board={props.board}
                                readOnly={true}
                                card={props.card}
                                propertyTemplate={template}
                                showEmptyPlaceholder={false}
                            />
                        </Tooltip>
                    )}
                </For>
                <Show when={folderChip()}>
                    <div class='KanbanCard__folder'>
                        <CompassIcon icon='folder-outline'/>
                        <span>{folderChip()}</span>
                    </div>
                </Show>
                <Show when={props.visibleBadges}>
                    <CardBadges card={props.card}/>
                </Show>
            </div>

            <Show when={showConfirmationDialogBox()}>
                <ConfirmationDialogBox dialogBox={confirmDialogProps}/>
            </Show>
        </>
    )
}

export default KanbanCard
