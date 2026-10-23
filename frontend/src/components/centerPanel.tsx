import {Loading, Match, Show, Switch, createEffect, createMemo, createSignal, lazy, onSettled} from 'solid-js'

import {useIntl} from '../intl'

import {useHotkeys} from '../hooks/hotkeys'
import {ClientConfig} from '../config/clientConfig'

import {Block} from '../blocks/block'
import {BlockIcons} from '../blockIcons'
import {Card, createCard} from '../blocks/card'
import {Board, IPropertyTemplate, BoardGroup} from '../blocks/board'
import {BoardView} from '../blocks/boardView'
import {CardFilter} from '../cardFilter'
import mutator from '../mutator'
import {Utils} from '../utils'
import {UserSettings} from '../userSettings'
import {personName} from '../userDisplay'
import {getCurrentCard} from '../store/cards'
import {getVisibleAndHiddenGroups, withoutEmptyGroup} from '../boardUtils'

import {getClientConfig} from '../store/clientConfig'

import './centerPanel.scss'

import {useAppSelector, useAppStore} from '../store/hooks'

import {
    getMe,
    getBoardUsers,
    getOnboardingTourCategory,
    getOnboardingTourStarted,
    getOnboardingTourStep,
} from '../store/users'

import {ExtensionSlot, withNewCardDefaults} from '../extensions'

import {cardColumnProperty} from '../boardFields'

import ShareBoardButton from './shareBoard/shareBoardButton'
import ShareBoardLoginButton from './shareBoard/shareBoardLoginButton'

import CardDialog from './cardDialog'
import RootPortal from './rootPortal'
import ViewHeader from './viewHeader/viewHeader'
import ViewTitle from './viewTitle'
import Kanban from './kanban/kanban'

import Table from './table/table'

import Gallery from './gallery/gallery'
import {FINISHED, TOUR_BOARD, TOUR_CARD} from './onboardingTour'
import {patchPreferences, useOnTourBoard} from './onboardingTour/tourState'
import ShareBoardTourStep from './onboardingTour/shareBoard/shareBoard'

// FullCalendar is half a megabyte of source, and a board opens on its kanban:
// the calendar arrives when somebody switches to it.
const CalendarFullView = lazy(() => import('./calendar/fullCalendar'))

type Props = {
    clientConfig?: ClientConfig
    board: Board
    cards: Card[]
    activeView: BoardView
    views: BoardView[]
    groupByProperty?: IPropertyTemplate
    dateDisplayProperty?: IPropertyTemplate
    readonly: boolean
    shownCardId?: string
    showCard: (cardId?: string) => void
}

const CenterPanel = (props: Props) => {
    const intl = useIntl()
    const [selectedCardIds, setSelectedCardIds] = createSignal<string[]>([])

    const [cardIdToFocusOnRender, setCardIdToFocusOnRender] = createSignal('')

    const onboardingTourStarted = useAppSelector(getOnboardingTourStarted)
    const onboardingTourCategory = useAppSelector(getOnboardingTourCategory)
    const onboardingTourStep = useAppSelector(getOnboardingTourStep)
    const me = useAppSelector(getMe)
    const currentCard = useAppSelector(getCurrentCard)
    const boardUsers = useAppSelector(getBoardUsers)
    const {actions} = useAppStore()

    const clientConfig = useAppSelector<ClientConfig>(getClientConfig)

    onSettled(() => {
    })

    useHotkeys('esc', (e: KeyboardEvent) => {
        if (e.target !== document.body || props.readonly) {
            return
        }
        if (selectedCardIds().length > 0) {
            setSelectedCardIds([])
            e.stopPropagation()
        }
    })

    useHotkeys('ctrl+d', (e: KeyboardEvent) => {
        if (e.target !== document.body || props.readonly) {
            return
        }

        if (selectedCardIds().length > 0) {
            // CTRL+D: Duplicate selected cards
            const {board} = props
            if (selectedCardIds().length < 1) {
                return
            }

            mutator.performAsUndoGroup(async () => {
                for (const cardId of selectedCardIds()) {
                    const card = props.cards.find((o) => o.id === cardId)
                    if (card) {
                        mutator.duplicateCard(cardId, board.id)
                    } else {
                        Utils.assertFailure(`Selected card not found: ${cardId}`)
                    }
                }
            })

            setSelectedCardIds([])
            e.stopPropagation()
            e.preventDefault()
        }
    })

    useHotkeys('del,backspace', (e: KeyboardEvent) => {
        if (e.target !== document.body || props.readonly) {
            return
        }

        if (selectedCardIds().length > 0) {
            // Backspace or Del: Delete selected cards
            if (selectedCardIds().length < 1) {
                return
            }

            mutator.performAsUndoGroup(async () => {
                for (const cardId of selectedCardIds()) {
                    const card = props.cards.find((o) => o.id === cardId)
                    if (card) {
                        mutator.deleteBlock(card, selectedCardIds().length > 1 ? `delete ${selectedCardIds().length} cards` : 'delete card')
                    } else {
                        Utils.assertFailure(`Selected card not found: ${cardId}`)
                    }
                }
            })

            setSelectedCardIds([])
            e.stopPropagation()
        }
    })

    const showCard = (cardId?: string) => {
        if (selectedCardIds().length > 0) {
            setSelectedCardIds([])
        }
        props.showCard(cardId)
    }

    // What a new card is born with: the view's own filter, the group it was
    // dropped into, its column, and what features default (extensions.tsx).
    const newCardProperties = (groupByOptionId?: string): Record<string, string> => {
        const {activeView, board, groupByProperty} = props

        const properties = CardFilter.propertiesThatMeetFilterGroup(activeView.fields.filter, board.cardProperties)
        if ((activeView.fields.viewType === 'board' || activeView.fields.viewType === 'table') && groupByProperty) {
            if (groupByOptionId) {
                properties[groupByProperty.id] = groupByOptionId
            } else {
                delete properties[groupByProperty.id]
            }
        }

        // A card stands in a column from the moment it exists: with no group
        // dropped into, the first one. The board's columns are its stages, and
        // a card in none of them would stand in a column for «no status».
        const column = cardColumnProperty(board, groupByProperty)
        if (column && !properties[column.id] && column.options.length > 0) {
            properties[column.id] = column.options[0].id
        }

        return withNewCardDefaults(board, properties, groupByProperty)
    }

    const addCard = async (groupByOptionId?: string, show = false, properties: Record<string, string> = {}): Promise<void> => {
        const {activeView, board} = props

        const card = createCard()

        card.parentId = board.id
        card.boardId = board.id
        card.fields.properties = {...card.fields.properties, ...properties, ...newCardProperties(groupByOptionId)}
        if (!card.fields.icon && UserSettings.prefillRandomIcons) {
            card.fields.icon = BlockIcons.shared.randomIcon()
        }
        mutator.performAsUndoGroup(async () => {
            const newCard = await mutator.insertBlock(
                card.boardId,
                card,
                'add card',
                async (block: Block) => {
                    if (show) {
                        actions.cards.addCard(createCard(block))
                        actions.views.updateView({...activeView, fields: {...activeView.fields, cardOrder: [...activeView.fields.cardOrder, block.id]}})
                        showCard(block.id)
                    } else {
                        // Focus on this card's title inline on next render
                        setCardIdToFocusOnRender(block.id)
                        setTimeout(() => setCardIdToFocusOnRender(''), 300)
                    }
                },
                async () => {
                    showCard(undefined)
                },
            )
            await mutator.changeViewCardOrder(board.id, activeView.id, activeView.fields.cardOrder, [...activeView.fields.cardOrder, newCard.id], 'add-card')
        })
    }

    const addEmptyCardAndShow = () => addCard('', true)

    // The card a board made from a template opens with, marked so rather than
    // found by its title.
    const readFirstCard = () => {
        const card = props.cards.find((c) => c.fields.readFirst)
        return card ? {title: card.title, open: () => showCard(card.id)} : undefined
    }

    // The card's tips are done and the card is closed: the board's own start,
    // on the tour's board and no other.
    const onTourBoard = useOnTourBoard()
    const shouldStartBoardsTour = (): boolean => {
        const completedCardsTour = onboardingTourCategory() === TOUR_CARD && onboardingTourStep() === FINISHED.toString()
        return onTourBoard() && onboardingTourStarted() && completedCardsTour && !currentCard()
    }

    createEffect(() => (shouldStartBoardsTour() ? me()?.id : undefined), (userId) => {
        if (userId) {
            patchPreferences(userId, {tourCategory: TOUR_BOARD, onboardingTourStep: '0'}, actions.users.patchProps)
        }
    })

    const backgroundClicked = (e: MouseEvent) => {
        if (selectedCardIds().length > 0) {
            setSelectedCardIds([])
            e.stopPropagation()
        }
    }

    const addCardFromTemplate = async (cardTemplateId: string, groupByOptionId?: string) => {
        const {activeView, board} = props

        const propertiesThatMeetFilters = newCardProperties(groupByOptionId)

        mutator.performAsUndoGroup(async () => {
            const [, newCardId] = await mutator.duplicateCard(
                cardTemplateId,
                board.id,
                true,
                intl.formatMessage({id: 'Mutator.new-card-from-template', defaultMessage: 'new card from template'}),
                false,
                propertiesThatMeetFilters,
                async (cardId) => {
                    actions.views.updateView({...activeView, fields: {...activeView.fields, cardOrder: [...activeView.fields.cardOrder, cardId]}})
                    showCard(cardId)
                },
                async () => {
                    showCard(undefined)
                },
            )
            await mutator.changeViewCardOrder(props.board.id, activeView.id, activeView.fields.cardOrder, [...activeView.fields.cardOrder, newCardId], 'add-card')
        })
    }

    const addCardTemplate = async () => {
        const {board} = props

        const cardTemplate = createCard()
        cardTemplate.fields.isTemplate = true
        cardTemplate.parentId = board.id
        cardTemplate.boardId = board.id

        await mutator.insertBlock(
            cardTemplate.boardId,
            cardTemplate,
            'add card template',
            async (newBlock: Block) => {
                const newTemplate = createCard(newBlock)
                actions.cards.addTemplate(newTemplate)
                showCard(newTemplate.id)
            }, async () => {
                showCard(undefined)
            },
        )
    }

    const editCardTemplate = (cardTemplateId: string) => {
        showCard(cardTemplateId)
    }

    const cardClicked = (e: MouseEvent, card: Card): void => {
        const {activeView, cards} = props

        if (e.shiftKey) {
            let newSelectedCardIds = [...selectedCardIds()]
            if (newSelectedCardIds.length > 0 && (e.metaKey || e.ctrlKey)) {
                // Cmd+Shift+Click: Extend the selection
                const orderedCardIds = cards.map((o) => o.id)
                const lastCardId = newSelectedCardIds[newSelectedCardIds.length - 1]
                const srcIndex = orderedCardIds.indexOf(lastCardId)
                const destIndex = orderedCardIds.indexOf(card.id)
                const newCardIds = (srcIndex < destIndex) ? orderedCardIds.slice(srcIndex, destIndex + 1) : orderedCardIds.slice(destIndex, srcIndex + 1)
                for (const newCardId of newCardIds) {
                    if (!newSelectedCardIds.includes(newCardId)) {
                        newSelectedCardIds.push(newCardId)
                    }
                }
                setSelectedCardIds(newSelectedCardIds)
            } else {
                // Shift+Click: add to selection
                if (newSelectedCardIds.includes(card.id)) {
                    newSelectedCardIds = selectedCardIds().filter((o) => o !== card.id)
                } else {
                    newSelectedCardIds.push(card.id)
                }
                setSelectedCardIds(newSelectedCardIds)
            }
        } else if (activeView.fields.viewType === 'board' || activeView.fields.viewType === 'gallery') {
            showCard(card.id)
        }

        e.stopPropagation()
    }

    // Sharing a board is offered where there is somebody to share it with, and
    // the same for the log-in button a shared board shows a stranger. The
    // question used to be asked of the current user's id — which stopped
    // answering it when the person at the machine got an account under that
    // very id (docs/teamwork.md).
    const showShareButton = () => !props.readonly && clientConfig().teamMode
    const showShareLoginButton = () => props.readonly && clientConfig().teamMode

    const getUserDisplayName = (boardGroup: BoardGroup) => {
        const user = boardUsers()[boardGroup.option.id]
        if (user) {
            return personName(intl, user, clientConfig().teammateNameDisplay, clientConfig().teamMode)
        } else if (boardGroup.option.id === 'undefined') {
            return intl.formatMessage({
                id: 'centerPanel.undefined',
                defaultMessage: 'No {propertyName}',
            }, {propertyName: props.groupByProperty?.name})
        }
        return intl.formatMessage({id: 'centerPanel.unknown-user', defaultMessage: 'Unknown user'})
    }

    const groups = createMemo(() => {
        const {cards, activeView, groupByProperty} = props
        const all = getVisibleAndHiddenGroups(cards, activeView.fields.visibleOptionIds, activeView.fields.hiddenOptionIds, groupByProperty)
        const vg = all.visible
        const hg = all.hidden
        if (groupByProperty?.type === 'createdBy' || groupByProperty?.type === 'updatedBy' || groupByProperty?.type === 'person') {
            if (boardUsers()) {
                vg.forEach((value) => {
                    value.option.value = getUserDisplayName(value)
                })
                hg.forEach((value) => {
                    value.option.value = getUserDisplayName(value)
                })
            }
        }
        return {visible: withoutEmptyGroup(vg), hidden: withoutEmptyGroup(hg)}
    })

    return (
        <div
            class='BoardComponent'
            onClick={backgroundClicked}
        >
            <Show when={props.shownCardId}>
                <RootPortal>
                    <CardDialog
                        board={props.board}
                        activeView={props.activeView}
                        views={props.views}
                        cards={props.cards}
                        cardId={props.shownCardId!}
                        onClose={() => showCard(undefined)}
                        showCard={(cardId) => showCard(cardId)}
                        readonly={props.readonly}
                    />
                </RootPortal>
            </Show>

            <div class='top-head'>
                <div class='mid-head'>
                    <ViewTitle
                        board={props.board}
                        readonly={props.readonly}
                    />
                    <div class='shareButtonWrapper'>
                        <Show when={showShareButton()}>
                            <ShareBoardButton
                                enableSharedBoards={props.clientConfig?.enablePublicSharedBoards || false}
                            />
                        </Show>
                        <Show when={showShareLoginButton()}>
                            <ShareBoardLoginButton/>
                        </Show>
                        <ShareBoardTourStep/>
                    </div>
                </div>
                <ViewHeader
                    board={props.board}
                    activeView={props.activeView}
                    cards={props.cards}
                    views={props.views}
                    groupByProperty={props.groupByProperty}
                    dateDisplayProperty={props.dateDisplayProperty}
                    addCard={addEmptyCardAndShow}
                    addCardFromTemplate={addCardFromTemplate}
                    addCardTemplate={addCardTemplate}
                    editCardTemplate={editCardTemplate}
                    readonly={props.readonly}
                />
                <ExtensionSlot
                    name='boardHeader'
                    board={props.board}
                    cards={props.cards}
                    columnPropertyId={cardColumnProperty(props.board, props.groupByProperty)?.id}
                    readFirst={readFirstCard()}
                />
            </div>

            <Switch>
                <Match when={props.activeView.fields.viewType === 'board'}>
                    <Kanban
                        board={props.board}
                        activeView={props.activeView}
                        cards={props.cards}
                        groupByProperty={props.groupByProperty}
                        visibleGroups={groups().visible}
                        hiddenGroups={groups().hidden}
                        selectedCardIds={selectedCardIds()}
                        readonly={props.readonly}
                        onCardClicked={cardClicked}
                        addCard={addCard}
                        addCardFromTemplate={addCardFromTemplate}
                        showCard={showCard}
                    />
                </Match>
                <Match when={props.activeView.fields.viewType === 'table'}>
                    <Table
                        board={props.board}
                        activeView={props.activeView}
                        cards={props.cards}
                        groupByProperty={props.groupByProperty}
                        views={props.views}
                        visibleGroups={groups().visible}
                        selectedCardIds={selectedCardIds()}
                        readonly={props.readonly}
                        cardIdToFocusOnRender={cardIdToFocusOnRender()}
                        showCard={showCard}
                        addCard={addCard}
                        onCardClicked={cardClicked}
                    />
                </Match>
                <Match when={props.activeView.fields.viewType === 'calendar'}>
                    <Loading fallback={null}>
                        <CalendarFullView
                            board={props.board}
                            cards={props.cards}
                            activeView={props.activeView}
                            readonly={props.readonly}
                            dateDisplayProperty={props.dateDisplayProperty}
                            showCard={showCard}
                            addCard={(properties: Record<string, string>) => {
                                addCard('', true, properties)
                            }}
                        />
                    </Loading>
                </Match>
                <Match when={props.activeView.fields.viewType === 'gallery'}>
                    <Gallery
                        board={props.board}
                        cards={props.cards}
                        activeView={props.activeView}
                        readonly={props.readonly}
                        onCardClicked={cardClicked}
                        selectedCardIds={selectedCardIds()}
                        addCard={(show) => addCard('', show)}
                    />
                </Match>
            </Switch>
        </div>
    )
}

export default CenterPanel
