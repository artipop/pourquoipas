import {commands} from 'vitest/browser'

// The real pointer outlives the test that moved it, and Chromium hovers
// whatever is drawn under a cursor at rest — so a later test, in this file or
// the next one on the same page, found a tooltip open that it never asked for.
// A test that moves the pointer says so, and it is parked off the page when
// that test ends (setup in parkPointer.ts).
let moved = false

export function pointerMoved(): void {
    moved = true
}

export async function parkPointerIfMoved(): Promise<void> {
    if (moved) {
        moved = false
        await commands.mouse('park', {x: 0, y: 0}, window.innerWidth)
    }
}
