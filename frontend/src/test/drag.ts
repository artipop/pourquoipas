import {commands} from 'vitest/browser'

import type {Point} from './mouseCommand'
import {pointerMoved} from './pointer'

type DragOptions = {

    // Where in the target the pointer is released, from its top left corner;
    // its middle when left out.
    at?: Point

    // Where the press lands, when that is not the dragged thing itself: a
    // sidebar category is dragged by its title row, and its middle is a board
    // inside it, which would be picked up instead.
    grip?: Element

    // How many moves the pointer makes on the way. One is a flick of the wrist.
    steps?: number
}

const frame = () => new Promise((resolve) => setTimeout(resolve, 16))

const middleOf = (element: Element): Point => {
    const r = element.getBoundingClientRect()
    return {x: r.left + (r.width / 2), y: r.top + (r.height / 2)}
}

const mouse = (action: 'down' | 'move' | 'up', point: Point) => {
    pointerMoved()
    return commands.mouse(action, point, window.innerWidth)
}

/**
 * Drags with the browser's own mouse over the page as it is laid out: press on
 * the middle of what is dragged, nudge it past the activation distance, then
 * aim at the target as it stands now — picking something up may have moved it
 * — and go there a frame at a time. dnd-kit measures and collides on its own
 * schedule, so the release waits a frame, and the drop is given a moment to
 * land.
 */
export async function drag(source: Element, target: Element, options: DragOptions = {}): Promise<void> {
    const from = middleOf(options.grip ?? source)
    await mouse('down', from)
    await frame()
    let last = {x: from.x, y: from.y + 8}
    await mouse('move', last)
    await frame()

    const box = target.getBoundingClientRect()
    const to = options.at ? {x: box.left + options.at.x, y: box.top + options.at.y} : middleOf(target)
    const steps = options.steps ?? 10
    for (let left = steps; left > 0; left--) {
        last = {x: last.x + ((to.x - last.x) / left), y: last.y + ((to.y - last.y) / left)}

        // One move a frame, in order: that is what a drag is.
        // eslint-disable-next-line no-await-in-loop
        await mouse('move', last)
        // eslint-disable-next-line no-await-in-loop
        await frame()
    }
    await mouse('up', to)
    await new Promise((resolve) => setTimeout(resolve, 50))
}
