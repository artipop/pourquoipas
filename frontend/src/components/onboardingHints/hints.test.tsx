import {createSignal} from 'solid-js'
import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {Card} from '../../blocks/card'
import octoClient from '../../octoClient'
import {AppStoreProvider} from '../../store'
import {TestBlockFactory} from '../../test/testBlockFactory'
import {mockAppStore, wrapIntl} from '../../testUtils'

import FirstRunStrip from './firstRunStrip'
import HintTip from './hintTip'
import {HINT_FIRST_RUN, cardMovedFrom} from './hints'

vi.mock('../../octoClient')
const mockedOctoClient = vi.mocked(octoClient)

// The agent event socket, kept where a test can make a session start.
const {handlers} = vi.hoisted(() => ({handlers: {} as Record<string, (payload?: any) => void>}))
vi.mock('../acp/agentEvents', () => ({
    onAgentEvent: (event: string, handler: (payload?: any) => void) => {
        handlers[event] = handler
        return () => delete handlers[event]
    },
}))

const storeWith = (shownHints: string[] = [], teamMode = false) => mockAppStore({
    users: {
        me: {id: 'user_id_1'},
        myConfig: {shownHints: {value: JSON.stringify(shownHints)}},
    },
    clientConfig: {value: {teamMode}},
})

// The server answers a patch with every preference it now holds.
const answerWith = (shownHints: string[]) => mockedOctoClient.patchUserConfig.mockResolvedValue([
    {user_id: 'user_id_1', category: 'focalboard', name: 'shownHints', value: JSON.stringify(shownHints)},
])

describe('components/onboardingHints', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        Object.keys(handlers).forEach((event) => delete handlers[event])
    })

    // A hint is shown once: «Got it» puts its id in the set, and a hint in the
    // set is never drawn again.
    describe('a hint', () => {
        const renderHint = (store: ReturnType<typeof storeWith>, wanted = true) => render(() => wrapIntl(() =>
            <AppStoreProvider store={store}>
                <HintTip
                    id='question'
                    wanted={wanted}
                    title='The agent is asking'
                    body='Open the card.'
                />
            </AppStoreProvider>,
        ))

        test('is drawn at its moment, and put away for good', async () => {
            answerWith(['inbox', 'question'])
            renderHint(storeWith(['inbox']))

            expect(screen.getByText('The agent is asking')).toBeInTheDocument()
            userEvent.click(await screen.findByRole('button', {name: 'Got it'}))

            await waitFor(() => expect(mockedOctoClient.patchUserConfig).toHaveBeenCalledWith('user_id_1', {
                updatedFields: {shownHints: JSON.stringify(['inbox', 'question'])},
            }))
            await waitFor(() => expect(screen.queryByText('The agent is asking')).toBeNull())
        })

        test('already shown is not drawn', () => {
            renderHint(storeWith(['question']))
            expect(screen.queryByText('The agent is asking')).toBeNull()
        })

        test('is not drawn before its moment', () => {
            renderHint(storeWith(), false)
            expect(screen.queryByText('The agent is asking')).toBeNull()
        })

        // Every card with a question on it wants the same hint; it is drawn
        // on one of them, and hints never stack.
        test('is drawn once however many places want it', () => {
            const store = storeWith()
            render(() => wrapIntl(() =>
                <AppStoreProvider store={store}>
                    <HintTip
                        id='question'
                        wanted={true}
                        title='The agent is asking'
                        body='Open the card.'
                    />
                    <HintTip
                        id='question'
                        wanted={true}
                        title='The agent is asking'
                        body='Open the card.'
                    />
                    <HintTip
                        id='inbox'
                        wanted={true}
                        title='A card from outside'
                        body='Move it on.'
                    />
                </AppStoreProvider>,
            ))

            expect(screen.getAllByText(/The agent is asking|A card from outside/)).toHaveLength(1)
        })
    })

    // A card moved to another column is drawn anew by that column; what it
    // was before is kept outside the card.
    test('a card drawn in another column is known to have moved', () => {
        expect(cardMovedFrom('card-moved', 'todo')).toBeUndefined()
        expect(cardMovedFrom('card-moved', 'todo')).toBeUndefined()
        expect(cardMovedFrom('card-moved', 'done')).toBe('todo')
    })

    describe('the strip over the first run', () => {
        const board = TestBlockFactory.createBoard()
        const cardIn = (column: string): Card => {
            const card = TestBlockFactory.createCard(board)
            card.id = 'card-1'
            card.title = 'Fix the build'
            card.fields.properties = {status: column}
            return card
        }

        const renderStrip = (store: ReturnType<typeof storeWith>) => {
            const [cards, setCards] = createSignal<Card[]>([cardIn('doing')])
            render(() => wrapIntl(() =>
                <AppStoreProvider store={store}>
                    <FirstRunStrip
                        cards={cards()}
                        columnPropertyId='status'
                    />
                </AppStoreProvider>,
            ))
            return setCards
        }

        test('says where the answer will be while the first card runs, and goes when the card moves on', async () => {
            answerWith([HINT_FIRST_RUN])
            const setCards = renderStrip(storeWith())

            expect(screen.queryByRole('status')).toBeNull()
            handlers['acp:session']({sessionId: 's1', cardId: 'card-1', status: 'running'})

            await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('The agent is working on “Fix the build”.'))
            expect(screen.getByRole('status')).toHaveTextContent('under “Changes”')

            // The agent moves the card on when it is done.
            setCards([cardIn('done')])

            await waitFor(() => expect(mockedOctoClient.patchUserConfig).toHaveBeenCalledWith('user_id_1', {
                updatedFields: {shownHints: JSON.stringify([HINT_FIRST_RUN])},
            }))
            await waitFor(() => expect(screen.queryByRole('status')).toBeNull())
        })

        // The agent's result is a comment in every install; a desktop adds the
        // changes in the folder.
        test('says the result arrives as a comment', async () => {
            renderStrip(storeWith())
            handlers['acp:session']({sessionId: 's1', cardId: 'card-1', status: 'running'})

            await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('arrives as a comment'))
        })

        test('never comes back once it has been shown', () => {
            renderStrip(storeWith([HINT_FIRST_RUN]))
            handlers['acp:session']({sessionId: 's1', cardId: 'card-1', status: 'running'})

            expect(screen.queryByRole('status')).toBeNull()
        })
    })
})
