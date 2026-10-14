import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'
import {Utils} from '../../utils'

import {fakeGoSide, goSide} from '../../test/goSide'

import BoardSourceSwitcher from './boardSourceSwitcher'

const open = () => render(() => wrapIntl(() => <BoardSourceSwitcher/>))

describe('components/sidebar/boardSourceSwitcher', () => {
    afterEach(() => {
        fakeGoSide()
    })

    // A switcher with one entry is a label pretending to be a control.
    test('is not drawn while there is only this machine', async () => {
        fakeGoSide({
            GetServerAttachment: vi.fn().mockResolvedValue(JSON.stringify({
                selected: '', sources: [{id: '', title: 'This machine'}],
            })),
        })
        const {container} = open()

        await waitFor(() => expect(goSide.GetServerAttachment).toHaveBeenCalled())
        expect(container.querySelector('.BoardSourceSwitcher')).toBeNull()
    })

    // In a browser there is no Go side and no second board to go to.
    test('is not drawn without the bindings', () => {
        const {container} = open()
        expect(container.querySelector('.BoardSourceSwitcher')).toBeNull()
    })

    test('shows which board the window is on and offers the other', async () => {
        fakeGoSide({
            GetServerAttachment: vi.fn().mockResolvedValue(JSON.stringify({
                selected: 'https://board.example.com',

                // The title Go sends for this machine is not what is drawn:
                // the page names it in its own language.
                sources: [{id: '', title: 'from Go'}, {id: 'https://board.example.com', title: 'https://board.example.com'}],
            })),
            SelectBoardSource: vi.fn().mockResolvedValue('{}'),
        })
        const reload = vi.spyOn(Utils, 'reloadPage').mockImplementation(() => {})

        const {container} = open()
        await waitFor(() => expect(container.querySelector('.BoardSourceSwitcher__title')).toBeTruthy())

        // What the window is on now is the server, so that is what the control
        // reads.
        expect(container.querySelector('.BoardSourceSwitcher__title')).toHaveTextContent('https://board.example.com')

        await userEvent.click(container.querySelector('.BoardSourceSwitcher__current') as HTMLElement)
        await userEvent.click(screen.getByText('This machine'))

        expect(goSide.SelectBoardSource).toHaveBeenCalledWith('')

        // The page holds one board's cards, members and sockets, all fetched
        // against the board it loaded from.
        expect(reload).toHaveBeenCalled()
    })
})
