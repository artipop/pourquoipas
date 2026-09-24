import Mutator from '../../mutator'
import {Board, createBoard} from '../../blocks/board'
import {createIntl} from '../../intl'

import {fakeGoSide} from '../../test/goSide'

import {saveBoardAsTemplate} from './saveAsTemplate'
import {BOARD_PROP_SETUP} from './automation'

vi.mock('../../mutator')

const mockedMutator = vi.mocked(Mutator)
const intl = createIntl({locale: 'en'})

describe('components/acp/saveAsTemplate', () => {
    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    // The copy is Focalboard's own duplicate-as-template, which loses two
    // things this puts back: what the board is called, and the questions it
    // declares. The columns travel by themselves — they are the board's own
    // property.
    test('the copy is called what the board was called and keeps its questions', async () => {
        const board: Board = {
            ...createBoard(),
            id: 'board-1',
            title: 'Работа',
            properties: {[BOARD_PROP_SETUP]: JSON.stringify({steps: [{kind: 'project'}]})} as Board['properties'],
        }
        const copy: Board = {...createBoard(), id: 'template-1', isTemplate: true, title: 'New board template'}
        mockedMutator.duplicateBoard.mockResolvedValue({boards: [copy], blocks: []})

        const saved = await saveBoardAsTemplate(board, intl)

        expect(mockedMutator.duplicateBoard).toHaveBeenCalledWith('board-1', expect.any(String), true)
        expect(saved.title).toBe('Работа')
        expect(JSON.parse(saved.properties[BOARD_PROP_SETUP] as string).steps).toEqual([{kind: 'project'}])
        expect(mockedMutator.updateBoard).toHaveBeenCalled()
    })

    // A board that declared nothing leaves the template declaring nothing,
    // which is what Go reads as "ask the ordinary questions".
    test('a board that asks for nothing writes no questions into the copy', async () => {
        const board: Board = {...createBoard(), id: 'board-2', title: 'Домашние дела'}
        const copy: Board = {...createBoard(), id: 'template-2', isTemplate: true}
        mockedMutator.duplicateBoard.mockResolvedValue({boards: [copy], blocks: []})

        const saved = await saveBoardAsTemplate(board, intl)

        expect(saved.properties[BOARD_PROP_SETUP]).toBeUndefined()
    })
})
