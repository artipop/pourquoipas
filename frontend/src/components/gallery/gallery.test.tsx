import {render, screen, fireEvent, waitFor} from '@solidjs/testing-library'

import userEvent from '@testing-library/user-event'

import {blocksById, mockAppStore, wrapDNDIntl, hoverOver} from '../../testUtils'
import {drag} from '../../test/drag'
import {AppStoreProvider} from '../../store'

import {TestBlockFactory} from '../../test/testBlockFactory'

import mutator from '../../mutator'

import Gallery from './gallery'

vi.mock('../../mutator')
const mockedMutator = vi.mocked(mutator)

describe('src/components/gallery/Gallery', () => {
    const board = TestBlockFactory.createBoard()
    const activeView = TestBlockFactory.createBoardView(board)
    activeView.fields.sortOptions = []
    const card = TestBlockFactory.createCard(board)
    const card2 = TestBlockFactory.createCard(board)
    const contents = [TestBlockFactory.createDivider(card), TestBlockFactory.createDivider(card), TestBlockFactory.createDivider(card2)]
    const state = {

        // The card menu drawn on a gallery card offers "Copy link", which a
        // team install has (docs/teamwork.md).
        clientConfig: {
            value: {teamMode: true},
        },
        contents: {
            contents: blocksById(contents),
            contentsByCard: {
                [card.id]: [contents[0], contents[1]],
                [card2.id]: [contents[2]],
            },
        },
        cards: {
            current: '',
            cards: {
                [card.id]: card,
            },
            templates: {},
        },
        teams: {
            current: {id: 'team-id'},
        },
        boards: {
            current: board.id,
            boards: {
                [board.id]: board,
            },
            myBoardMemberships: {
                [board.id]: {userId: 'user_id_1', schemeAdmin: true},
            },
        },
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
    beforeEach(() => {
        vi.clearAllMocks()
    })
    test('should match snapshot', async () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Gallery
                    board={board}
                    cards={[card, card2]}
                    activeView={activeView}
                    readonly={false}
                    addCard={vi.fn()}
                    selectedCardIds={[card.id]}
                    onCardClicked={vi.fn()}
                />
            </AppStoreProvider>,
        ))
        await hoverOver(document.querySelector('.GalleryCard')!)
        const buttonElement = screen.getAllByRole('button', {name: 'menuwrapper'})[0]
        userEvent.click(buttonElement)
        expect(container).toMatchSnapshot()
    })
    test('should match snapshot without permissions', async () => {
        const localStore = mockAppStore({...state, teams: {current: undefined}})
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={localStore}>
                <Gallery
                    board={board}
                    cards={[card, card2]}
                    activeView={activeView}
                    readonly={false}
                    addCard={vi.fn()}
                    selectedCardIds={[card.id]}
                    onCardClicked={vi.fn()}
                />
            </AppStoreProvider>,
        ))
        await hoverOver(document.querySelector('.GalleryCard')!)
        const buttonElement = screen.getAllByRole('button', {name: 'menuwrapper'})[0]
        userEvent.click(buttonElement)
        expect(container).toMatchSnapshot()
    })
    test('return Gallery and click new', () => {
        const mockAddCard = vi.fn()
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Gallery
                    board={board}
                    cards={[card, card2]}
                    activeView={activeView}
                    readonly={false}
                    addCard={mockAddCard}
                    selectedCardIds={[card.id]}
                    onCardClicked={vi.fn()}
                />
            </AppStoreProvider>,
        ))
        expect(container).toMatchSnapshot()

        const elementNew = container.querySelector('.octo-gallery-new')!
        expect(elementNew).toBeDefined()
        userEvent.click(elementNew)
        expect(mockAddCard).toHaveBeenCalledTimes(1)
    })

    test('return Gallery readonly', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Gallery
                    board={board}
                    cards={[card, card2]}
                    activeView={activeView}
                    readonly={true}
                    addCard={vi.fn()}
                    selectedCardIds={[card.id]}
                    onCardClicked={vi.fn()}
                />
            </AppStoreProvider>,
        ))
        expect(container).toMatchSnapshot()
    })

    test('return Gallery and drag and drop card', async () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Gallery
                    board={board}
                    cards={[card, card2]}
                    activeView={activeView}
                    readonly={false}
                    addCard={vi.fn()}
                    selectedCardIds={[]}
                    onCardClicked={vi.fn()}
                />
            </AppStoreProvider>,
        ))
        const allGalleryCard = container.querySelectorAll('.GalleryCard')
        await drag(allGalleryCard[0], allGalleryCard[1])
        await waitFor(() => expect(mockedMutator.performAsUndoGroup).toHaveBeenCalledTimes(1))
    })
})
