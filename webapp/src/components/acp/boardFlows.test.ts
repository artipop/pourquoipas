import {createRoot, createSignal, flush} from 'solid-js'

import {BoardGroup, IPropertyTemplate} from '../../blocks/board'

import {Flow} from './automation'
import * as bindings from './bindings'
import {flowById, flowView, routeRunsOn, useBoardFlows} from './boardFlows'

const status = {
    id: 'status',
    name: 'Статус',
    type: 'select',
    options: [
        {id: 'todo', value: 'Не начата', color: ''},
        {id: 'work', value: 'В работе', color: ''},
        {id: 'review', value: 'На ревью', color: ''},
        {id: 'deploy', value: 'Деплой', color: ''},
    ],
} as IPropertyTemplate

const feature: Flow = {
    id: 'flow-feature',
    name: 'Фича',
    property: 'Статус',
    nodes: [
        {id: 'work', column: 'В работе', optionId: 'work', action: ''},
        {id: 'deploy', column: 'Деплой', optionId: 'deploy', action: ''},
    ],
    edges: [],
}

function group(optionId: string, cardIds: string[]): BoardGroup {
    const option = status.options.find((o) => o.id === optionId)
    return {
        option: option || {id: optionId, value: optionId, color: ''},
        cards: cardIds.map((id) => ({id} as never)),
    } as BoardGroup
}

const board = (): BoardGroup[] => [
    group('', ['loose']),
    group('todo', ['fresh', 'hotfix']),
    group('work', ['building']),
    group('review', ['reviewing']),
    group('deploy', []),
]

describe('a board drawn by one of its routes', () => {
    // The point of the whole thing: a route of two stages should not be read
    // off a kanban of five columns.
    test('draws the route’s own stages and drops the rest', () => {
        const {groups} = flowView(board(), feature, {}, status)
        expect(groups.map((g) => g.option.id)).toEqual(['todo', 'work', 'deploy'])
    })

    // A card arrives before anything has decided which route it takes, so the
    // column it arrives in belongs to every route — and the line says so.
    test('keeps the first column whatever the route, and marks it off', () => {
        const {groups, dividerAfter} = flowView(board(), feature, {}, status)
        expect(groups[0].option.id).toEqual('todo')
        expect(dividerAfter).toEqual('todo')
    })

    // Two routes share a column; a card travelling the other one is not this
    // route's business.
    test('drops the cards travelling another route', () => {
        const {groups} = flowView(board(), feature, {hotfix: 'flow-hotfix', building: 'flow-feature'}, status)
        expect(groups[0].cards.map((c) => c.id)).toEqual(['fresh'])
        expect(groups[1].cards.map((c) => c.id)).toEqual(['building'])
    })

    // A card on no route at all is nobody else's, so it is drawn everywhere.
    test('keeps the cards that are on no route', () => {
        const {groups} = flowView(board(), feature, {}, status)
        expect(groups[0].cards.map((c) => c.id)).toEqual(['fresh', 'hotfix'])
    })

    // The rule is not about a route: it divides the column cards arrive in from
    // what the board does with them, which is true of the whole board too.
    test('draws the whole board when no route is picked, and still divides off the intake column', () => {
        const {groups, dividerAfter} = flowView(board(), undefined, {}, status)
        expect(groups.length).toEqual(5)
        expect(dividerAfter).toEqual('todo')
    })

    // One column has nothing on the other side of the rule.
    test('draws no rule on a board of one column', () => {
        const one = board().slice(0, 1)
        expect(flowView(one, undefined, {}, status).dividerAfter).toEqual('')
    })

    // The inbox is grouped by who brought the card, and its columns are people.
    // A route has no stage among them, and narrowing would empty the screen.
    test('leaves a view grouped by something else alone', () => {
        const authors = {id: 'author', name: 'Автор', type: 'createdBy', options: []} as unknown as IPropertyTemplate
        const {groups, dividerAfter} = flowView(board(), feature, {}, authors)
        expect(groups.length).toEqual(5)
        expect(dividerAfter).toEqual('')
    })
})

describe('which property a route runs on', () => {
    test('is what the route says', () => {
        expect(routeRunsOn(feature, status)).toBe(true)
    })

    // A route written before it recorded its property is answered by its stages.
    test('is otherwise read off the stages themselves', () => {
        expect(routeRunsOn({...feature, property: ''}, status)).toBe(true)
        expect(routeRunsOn({...feature, property: '', nodes: [{id: 'x', column: 'Ничего', action: ''}]}, status)).toBe(false)
    })
})

describe('the route a view was pointed at', () => {
    // A route that has been deleted must not leave a view drawing nothing.
    test('is nothing once the board no longer has it', () => {
        expect(flowById([feature], 'flow-gone')).toBeUndefined()
        expect(flowById([feature], '')).toBeUndefined()
        expect(flowById([feature], 'flow-feature')).toEqual(feature)
    })
})

describe('the subscription behind the picker', () => {
    // The workspace keeps one centre panel across a board switch, so the hook is
    // never remounted: a route list from the board before it is the wrong list.
    test('follows the board it is pointed at', async () => {
        const routes: Record<string, string> = {
            'board-a': JSON.stringify([{id: 'a1', name: 'Фича', nodes: [], edges: []}]),
            'board-b': JSON.stringify([{id: 'b1', name: 'Уборка', nodes: [], edges: []}]),
        }
        vi.spyOn(bindings, 'agentBindings').mockReturnValue({
            ListFlows: (id: string) => Promise.resolve(routes[id] || '[]'),
            ListCardFlows: () => Promise.resolve('{}'),
        } as never)

        const [boardId, setBoardId] = createSignal('board-a')
        const seen = await createRoot(async (dispose) => {
            const held = useBoardFlows(boardId)

            // No render drives this root, so the hook's settle and each write
            // below are drained here.
            flush()
            await Promise.resolve()
            await Promise.resolve()
            const first = held.flows().map((f) => f.name)
            setBoardId('board-b')
            flush()
            await Promise.resolve()
            await Promise.resolve()
            const second = held.flows().map((f) => f.name)
            dispose()
            return [first, second]
        })
        expect(seen[0]).toEqual(['Фича'])
        expect(seen[1]).toEqual(['Уборка'])
    })
})
