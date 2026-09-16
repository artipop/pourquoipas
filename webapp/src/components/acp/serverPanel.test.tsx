import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'

import {fakeGoSide, goSide} from '../../test/goSide'

import ServerPanel, {isServerAttachAvailable} from './serverPanel'

function bindings(state: Record<string, unknown>) {
    return {
        GetServerAttachment: vi.fn().mockResolvedValue(JSON.stringify(state)),
        AttachServer: vi.fn().mockImplementation((json: string) => {
            const want = JSON.parse(json)
            return Promise.resolve(JSON.stringify({
                url: want.url,
                username: want.username,
                running: '',
            }))
        }),
        DetachServer: vi.fn().mockResolvedValue(JSON.stringify({url: '', username: '', running: ''})),
    }
}

const open = () => render(() => wrapIntl(() => <ServerPanel/>))

describe('components/acp/serverPanel', () => {
    afterEach(() => {
        fakeGoSide()
    })

    // In a browser there is no Go side, so there is nothing to attach and no
    // section to draw.
    test('is not offered where there are no bindings', () => {
        expect(isServerAttachAvailable()).toBe(false)
        fakeGoSide(bindings({url: '', username: '', running: ''}))
        expect(isServerAttachAvailable()).toBe(true)
    })

    test('asks for an address and a login while this desktop works its own board', async () => {
        fakeGoSide(bindings({url: '', username: '', running: ''}))
        open()

        await waitFor(() => expect(goSide.GetServerAttachment).toHaveBeenCalled())
        expect(screen.getByText('Server address')).toBeInTheDocument()
        expect(screen.getByText('Username')).toBeInTheDocument()
    })

    // The board a card lives on is decided when the app starts, so the panel
    // has to say a restart is owed rather than pretend the switch took effect.
    test('says a restart is owed once a server is named', async () => {
        fakeGoSide(bindings({url: '', username: '', running: ''}))
        const {container} = open()
        await waitFor(() => expect(goSide.GetServerAttachment).toHaveBeenCalled())

        await userEvent.type(screen.getByPlaceholderText('board.example.com'), 'board.example.com')
        await userEvent.click(screen.getByRole('button', {name: 'Connect'}))

        await waitFor(() => expect(container.querySelector('.ServerPanel__restart')).toBeTruthy())
        expect(screen.getByText('board.example.com')).toBeInTheDocument()
    })

    test('offers the way back to this machine\'s own board', async () => {
        fakeGoSide(bindings({url: 'https://board.example.com', username: 'anna', running: 'https://board.example.com'}))
        const {container} = open()

        await waitFor(() => expect(screen.getByText('https://board.example.com')).toBeInTheDocument())
        expect(screen.getByText('Signed in as anna')).toBeInTheDocument()

        // Attached and running the same address: nothing is owed.
        expect(container.querySelector('.ServerPanel__restart')).toBeNull()

        await userEvent.click(screen.getByRole('button', {name: 'Work this machine\'s own board'}))
        await waitFor(() => expect(goSide.DetachServer).toHaveBeenCalled())
        expect(screen.getByText('Server address')).toBeInTheDocument()
    })

    // The password buys a session and is not itself worth keeping, so the field
    // must not go on holding it after the call.
    test('does not keep the password after connecting', async () => {
        fakeGoSide(bindings({url: '', username: '', running: ''}))
        const {container} = open()
        await waitFor(() => expect(goSide.GetServerAttachment).toHaveBeenCalled())

        await userEvent.type(screen.getByPlaceholderText('board.example.com'), 'board.example.com')
        const password = container.querySelector('input[type=password]') as HTMLInputElement
        await userEvent.type(password, 'parol123')
        await userEvent.click(screen.getByRole('button', {name: 'Connect'}))

        await waitFor(() => expect(goSide.AttachServer).toHaveBeenCalled())
        const sent = JSON.parse(goSide.AttachServer.mock.calls[0][0])
        expect(sent.password).toBe('parol123')
    })
})
