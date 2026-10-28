import {createEffect, createSignal} from 'solid-js'
import type {Accessor} from 'solid-js'

// An answer from somewhere asynchronous — Go, the server — kept in a signal,
// asked again when its key changes or on demand. This keeps explicit loading
// and error state and retains the last answer during revalidation or failure.
// Async computations have a different boundary/pending contract; adopting
// them is separate from the rc.18 fix for the old Show issue (docs/solid2.md).
//
// The latest question wins: an answer for a key already left, or overtaken by
// a reload, is dropped. A failed read keeps the answer that was there.
export type Loader<T> = [value: Accessor<T>, reload: () => void, loading: Accessor<boolean>, error: Accessor<unknown>]

export function createLoader<T, K = undefined>(
    load: (key: K) => Promise<T>,
    initial: T,
    key: Accessor<K> = () => undefined as K,
): Loader<T> {
    // Boxed, because a bare function handed to createSignal is a writable memo.
    const [box, setBox] = createSignal<{value: T}>({value: initial})
    const [asked, setAsked] = createSignal(0)
    const [loading, setLoading] = createSignal(true)
    const [error, setError] = createSignal<{cause: unknown}>({cause: undefined})

    createEffect(() => [key(), asked()] as const, ([current]) => {
        let live = true
        setLoading(true)
        setError({cause: undefined})
        const failed = (cause: unknown) => {
            if (live) {
                setError({cause})
                setLoading(false)
            }
        }
        let pending: Promise<T>
        try {
            pending = load(current)
        } catch (cause) {
            failed(cause)
            return undefined
        }
        pending.then((next) => {
            if (live) {
                setBox({value: next})
                setLoading(false)
            }
        }, failed)
        return () => {
            live = false
        }
    })

    return [() => box().value, () => setAsked((n) => n + 1), loading, () => error().cause]
}
