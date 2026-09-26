import {render, waitFor} from '@solidjs/testing-library'

import userEvent from '@testing-library/user-event'

import {FetchMock} from '../../test/fetchMock'
import {TestBlockFactory} from '../../test/testBlockFactory'

import {mockAppStore, mockDOM, wrapDNDIntl, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'

import octoClient from '../../octoClient'

import {createTextBlock} from '../../blocks/textBlock'

import {fakeGoSide} from '../../test/goSide'

import CardDetail from './cardDetail'

global.fetch = FetchMock.fn
vi.mock('../../octoClient')

const mockedOctoClient = vi.mocked(octoClient)

beforeEach(() => {
    FetchMock.fn.mockReset()
})

// A Go side a test installed outlives it, and a card left holding one decides for
// every test after it whether this app has an agent integration at all.
afterEach(() => {
    fakeGoSide()
})

// This is needed to run EasyMDE in tests.
// It needs bounding rectangle box property
// on HTML elements, but Jest's HTML engine jsdom
// doesn't provide it.
// So we mock it.
beforeAll(() => {
    mockDOM()
})

describe('components/cardDetail/CardDetail', () => {
    const board = TestBlockFactory.createBoard()

    const view = TestBlockFactory.createBoardView(board)
    view.fields.sortOptions = []
    view.fields.groupById = undefined
    view.fields.hiddenOptionIds = []

    const card = TestBlockFactory.createCard(board)

    // A rule across the card means a section starts here, and a card with
    // nothing under its properties — no attachments — starts none.
    test('draws no rule for a section it is not drawing', async () => {
        const bindings = {
            GetCardAgent: vi.fn().mockResolvedValue('{}'),
            ListAgents: vi.fn().mockResolvedValue('[]'),
            ListAgentWorkdirs: vi.fn().mockResolvedValue('[]'),
        }
        fakeGoSide(bindings)

        const store = mockAppStore({
            users: {
                boardUsers: {
                    'user-id-1': {username: 'username_1'},
                },
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
            cards: {
                cards: {
                    [card.id]: card,
                },
                current: card.id,
            },
            clientConfig: {
                value: {},
            },
        })

        const {container} = render(() => (
            <AppStoreProvider store={store}>
                {wrapIntl(() =>
                    <CardDetail
                        board={board}
                        activeView={view}
                        views={[view]}
                        cards={[card]}
                        card={card}
                        contents={[]}
                        attachments={[]}
                        readonly={false}
                        onClose={vi.fn()}
                        onDelete={vi.fn()}
                        addAttachment={vi.fn()}
                    />,
                )}
            </AppStoreProvider>
        ))

        await waitFor(() => expect(bindings.GetCardAgent).toHaveBeenCalled())

        // Nothing under the properties has anything to show — no attachments —
        // so the card draws no rule at all. A rule belongs to the section under
        // it, and a card with no such section draws none.
        expect(container.querySelectorAll('hr').length).toBe(0)
    })

    // The card's amber badge says the agent is asking about this card, and
    // this is what it opens onto. The notification says the same thing, but it
    // is dismissible: a question outlives being read, so the card keeps it.
    test('the agent’s question is on the card, with its answers', async () => {
        const AnswerQuestion = vi.fn().mockResolvedValue(undefined)
        fakeGoSide({
            GetCardAgent: vi.fn().mockResolvedValue('{}'),
            ListAgents: vi.fn().mockResolvedValue('[]'),
            ListAgentWorkdirs: vi.fn().mockResolvedValue('[]'),
            AnswerQuestion,
            AckAttention: vi.fn().mockResolvedValue(undefined),
            ListAttention: vi.fn().mockResolvedValue(JSON.stringify([{
                key: 'q:question-1',
                cardId: card.id,
                boardId: board.id,
                reason: 'question',
                agent: 'клаус',
                questionId: 'question-1',
                text: 'Удалить старую миграцию?',
                options: [{id: 'allow', label: 'Разрешить'}, {id: 'deny', label: 'Запретить'}],
                awaiting: true,
            }])),
        })

        const store = mockAppStore({
            teams: {current: {id: 'team-id'}},
            boards: {boards: {[board.id]: board}, current: board.id},
            cards: {cards: {[card.id]: card}, current: card.id},
            clientConfig: {value: {}},
        })

        const {getByText, getByRole} = render(() => (
            <AppStoreProvider store={store}>
                {wrapIntl(() =>
                    <CardDetail
                        board={board}
                        activeView={view}
                        views={[view]}
                        cards={[card]}
                        card={card}
                        contents={[]}
                        attachments={[]}
                        readonly={false}
                        onClose={vi.fn()}
                        onDelete={vi.fn()}
                        addAttachment={vi.fn()}
                    />,
                )}
            </AppStoreProvider>
        ))

        await waitFor(() => expect(getByText('Удалить старую миграцию?')).toBeTruthy())

        userEvent.click(getByRole('button', {name: 'Разрешить'}))
        await waitFor(() => expect(AnswerQuestion).toHaveBeenCalledWith('question-1', 'allow', ''))
    })

    // The card's first tip is about the field that decides whether an agent
    // works the card, on the board the tour runs on — found by its id, whatever
    // it and the card are called.
    test('should show the assignee tour tip', async () => {
        const welcomeBoard = TestBlockFactory.createBoard()
        welcomeBoard.title = 'Any board'
        welcomeBoard.cardProperties = [...welcomeBoard.cardProperties, {id: 'who', name: 'Who is on it', type: 'person', options: []}]

        const welcomeCard = TestBlockFactory.createCard(welcomeBoard)
        welcomeCard.title = 'Any card'

        const store = mockAppStore({
            users: {
                me: {
                    id: 'user_id_1',
                },
                myConfig: {
                    welcomePageViewed: {value: '1'},
                    onboardingTourStarted: {value: true},
                    tourCategory: {value: 'card'},
                    onboardingTourStep: {value: '0'},
                    tourBoard: {value: welcomeBoard.id},
                },
                boardUsers: {
                    'user-id-1': {username: 'username_1'},
                },
            },
            teams: {
                current: {id: 'team-id'},
            },
            boards: {
                boards: {
                    [welcomeBoard.id]: welcomeBoard,
                },
                current: welcomeBoard.id,
                myBoardMemberships: {
                    [welcomeBoard.id]: {userId: 'user_id_1', schemeAdmin: true},
                },
            },
            cards: {
                cards: {
                    [welcomeCard.id]: welcomeCard,
                },
                current: welcomeCard.id,
            },
            clientConfig: {
                value: {},
            },
        })

        const component = () => (
            <AppStoreProvider store={store}>
                {wrapIntl(() =>
                    <CardDetail
                        board={welcomeBoard}
                        activeView={view}
                        views={[view]}
                        cards={[welcomeCard]}
                        card={welcomeCard}
                        contents={[]}
                        attachments={[]}
                        readonly={false}
                        onClose={vi.fn()}
                        onDelete={vi.fn()}
                        addAttachment={vi.fn()}
                    />,
                )}
            </AppStoreProvider>
        )

        let container: Element | DocumentFragment | null = null

        const result = render(component)
        container = result.container

        expect(container).toBeDefined()
        expect(container).not.toBeNull()
        const tourTip = document.querySelectorAll('.AssignAgentTourStep')
        expect(tourTip.length).toBe(2)
        expect(tourTip[1]).toHaveTextContent('Who works on the card')

        // moving to next step
        mockedOctoClient.patchUserConfig.mockResolvedValueOnce([])

        const nextBtn = document!.querySelector('.tipNextButton')
        expect(nextBtn).toBeDefined()
        expect(nextBtn).not.toBeNull()
        userEvent.click(nextBtn!)
        expect(mockedOctoClient.patchUserConfig).toHaveBeenCalledWith(
            'user_id_1',
            {
                updatedFields: {
                    onboardingTourStep: '1',
                },
            },
        )
    })

    test('should show add description tour tip', async () => {
        const welcomeBoard = TestBlockFactory.createBoard()
        welcomeBoard.title = 'Any board'

        const welcomeCard = TestBlockFactory.createCard(welcomeBoard)
        welcomeCard.title = 'Any card'
        const state = {
            users: {
                me: {
                    id: 'user_id_1',
                },
                myConfig: {
                    welcomePageViewed: {value: '1'},
                    onboardingTourStarted: {value: true},
                    tourCategory: {value: 'card'},
                    onboardingTourStep: {value: '2'},
                    tourBoard: {value: welcomeBoard.id},
                },
                boardUsers: {
                    'user-id-1': {username: 'username_1'},
                },
            },
            teams: {
                current: {id: 'team-id'},
            },
            boards: {
                boards: {
                    [welcomeBoard.id]: welcomeBoard,
                },
                current: welcomeBoard.id,
                myBoardMemberships: {
                    [welcomeBoard.id]: {userId: 'user_id_1', schemeAdmin: true},
                },
            },
            cards: {
                cards: {
                    [welcomeCard.id]: welcomeCard,
                },
                current: welcomeCard.id,
            },
            clientConfig: {
                value: {},
            },
        }
        const store = mockAppStore(state)

        const text = createTextBlock()
        text.title = 'description'
        text.parentId = welcomeCard.id
        welcomeCard.fields.contentOrder = [text.id]

        const component = () => (
            <AppStoreProvider store={store}>
                {wrapDNDIntl(() =>
                    <CardDetail
                        board={welcomeBoard}
                        activeView={view}
                        views={[view]}
                        cards={[welcomeCard]}
                        card={welcomeCard}
                        contents={[text]}
                        attachments={[]}
                        readonly={false}
                        onClose={vi.fn()}
                        onDelete={vi.fn()}
                        addAttachment={vi.fn()}
                    />,
                )}
            </AppStoreProvider>
        )

        let container: Element | DocumentFragment | null = null

        const result = render(component)
        container = result.container

        expect(container).toBeDefined()
        expect(container).not.toBeNull()

        const tourTip = document.querySelectorAll('.AddDescriptionTourStep')
        expect(tourTip.length).toBe(2)
        expect(tourTip[1]).toHaveTextContent('Description')

        // On a desktop of one there are no comments, so the description is
        // the card's last tip and «Done» finishes the card's part.

        mockedOctoClient.patchUserConfig.mockResolvedValueOnce([])

        const nextBtn = document!.querySelector('.tipNextButton')
        expect(nextBtn).toBeDefined()
        expect(nextBtn).not.toBeNull()
        userEvent.click(nextBtn!)
        expect(mockedOctoClient.patchUserConfig).toHaveBeenCalledWith(
            'user_id_1',
            {
                updatedFields: {
                    onboardingTourStep: '999',
                },
            },
        )
    })
})
