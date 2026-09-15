import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'

import BoardSourceSwitcher from './boardSourceSwitcher'

const anyWindow = window as any

const open = () => render(() => wrapIntl(() => <BoardSourceSwitcher/>))

describe('components/sidebar/boardSourceSwitcher', () => {
    afterEach(() => {
        delete anyWindow.go
    })

    // A switcher with one entry is a label pretending to be a control.
    test('is not drawn while there is only this machine', async () => {
        anyWindow.go = {main: {App: {
            GetServerAttachment: vi.fn().mockResolvedValue(JSON.stringify({
                selected: '', sources: [{id: '', title: 'Эта машина'}],
            })),
        }}}
        const {container} = open()

        await waitFor(() => expect(anyWindow.go.main.App.GetServerAttachment).toHaveBeenCalled())
        expect(container.querySelector('.BoardSourceSwitcher')).toBeNull()
    })

    // In a browser there is no Go side and no second board to go to.
    test('is not drawn without the bindings', () => {
        const {container} = open()
        expect(container.querySelector('.BoardSourceSwitcher')).toBeNull()
    })

    test('shows which board the window is on and offers the other', async () => {
        anyWindow.go = {main: {App: {
            GetServerAttachment: vi.fn().mockResolvedValue(JSON.stringify({
                selected: 'https://board.example.com',
                sources: [{id: '', title: 'Эта машина'}, {id: 'https://board.example.com', title: 'https://board.example.com'}],
            })),
            SelectBoardSource: vi.fn().mockResolvedValue('{}'),
        }}}
        const reload = vi.fn()
        Object.defineProperty(window, 'location', {value: {reload}, writable: true})

        const {container} = open()
        await waitFor(() => expect(container.querySelector('.BoardSourceSwitcher__title')).toBeTruthy())
        // What the window is on now is the server, so that is what the control
        // reads.
        expect(container.querySelector('.BoardSourceSwitcher__title')).toHaveTextContent('https://board.example.com')

        await userEvent.click(container.querySelector('.BoardSourceSwitcher__current') as HTMLElement)
        await userEvent.click(screen.getByText('Эта машина'))

        expect(anyWindow.go.main.App.SelectBoardSource).toHaveBeenCalledWith('')
        // The page holds one board's cards, members and sockets, all fetched
        // against the board it loaded from.
        expect(reload).toHaveBeenCalled()
    })
})
