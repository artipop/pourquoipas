// Dragging in the sidebar has no other guard that touches the wiring: the tests
// below are the whole difference between "a board moves between categories" and
// a drop quietly thrown away, which is how it was for a while -- the sidebar
// read where a drop landed off the dragged item's own index and group, and
// those only move when a dnd-kit plugin this application deliberately leaves
// out is installed. sidebar.test.tsx pins the mapping from a finished operation
// to a drop; only a real drag says the operation arrives at it at all.
import {render, waitFor} from '@solidjs/testing-library'

import {TestRouter, mockAppStore, mockMatchMedia, wrapDNDIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'
import {TestBlockFactory} from '../../test/testBlockFactory'
import {drag} from '../../test/drag'
import octoClient from '../../octoClient'

import Sidebar from './sidebar'

vi.mock('../../octoClient')
const mockedOctoClient = vi.mocked(octoClient)

describe('components/sidebar dragging', () => {
    const boards = ['board1', 'board2', 'board3'].map((id) => {
        const board = TestBlockFactory.createBoard()
        board.id = id
        board.title = id
        return board
    })

    beforeAll(() => {
        mockMatchMedia({matches: true})
    })

    beforeEach(() => {
        vi.clearAllMocks()
    })

    // Three categories under one another, the first holding two boards, the
    // second one, the third none -- an empty category being the only place
    // where the category's own drop zone is what a board can land on.
    async function setup() {
        const categories = [
            {id: 'category1', boardIDs: ['board1', 'board2']},
            {id: 'category2', boardIDs: ['board3']},
            {id: 'category3', boardIDs: []},
        ].map(({id, boardIDs}) => {
            const category = TestBlockFactory.createCategoryBoards()
            category.id = id
            category.name = id
            category.boardMetadata = boardIDs.map((boardID) => ({boardID, hidden: false}))
            return category
        })

        // The sidebar refetches its categories on mount and writes what comes
        // back over the initial state, so the client has to answer with the same
        // thing or the categories are gone by the time anything is dragged.
        mockedOctoClient.getSidebarCategories.mockResolvedValue(categories)

        const store = mockAppStore({
            teams: {
                current: {id: 'team-id'},
            },
            boards: {
                current: 'board1',
                boards: Object.fromEntries(boards.map((board) => [board.id, board])),
                myBoardMemberships: Object.fromEntries(boards.map((board) => [board.id, board])),
            },
            cards: {cards: {}, current: ''},
            views: {views: {}},
            users: {me: {id: 'user_id_1', props: {}}},
            sidebar: {
                categoryAttributes: categories,
                hiddenBoardIDs: [],
            },
        } as never, {client: mockedOctoClient as never})

        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <TestRouter>
                    <Sidebar onBoardTemplateSelectorOpen={vi.fn()}/>
                </TestRouter>
            </AppStoreProvider>,
        ))

        // The sidebar draws its categories once the refetch is in.
        await waitFor(() => expect(container.querySelectorAll('.SidebarBoardItem')).toHaveLength(3))

        // dnd-kit was handed the wrapper carrying the sortable ref, not the
        // visible row inside it.
        const areas = Array.from(container.querySelectorAll('.categoryBoardsDroppableArea'))
        const rows = Array.from(container.querySelectorAll('.SidebarBoardItem')).map((row) => row.parentElement!)
        const headers = Array.from(container.querySelectorAll('.octo-sidebar-item.category'))

        const boardRow = (id: string) => rows[boards.findIndex((board) => board.id === id)]
        const categoryOf = (index: number) => ({
            wrapper: areas[index].parentElement!.parentElement!,
            header: headers[index],
        })

        return {boardRow, categoryOf}
    }

    // A row shares its category's drop zone, and a pointer intersection scores
    // 1/distance-to-centre, so the aim has to be nearer the row's middle than
    // the category's or which of the two is the target comes down to a tie
    // broken by measurement order. Both halves of board3 satisfy that here.
    const upperHalfOf = (row: Element) => ({x: row.clientWidth / 2, y: (row.clientHeight * 3) / 8})
    const lowerHalfOf = (row: Element) => ({x: row.clientWidth / 2, y: (row.clientHeight * 3) / 4})

    it('moves a board into another category', async () => {
        const {boardRow} = await setup()

        await drag(boardRow('board1'), boardRow('board3'), {at: upperHalfOf(boardRow('board3'))})

        expect(mockedOctoClient.moveBoardToCategory).toHaveBeenCalledWith('team-id', 'board1', 'category2', 'category1')
        expect(mockedOctoClient.reorderSidebarCategoryBoards).toHaveBeenCalledWith('team-id', 'category2', ['board1', 'board3'])
    })

    // Released over the bottom of board3 rather than the top of it, which is the
    // whole difference between landing above it and landing below it.
    it('puts a board under the one it was dropped on the lower half of', async () => {
        const {boardRow} = await setup()

        await drag(boardRow('board1'), boardRow('board3'), {at: lowerHalfOf(boardRow('board3'))})

        expect(mockedOctoClient.moveBoardToCategory).toHaveBeenCalledWith('team-id', 'board1', 'category2', 'category1')
        expect(mockedOctoClient.reorderSidebarCategoryBoards).toHaveBeenCalledWith('team-id', 'category2', ['board3', 'board1'])
    })

    // The category that holds no boards offers nothing to drop onto but itself,
    // and its drop zone is the one that used to share an id with the category's
    // own sortable -- so this is also what says the two no longer evict each
    // other from dnd-kit's registry.
    it('moves a board into a category that holds none', async () => {
        const {boardRow, categoryOf} = await setup()

        await drag(boardRow('board1'), categoryOf(2).header)

        expect(mockedOctoClient.moveBoardToCategory).toHaveBeenCalledWith('team-id', 'board1', 'category3', 'category1')
        expect(mockedOctoClient.reorderSidebarCategoryBoards).toHaveBeenCalledWith('team-id', 'category3', ['board1'])
    })

    it('reorders boards within one category', async () => {
        const {boardRow} = await setup()

        await drag(boardRow('board1'), boardRow('board2'))

        expect(mockedOctoClient.moveBoardToCategory).not.toHaveBeenCalled()
        expect(mockedOctoClient.reorderSidebarCategoryBoards).toHaveBeenCalledWith('team-id', 'category1', ['board2', 'board1'])
    })

    // A category is dragged by its title row: its middle is a board inside it,
    // which is what a press there would pick up instead.
    it('reorders the categories themselves', async () => {
        const {categoryOf} = await setup()

        const first = categoryOf(0)
        await drag(first.wrapper, categoryOf(1).header, {grip: first.header})

        expect(mockedOctoClient.reorderSidebarCategories).toHaveBeenCalledWith('team-id', ['category2', 'category1', 'category3'])
    })
})
