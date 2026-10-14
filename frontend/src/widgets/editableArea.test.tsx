import {render, waitFor} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import EditableArea from './editableArea'

// How many lines a text takes depends on how wide it is, and the box that holds
// it is sized in pixels — so a width that changes without the value changing
// leaves the box the wrong height. That is what opening the panel beside a card
// does: «Поднять зависимости» rewrapped to two lines while the box kept the
// height of one, and the title read as «Поднять» with the rest clipped.
describe('widgets/editableArea', () => {
    it('grows to fit its text again when its width changes', async () => {
        render(() => (
            <div
                class='column'
                style={{width: '600px'}}
            >
                <EditableArea
                    value='Поднять зависимости'
                    onChange={() => undefined}
                />
            </div>
        ))
        const column = document.querySelector('.column') as HTMLElement
        const box = document.querySelector('textarea.EditableArea') as HTMLTextAreaElement

        await waitFor(() => expect(box.style.height).not.toBe(''))
        const oneLine = parseInt(box.style.height, 10)

        // What a narrower column makes of the same text: more lines.
        column.style.width = '80px'
        await waitFor(() => expect(parseInt(box.style.height, 10)).toBeGreaterThan(oneLine))
    })
})
