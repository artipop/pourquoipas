import {fireEvent} from '@testing-library/dom'
import {flush} from 'solid-js'

// Solid 2 stages a write and drains it on the next microtask, where Solid 1
// applied it there and then. The suite is written the Solid 1 way — fire a
// click, then read the DOM in the next line — so the drain happens here, once,
// rather than as a `flush()` after every event in two hundred files.
//
// `@solidjs/testing-library` re-exports `@testing-library/dom`'s fireEvent
// untouched, and that one knows nothing about Solid. Patched in place, because
// what the tests hold is a reference to this very object.
//
// This covers a write a person's event caused. A test that writes directly —
// calling a module's own function, driving a signal — still says `flush()`
// itself, because there is no event to hang it on.
const patch = (target: Record<string, any>) => {
    for (const name of Object.keys(target)) {
        const original = target[name]
        if (typeof original !== 'function') {
            continue
        }
        target[name] = (...args: unknown[]) => {
            const result = original(...args)
            flush()
            return result
        }
    }
}

patch(fireEvent as unknown as Record<string, any>)

// Advancing fake timers runs the callbacks a component scheduled, and those
// write signals for exactly the same reason an event handler does — a fade-out,
// a debounce, a delayed tour tip. Same drain, same place.
for (const name of ['advanceTimersByTime', 'advanceTimersToNextTimer', 'runAllTimers', 'runOnlyPendingTimers'] as const) {
    const original = vi[name] as (...args: unknown[]) => unknown
    if (typeof original !== 'function') {
        continue
    }
    (vi as unknown as Record<string, unknown>)[name] = (...args: unknown[]) => {
        const result = original.apply(vi, args)
        flush()
        return result
    }
}
