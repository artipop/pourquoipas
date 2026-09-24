import {render, screen, waitFor} from '@solidjs/testing-library'

import {mockAppStore, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'

import octoClient from '../../octoClient'

import CardHistory from './cardHistory'

vi.mock('../../octoClient')
const mockedClient = vi.mocked(octoClient)

describe('components/cardDetail/cardHistory', () => {
    afterEach(() => {
        vi.clearAllMocks()
    })

    const draw = () => {
        const store = mockAppStore({
            users: {boardUsers: {'agent-1': {username: 'клаус'}, 'person-1': {username: 'artem'}}},
        })
        return render(() => (
            <AppStoreProvider store={store}>
                {wrapIntl(() => (
                    <CardHistory
                        cardId='card-1'
                        onClose={vi.fn()}
                    />
                ))}
            </AppStoreProvider>
        ))
    }

    // The question a history is asked is "who", and on a desktop every write
    // used to land under the one account the app runs as. An agent has a board
    // account of its own, so the line about the move names the agent.
    test('says who did it, by the name the board knows them under', async () => {
        mockedClient.getCardHistory.mockResolvedValue([
            {at: 300, userId: 'agent-1', kind: 'property', property: 'Статус', from: 'В работе', to: 'Готово'},
            {at: 100, userId: 'person-1', kind: 'created'},
        ])

        draw()

        await waitFor(() => expect(screen.getByText('клаус')).toBeInTheDocument())
        expect(screen.getByText(/“Статус”: В работе → Готово/)).toBeInTheDocument()
        expect(screen.getByText('artem')).toBeInTheDocument()
    })

    // "Wrote a comment" is not what somebody came to read.
    test('a comment is shown with what it said', async () => {
        mockedClient.getCardHistory.mockResolvedValue([
            {at: 200, userId: 'agent-1', kind: 'comment', text: 'Сделал, проверьте', agent: true},
        ])

        draw()

        await waitFor(() => expect(screen.getByText('Сделал, проверьте')).toBeInTheDocument())
    })

    // A card nobody has touched still has a history — it was made — so an empty
    // panel means the call failed, and saying so is better than an empty box.
    test('a card with no history says so', async () => {
        mockedClient.getCardHistory.mockResolvedValue([])

        draw()

        await waitFor(() => expect(screen.getByText(/Nothing has happened on this card yet/)).toBeInTheDocument())
    })
})
