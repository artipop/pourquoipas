import {fireEvent, render, screen, waitFor, within} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import userEvent from '@testing-library/user-event'

import Mutator from '../../mutator'
import {Utils} from '../../utils'

import {TestBlockFactory} from '../../test/testBlockFactory'
import {IPropertyTemplate} from '../../blocks/board'
import {TestRouter, mockAppStore, wrapDNDIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'

import {fakeGoSide} from '../../test/goSide'

import KanbanCard from './kanbanCard'

vi.mock('../../mutator')
vi.mock('../../utils')
const mockedUtils = vi.mocked(Utils)
const mockedMutator = vi.mocked(Mutator)

describe('src/components/kanban/kanbanCard', () => {
    const board = TestBlockFactory.createBoard()
    const card = TestBlockFactory.createCard(board)

    // …and the board for the same reason: what the card offers from where it
    // stands is asked for the whole board at once, by its id.
    board.id = 'board_id_1'

    // Utils is mocked here, so createGuid returns undefined and the card would
    // have no id -- which dnd-kit reads as `source?.id === this.id`, i.e. every
    // card claiming to be the one being dragged. Give it one.
    card.id = 'card_id_1'
    const propertyTemplate: IPropertyTemplate = {
        id: 'id',
        name: 'name',
        type: 'text',
        options: [
            {
                color: 'propColorOrange',
                id: 'property_value_id_1',
                value: 'Q1',
            },
            {
                color: 'propColorBlue',
                id: 'property_value_id_2',
                value: 'Q2',
            },
        ],
    }
    const state = {

        // The card menu offers "Copy link" where somebody else can open it,
        // which is a team install (docs/teamwork.md).
        clientConfig: {
            value: {teamMode: true},
        },
        cards: {
            cards: [card],
        },
        teams: {
            current: {id: 'team-id'},
        },
        boards: {
            current: 'board_id_1',
            boards: {
                board_id_1: {id: 'board_id_1'},
            },
            myBoardMemberships: {
                board_id_1: {userId: 'user_id_1', schemeAdmin: true},
            },
        },
        contents: {},
        comments: {
            comments: {},
        },
        users: {
            me: {
                id: 'user_id_1',
                props: {},
            },
        },
    }
    const store = mockAppStore(state)
    beforeEach(vi.clearAllMocks)

    // The native HTML5 attribute, left over from react-dnd's HTML5 backend.
    // dnd-kit drags on pointer events, and when it finds a natively draggable
    // element it stands aside for the browser -- it binds `dragstart` to its own
    // cancel. So the browser began a native drag nobody handles, sent
    // pointercancel, and dnd-kit called off a drag that had never started: a
    // pointerdown with nothing after it, not even a pointerup. Whether the
    // native drag or the five-pixel threshold got there first came down to how
    // fast the hand moved, which is what made it look random.
    //
    // The browser's drag is still the only one another application can take a
    // card from, so it has a grip of its own, and the grip keeps its press from
    // the card: dnd-kit never sees it and nothing races.
    test('offers the card to the browser own drag and drop only from its grip', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={false}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})

        const draggables = container.querySelectorAll('[draggable="true"]')
        expect(draggables).toHaveLength(1)
        expect(draggables[0]).toHaveClass('KanbanCard__dragOut')

        const pressed = vi.fn()
        container.querySelector('.KanbanCard')!.addEventListener('pointerdown', pressed)
        fireEvent.pointerDown(draggables[0])
        expect(pressed).not.toHaveBeenCalled()
    })

    // A terminal takes a drop as typed text, which is how the card reaches an
    // agent running in one.
    test('a card dragged out of the window carries its text', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={true}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})

        const data: {[type: string]: string} = {}
        const dataTransfer = {
            setData: (type: string, value: string) => {
                data[type] = value
            },
            setDragImage: vi.fn(),
            effectAllowed: 'all',
        }
        fireEvent.dragStart(container.querySelector('.KanbanCard__dragOut')!, {dataTransfer})
        expect(data['text/plain']).toMatch(new RegExp(`^# ${card.title}`))
        expect(dataTransfer.effectAllowed).toBe('copy')
    })

    test('should match snapshot', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={false}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        expect(container).toMatchSnapshot()
    })
    test('should match snapshot with readonly', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={true}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        expect(container).toMatchSnapshot()
    })
    test('return kanbanCard and click on delete menu ', () => {
        const result = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={false}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})

        const {container} = result

        const elementMenuWrapper = screen.getByRole('button', {name: 'menuwrapper'})
        expect(elementMenuWrapper).not.toBeNull()
        userEvent.click(elementMenuWrapper)
        expect(container).toMatchSnapshot()
        const elementButtonDelete = within(elementMenuWrapper).getByRole('button', {name: 'Delete'})
        expect(elementButtonDelete).not.toBeNull()
        userEvent.click(elementButtonDelete)

        const confirmDialog = screen.getByTitle('Confirmation dialog')
        expect(confirmDialog).toBeDefined()
        const confirmButton = within(confirmDialog).getByRole('button', {name: 'Delete'})
        expect(confirmButton).toBeDefined()
        userEvent.click(confirmButton)

        expect(mockedMutator.deleteBlock).toHaveBeenCalledWith(card, 'delete card')
    })

    test('return kanbanCard and click on duplicate menu ', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={false}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        const elementMenuWrapper = screen.getByRole('button', {name: 'menuwrapper'})
        expect(elementMenuWrapper).not.toBeNull()
        userEvent.click(elementMenuWrapper)
        expect(container).toMatchSnapshot()
        const elementButtonDuplicate = within(elementMenuWrapper).getByRole('button', {name: 'Duplicate'})
        expect(elementButtonDuplicate).not.toBeNull()
        userEvent.click(elementButtonDuplicate)
        expect(mockedMutator.duplicateCard).toHaveBeenCalledTimes(1)
    })

    test('return kanbanCard and click on copy link menu ', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={false}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        const elementMenuWrapper = screen.getByRole('button', {name: 'menuwrapper'})
        expect(elementMenuWrapper).not.toBeNull()
        userEvent.click(elementMenuWrapper)
        expect(container).toMatchSnapshot()
        const elementButtonCopyLink = within(elementMenuWrapper).getByRole('button', {name: 'Copy link'})
        expect(elementButtonCopyLink).not.toBeNull()
        userEvent.click(elementButtonCopyLink)
        expect(mockedUtils.copyTextToClipboard).toHaveBeenCalledTimes(1)
    })

    // An agent holding its turn open on a question is the one thing about a
    // card a person has to notice without opening it — and it is noticed on the
    // card the agent is working, not on the board as a whole.
    test('shows that the card agent is waiting for a person', async () => {
        fakeGoSide({ListAttention: vi.fn().mockResolvedValue(JSON.stringify([
            {key: 'q:q-1', questionId: 'q-1', cardId: card.id, agent: 'clauuus', reason: 'question', awaiting: true},
        ]))})

        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={false}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})

        expect(await screen.findByTitle('clauuus is asking')).toBeInTheDocument()
        fakeGoSide()
    })

    // Where the card goes next is not answered on its face. Two buttons stood
    // here for a while, and what that cost was a face saying two things at
    // once: what the work is, and a decision about it.
    test('does not offer where the card goes next on its face', async () => {
        const bindings = {
            ListAttention: vi.fn().mockResolvedValue('[]'),
            MarkCardOutcome: vi.fn().mockResolvedValue(undefined),
        }
        fakeGoSide(bindings)

        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={false}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})

        await waitFor(() => expect(bindings.ListAttention).toHaveBeenCalled())
        expect(screen.queryByRole('button', {name: /Деплой/})).toBeNull()
        expect(screen.queryByRole('button', {name: /В работе/})).toBeNull()
        fakeGoSide()
    })

    test('says nothing on a card no agent is waiting on', async () => {
        fakeGoSide({ListAttention: vi.fn().mockResolvedValue('[]')})

        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <KanbanCard
                    card={card}
                    board={board}
                    visiblePropertyTemplates={[propertyTemplate]}
                    visibleBadges={false}
                    isSelected={false}
                    readonly={false}
                    onDrop={vi.fn()}
                    showCard={vi.fn()}
                    isManualSort={false}
                    index={0}
                    groupId='group-1'
                />
            </AppStoreProvider>,
        ), {wrapper: TestRouter})

        expect(screen.queryByRole('status')).toBeNull()
        fakeGoSide()
    })
})
