import Mutator from '../../mutator'
import {Board, createBoard} from '../../blocks/board'
import {createIntl} from '../../intl'

import {fakeGoSide} from '../../test/goSide'

import {saveBoardAsTemplate, isSaveAsTemplateAvailable} from './saveAsTemplate'
import {BOARD_PROP_COLUMNS, BOARD_PROP_SETUP} from './automation'

vi.mock('../../mutator')

const mockedMutator = vi.mocked(Mutator)
const intl = createIntl({locale: 'en'})

describe('components/acp/saveAsTemplate', () => {
    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    test('is offered only where there is an agent side to read the automation from', () => {
        expect(isSaveAsTemplateAvailable()).toBe(false)
        fakeGoSide({ExportBoardAutomation: vi.fn()})
        expect(isSaveAsTemplateAvailable()).toBe(true)
    })

    // What a board does lives in the registry rather than on the board, so a
    // copy made without it is a template with the columns drawn and nothing
    // happening in them.
    test('the copy carries the automation the original runs', async () => {
        fakeGoSide({
            ExportBoardAutomation: vi.fn().mockResolvedValue(JSON.stringify({
                columns: [{optionId: 'opt-work', property: 'Статус', column: 'В работе', action: 'agent'}],
            })),
        })
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

        // A template made from a board is called what the board is called: the
        // copy arrives as "New board template", which says nothing about it.
        expect(saved.title).toBe('Работа')
        expect(JSON.parse(saved.properties[BOARD_PROP_COLUMNS] as string)[0].column).toBe('В работе')

        // What the board itself declared it needs asking travels with it.
        expect(JSON.parse(saved.properties[BOARD_PROP_SETUP] as string).steps).toEqual([{kind: 'project'}])
        expect(mockedMutator.updateBoard).toHaveBeenCalled()
    })

    // A board made from a template and saved straight back has all of its
    // automation on the board and none of it in the registry, which is only
    // read when something first asks. Writing the empty answer over the copy
    // erased the very thing the template was being made for.
    test('a board whose automation has not reached the registry keeps its own', async () => {
        fakeGoSide({ExportBoardAutomation: vi.fn().mockResolvedValue('{}')})
        const board: Board = {
            ...createBoard(),
            id: 'board-1',
            properties: {
                [BOARD_PROP_COLUMNS]: JSON.stringify([{optionId: 'opt-work', property: 'Статус', column: 'В работе', action: 'agent'}]),
            } as Board['properties'],
        }
        const copy: Board = {...createBoard(), id: 'template-1', isTemplate: true}
        mockedMutator.duplicateBoard.mockResolvedValue({boards: [copy], blocks: []})

        const saved = await saveBoardAsTemplate(board, intl)

        expect(JSON.parse(saved.properties[BOARD_PROP_COLUMNS] as string)[0].column).toBe('В работе')
    })
})
