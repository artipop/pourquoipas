import {fireEvent, render, screen, waitFor} from '@solidjs/testing-library'
import {userEvent as pointer} from 'vitest/browser'

import {hoverOver} from '../testUtils'

import Tooltip from './tooltip'

// Far enough from the top of the page that the bubble has room above.
const draw = (style = 'padding: 200px') => {
    render(() => (
        <div style={style}>
            <Tooltip title='Оценка, часы'>
                <span>{'8'}</span>
            </Tooltip>
        </div>
    ))
    return screen.getByText('8').parentElement as HTMLElement
}

// The bubble used to be a `::after` on the wrapper, so it was drawn inside
// whatever the wrapper stood in — on a kanban card, the card, whose column
// scrollbox was then free to clip it.
describe('a tooltip on the page', () => {
    // Nothing is said until somebody is looking at the thing: a bubble that is
    // always in the DOM is a bubble that is always in the way.
    it('appears while the pointer is on what it names', async () => {
        const host = draw()
        await hoverOver(host)
        expect(await screen.findByRole('tooltip')).toHaveTextContent('Оценка, часы')

        await pointer.unhover(host)
        await waitFor(() => expect(screen.queryByRole('tooltip')).toBeNull())
    })

    it('is placed against the viewport, clear of what it names', async () => {
        const host = draw()
        await hoverOver(host)

        const bubble = await screen.findByRole('tooltip')
        expect(bubble.style.transform).toMatch(/^translate\(-?\d+px, -?\d+px\)$/)

        // Just above the anchor and over it: a bubble left in the corner of
        // the screen is clear of it too.
        const gap = host.getBoundingClientRect().top - bubble.getBoundingClientRect().bottom
        expect(gap).toBeGreaterThanOrEqual(0)
        expect(gap).toBeLessThan(16)
        const middle = host.getBoundingClientRect().left + (host.getBoundingClientRect().width / 2)
        expect(bubble.getBoundingClientRect().left).toBeLessThan(middle)
        expect(bubble.getBoundingClientRect().right).toBeGreaterThan(middle)
    })

    // Guessing the origin would put every tooltip in the corner of the screen.
    it('stays where the stylesheet put it when nothing has been laid out', async () => {
        const host = draw('display: none')
        fireEvent.mouseEnter(host)

        const bubble = screen.getByRole('tooltip', {hidden: true})
        await new Promise(requestAnimationFrame)
        expect(bubble.classList.contains('is-positioned')).toBe(false)
    })
})
