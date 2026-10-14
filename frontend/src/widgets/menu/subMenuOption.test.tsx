import {render, screen, fireEvent} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'

import Menu from './menu'

// Stood 100px wide at a chosen distance from the left edge of the window.
const autoSubmenu = (left: number) => () => (
    <div style={{position: 'absolute', top: '100px', left: `${left}px`, width: '100px'}}>
        <Menu>
            <Menu.SubMenu
                id='export'
                name='Export'
                position='auto'
            >
                <Menu.Text
                    id='csv'
                    name='To CSV'
                    onClick={() => undefined}
                />
            </Menu.SubMenu>
        </Menu>
    </div>
)

const submenu = () => (
    <Menu>
        <Menu.SubMenu
            id='theme'
            name='Theme'
        >
            <Menu.Text
                id='dark'
                name='Dark theme'
                onClick={() => undefined}
            />
        </Menu.SubMenu>
        <Menu.Text
            id='other'
            name='Something else'
            onClick={() => undefined}
        />
    </Menu>
)

describe('widgets/menu/subMenuOption', () => {
    // The way a menu is used: the pointer runs down it and each submenu opens
    // as it is passed. This was broken for a while and read as the menu being
    // dead — a submenu that only answers a click looks like one that is stuck.
    it('opens when the pointer arrives and closes when it leaves', () => {
        render(() => wrapIntl(submenu))

        const option = document.getElementById('theme') as HTMLElement
        expect(screen.queryByText('Dark theme')).not.toBeInTheDocument()

        fireEvent.mouseEnter(option)
        expect(screen.getByText('Dark theme')).toBeInTheDocument()

        fireEvent.mouseLeave(option)
        expect(screen.queryByText('Dark theme')).not.toBeInTheDocument()
    })

    // A phone has no pointer to hover with, and this menu is on the phone too.
    // A click opens rather than toggles: with a pointer, hovering has already
    // opened it, and clicking the thing you are pointing at must not shut it.
    it('also opens on a click, and a second one leaves it open', () => {
        render(() => wrapIntl(submenu))

        const option = document.getElementById('theme') as HTMLElement
        fireEvent.click(option)
        expect(screen.getByText('Dark theme')).toBeInTheDocument()

        fireEvent.click(option)
        expect(screen.getByText('Dark theme')).toBeInTheDocument()
    })

    // The ⋯ menu of the board's header stands at the right edge of the
    // window, and a submenu that only ever hangs right opens clipped there.
    // 'auto' hangs it from whichever side of its option has more room —
    // measured when it opens, because at mount the menu has not been placed
    // yet and the rect describes a corner the option never stands in.
    it('auto hangs the submenu left when the right edge is closer', () => {
        render(() => wrapIntl(autoSubmenu(window.innerWidth - 120)))

        const option = document.getElementById('export') as HTMLElement
        fireEvent.mouseEnter(option)

        const sub = option.querySelector('.SubMenu') as HTMLElement
        expect(sub.style.right).toBe('100%')
        expect(sub.style.left).toBe('auto')
    })

    it('auto keeps the submenu on the right where the room is', () => {
        render(() => wrapIntl(autoSubmenu(20)))

        const option = document.getElementById('export') as HTMLElement
        fireEvent.mouseEnter(option)

        const sub = option.querySelector('.SubMenu') as HTMLElement
        expect(sub.style.right).toBe('')
        expect(sub.style.left).toBe('')
    })
})
