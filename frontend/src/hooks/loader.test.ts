import {createRoot, createSignal, flush} from 'solid-js'

import {createLoader} from './loader'

type Deferred<T> = {promise: Promise<T>, resolve: (value: T) => void, reject: (e: unknown) => void}

const deferred = <T>(): Deferred<T> => {
    let resolve!: (value: T) => void
    let reject!: (e: unknown) => void
    const promise = new Promise<T>((res, rej) => {
        resolve = res
        reject = rej
    })
    return {promise, resolve, reject}
}

const settle = async () => {
    await Promise.resolve()
    await Promise.resolve()
    flush()
}

describe('hooks/loader', () => {
    // Switching boards while the first is still loading is the ordinary case,
    // and the slow answer for the board left must not land over the one opened.
    test('keeps the answer to the latest question, not the last to arrive', async () => {
        const asks: {[key: string]: Deferred<string>} = {a: deferred(), b: deferred()}
        const [key, setKey] = createSignal('a')
        let value!: () => string
        const dispose = createRoot((d) => {
            [value] = createLoader((k: string) => asks[k].promise, '', key)
            return d
        })
        flush()

        setKey('b')
        flush()
        asks.b.resolve('answer for b')
        await settle()
        asks.a.resolve('answer for a')
        await settle()

        expect(value()).toBe('answer for b')
        dispose()
    })

    // An event saying "look again" must not blank what is on screen while the
    // new answer is on its way.
    test('asks again on reload and keeps the old answer meanwhile', async () => {
        const answers = [deferred<number>(), deferred<number>()]
        let calls = 0
        let value!: () => number
        let reload!: () => void
        let loading!: () => boolean
        const dispose = createRoot((d) => {
            [value, reload, loading] = createLoader(() => answers[calls++].promise, 0)
            return d
        })
        flush()
        answers[0].resolve(1)
        await settle()
        expect([value(), loading()]).toEqual([1, false])

        reload()
        flush()
        expect(calls).toBe(2)
        expect([value(), loading()]).toEqual([1, true])

        answers[1].resolve(2)
        await settle()
        expect([value(), loading()]).toEqual([2, false])
        dispose()
    })

    // A read that failed says nothing new about the thing it was reading.
    test('keeps the answer it had when a read fails', async () => {
        const answers = [deferred<string>(), deferred<string>()]
        let calls = 0
        let value!: () => string
        let reload!: () => void
        const dispose = createRoot((d) => {
            [value, reload] = createLoader(() => answers[calls++].promise, '')
            return d
        })
        flush()
        answers[0].resolve('first')
        await settle()

        reload()
        flush()
        answers[1].reject(new Error('gone'))
        await settle()

        expect(value()).toBe('first')
        dispose()
    })
})
