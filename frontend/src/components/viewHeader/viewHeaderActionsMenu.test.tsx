import {render, screen, waitFor} from '@solidjs/testing-library'

import '@testing-library/jest-dom'
import userEvent from '@testing-library/user-event'

import {TestBlockFactory} from '../../test/testBlockFactory'

import {mockAppStore, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'

import {Archiver} from '../../archiver'

import {CsvExporter} from '../../csvExporter'

import {fakeGoSide} from '../../test/goSide'
import {setHosted} from '../../buildKind'

import ViewHeaderActionsMenu from './viewHeaderActionsMenu'

vi.mock('../../archiver')
vi.mock('../../csvExporter')
vi.mock('../../mutator')
const mockedArchiver = vi.mocked(Archiver)
const mockedCsvExporter = vi.mocked(CsvExporter)

const board = TestBlockFactory.createBoard()
const activeView = TestBlockFactory.createBoardView(board)
const card = TestBlockFactory.createCard(board)

// The inbox is a board of its own, marked as such by the app, and its menu is
// where the question about sources is asked; every other board's says nothing
// of it.
const inboxBoard = {...TestBlockFactory.createBoard(), id: 'board-inbox', properties: {inbox: true}} as unknown as typeof board
const inboxView = TestBlockFactory.createBoardView(inboxBoard)
inboxView.id = 'view-inbox'
inboxView.title = 'Входящие'
const withAnInbox = {
    boards: {current: board.id},
    views: {views: {[inboxView.id]: inboxView}, current: activeView.id},
}

describe('components/viewHeader/viewHeaderActionsMenu', () => {
    const state = {
        users: {
            me: {
                id: 'user-id-1',
                username: 'username_1',
            },
        },
    }
    const store = mockAppStore(state)
    beforeEach(() => {
        vi.clearAllMocks()
    })

    test('return menu', () => {
        const {container} = render(() =>
            wrapIntl(() =>
                <AppStoreProvider store={store}>
                    <ViewHeaderActionsMenu
                        board={board}
                        activeView={activeView}
                        cards={[card]}
                    />
                </AppStoreProvider>,
            ),
        )
        const buttonElement = screen.getByRole('button', {
            name: 'View header menu',
        })
        userEvent.click(buttonElement)
        expect(container).toMatchSnapshot()
    })

    test('return menu and verify call to csv exporter', () => {
        const {container} = render(() =>
            wrapIntl(() =>
                <AppStoreProvider store={store}>
                    <ViewHeaderActionsMenu
                        board={board}
                        activeView={activeView}
                        cards={[card]}
                    />
                </AppStoreProvider>,
            ),
        )
        const buttonElement = screen.getByRole('button', {name: 'View header menu'})
        userEvent.click(buttonElement)
        expect(container).toMatchSnapshot()
        userEvent.click(screen.getByText('Export'))
        const buttonExportCSV = screen.getByRole('button', {name: 'To CSV'})
        userEvent.click(buttonExportCSV)
        expect(mockedCsvExporter.exportTableCsv).toHaveBeenCalledTimes(1)
    })

    // This menu is about the board and nothing else. Agents are the person's
    // and are connected in Settings → AI; a board neither adds nor lists them.
    describe('the menu holds what is true of this board', () => {
        const openMenu = () => {
            render(() =>
                wrapIntl(() =>
                    <AppStoreProvider store={mockAppStore({...state, ...withAnInbox})}>
                        <ViewHeaderActionsMenu
                            board={board}
                            activeView={activeView}
                            cards={[card]}
                        />
                    </AppStoreProvider>,
                ),
            )
            userEvent.click(screen.getByRole('button', {name: 'View header menu'}))
        }
        beforeEach(() => {
            fakeGoSide({
                ListAgents: vi.fn(),
                ListAgentWorkdirs: vi.fn(),
                ListSources: vi.fn().mockResolvedValue('[]'),
                GetBoardPrompt: vi.fn().mockResolvedValue(''),
            })
        })
        afterEach(() => {
            fakeGoSide()
            setHosted(false)
        })

        test('the folders and what the board tells its agents are', async () => {
            openMenu()
            await waitFor(() => expect(screen.getByRole('button', {name: 'Folders…'})).toBeInTheDocument())
            expect(screen.getByRole('button', {name: 'The board’s system prompt…'})).toBeInTheDocument()
        })

        test('agents, setup and templates are not', () => {
            openMenu()
            for (const gone of ['Agents…', 'Walk the setup again…', 'Save as a template…', 'Sources…']) {
                expect(screen.queryByRole('button', {name: gone})).toBeNull()
            }
        })

        // A server has no machine to keep folders on (docs/modes.md).
        test('a server offers no folders', () => {
            setHosted(true)
            openMenu()
            expect(screen.queryByRole('button', {name: 'Folders…'})).toBeNull()
            expect(screen.getByRole('button', {name: 'The board’s system prompt…'})).toBeInTheDocument()
        })
    })

    // The inbox is the screen about what arrives, so its menu is about that and
    // about nothing else: it runs no routes, and exporting it or saving it as a
    // template is not what an inbox is for.
    describe('on «Входящие» the menu is the sources and nothing else', () => {
        beforeEach(() => {
            fakeGoSide({
                ListSources: vi.fn().mockResolvedValue('[]'),
            })
            render(() =>
                wrapIntl(() =>
                    <AppStoreProvider store={mockAppStore({...state, ...withAnInbox})}>
                        <ViewHeaderActionsMenu
                            board={inboxBoard}
                            activeView={inboxView}
                            cards={[card]}
                        />
                    </AppStoreProvider>,
                ),
            )
            userEvent.click(screen.getByRole('button', {name: 'View header menu'}))
        })
        afterEach(() => {
            fakeGoSide()
        })

        test('the sources are offered here', async () => {
            await waitFor(() => expect(screen.getByRole('button', {name: 'Sources…'})).toBeInTheDocument())
        })

        test('the board menu is not', async () => {
            await waitFor(() => expect(screen.getByRole('button', {name: 'Sources…'})).toBeInTheDocument())
            for (const gone of ['Export', 'Columns and routes…', 'Save as a template…']) {
                expect(screen.queryByRole('button', {name: gone})).toBeNull()
            }
        })
    })

    // A build with no Go side behind it — the plugin, a browser — has no
    // sources to offer, and an inbox menu with nothing in it is a button that
    // opens an empty box.
    test('with nothing to offer, «Входящие» has no menu at all', () => {
        render(() =>
            wrapIntl(() =>
                <AppStoreProvider store={mockAppStore({...state, ...withAnInbox})}>
                    <ViewHeaderActionsMenu
                        board={inboxBoard}
                        activeView={inboxView}
                        cards={[card]}
                    />
                </AppStoreProvider>,
            ),
        )
        expect(screen.queryByRole('button', {name: 'View header menu'})).toBeNull()
    })

    test('return menu and verify call to board archive', () => {
        const {container} = render(() =>
            wrapIntl(() =>
                <AppStoreProvider store={store}>
                    <ViewHeaderActionsMenu
                        board={board}
                        activeView={activeView}
                        cards={[card]}
                    />
                </AppStoreProvider>,
            ),
        )
        const buttonElement = screen.getByRole('button', {name: 'View header menu'})
        userEvent.click(buttonElement)
        expect(container).toMatchSnapshot()
        userEvent.click(screen.getByText('Export'))
        const buttonExportBoardArchive = screen.getByRole('button', {name: 'Board archive'})
        userEvent.click(buttonExportBoardArchive)
        expect(mockedArchiver.exportBoardArchive).toHaveBeenCalledTimes(1)
        expect(mockedArchiver.exportBoardArchive).toHaveBeenCalledWith(board)
    })
})
