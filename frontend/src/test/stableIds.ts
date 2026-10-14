// Solid's createUniqueId is a counter global to the test file, so the ids a
// snapshot recorded depended on how many components earlier tests had drawn:
// adding, skipping or reordering a test rewrote unrelated snapshots. Counting
// from zero per test keeps the ids — and the aria-controls and
// aria-activedescendant that point at them — in the snapshot and still
// deterministic. A stopgap while whole-DOM snapshots last (docs/todo.md).
let next = 0

beforeEach(() => {
    next = 0
})

vi.mock('solid-js', async (importOriginal) => ({
    ...await importOriginal<typeof import('solid-js')>(),
    createUniqueId: () => `id-${next++}`,
}))
