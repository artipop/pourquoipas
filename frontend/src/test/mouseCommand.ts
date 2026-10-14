// Runs in Node, beside Playwright, not in the page: vitest's own userEvent
// drags in one go, measuring the target before the press and pausing nowhere.
// A drag a hand makes looks at the page while it moves — a category picked up
// collapses every category, so where to drop it is only known after the press
// — and takes time over the release, which dnd-kit needs to have collided the
// pointer with anything at all. The page names its points in its own
// coordinates, and they are moved into the window's here.
import type {BrowserCommand} from 'vitest/node'

export type Point = {x: number, y: number}

type Action = 'down' | 'move' | 'up' | 'park'

export const mouse: BrowserCommand<[action: Action, point: Point, pageWidth: number]> = async (context, action, point, pageWidth) => {
    // Off the page altogether, so nothing drawn next finds the pointer
    // resting on it: Chromium hovers what appears under a still cursor.
    if (action === 'park') {
        await context.page.mouse.move(-1, -1)
        return
    }
    const frame = await context.iframe.owner().boundingBox()
    if (!frame) {
        throw new Error('mouse: no test frame to move in')
    }
    const scale = frame.width / pageWidth
    await context.page.mouse.move(frame.x + (point.x * scale), frame.y + (point.y * scale))
    if (action === 'down') {
        await context.page.mouse.down()
    } else if (action === 'up') {
        await context.page.mouse.up()
    }
}

declare module 'vitest/browser' {
    interface BrowserCommands {
        mouse: (action: Action, point: Point, pageWidth: number) => Promise<void>
    }
}
