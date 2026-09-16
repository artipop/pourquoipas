// debounce and throttle, the two of lodash's that this page actually used.
//
// Both keep the shape the call sites already rely on: the returned function
// takes the same arguments, returns nothing, and carries `cancel()` for a
// component that goes away before the timer fires.

type Timer = ReturnType<typeof setTimeout>

export type Cancellable<A extends unknown[]> = ((...args: A) => void) & {cancel: () => void}

// debounce runs fn once the calls stop for `ms`. The last arguments win, which
// is what a search box wants: the query as it was typed last.
export function debounce<A extends unknown[]>(fn: (...args: A) => void, ms: number): Cancellable<A> {
    let timer: Timer | undefined

    const debounced = (...args: A) => {
        if (timer !== undefined) {
            clearTimeout(timer)
        }
        timer = setTimeout(() => {
            timer = undefined
            fn(...args)
        }, ms)
    }

    debounced.cancel = () => {
        if (timer !== undefined) {
            clearTimeout(timer)
            timer = undefined
        }
    }

    return debounced
}

// throttle runs fn at once and then at most once per `ms`, with the last call
// of a busy stretch delivered at the end of it — lodash's default, and what a
// resize handler is written against: the first frame is drawn immediately and
// the final size is not lost.
export function throttle<A extends unknown[]>(fn: (...args: A) => void, ms: number): Cancellable<A> {
    let timer: Timer | undefined
    let last = 0
    let pending: A | undefined

    const run = (args: A) => {
        last = Date.now()
        fn(...args)
    }

    const throttled = (...args: A) => {
        const since = Date.now() - last
        if (since >= ms && timer === undefined) {
            run(args)
            return
        }
        pending = args
        if (timer === undefined) {
            timer = setTimeout(() => {
                timer = undefined
                if (pending) {
                    const next = pending
                    pending = undefined
                    run(next)
                }
            }, Math.max(ms - since, 0))
        }
    }

    throttled.cancel = () => {
        if (timer !== undefined) {
            clearTimeout(timer)
            timer = undefined
        }
        pending = undefined
    }

    return throttled
}
