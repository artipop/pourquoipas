import {Board, IPropertyOption, IPropertyTemplate} from '../../blocks/board'
import {TestBlockFactory} from '../../test/testBlockFactory'
import {fakeGoSide} from '../../test/goSide'

import {agentAccountId, firstTaskCard, firstTaskColumns} from './firstTask'

const option = (id: string, value: string): IPropertyOption => ({id, value, color: ''})

const status: IPropertyTemplate = {
    id: 'status',
    name: 'Status',
    type: 'select',
    options: [option('todo', 'To do'), option('doing', 'In progress'), option('done', 'Done')],
}

function boardWith(...properties: IPropertyTemplate[]): Board {
    const board = TestBlockFactory.createBoard()
    board.cardProperties = properties
    return board
}

// A card starts when it is moved into a column, not when it is made in one, so
// the first task is born in the first column and moved into the second.
describe('components/acp/firstTask', () => {
    afterEach(() => fakeGoSide())

    test('is born in the first column and moved into the second, as the view shows them', () => {
        const board = boardWith(status)
        const shown = [option('doing', 'In progress'), option('todo', 'To do')]

        const columns = firstTaskColumns(board, status, shown)!
        expect(columns.from.id).toBe('doing')
        expect(columns.to.id).toBe('todo')
    })

    test('off a grouped view, takes the board’s column property in its own order', () => {
        const columns = firstTaskColumns(boardWith(status))!
        expect([columns.from.id, columns.to.id]).toEqual(['todo', 'doing'])
    })

    test('a board without two columns has no first task', () => {
        const one = {...status, options: [option('todo', 'To do')]}
        expect(firstTaskColumns(boardWith(one), one)).toBeUndefined()
        expect(firstTaskColumns(boardWith())).toBeUndefined()
    })

    test('the card names the agent in the board’s person field', () => {
        const who: IPropertyTemplate = {id: 'who', name: 'Who is on it', type: 'multiPerson', options: []}
        const board = boardWith(status, who)
        const card = firstTaskCard(board, firstTaskColumns(board)!, 'Fix the build', 'agent-user', {folder: 'f1'})!

        expect(card.title).toBe('Fix the build')
        expect(card.boardId).toBe(board.id)
        expect(card.fields.properties).toEqual({folder: 'f1', status: 'todo', who: ['agent-user']})
    })

    test('a board with no person field makes no card', () => {
        const board = boardWith(status)
        expect(firstTaskCard(board, firstTaskColumns(board)!, 'Fix the build', 'agent-user', {})).toBeUndefined()
    })

    // The wizard may have registered the agent a moment ago, so its account is
    // asked for by making sure it exists.
    test('the agent’s account is the one the board sync answers with', async () => {
        const SyncAgentUsers = vi.fn().mockResolvedValue(JSON.stringify([
            {name: 'Codex', username: 'codex', userId: 'u-codex'},
            {name: 'Claude', username: 'claude', userId: 'u-claude'},
        ]))
        fakeGoSide({SyncAgentUsers})

        expect(await agentAccountId('board-1', 'Claude')).toBe('u-claude')
        expect(SyncAgentUsers).toHaveBeenCalledWith('board-1')
    })
})
