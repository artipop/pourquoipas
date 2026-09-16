import {createEffect, createSignal} from 'solid-js'
import type {Accessor} from 'solid-js'

// An answer from somewhere asynchronous — Go, the server — kept in a signal,
// asked again when its key changes or on demand. Not an async memo: while one
// is being asked again, a <Show> going falsy anywhere on the page recomputes
// its children once with the falsy value, and that throw halts the whole page
// (a Solid 2 RC bug, docs/solid2.md). A signal is only ever written with an
// answer, so nothing is pending in between.
//
// The latest question wins: an answer for a key already left, or overtaken by
// a reload, is dropped. A failed read keeps the answer that was there.
export type Loader<T> = [value: Accessor<T>, reload: () => void, loading: Accessor<boolean>]

export function createLoader<T, K = undefined>(
    load: (key: K) => Promise<T>,
    initial: T,
    key: Accessor<K> = () => undefined as K,
): Loader<T> {
    // Boxed, because a bare function handed to createSignal is a writable memo.
    const [box, setBox] = createSignal<{value: T}>({value: initial})
    const [asked, setAsked] = createSignal(0)
    const [loading, setLoading] = createSignal(true)

    createEffect(() => [key(), asked()] as const, ([current]) => {
        let live = true
        setLoading(true)
        load(current).then((next) => {
            if (live) {
                setBox({value: next})
                setLoading(false)
            }
        }, () => {
            if (live) {
                setLoading(false)
            }
        })
        return () => {
            live = false
        }
    })

    return [() => box().value, () => setAsked((n) => n + 1), loading]
}
