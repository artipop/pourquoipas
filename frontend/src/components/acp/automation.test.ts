import {Board, createBoard} from '../../blocks/board'

import {
    Automation,
    BOARD_PROP_COLUMNS,
    BOARD_PROP_PROMPT,
    BOARD_PROP_SETUP,
    automationChanges,
    automationProperty,
    boardAutomationProperties,
    boardColumns,
    impliedSetupSteps,
    readBoardAutomation,
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
    // Which property the columns are options of is a fact about the board, not
    // a preference: the specs were written for one of them. It used to be "the
    // board's first select" — an accident of the order fields were added in.
    test('the property the automation names wins over the board’s first select', () => {
        const board: Board = {
            ...createBoard(),
            cardProperties: [
                {id: 'prop-stage', name: 'Этап', type: 'select', options: [{id: 'o1', value: 'Черновик', color: ''}]},
                {id: 'prop-status', name: 'Статус', type: 'select', options: [{id: 'opt-work', value: 'В работе', color: ''}]},
            ],
            properties: {} as Board['properties'],
        }

        expect(automationProperty(board)?.name).toBe('Этап')
        expect(automationProperty(board, {columns: [{propertyId: 'prop-status', property: 'Статус', column: 'В работе', action: 'agent'}]})?.name).toBe('Статус')
    })

    test('a board’s columns are the options of its select property', () => {
        expect(boardColumns(boardWith({}), 'Статус').map((c) => c.name)).toEqual(['В работе', 'На ревью'])
    })

    // A column is its option: renaming it on the board must not read as one
    // column going away and another arriving.
    test('what changed is worked out per column', () => {
        const before: Automation = {
            columns: [{optionId: 'opt-work', property: 'Статус', column: 'В работе', action: 'agent'}],
        }
        const after: Automation = {
            columns: [{optionId: 'opt-review', property: 'Статус', column: 'На ревью', action: 'none'}],
        }
        const changes = automationChanges(before, after)
        expect(changes.savedColumns.map((c) => c.column)).toEqual(['На ревью'])
        expect(changes.removedColumns.map((c) => c.column)).toEqual(['В работе'])
    })

    // A template carries its automation in the board's own properties, and Go
    // reads either the object or the JSON text of it.
    test('a template’s automation is read whether it is an object or its text', () => {
        const asObjects = readBoardAutomation(boardWith({
            [BOARD_PROP_COLUMNS]: [{property: 'Статус', column: 'В работе', action: 'agent'}],
        }))
        expect(asObjects.columns[0].column).toBe('В работе')

        const asText = readBoardAutomation(boardWith({
            [BOARD_PROP_COLUMNS]: JSON.stringify([{property: 'Статус', column: 'На ревью', action: 'none'}]),
        }))
        expect(asText.columns[0].column).toBe('На ревью')
    })

    test('a template with unreadable automation reads as one with none', () => {
        expect(readBoardAutomation(boardWith({[BOARD_PROP_COLUMNS]: '{'}))).toEqual({columns: []})
        expect(readBoardSetup(boardWith({[BOARD_PROP_SETUP]: 'nonsense'}))).toBeUndefined()
    })

    test('saving a template writes the properties Go reads', () => {
        const board = boardWith({keepMe: 'yes'})
        const properties = boardAutomationProperties(
            board,
            {columns: [{property: 'Статус', column: 'В работе', action: 'agent'}]},
            {steps: [{kind: 'project'}]},
        )
        expect(properties.keepMe).toBe('yes')
        expect(JSON.parse(properties[BOARD_PROP_COLUMNS] as string)[0].column).toBe('В работе')
        expect(JSON.parse(properties[BOARD_PROP_SETUP] as string).steps).toEqual([{kind: 'project'}])

        // Going back to "work it out from the automation" is the property not
        // being there at all, which is what Go reads as "the board said nothing".
        expect(boardAutomationProperties(board, {columns: []}, undefined)[BOARD_PROP_SETUP]).toBeUndefined()
    })

    test('saving a template keeps the instructions its agents are given', () => {
        // The prompt is written by Go, not here, so the only way this page can
        // affect it is by dropping it — which would leave a template that runs
        // the right columns and briefs nobody.
        const board = boardWith({[BOARD_PROP_PROMPT]: 'Отвечай по-русски.'})
        const properties = boardAutomationProperties(board, {columns: []}, undefined)
        expect(properties[BOARD_PROP_PROMPT]).toBe('Отвечай по-русски.')
    })

    test('the steps offered to a template follow what its automation does', () => {
        const defs = [
            {kind: 'project', optional: false},
            {kind: 'agent', optional: false},
            {kind: 'deploy', optional: true},
            {kind: 'browser', optional: true},
            {kind: 'done', optional: false},
        ]
        const plain = impliedSetupSteps({columns: [{property: 'Статус', column: 'В работе', action: 'agent'}]}, defs)
        expect(plain.map((s) => s.kind)).toEqual(['project', 'agent', 'done'])

        const publishing = impliedSetupSteps({
            columns: [
                {property: 'Статус', column: 'Деплой', action: 'deploy'},
                {property: 'Статус', column: 'Тест', action: 'test'},
            ],
        }, defs)
        expect(publishing.map((s) => s.kind)).toEqual(['project', 'agent', 'deploy', 'browser', 'done'])
    })
})
