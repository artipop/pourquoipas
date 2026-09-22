import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {TestRouter, wrapIntl} from '../../testUtils'

import {fakeGoSide} from '../../test/goSide'

import SharePage from './sharePage'

const home = {id: 'board-home', title: 'Домашние дела', icon: '🏠'}
const work = {id: 'board-work', title: 'Разработка', icon: '💻'}
const inbox = {id: 'board-inbox', title: 'Входящие', icon: '📥', inbox: true}

function bindings(result: object = {created: 1}) {
    return {
        // The inbox listed last, as Go's alphabetical order may well put it.
        ListBoards: vi.fn().mockResolvedValue(JSON.stringify([home, work, inbox])),
        ShareItem: vi.fn().mockResolvedValue(JSON.stringify(result)),
        CloseShareWindow: vi.fn().mockResolvedValue(undefined),
    }
}

const renderShare = (query = '?url=https%3A%2F%2Fexample.com%2Fa&title=%D0%A1%D1%82%D0%B0%D1%82%D1%8C%D1%8F') =>
    render(() => wrapIntl(() => (
        <TestRouter path={`/share${query}`}>
            <SharePage/>
        </TestRouter>
    )))

// The button is disabled until the boards are in, so every test that does not
// pick a board waits for the list first — as a person does.
const save = async () => {
    await screen.findByRole('button', {name: /Домашние дела/})
    await userEvent.click(screen.getByRole('button', {name: 'Save'}))
}

describe('pages/share/sharePage', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        localStorage.clear()
    })

    afterEach(() => {
        fakeGoSide()
    })

    // The whole dialog: what another app shared is already filled in, the
    // person names a board, and the link is filed there — and the dialog says
    // the board, not the inbox, since that is not where it went.
    it('files the shared link on the board that was picked', async () => {
        const app = bindings()
        fakeGoSide(app)

        renderShare()

        expect(await screen.findByDisplayValue('Статья')).toBeInTheDocument()
        expect(screen.getByText('https://example.com/a')).toBeInTheDocument()

        await userEvent.click(await screen.findByRole('button', {name: /Разработка/}))
        await userEvent.click(screen.getByRole('button', {name: 'Save'}))

        await waitFor(() => expect(app.ShareItem).toHaveBeenCalledWith(
            'board-work', 'Статья', 'https://example.com/a', ''))
        expect(await screen.findByText('Filed on the board.')).toBeInTheDocument()
    })

    // A link already in the inbox is not a failure and must not read as one:
    // the card is there, which is what the person wanted.
    it('says the link is already there rather than showing an error', async () => {
        fakeGoSide(bindings({skipped: 1}))

        renderShare()

        await save()

        expect(await screen.findByText('It is already in the inbox.')).toBeInTheDocument()
    })

    // Collect first, decide later: the inbox is offered first and picked
    // already, so a link a person has not thought about yet is two taps.
    it('opens on the inbox, offered first', async () => {
        const app = bindings()
        fakeGoSide(app)

        renderShare()

        await screen.findByRole('button', {name: /Домашние дела/})
        const boards = screen.getAllByRole('button', {name: /Входящие|Домашние дела|Разработка/})
        expect(boards[0]).toHaveTextContent('Входящие')
        expect(boards[0]).toHaveAttribute('aria-pressed', 'true')

        await userEvent.click(screen.getByRole('button', {name: 'Save'}))
        await waitFor(() => expect(app.ShareItem).toHaveBeenCalledWith(
            'board-inbox', expect.anything(), expect.anything(), expect.anything()))
        expect(await screen.findByText('Filed in the inbox.')).toBeInTheDocument()
    })

    // Nothing came with a title — a bare link, which is most of what gets
    // shared. The dialog still has to be answerable.
    it('works on a share that is a link and nothing else', async () => {
        const app = bindings()
        fakeGoSide(app)

        renderShare('?url=https%3A%2F%2Fexample.com%2Fb')

        await save()

        await waitFor(() => expect(app.ShareItem).toHaveBeenCalledWith(
            'board-inbox', '', 'https://example.com/b', ''))
    })

    // The pipeline counts a refusal rather than throwing it, so a delivery of
    // one that failed would otherwise look exactly like success.
    it('shows a refusal that came back as a count', async () => {
        fakeGoSide(bindings({failed: 1}))

        renderShare()

        await save()

        expect(await screen.findByText('Could not file it. Try again.')).toBeInTheDocument()
    })
})
