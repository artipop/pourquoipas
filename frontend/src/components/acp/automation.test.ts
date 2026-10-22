import {Board, createBoard} from '../../blocks/board'

import {
    BOARD_PROP_PROMPT,
    BOARD_PROP_SETUP,
    boardAutomationProperties,
    boardColumns,
    cardColumnProperty,
    impliedSetupSteps,
    readBoardSetup,
} from './automation'

function boardWith(properties: Record<string, unknown>): Board {
    return {
        ...createBoard(),
        cardProperties: [{
            id: 'prop-status',
            name: 'Статус',
            type: 'select',
            options: [
                {id: 'opt-work', value: 'В работе', color: 'propColorBlue'},
                {id: 'opt-review', value: 'На ревью', color: 'propColorPink'},
            ],
        }],
        properties: properties as Board['properties'],
    }
}

describe('components/acp/automation', () => {
    // The board's columns are the options of its select property and nothing
    // else: there is no registry saying what any of them does.
    test('a board’s columns are the options of its select property', () => {
        expect(boardColumns(boardWith({}), 'Статус').map((c) => c.name)).toEqual(['В работе', 'На ревью'])
    })

    test('a template’s setup steps are read whether they are an object or text', () => {
        const asObject = readBoardSetup(boardWith({[BOARD_PROP_SETUP]: {steps: [{kind: 'project'}]}}))
        expect(asObject?.steps).toEqual([{kind: 'project'}])

        const asText = readBoardSetup(boardWith({[BOARD_PROP_SETUP]: JSON.stringify({steps: [{kind: 'agent'}]})}))
        expect(asText?.steps).toEqual([{kind: 'agent'}])
    })

    test('a template with unreadable setup steps reads as one with none', () => {
        expect(readBoardSetup(boardWith({[BOARD_PROP_SETUP]: 'nonsense'}))).toBeUndefined()
    })

    test('saving a template writes the steps Go reads and keeps the rest', () => {
        const board = boardWith({keepMe: 'yes'})
        const properties = boardAutomationProperties(board, {steps: [{kind: 'project'}]})
        expect(properties.keepMe).toBe('yes')
        expect(JSON.parse(properties[BOARD_PROP_SETUP] as string).steps).toEqual([{kind: 'project'}])

        // Going back to "work it out yourself" is the property not being there
        // at all, which is what Go reads as "the board said nothing".
        expect(boardAutomationProperties(board, undefined)[BOARD_PROP_SETUP]).toBeUndefined()
    })

    test('saving a template keeps the instructions its agents are given', () => {
        // The prompt is written by Go, not here, so the only way this page can
        // affect it is by dropping it — which would leave a template that runs
        // the right columns and briefs nobody.
        const board = boardWith({[BOARD_PROP_PROMPT]: 'Отвечай по-русски.'})
        expect(boardAutomationProperties(board, undefined)[BOARD_PROP_PROMPT]).toBe('Отвечай по-русски.')
    })

    // Naming the steps by hand starts from the ones the app would ask for
    // anyway: nothing is inferred from the board itself any more.
    test('the steps offered to a template are the ones this build can ask', () => {
        const defs = [
            {kind: 'project', optional: false},
            {kind: 'agent', optional: false},
            {kind: 'done', optional: false},
        ]
        expect(impliedSetupSteps(defs).map((s) => s.kind)).toEqual(['project', 'agent', 'done'])
    })

    // A new card stands in the first column of the board's stages. The folder
    // is a select too, but its empty value is an answer — no folder yet — so a
    // view grouped by it does not make it the column a card is put in.
    test('the column a new card stands in is never the folder', () => {
        const board = boardWith({projectProperty: 'prop-folder'})
        const folder = {id: 'prop-folder', name: 'Папка', type: 'select' as const, options: [{id: 'f1', value: 'xciii', color: ''}]}
        board.cardProperties = [folder, ...board.cardProperties]

        expect(cardColumnProperty(board)?.id).toBe('prop-status')
        expect(cardColumnProperty(board, folder)?.id).toBe('prop-status')
        expect(cardColumnProperty(board, board.cardProperties[1])?.id).toBe('prop-status')
    })
})
