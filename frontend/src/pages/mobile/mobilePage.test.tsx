import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {TestRouter, wrapIntl} from '../../testUtils'

import {fakeGoSide} from '../../test/goSide'

import MobilePage from './mobilePage'

// The agent event socket, kept where a test can make an agent stop and wait.
// vi.mock is hoisted above the imports, so what it captures has to be too.
const {handlers} = vi.hoisted(() => ({handlers: {} as Record<string, (payload?: any) => void>}))

vi.mock('../../components/acp/agentEvents', () => ({
    onAgentEvent: (event: string, handler: (payload?: any) => void) => {
        handlers[event] = handler
        return () => delete handlers[event]
    },
}))

const askedOnCard = {
    key: 'q:q-1',
    cardId: 'card-2',
    title: 'Выкатить релиз',
    agent: 'clauuus',
    reason: 'question',
    questionId: 'q-1',
    text: 'Разрешить Bash: git push?',
    options: [{id: 'allow', label: 'Разрешить'}],
    freeText: false,
    awaiting: true,
    since: '2026-08-06T11:00:00Z',
}

const waitingOnCard = {
    key: 'q:q-2',
    questionId: 'q-2',
    cardId: 'card-1',
    title: 'Починить логин',
    agent: 'clauuus',
    reason: 'question',
    text: 'Переписать форму целиком?',
    options: [{id: 'yes', label: 'Да'}],
    awaiting: true,
    since: '2026-08-06T10:00:00Z',
}

const letter = {
    id: 'card-9',
    boardId: 'board-home',
    title: 'Доставка приедет завтра',
    column: 'Входящие',
    author: 'почта',
    properties: {Ссылка: 'https://example.com/1'},
}

const work = {
    id: 'board-work',
    title: 'Разработка',
    property: 'Статус',
    columns: [{value: 'Входящие', color: 'propColorGray'}, {value: 'В работе', color: 'propColorYellow'}],
}

function bindings(waiting: any[] = [], inbox: any[] = []) {
    return {
        ListAttention: vi.fn().mockResolvedValue(JSON.stringify(waiting)),
        AnswerQuestion: vi.fn().mockResolvedValue(undefined),
        ListInbox: vi.fn().mockResolvedValue(JSON.stringify(inbox)),
        ListBoards: vi.fn().mockResolvedValue(JSON.stringify([{id: 'board-home', title: 'Домашние дела'}, work])),
        ListBoardCards: vi.fn().mockResolvedValue(JSON.stringify([
            {id: 'card-3', boardId: 'board-home', title: 'Полить цветы', column: 'К выполнению'},
        ])),
        MoveCardToBoard: vi.fn().mockResolvedValue(undefined),
    }
}

const renderPage = () => render(() => wrapIntl(() => (
    <TestRouter path='/m'>
        <MobilePage/>
    </TestRouter>
)))

// The bar at the bottom is how the four screens are reached; a tab is a button
// and its name carries the count, so it is matched loosely.
const openTab = (name: RegExp) => userEvent.click(screen.getByRole('button', {name}))

describe('pages/mobile/mobilePage', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        Object.keys(handlers).forEach((event) => delete handlers[event])
    })

    afterEach(() => {
        fakeGoSide()
    })

    // A phone is where you find out nothing needs you, most of the time — and
    // that is the screen it opens on, because it is the only one of the three
    // that cannot wait.
    it('opens on what is waiting and says so when nothing is', async () => {
        fakeGoSide(bindings())

        renderPage()

        expect(await screen.findByText('Nothing is waiting. The agents are working.')).toBeInTheDocument()
    })

    // Which card is waiting, what is being asked, and the answers with it: the
    // agent holds its turn open until somebody answers, and a phone is where a
    // person is when they are not at the desk.
    it('says which card is waiting and answers it', async () => {
        const app = bindings([waitingOnCard])
        fakeGoSide(app)

        renderPage()

        expect(await screen.findByText('Починить логин')).toBeInTheDocument()
        expect(screen.getByText('Переписать форму целиком?')).toBeInTheDocument()

        await userEvent.click(await screen.findByRole('button', {name: 'Да'}))
        await waitFor(() => expect(app.AnswerQuestion).toHaveBeenCalledWith('q-2', 'yes', ''))
    })

    // The phone app puts one tab per machine around this page, and the number
    // on a tab is the only thing it can learn about the frame behind it: a
    // person has to see which desktop is asking without opening its tab.
    it('tells the app around it how many things are waiting', async () => {
        fakeGoSide(bindings([askedOnCard, {...askedOnCard, key: 'q:q-2', questionId: 'q-2', cardId: 'card-2'}]))

        const parent = {postMessage: vi.fn()}
        const original = Object.getOwnPropertyDescriptor(window, 'parent')
        Object.defineProperty(window, 'parent', {value: parent, configurable: true})

        try {
            renderPage()

            await waitFor(() => expect(parent.postMessage).toHaveBeenCalledWith({type: 'xciii:waiting', count: 2}, '*'))
        } finally {
            if (original) {
                Object.defineProperty(window, 'parent', original)
            }
        }
    })

    // The count on a tab has to be right before anybody opens it: a tab that
    // only counts once you are looking at it counts nothing.
    it('counts the inbox on its tab without the tab being open', async () => {
        fakeGoSide(bindings([], [letter]))

        renderPage()

        await waitFor(() => expect(screen.getByRole('button', {name: /Inbox/})).toHaveTextContent('1'))
        expect(screen.queryByText('Доставка приедет завтра')).not.toBeInTheDocument()
    })

    // What the inbox is for: a letter arrived, and the person says which board
    // it belongs on and where on it.
    it('moves a card from the inbox onto a board and into a column', async () => {
        const app = bindings([], [letter])
        fakeGoSide(app)

        renderPage()

        await openTab(/Inbox/)
        expect(await screen.findByText('Доставка приедет завтра')).toBeInTheDocument()
        expect(screen.getByText('почта')).toBeInTheDocument()

        await userEvent.click(screen.getByText('Move to a board…'))
        await userEvent.click(await screen.findByText('Разработка'))
        await userEvent.click(await screen.findByText('В работе'))

        await waitFor(() => expect(app.MoveCardToBoard).toHaveBeenCalledWith('card-9', 'board-work', 'В работе'))
    })

    // The cards tab is one board at a time, and it opens on the first one so
    // there is something to read the moment it is reached.
    it('lists one board cards on the cards tab', async () => {
        const app = bindings()
        fakeGoSide(app)

        renderPage()

        await openTab(/Cards/)

        expect(await screen.findByText('Полить цветы')).toBeInTheDocument()
        expect(screen.getByText('К выполнению')).toBeInTheDocument()
        await waitFor(() => expect(app.ListBoardCards).toHaveBeenCalledWith('board-home'))
    })
})
