import {Board, createBoard} from '../../blocks/board'

import {boardColumns, cardColumnProperty} from './automation'

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
