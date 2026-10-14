import type {IAppWindow} from '../types'

let installed: Record<string, any> = {}

// What a test's Go side answers. The bindings module is mocked for the whole
// suite (goSideMock.ts) and every export of the mock reads from here when it
// is called, so a fake installed after a component imported the bindings is
// still the one it calls. Reads and writes go to the very object the test
// installed: a test that swaps one method on its own object mid-test is
// swapping it on the Go side too.
export const goSide: Record<string, any> = new Proxy({}, {
    get: (_, key) => (typeof key === 'string' ? installed[key] : undefined),
    set: (_, key, value) => {
        installed[key as string] = value
        return true
    },
})

// fakeGoSide puts a Go side behind the page answering with these methods,
// replacing whatever an earlier test installed; with no argument there is
// none, as in a test that never asked. A method left out answers a promise
// of undefined (goSideMock.ts).
export function fakeGoSide(bindings?: Record<string, any>): void {
    installed = bindings ?? {}
    const w = window as unknown as IAppWindow
    if (bindings) {
        w.hasGoSide = true
    } else {
        delete w.hasGoSide
    }
}
