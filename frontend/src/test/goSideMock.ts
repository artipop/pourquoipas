// The bindings generated from App, mocked for every test file: the real ones
// call Go over @wailsio/runtime, and there is no Go behind the test page. Each
// export forwards to whatever fakeGoSide installed (goSide.ts) when it is
// called, so the mock is set up once here rather than hoisted into forty files.
//
// In browser mode a mocked module exports exactly the keys the factory's
// result enumerates, each copied once, so a Proxy answering every name exports
// nothing at all. The names come from the real module, and every one is a
// forwarder rather than the installed method. So every method is present, as
// it is in a real build, and one the fake left out answers what a Go method
// with nothing to say would: a promise of undefined.
vi.mock('../../bindings/github.com/artipop/pourquoipas/app', async (importOriginal) => {
    const {goSide} = await import('./goSide')
    const names = Object.keys(await importOriginal<Record<string, unknown>>())
    return Object.fromEntries(names.map((name) => [name, (...args: unknown[]) => (goSide[name] ? goSide[name](...args) : Promise.resolve(undefined))]))
})
