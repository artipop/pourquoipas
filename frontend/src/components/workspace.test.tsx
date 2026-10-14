import {createSignal, flush} from 'solid-js'
import {render, waitFor} from '@solidjs/testing-library'

import userEvent from '@testing-library/user-event'

import {IUser} from '../user'
import octoClient from '../octoClient'
import {TestBlockFactory} from '../test/testBlockFactory'
import {TestRouter, mockAppStore, mockMatchMedia, wrapDNDIntl} from '../testUtils'
import {AppStoreProvider} from '../store'
import {Constants} from '../constants'
import {Utils} from '../utils'

import Workspace from './workspace'

Object.defineProperty(Constants, 'versionString', {value: '1.0.0'})
vi.useFakeTimers()
vi.mock('../utils')
vi.mock('../octoClient')
const mockedUtils = vi.mocked(Utils)
const mockedOctoClient = vi.mocked(octoClient)

// vi.importActual is a promise where jest.requireActual was a require, so the
// real module is fetched once here rather than from inside beforeEach.
const {Utils: actualUtils} = await vi.importActual<typeof import('../utils')>('../utils')
const board = TestBlockFactory.createBoard()
board.id = 'board1'
board.teamId = 'team-id'
board.cardProperties = [
    {
        id: 'property1',
        name: 'Property 1',
        type: 'text',
        options: [
            {
                id: 'value1',
                value: 'value 1',
                color: 'propColorBrown',
            },
        ],
    },
    {
        id: 'property2',
        name: 'Property 2',
        type: 'select',
        options: [
            {
                id: 'value2',
                value: 'value 2',
                color: 'propColorBlue',
            },
        ],
    },
]
const activeView = TestBlockFactory.createBoardView(board)
activeView.id = 'view1'
activeView.fields.hiddenOptionIds = []
activeView.fields.visiblePropertyIds = ['property1']
activeView.fields.visibleOptionIds = ['value1']
const fakeBoard = {id: board.id}
activeView.boardId = fakeBoard.id
const card1 = TestBlockFactory.createCard(board)
card1.id = 'card1'
card1.title = 'card-1'
card1.boardId = fakeBoard.id
const card2 = TestBlockFactory.createCard(board)
card2.id = 'card2'
card2.title = 'card-2'
card2.boardId = fakeBoard.id
const card3 = TestBlockFactory.createCard(board)
card3.id = 'card3'
card3.title = 'card-3'
card3.boardId = fakeBoard.id

const me: IUser = {
    id: 'user-id-1',
    username: 'username_1',
    email: '',
    nickname: '',
    firstname: '',
    lastname: '',
    props: {},
    create_at: 0,
    update_at: 0,
    is_bot: false,
    is_guest: false,
    roles: 'system_user',
}

const categoryAttribute1 = TestBlockFactory.createCategoryBoards()
categoryAttribute1.name = 'Category 1'
categoryAttribute1.boardMetadata = [{boardID: board.id, hidden: false}]

const mockRouteParams: Record<string, string> = {}
vi.mock('../hooks/routerMatch', () => ({
    useRouteMatch: () => () => ({
        path: '/',
        params: mockRouteParams,
    }),
}))

describe('src/components/workspace', () => {
    beforeEach(() => {
        mockRouteParams.boardId = board.id
        mockRouteParams.viewId = activeView.id
    })

    const state = {
        teams: {
            current: {id: 'team-id', title: 'Test Team'},
        },
        users: {
            me,
            boardUsers: {[me.id]: me},
            blockSubscriptions: [],
        },
        boards: {
            current: board.id,
            boards: {
                [board.id]: board,
            },
            templates: [],
            myBoardMemberships: {
                [board.id]: {userId: 'user_id_1', schemeAdmin: true},
            },
        },
        globalTemplates: {
            value: [],
        },
        views: {
            views: {
                [activeView.id]: activeView,
            },
            current: activeView.id,
        },
        cards: {
            templates: [],
            cards: [card1, card2, card3],
        },
        searchText: {},
        clientConfig: {
            value: {
                enablePublicSharedBoards: true,
                teammateNameDisplay: 'username',
                teamMode: true,
            },
        },
        contents: {
            contents: {},
        },
        comments: {
            comments: {},
        },
        sidebar: {
            categoryAttributes: [
                categoryAttribute1,
            ],
            hiddenBoardIDs: [],
        },
    }
    mockedOctoClient.searchTeamUsers.mockResolvedValue(Object.values(state.users.boardUsers))
    const store = mockAppStore(state, {client: mockedOctoClient as any})
    beforeAll(() => {
        mockMatchMedia({matches: true})
    })
    beforeEach(() => {
        vi.clearAllMocks()
        mockedUtils.createGuid.mockReturnValue('test-id')

        // Navigation builds a real path even under the Utils auto-mock:
        // navigate(undefined) would crash inside the router instead.
        mockedUtils.generatePath = actualUtils.generatePath
    })
    test('should match snapshot', async () => {
        const result = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Workspace readonly={false}/>
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        const container = result.container
        vi.runOnlyPendingTimers()
        expect(container).toMatchSnapshot()
    })
    test('should match snapshot with readonly', async () => {
        const result = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Workspace readonly={true}/>
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        const container = result.container
        vi.runOnlyPendingTimers()
        expect(container).toMatchSnapshot()
    })

    test('return workspace and showcard', async () => {
        const result = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Workspace readonly={false}/>
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        const container = result.container
        vi.runOnlyPendingTimers()

        // React 19 commits when the act callback returns, so the cards only
        // exist to be clicked from a second act.
        const cardElements = container!.querySelectorAll('.KanbanCard')
        expect(cardElements).toBeDefined()
        userEvent.click(cardElements[0])
        expect(container).toMatchSnapshot()
    })

    test('return workspace readonly and showcard', async () => {
        const result = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Workspace readonly={true}/>
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        const container = result.container
        vi.runOnlyPendingTimers()

        // React 19 commits when the act callback returns, so the cards only
        // exist to be clicked from a second act.
        const cardElements = container!.querySelectorAll('.KanbanCard')
        expect(cardElements).toBeDefined()
        userEvent.click(cardElements[0])
        expect(container).toMatchSnapshot()
        expect(mockedUtils.getReadToken).toHaveBeenCalledTimes(1)
    })

    test('return workspace with BoardTemplateSelector component', async () => {
        const emptyStore = mockAppStore({
            users: {
                me,
                boardUsers: {[me.id]: me},
            },
            teams: {
                current: {id: 'team-id', title: 'Test Team'},
            },
            boards: {
                current: board.id,
                boards: {
                    [board.id]: board,
                },
                templates: [],
                myBoardMemberships: {
                    [board.id]: {userId: 'user_id_1', schemeAdmin: true},
                },
            },
            views: {
                views: {},
            },
            cards: {
                cards: [],
            },
            globalTemplates: {
                value: [],
            },
            searchText: {},
            clientConfig: {
                value: {
                    enablePublicSharedBoards: true,
                    teammateNameDisplay: 'username',
                    teamMode: true,
                },
            },
            sidebar: {
                categoryAttributes: [
                    categoryAttribute1,
                ],
                hiddenBoardIDs: [],
            },
        })
        const result = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={emptyStore}>
                <Workspace readonly={true}/>
            </AppStoreProvider>,
        ), {wrapper: TestRouter})
        const container = result.container
        vi.runOnlyPendingTimers()

        expect(container).toMatchSnapshot()
    })

    // A board somebody opens by its address may not be in the store yet — one
    // they are about to join is not — and "Create a board" drawn while it loads
    // tells them the board they asked for does not exist.
    test('does not offer to create a board while the one opened is loading', async () => {
        const loadingStore = mockAppStore({
            users: {me, boardUsers: {[me.id]: me}},
            teams: {current: {id: 'team-id', title: 'Test Team'}},
            boards: {current: 'not-here-yet', boards: {}, templates: [], myBoardMemberships: {}},
            views: {views: {}},
            cards: {cards: []},
            globalTemplates: {value: []},
            searchText: {},
            clientConfig: {value: {enablePublicSharedBoards: true, teammateNameDisplay: 'username', teamMode: true}},
            sidebar: {categoryAttributes: [categoryAttribute1], hiddenBoardIDs: []},
        })
        const [loading, setLoading] = createSignal(true)
        const result = render(() => wrapDNDIntl(() => (
            <AppStoreProvider store={loadingStore}>
                <Workspace
                    readonly={true}
                    boardLoading={loading}
                />
            </AppStoreProvider>
        )), {wrapper: TestRouter})
        vi.runOnlyPendingTimers()
        expect(result.container.textContent).not.toContain('Create a board')

        setLoading(false)
        flush()
        vi.runOnlyPendingTimers()
        await waitFor(() => expect(result.container.textContent).toContain('Create a board'))
    })
})
