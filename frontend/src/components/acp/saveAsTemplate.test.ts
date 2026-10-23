import Mutator from '../../mutator'
import {Board, createBoard} from '../../blocks/board'
import {createIntl} from '../../intl'

import {saveBoardAsTemplate} from './saveAsTemplate'

vi.mock('../../mutator')

const mockedMutator = vi.mocked(Mutator)
const intl = createIntl({locale: 'en'})

describe('components/acp/saveAsTemplate', () => {
    afterEach(() => {
        vi.clearAllMocks()
    })

    // The copy is Focalboard's own duplicate-as-template, which calls it "New
    // board template". The columns travel by themselves — they are the board's
    // own property — and the name is put back here.
    test('the copy is called what the board was called', async () => {
        const board: Board = {...createBoard(), id: 'board-1', title: 'Работа'}
        const copy: Board = {...createBoard(), id: 'template-1', isTemplate: true, title: 'New board template'}
        mockedMutator.duplicateBoard.mockResolvedValue({boards: [copy], blocks: []})

        const saved = await saveBoardAsTemplate(board, intl)

        expect(mockedMutator.duplicateBoard).toHaveBeenCalledWith('board-1', expect.any(String), true)
        expect(saved.title).toBe('Работа')
        expect(mockedMutator.updateBoard).toHaveBeenCalled()
    })
})
