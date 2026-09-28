import {Board} from '../../blocks/board'
import {TestBlockFactory} from '../../test/testBlockFactory'
import mutator from '../../mutator'

import {fakeGoSide} from '../../test/goSide'

import {BOARD_PROP_PROJECT_PROPERTY} from './automation'
import {canAddFolder, pickFolderForBoard} from './cardFolder'

vi.mock('../../mutator')
const mockedMutator = vi.mocked(mutator)

const NAMES = {folder: 'Папка', branch: 'Ветка'}

// A board made from a template: it has the folder field, and nothing in it.
function templateBoard(): Board {
    const board = TestBlockFactory.createBoard()
    board.cardProperties = [{id: 'prop-folder', name: 'Папка', type: 'select', options: []}]
    board.properties = {[BOARD_PROP_PROJECT_PROPERTY]: 'prop-folder'}
    return board
}

// A folder is asked on the card the first time one is wanted, and added there
// once: after that it is an option of the board's field that every other card
// picks from the list.
describe('components/acp/cardFolder', () => {
    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    test('a page with no disk to pick from offers no folder to add', () => {
        expect(canAddFolder()).toBe(false)
    })

    test('a folder picked on a card is registered for the board and becomes its option', async () => {
        const entry = {id: 'wd-1', name: 'webapp', path: '/Users/me/src/webapp', boardId: 'b'}
        const bindings = {
            PickDirectory: vi.fn().mockResolvedValue('/Users/me/src/webapp'),
            FindAgentWorkdir: vi.fn().mockResolvedValue(''),
            AddAgentWorkdir: vi.fn().mockResolvedValue(JSON.stringify(entry)),
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([entry])),
        }
        fakeGoSide(bindings)
        const board = templateBoard()

        const added = await pickFolderForBoard(board, 'Choose', NAMES)

        // Named after the folder, since nobody was asked for a name, and filed
        // on this board only.
        expect(bindings.AddAgentWorkdir).toHaveBeenCalledWith('webapp', '/Users/me/src/webapp', board.id, '', false)

        // The option carries the registry's id, so the card's value is a
        // reference to the folder and not a copy of its name.
        const written = mockedMutator.updateBoardCardProperties.mock.calls[0][2]
        expect(written.find((p) => p.id === 'prop-folder')?.options.map((o) => [o.id, o.value])).toEqual([['wd-1', 'webapp']])
        expect(added).toEqual({propertyId: 'prop-folder', optionId: 'wd-1'})
    })

    // Two projects called `app` are ordinary. Nobody was asked for a name, so
    // there is nobody to ask for another: the second is numbered.
    test('a name another folder has is numbered rather than refused', async () => {
        const entry = {id: 'wd-2', name: 'app 2', path: '/work/app', boardId: 'b'}
        const taken = Object.assign(new Error('taken'), {cause: {code: 'folders.name-taken'}})
        const bindings = {
            PickDirectory: vi.fn().mockResolvedValue('/work/app'),
            FindAgentWorkdir: vi.fn().mockResolvedValue(''),
            AddAgentWorkdir: vi.fn().mockRejectedValueOnce(taken).mockResolvedValue(JSON.stringify(entry)),
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([entry])),
        }
        fakeGoSide(bindings)

        const added = await pickFolderForBoard(templateBoard(), 'Choose', NAMES)

        expect(bindings.AddAgentWorkdir.mock.calls.map((c) => c[0])).toEqual(['app', 'app 2'])
        expect(added?.optionId).toBe('wd-2')
    })

    // A folder the machine already knows is used, not added twice: one
    // nobody's board has is attached to this one.
    test('a folder the machine already knows is attached, not added again', async () => {
        const known = {id: 'wd-3', name: 'notes', path: '/Users/me/notes'}
        const bindings = {
            PickDirectory: vi.fn().mockResolvedValue('/Users/me/notes'),
            FindAgentWorkdir: vi.fn().mockResolvedValue(JSON.stringify(known)),
            AttachAgentWorkdir: vi.fn().mockResolvedValue('{}'),
            AddAgentWorkdir: vi.fn(),
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([{...known, boardId: 'b'}])),
        }
        fakeGoSide(bindings)
        const board = templateBoard()

        const added = await pickFolderForBoard(board, 'Choose', NAMES)

        expect(bindings.AddAgentWorkdir).not.toHaveBeenCalled()
        expect(bindings.AttachAgentWorkdir).toHaveBeenCalledWith('notes', board.id)
        expect(added).toEqual({propertyId: 'prop-folder', optionId: 'wd-3'})
    })

    // A board that never had a folder has no field for one; the first folder
    // added from a card makes it, and the board records which field it is.
    test('the first folder on a board without a folder field makes the field', async () => {
        const entry = {id: 'wd-4', name: 'site', path: '/srv/site', boardId: 'b'}
        fakeGoSide({
            PickDirectory: vi.fn().mockResolvedValue('/srv/site'),
            FindAgentWorkdir: vi.fn().mockResolvedValue(''),
            AddAgentWorkdir: vi.fn().mockResolvedValue(JSON.stringify(entry)),
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([entry])),
        })
        const board = TestBlockFactory.createBoard()
        board.cardProperties = []
        board.properties = {}

        const added = await pickFolderForBoard(board, 'Choose', NAMES)

        const field = mockedMutator.updateBoardCardProperties.mock.calls[0][2].find((p) => p.name === 'Папка')
        expect(field?.type).toBe('select')
        expect(added).toEqual({propertyId: field!.id, optionId: 'wd-4'})
        const recorded = mockedMutator.updateBoard.mock.calls[0][0] as Board
        expect(recorded.properties[BOARD_PROP_PROJECT_PROPERTY]).toBe(field!.id)
    })

    test('closing the picker adds nothing', async () => {
        const bindings = {
            PickDirectory: vi.fn().mockResolvedValue(''),
            AddAgentWorkdir: vi.fn(),
            ListAgentWorkdirs: vi.fn(),
        }
        fakeGoSide(bindings)

        expect(await pickFolderForBoard(templateBoard(), 'Choose', NAMES)).toBeUndefined()
        expect(bindings.AddAgentWorkdir).not.toHaveBeenCalled()
        expect(mockedMutator.updateBoardCardProperties).not.toHaveBeenCalled()
    })
})
