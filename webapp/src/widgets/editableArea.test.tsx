import {render, screen} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import EditableArea from './editableArea'

// How many lines a text takes depends on how wide it is, and the box that holds
// it is sized in pixels — so a width that changes without the value changing
// leaves the box the wrong height. That is what opening the panel beside a card
// does: «Поднять зависимости» rewrapped to two lines while the box kept the
// height of one, and the title read as «Поднять» with the rest clipped.

type Observed = {target: Element, run: () => void}

function fakeResizeObserver(): Observed[] {
    const observed: Observed[] = []
    ;(window as any).ResizeObserver = class {
        constructor(private readonly cb: () => void) {}
        observe(target: Element) {
            observed.push({target, run: () => this.cb()})
        }
        disconnect() {
            observed.length = 0
        }
    }
    return observed
}

describe('widgets/editableArea', () => {
    afterEach(() => {
        delete (window as any).ResizeObserver
    })

    it('grows to fit its text again when its width changes', async () => {
        const observed = fakeResizeObserver()
        render(() => (
            <EditableArea
                value='Поднять зависимости'
                onChange={() => undefined}
            />
        ))

        const boxes = screen.getAllByRole('textbox', {hidden: true}) as HTMLTextAreaElement[]
        const visible = document.querySelector('textarea.EditableArea') as HTMLTextAreaElement
        const reference = document.querySelector('textarea.EditableAreaReference') as HTMLTextAreaElement
        expect(boxes.length).toBeGreaterThan(0)
        expect(observed.map((o) => o.target)).toContain(reference)

        // jsdom lays nothing out, so nothing has a height to begin with, and
        // the reference is what says how tall the text is — here, what a
        // narrower column would make of it: two lines instead of one.
        expect(visible.style.height).toBe('')
        Object.defineProperty(reference, 'scrollHeight', {value: 80, configurable: true})
        observed[0].run()

        expect(parseInt(visible.style.height, 10)).toBeGreaterThanOrEqual(80)
    })
})
