import {Show, createEffect, createSignal, onSettled} from 'solid-js'

import {FormattedMessage, useIntl} from '../../intl'

import ViewMenu from '../../components/viewMenu'
import mutator from '../../mutator'
import {Board, IPropertyTemplate} from '../../blocks/board'
import {BoardView} from '../../blocks/boardView'
import {Card} from '../../blocks/card'
import Button from '../../widgets/buttons/button'
import IconButton from '../../widgets/buttons/iconButton'
import DropdownIcon from '../../widgets/icons/dropdown'
import MenuWrapper from '../../widgets/menuWrapper'
import Editable from '../../widgets/editable'
import type {Focusable} from '../../widgets/editable'

import ModalWrapper from '../modalWrapper'

import {useAppSelector} from '../../store/hooks'
import {Permission} from '../../constants'
import {useHasCurrentBoardPermissions} from '../../hooks/permissions'
import {
    getOnboardingTourCategory,
    getOnboardingTourStarted,
    getOnboardingTourStep,
} from '../../store/users'
import {
    BoardTourSteps,
    TOUR_BOARD,
    TourCategoriesMapToSteps,
} from '../onboardingTour'
import {OnboardingBoardTitle} from '../cardDetail/cardDetail'
import AddViewTourStep from '../onboardingTour/addView/add_view'
import {getCurrentCard} from '../../store/cards'
import BoardPermissionGate from '../permissions/boardPermissionGate'
import type {Flow} from '../acp/automation'

import NewCardButton from './newCardButton'
import ViewHeaderFlowMenu from './viewHeaderFlowMenu'
import ViewHeaderDisplayByMenu from './viewHeaderDisplayByMenu'
import ViewHeaderSortMenu from './viewHeaderSortMenu'
import ViewHeaderActionsMenu from './viewHeaderActionsMenu'
import ViewHeaderSearch from './viewHeaderSearch'
import FilterComponent from './filterComponent'

import './viewHeader.scss'

type Props = {
    board: Board
    activeView: BoardView
    views: BoardView[]
    cards: Card[]

    // The routes this board may be drawn by. Read by whoever owns the board's
    // subscription, so the picker and the columns are narrowed from one list.
    flows: Flow[]
    groupByProperty?: IPropertyTemplate
    addCard: () => void
    addCardFromTemplate: (cardTemplateId: string) => void
    addCardTemplate: () => void
    editCardTemplate: (cardTemplateId: string) => void
    readonly: boolean
    dateDisplayProperty?: IPropertyTemplate
}

const ViewHeader = (props: Props) => {
    const [showFilter, setShowFilter] = createSignal(false)
    const [lockFilterOnClose, setLockFilterOnClose] = createSignal(false)
    const intl = useIntl()
    const canEditBoardProperties = useHasCurrentBoardPermissions([Permission.ManageBoardProperties])

    // withGroupBy is parked with the control it gated — see the render below.
    // const withGroupBy = () => props.activeView.fields.viewType === 'board' || props.activeView.fields.viewType === 'table'
    const withDisplayBy = () => props.activeView.fields.viewType === 'calendar'
    const withSortBy = () => props.activeView.fields.viewType !== 'calendar'

    const [viewTitle, setViewTitle] = createSignal(() => props.activeView.title)

    const hasFilter = () => props.activeView.fields.filter && props.activeView.fields.filter.filters?.length > 0

    const isOnboardingBoard = () => props.board.title === OnboardingBoardTitle
    const onboardingTourStarted = useAppSelector(getOnboardingTourStarted)
    const onboardingTourCategory = useAppSelector(getOnboardingTourCategory)
    const onboardingTourStep = useAppSelector(getOnboardingTourStep)

    const currentCard = useAppSelector(getCurrentCard)
    const noCardOpen = () => !currentCard()

    const showTourBaseCondition = () => isOnboardingBoard() &&
        onboardingTourStarted() &&
        noCardOpen() &&
        onboardingTourCategory() === TOUR_BOARD &&
        onboardingTourStep() === BoardTourSteps.ADD_VIEW.toString()

    const [delayComplete, setDelayComplete] = createSignal(false)

    createEffect(showTourBaseCondition, (shown) => {
        if (!shown) {
            return undefined
        }
        const timeout = setTimeout(() => setDelayComplete(true), 800)
        return () => clearTimeout(timeout)
    })

    onSettled(() => {
        if (!BoardTourSteps.SHARE_BOARD) {
            BoardTourSteps.SHARE_BOARD = 2
        }

        TourCategoriesMapToSteps[TOUR_BOARD] = BoardTourSteps
    })

    const showAddViewTourStep = () => showTourBaseCondition() && delayComplete()

    // Renaming is a mode now rather than "the field is always a field": the
    // name is the menu's trigger, so it is read-only until «Переименовать вид»
    // asks for it, and it goes back to read-only on save, on cancel and on the
    // blur Editable already reports as a save.
    const [renaming, setRenaming] = createSignal(false)
    let titleFocus: Focusable | undefined
    const startRenaming = () => {
        setRenaming(true)

        // After the menu has closed and the input has stopped being read-only,
        // or the focus lands on something that cannot take it.
        queueMicrotask(() => titleFocus?.focus(true))
    }

    return (
        <div class='ViewHeader'>
            <div class='viewSelector'>
                <div>
                    <MenuWrapper
                        label={intl.formatMessage({id: 'ViewHeader.view-menu', defaultMessage: 'View menu'})}
                        menu={
                            <ViewMenu
                                board={props.board}
                                activeView={props.activeView}
                                views={props.views}
                                readonly={props.readonly || !canEditBoardProperties()}
                                onRename={canEditBoardProperties() && !props.readonly ? startRenaming : undefined}
                            />
                        }
                    >
                        {/* The name is inside the trigger, so clicking the word
                            «Задачи» opens the view menu. It used to be a text
                            field 131px wide with the chevron parked past the
                            end of it, and the only 24px that opened anything
                            were the ones nobody aims at: clicking the name put
                            a cursor in it and nothing else happened. */}
                        <div onClick={(e) => renaming() && e.stopPropagation()}>
                            <Show
                                when={renaming()}
                                fallback={<span class='viewTitle'>{viewTitle() || 'Untitled View'}</span>}
                            >
                                {/* A field only while it is one. An input is as
                                    wide as its `size` whatever the CSS says, so
                                    a read-only one drew a 20-character box
                                    around a seven-letter name and put the
                                    chevron past the end of it. */}
                                <Editable
                                    ref={(f) => {
                                        titleFocus = f
                                    }}
                                    value={viewTitle()}
                                    placeholderText='Untitled View'
                                    onSave={(): void => {
                                        setRenaming(false)
                                        mutator.changeBlockTitle(props.activeView.boardId, props.activeView.id, props.activeView.title, viewTitle())
                                    }}
                                    onCancel={(): void => {
                                        setRenaming(false)
                                        setViewTitle(props.activeView.title)
                                    }}
                                    onChange={setViewTitle}
                                    saveOnEsc={true}
                                    readonly={props.readonly || !canEditBoardProperties()}
                                    spellCheck={true}
                                    autoExpand={false}
                                />
                            </Show>
                        </div>
                        <Show when={!props.readonly}>
                            <IconButton icon={<DropdownIcon/>}/>
                        </Show>
                    </MenuWrapper>
                    <Show when={showAddViewTourStep()}>
                        <AddViewTourStep/>
                    </Show>
                </div>
            </div>

            <div class='octo-spacer'/>

            <Show when={!props.readonly && canEditBoardProperties()}>
                {/* Card properties and Group by are parked rather than removed.
                    Which fields a card shows is a question nobody has been
                    asked whether they want here, and the board is always
                    grouped by the column property — the stages of a route are
                    its options, so grouping by anything else is a board the
                    automation cannot read. Both cost a slot in a bar that was
                    already overflowing. The machinery is untouched, and this
                    is one uncomment away.

                <ViewHeaderPropertiesMenu
                    properties={props.board.cardProperties}
                    activeView={props.activeView}
                />

                <Show when={withGroupBy()}>
                    <ViewHeaderGroupByMenu
                        properties={props.board.cardProperties}
                        activeView={props.activeView}
                        groupByProperty={props.groupByProperty}
                    />
                </Show>
                */}

                {/* Route */}

                <Show when={props.activeView.fields.viewType === 'board'}>
                    <ViewHeaderFlowMenu
                        activeView={props.activeView}
                        flows={props.flows}
                    />
                </Show>

                {/* Display by */}

                <Show when={withDisplayBy()}>
                    <ViewHeaderDisplayByMenu
                        properties={props.board.cardProperties}
                        activeView={props.activeView}
                        dateDisplayPropertyName={props.dateDisplayProperty?.name}
                    />
                </Show>

                {/* Filter */}

                <ModalWrapper>
                    <Button
                        active={hasFilter()}
                        onClick={() => setShowFilter(!showFilter())}
                        onMouseOver={() => setLockFilterOnClose(true)}
                        onMouseLeave={() => setLockFilterOnClose(false)}
                    >
                        <FormattedMessage
                            id='ViewHeader.filter'
                            defaultMessage='Filter'
                        />
                    </Button>
                    <Show when={showFilter()}>
                        <FilterComponent
                            board={props.board}
                            activeView={props.activeView}
                            onClose={() => {
                                if (!lockFilterOnClose()) {
                                    setShowFilter(false)
                                }
                            }}
                        />
                    </Show>
                </ModalWrapper>

                {/* Sort */}

                <Show when={withSortBy()}>
                    <ViewHeaderSortMenu
                        properties={props.board.cardProperties}
                        activeView={props.activeView}
                        orderedCards={props.cards}
                    />
                </Show>
            </Show>

            {/* Search */}

            <ViewHeaderSearch/>

            {/* Options menu */}

            <Show when={!props.readonly}>
                <ViewHeaderActionsMenu
                    board={props.board}
                    activeView={props.activeView}
                    cards={props.cards}
                />

                {/* New card button */}

                <BoardPermissionGate permissions={[Permission.ManageBoardCards]}>
                    <NewCardButton
                        board={props.board}
                        addCard={props.addCard}
                        addCardFromTemplate={props.addCardFromTemplate}
                        addCardTemplate={props.addCardTemplate}
                        editCardTemplate={props.editCardTemplate}
                    />
                </BoardPermissionGate>
            </Show>
        </div>
    )
}

export default ViewHeader
