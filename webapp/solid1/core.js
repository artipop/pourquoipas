// The Solid 1 `solid-js` surface, on Solid 2. See README.md.

import {
    createComponent,
    createContext as createContext2,
    useContext as useContext2,
    createEffect as createEffect2,
    createMemo,
    createRenderEffect as createRenderEffect2,
    Errored,
    For,
    getObserver,
    Loading,
    merge,
    omit,
    untrack,
} from 'solid-js'

export {
    children,
    createComponent,
    createMemo,
    createRoot,
    createSignal,
    createUniqueId,
    DEV,
    Errored,
    flush,
    For,
    getOwner,
    isEqual,
    isWrappable,
    lazy,
    Loading,
    mapArray,
    Match,
    merge,
    omit,
    onCleanup,
    resolve,
    runWithOwner,
    sharedConfig,
    Show,
    Switch,
    untrack,
} from 'solid-js'

export {getObserver as getListener} from 'solid-js'

// Solid 2 batches every write until the microtask drains, so the callback is
// the whole of what Solid 1's batch did that still needs doing.
export const batch = (fn) => fn()

// Solid 1 tracked the entire effect body. That is exactly what the compute
// half of a Solid 2 effect is, so the body goes there and the apply half is
// empty — a faithful translation rather than an idiomatic one.
export const createEffect = (fn, value) => createEffect2((prev) => fn(prev), () => {}, value)
export const createRenderEffect = (fn, value) => createRenderEffect2((prev) => fn(prev), () => {}, value)
export const createComputed = (fn, value) => createEffect2((prev) => fn(prev), () => {}, value)

export const onMount = (fn) => createEffect2(() => undefined, () => fn())

// Two differences at once. A Solid 2 context *is* its provider component,
// where Solid 1 hung one off it; and Solid 2 throws from useContext when a
// context was made with no default and has no provider above it, where Solid 1
// answered undefined. The sentinel stands in for "no default" so useContext
// below can give the Solid 1 answer back.
const noDefault = Symbol('solid1.noDefault')

export const createContext = (...args) => {
    const ctx = createContext2(args.length ? args[0] : noDefault)
    ctx.Provider = ctx
    return ctx
}

export const useContext = (ctx) => {
    const value = useContext2(ctx)
    return value === noDefault ? undefined : value
}

export const on = (deps, fn, options = {}) => {
    const read = Array.isArray(deps) ? () => deps.map((d) => d()) : deps
    let deferred = Boolean(options.defer)
    let previous
    return (prevValue) => {
        const input = read()
        const wasDeferred = deferred
        const prevInput = previous
        deferred = false
        previous = input
        if (wasDeferred) {
            return prevValue
        }
        return untrack(() => fn(input, prevInput, prevValue))
    }
}

export const mergeProps = merge

// Each group keeps reading through to the source, because a Solid 1 caller
// expects the split halves to stay reactive.
export const splitProps = (props, ...groups) => {
    const taken = []
    const split = groups.map((keys) => {
        const group = {}
        for (const key of keys) {
            taken.push(key)
            Object.defineProperty(group, key, {
                get: () => props[key],
                enumerable: true,
            })
        }
        return group
    })
    return [...split, omit(props, ...taken)]
}

export const createSelector = (source, equals = (key, value) => key === value) =>
    (key) => equals(key, source())

export const Index = (props) => createComponent(For, {
    keyed: false,
    get each() {
        return props.each
    },
    get fallback() {
        return props.fallback
    },
    get children() {
        return props.children
    },
})

export const Suspense = Loading
export const ErrorBoundary = Errored

export const equalFn = (a, b) => a === b

export const indexArray = (list, mapFn) =>
    createMemo(() => (list() ?? []).map((_, i) => mapFn(() => list()[i], i)))

export const createDeferred = (source) => createMemo(() => source())

export const startTransition = (fn) => Promise.resolve(fn())
export const useTransition = () => [() => false, startTransition]

const gone = (name) => () => {
    throw new Error(`solid1 compat: ${name} has no Solid 2 equivalent`)
}
export const observable = gone('observable')
export const from = gone('from')
export const createMutable = gone('createMutable')
export const modifyMutable = gone('modifyMutable')
