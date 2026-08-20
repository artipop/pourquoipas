// The Solid 1 `solid-js/store` surface, on Solid 2. See README.md.
//
// Solid 2 sets a store through a draft — `set(d => { d.a.b = 1 })` — where
// Solid 1 set it through a path — `set('a', 'b', 1)`. `@dschz/solid-flow`
// writes the second form, including its predicate variant
// (`set(node => ids.has(node.id), produce(...))`), so the path is walked over
// the draft here instead.

import {createStore as createStore2, snapshot} from 'solid-js'

export {reconcile, snapshot as unwrap} from 'solid-js'

const PRODUCE = Symbol('solid1.produce')

export const produce = (fn) => {
    const wrapped = (draft) => {
        fn(draft)
        return draft
    }
    wrapped[PRODUCE] = true
    return wrapped
}

const keysAt = (target, part) => {
    if (Array.isArray(part)) {
        return part
    }
    if (typeof part === 'function') {
        if (!Array.isArray(target)) {
            throw new Error('solid1 compat: a predicate path needs an array')
        }
        const hit = []
        for (let i = 0; i < target.length; i++) {
            if (part(target[i], i)) {
                hit.push(i)
            }
        }
        return hit
    }
    if (part !== null && typeof part === 'object') {
        throw new Error('solid1 compat: range paths are not supported')
    }
    return [part]
}

const write = (target, key, value) => {
    if (typeof value === 'function') {
        if (value[PRODUCE]) {
            value(target[key])
            return
        }
        target[key] = value(target[key])
        return
    }
    target[key] = value
}

const walk = (target, path, value) => {
    if (target === undefined || target === null) {
        return
    }
    const [head, ...rest] = path
    for (const key of keysAt(target, head)) {
        if (rest.length) {
            walk(target[key], rest, value)
        } else {
            write(target, key, value)
        }
    }
}

const mergeInto = (draft, value) => {
    const patch = typeof value === 'function' ? value(snapshot(draft)) : value
    if (!patch) {
        return
    }
    for (const key of Object.keys(patch)) {
        draft[key] = patch[key]
    }
}

export const createStore = (init, options) => {
    const [state, setState] = createStore2(init, options)
    const set = (...args) => {
        if (args.length === 1) {
            const only = args[0]
            if (typeof only === 'function' && only[PRODUCE]) {
                return setState((draft) => {
                    only(draft)
                })
            }
            return setState((draft) => mergeInto(draft, only))
        }
        const value = args[args.length - 1]
        const path = args.slice(0, -1)
        return setState((draft) => walk(draft, path, value))
    }
    return [state, set]
}
