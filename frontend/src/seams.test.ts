// The board is Focalboard's, and the agents sit on top of it through
// extensions.tsx (docs/architecture.md, «The board knows nothing of the
// agents»). This holds the line: a board file that starts importing the
// agents' code fails here, and the way forward is a seam, not a new entry.
//
// The files listed import components/acp for things that are not the agents:
// the inbox and its sources, templates, a column helper, the event socket and
// attaching to a server live in that directory too. Each is to leave the list
// when its code moves out, never to join it.

const allowed = new Set([
    'main.tsx', // installs the agents
    'components/boardTemplateSelector/boardTemplateSelector.tsx', // templates
    'components/workspace.tsx', // templates
    'components/centerPanel.tsx', // a card's column
    'components/moveCardToBoard.tsx', // a card's column
    'components/kanban/kanbanCard.tsx', // the inbox
    'components/kanban/kanbanColumnHeader.tsx', // the inbox
    'components/sidebar/sidebar.tsx', // the inbox
    'components/sidebar/sidebarCategory.tsx', // the inbox
    'components/viewHeader/viewHeaderActionsMenu.tsx', // the inbox, templates
    'components/settings/appSettingsDialog.tsx', // attaching to a server
    'components/settings/updates.ts', // the event socket
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
