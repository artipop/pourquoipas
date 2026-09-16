// The bindings generated from App, mocked for every test file: the real ones
// call Go over @wailsio/runtime, and there is no Go behind jsdom. Each export
// forwards to whatever fakeGoSide installed (goSide.ts) when it is called, so
// the mock is set up once here rather than hoisted into forty files.
vi.mock('../../bindings/github.com/artipop/xciii/app', async () => {
    const {goSide} = await import('./goSide')
    return new Proxy({}, {
        has: () => true,
        get: (_, key) => (typeof key === 'string' ? goSide[key] : undefined),
    })
})
