// The board is Focalboard's, and the agents sit on top of it through
// extensions.tsx (docs/architecture.md, «The board knows nothing of the
// agents»). This holds the line: a board file that starts importing the
// agents' code fails here, and the way forward is a seam, not a new entry.
//
// Two files may: main.tsx, which installs the agents, and the phone's page,
// which is the agents' own screen (its questions to answer).

const allowed = new Set([
    'main.tsx', // installs the agents
    'pages/mobile/mobilePage.tsx', // the phone is the agents' screen
])

const sources = import.meta.glob(['./**/*.ts', './**/*.tsx', '!./**/*.test.*', '!./components/acp/**', '!./test/**'], {query: '?raw', import: 'default', eager: true}) as Record<string, string>

describe('the seam between the board and the agents', () => {
    test('no board file imports the agents’ code beyond the known list', () => {
        const importing = Object.entries(sources).
            filter(([, text]) => (/(from |import )'[./]*(components\/)?acp\//).test(text)).
            map(([path]) => path.replace(/^\.\//, ''))

        // The glob has to have read the board for the check to mean anything.
        expect(Object.keys(sources)).toContain('./components/cardDialog.tsx')
        expect(importing).toContain('main.tsx')
        expect(importing.filter((path) => !allowed.has(path))).toEqual([])
    })
})
