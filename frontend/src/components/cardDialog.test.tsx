import '@testing-library/jest-dom'
import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'

import mutator from '../mutator'
import {IUser} from '../user'
import {Utils} from '../utils'
import octoClient from '../octoClient'
import {TestBlockFactory} from '../test/testBlockFactory'
import {mockAppStore, wrapDNDIntl} from '../testUtils'
import {AppStoreProvider} from '../store'

import {fakeGoSide} from '../test/goSide'

import CardDialog from './cardDialog'

vi.mock('../mutator')
vi.mock('../octoClient')
vi.mock('../utils')

// A Go side a test installed outlives it, and a test left holding one decides for
// every test after it whether this app has an agent integration at all.
afterEach(() => {
    fakeGoSide()
})

const mockedUtils = vi.mocked(Utils)
const mockedMutator = vi.mocked(mutator)
const mockedOctoClient = vi.mocked(octoClient)
mockedUtils.createGuid.mockReturnValue('test-id')

describe('components/cardDialog', () => {
    const board = TestBlockFactory.createBoard()
    board.cardProperties = []
    board.id = 'test-id'
    board.teamId = 'team-id'
    const boardView = TestBlockFactory.createBoardView(board)
    boardView.id = board.id
    const card = TestBlockFactory.createCard(board)
    card.id = board.id
    card.createdBy = 'user-id-1'

    const state = {

        // One person by default, which is what an install is until somebody
        // turns team mode on: the tests about what a team offers say so
        // themselves (docs/teamwork.md).
        clientConfig: {
            value: {},
        },
        comments: {
            comments: {},
            commentsByCard: {},
        },
        contents: {
            contents: {},
            contentsByCard: {},
        },
        cards: {
            cards: {
                [card.id]: card,
            },
            current: card.id,
        },
        teams: {
            current: {id: 'team-id'},
        },
        boards: {
            boards: {
                [board.id]: board,
            },
            current: board.id,
            myBoardMemberships: {
                [board.id]: {userId: 'user_id_1', schemeAdmin: true},
            },
        },
        users: {
            boardUsers: {
                1: {username: 'abc'},
                2: {username: 'd'},
                3: {username: 'e'},
                4: {username: 'f'},
                5: {username: 'g'},
            },
            blockSubscriptions: [],
        },
    }

    mockedOctoClient.searchTeamUsers.mockResolvedValue(Object.values(state.users.boardUsers) as IUser[])
    const store = mockAppStore(state)
    beforeEach(() => {
        vi.clearAllMocks()
    })

    // What a person who may edit the card is offered and one who may not is
    // not. Asked of the page rather than snapshotted: the title box is sized
    // to its text, and that height is the fonts of the machine.
    const expectCardEditable = (editable: boolean) => {
        const title = document.querySelector('textarea.EditableArea.title') as HTMLTextAreaElement
        expect(title.readOnly).toBe(!editable)
        for (const offer of ['Add a property', 'Add content']) {
            expect(Boolean(screen.queryByText(offer))).toBe(editable)
        }
    }
    const cardMenu = () => document.querySelector('.toolbar--right .OptionsIcon')

    test('should match snapshot', async () => {
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        expectCardEditable(true)
        expect(screen.getByText('Attach')).toBeInTheDocument()
        expect(cardMenu()).not.toBeNull()
    })
    test('should match snapshot without permissions', async () => {
        const localStore = mockAppStore({...state, teams: {current: undefined}})
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={localStore}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        expectCardEditable(false)
        expect(screen.queryByText('Attach')).toBeNull()
        expect(cardMenu()).not.toBeNull()
    })
    test('return a cardDialog readonly', async () => {
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={true}
                />
            </AppStoreProvider>,
        ))
        expectCardEditable(false)
        expect(cardMenu()).toBeNull()
    })
    test('return cardDialog and do a close action', async () => {
        const closeFn = vi.fn()
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={closeFn}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        const buttonElement = screen.getByRole('button', {name: 'Close dialog'})
        userEvent.click(buttonElement)
        expect(closeFn).toHaveBeenCalledTimes(1)
    })
    test('return cardDialog menu content', async () => {
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        const buttonMenu = screen.getAllByRole('button', {name: 'menuwrapper'})[0]
        userEvent.click(buttonMenu)
        for (const action of ['Delete', 'Move to a board…', 'New template from card']) {
            expect(screen.getByRole('button', {name: action})).toBeInTheDocument()
        }
    })
    test('return cardDialog menu content and verify delete action', async () => {
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        const buttonMenu = screen.getAllByRole('button', {name: 'menuwrapper'})[0]
        userEvent.click(buttonMenu)
        const buttonDelete = screen.getByRole('button', {name: 'Delete'})
        userEvent.click(buttonDelete)

        const confirmDialog = screen.getByTitle('Confirmation dialog')
        expect(confirmDialog).toBeDefined()

        const confirmButton = screen.getByTitle('Delete')
        expect(confirmButton).toBeDefined()

        //click delete button
        userEvent.click(confirmButton!)

        // should be called once on confirming delete
        expect(mockedMutator.deleteBlock).toHaveBeenCalledTimes(1)
    })

    test('return cardDialog menu content and cancel delete confirmation do nothing', async () => {
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))

        const buttonMenu = screen.getAllByRole('button', {name: 'menuwrapper'})[0]
        userEvent.click(buttonMenu)
        const buttonDelete = screen.getByRole('button', {name: 'Delete'})
        userEvent.click(buttonDelete)

        const confirmDialog = screen.getByTitle('Confirmation dialog')
        expect(confirmDialog).toBeDefined()

        const cancelButton = screen.getByTitle('Cancel')
        expect(cancelButton).toBeDefined()

        //click delete button
        userEvent.click(cancelButton!)

        // should do nothing  on cancel delete dialog
        expect(screen.queryByTitle('Confirmation dialog')).toBeNull()
        expect(mockedMutator.deleteBlock).not.toHaveBeenCalled()
        expectCardEditable(true)
    })

    test('return cardDialog menu content and do a New template from card', async () => {
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        const buttonMenu = screen.getAllByRole('button', {name: 'menuwrapper'})[0]
        userEvent.click(buttonMenu)
        const buttonTemplate = screen.getByRole('button', {name: 'New template from card'})
        userEvent.click(buttonTemplate)
        expect(mockedMutator.duplicateCard).toHaveBeenCalledTimes(1)
    })

    test('return cardDialog menu content and do a copy Link', async () => {
        // A link to a card is worth copying where somebody else can open it,
        // so this control belongs to a team install (docs/teamwork.md).
        const team = mockAppStore({...state, clientConfig: {value: {teamMode: true}}})
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={team}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        const buttonMenu = screen.getAllByRole('button', {name: 'menuwrapper'})[0]
        userEvent.click(buttonMenu)
        const buttonCopy = screen.getByRole('button', {name: 'Copy link'})
        userEvent.click(buttonCopy)
        expect(mockedUtils.copyTextToClipboard).toHaveBeenCalledTimes(1)
    })

    test('already following card', async () => {
        // simply doing {...state} gives a TypeScript error
        // when you try updating it's values.
        const newState = JSON.parse(JSON.stringify(state))
        newState.users.blockSubscriptions = [{blockId: card.id}]
        newState.clientConfig = {
            value: {},
        }

        const newStore = mockAppStore(newState)

        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={newStore}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        expectCardEditable(true)
    })

    // The changes are worth a button only when there are some. On «На ревью»
    // the card's working copy has already been folded away, so the count comes
    // from what the Go side recorded as the work moved — never from a look.
    it('offers the changes only when the card has some', async () => {
        const bindings = {
            GetCardAgent: vi.fn().mockResolvedValue('{}'),
            GetCardDiff: vi.fn(),
            ListAgentAccounts: vi.fn().mockResolvedValue('[]'),
        }
        fakeGoSide(bindings)

        const {unmount} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        await waitFor(() => expect(bindings.GetCardAgent).toHaveBeenCalled())
        expect(screen.queryByRole('button', {name: 'Changes'})).toBeNull()
        unmount()

        bindings.GetCardAgent = vi.fn().mockResolvedValue(JSON.stringify({
            changes: {branch: 'fix-login', files: 2, added: 9, removed: 1, dirty: 0},
        }))
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        expect(await screen.findByRole('button', {name: 'Changes'})).toBeInTheDocument()
    })

    // The comments are offered in every install, a desktop included: an
    // agent's result is a comment, and it used to be written and never shown.
    it('offers the comments while one person works the board', async () => {
        render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))
        expect(screen.getByRole('button', {name: 'Comments'})).toBeInTheDocument()
    })

    // The comments share the panel beside the card rather than getting one of their
    // own: they are the same question asked of the other reader.
    it('draws the comments in the panel beside the card', async () => {
        const comment = TestBlockFactory.createComment(card)
        comment.title = 'проверь ветку'
        const team = mockAppStore({
            ...state,
            clientConfig: {value: {teamMode: true, featureFlags: {}}},
            comments: {comments: {[comment.id]: comment}, commentsByCard: {[card.id]: [comment]}},
        })

        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={team}>
                <CardDialog
                    board={board}
                    activeView={boardView}
                    views={[boardView]}
                    cards={[card]}
                    cardId={card.id}
                    onClose={vi.fn()}
                    showCard={vi.fn()}
                    readonly={false}
                />
            </AppStoreProvider>,
        ))

        await userEvent.click(await screen.findByRole('button', {name: 'Comments'}))
        expect(container.querySelector('.cardDialog__side')).not.toBeNull()

        // One row for the one comment. What it says is markdown rendered
        // through Utils, which this file mocks wholesale, so the row itself is
        // what there is to see here.
        expect(container.querySelectorAll('.CommentsList .comment-text').length).toBe(1)

        await userEvent.click(screen.getByRole('button', {name: 'Close the panel'}))
        expect(container.querySelector('.cardDialog__side')).toBeNull()
    })
})
