// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Accessor, createEffect, createSignal, onSettled} from 'solid-js'

import {BoardGroup, IPropertyTemplate} from '../../blocks/board'

import {agentBindings} from './bindings'
import {onAgentEvent} from './agentEvents'
import {Flow, nodeFor} from './automation'

// A board seen one route at a time. A board carries several routes — not every
// task is deployed — and its kanban draws the union of them: columns no card of
// this route will ever stand in, and cards travelling somebody else's. Picking
// a route narrows the board to the stages that route goes through and to the
// cards on it.
//
// The first column is the exception, and it is the reason the divider exists: a
// card arrives there before anything has decided which route it takes, so it
// belongs to every route's view and is not a stage of any of them.

const [flows, setFlows] = createSignal<Flow[]>([])
const [cardFlows, setCardFlows] = createSignal<Record<string, string>>({})

// Counted rather than assumed, as in cardMarks and liveTerminals: the
// subscription is worth holding only while a board is drawing by route.
let consumers = 0
let unsubscribe: (() => void) | undefined
let board = ''

async function reload(): Promise<void> {
    const bindings = agentBindings()
    if (!bindings || !board) {
        setFlows([])
        setCardFlows({})
        return
    }
    try {
        setFlows(JSON.parse(await bindings.ListFlows(board)) || [])
    } catch (e) {
        setFlows([])
    }
    try {
        setCardFlows(bindings.ListCardFlows ? JSON.parse(await bindings.ListCardFlows(board)) || {} : {})
    } catch (e) {
        setCardFlows({})
    }
}

function subscribe(boardId: string): () => void {
    board = boardId
    consumers++
    if (consumers === 1) {
        // A card moving, a stage starting or finishing — each changes which
        // route a card is on, and all of them arrive as this one event.
        unsubscribe = onAgentEvent('acp:session', () => reload())
    }
    reload()
    return () => {
        consumers--
        if (consumers === 0) {
            unsubscribe?.()
            unsubscribe = undefined

            // After the disposal, not during it: see attention.ts.
            queueMicrotask(() => {
                if (consumers === 0) {
                    setFlows([])
                    setCardFlows({})
                }
            })
        }
    }
}

// retarget points the subscription at another board. The workspace keeps one
// centre panel across a board switch — the component is never remounted — so
// without this the picker would go on offering the previous board's routes.
function retarget(boardId: string): void {
    if (consumers === 0 || boardId === board) {
        return
    }
    board = boardId
    setFlows([])
    setCardFlows({})
    reload()
}

// reloadBoardFlows is what an edit to the board's automation asks for: a route
// added, renamed or deleted changes what the picker offers.
export function reloadBoardFlows(): void {
    if (consumers > 0) {
        reload()
    }
}

// BoardRoutes is what a board drawn by route needs: which routes it has, and
// which of them each card is on. The two are read together and invalidated by
// the same event, so they are one subscription rather than two.
export type BoardRoutes = {
    flows: Accessor<Flow[]>
    onFlow: Accessor<Record<string, string>>
}

// useBoardFlows holds that subscription for as long as the component asking
// lives. A card missing from onFlow is on no route and is drawn under every one.
export function useBoardFlows(boardId: Accessor<string>): BoardRoutes {
    onSettled(() => subscribe(boardId()))
    createEffect(boardId, (id) => {
        retarget(id)
    })
    return {flows, onFlow: cardFlows}
}

// flowById is the route a view was pointed at, if the board still has it. A
// route that has been deleted answers with nothing, and the board is then drawn
// whole rather than empty.
export function flowById(list: Flow[], id: string): Flow | undefined {
    if (!id) {
        return undefined
    }
    return list.find((f) => f.id === id)
}

// FlowView is the board narrowed to one route: which columns are drawn, and
// after which of them the intake column is separated off.
export type FlowView = {
    groups: BoardGroup[]

    // The column cards arrive in, by option id, with a rule drawn after it.
    // Empty where the division means nothing: a board grouped by a person, or
    // one with a single column.
    dividerAfter: string
}

// routeRunsOn says whether a route's stages are options of the property this
// view groups by. A route is a path across one select property's options, and a
// board grouped by something else — the inbox, grouped by who brought the card
// — has no column a stage could stand in: narrowing it by a route would leave
// one group and hide the rest for no reason anybody could see.
export function routeRunsOn(flow: Flow, property?: IPropertyTemplate): boolean {
    if (!property) {
        return false
    }
    if (flow.property) {
        return flow.property.toLowerCase() === property.name.toLowerCase()
    }

    // A route written before it recorded its property answers structurally:
    // the stages are options of this one.
    return flow.nodes.some((n) => property.options.some((o) =>
        (n.optionId && n.optionId === o.id) || o.value.toLowerCase() === n.column.toLowerCase()))
}

// intakeColumn is the column cards arrive in, which is the view's own first
// column group and never a name: each shipped board names that column its own
// way and nothing here may know which.
//
// The rule after it divides what arrives from what the board does with it, so
// it is drawn whether or not a route is picked — narrowing to one route only
// takes columns away from the right of it. Two boards have nothing to divide:
// one grouped by a person, where the groups are authors rather than places (the
// inbox is that board), and one with a single column, where a rule after the
// only group is furniture.
function intakeColumn(groups: BoardGroup[], property?: IPropertyTemplate): string {
    if (property?.type === 'createdBy' || property?.type === 'updatedBy' || property?.type === 'person') {
        return ''
    }
    const columns = groups.filter((g) => g.option.id)
    return columns.length > 1 ? columns[0].option.id : ''
}

// flowView narrows the board's columns and cards to one route.
//
// Columns keep the board's own order rather than the route's: the order is the
// view's, and the arrows are what say where a card goes next. What is dropped
// is the group of cards standing in no column at all — a card with no column is
// on no stage, so no route passes through it.
export function flowView(groups: BoardGroup[], flow: Flow | undefined, onFlow: Record<string, string>, property?: IPropertyTemplate): FlowView {
    const dividerAfter = intakeColumn(groups, property)
    if (!flow || !routeRunsOn(flow, property)) {
        return {groups, dividerAfter}
    }
    const columns = groups.filter((g) => g.option.id)
    if (columns.length === 0) {
        return {groups: columns, dividerAfter: ''}
    }
    const kept = columns.filter((g, i) => i === 0 || Boolean(nodeFor(flow, {
        optionId: g.option.id,
        name: g.option.value,
        color: g.option.color,
    })))
    return {
        groups: kept.map((g) => ({...g, cards: g.cards.filter((c) => !onFlow[c.id] || onFlow[c.id] === flow.id)})),

        // Narrowing can leave the intake column alone on screen — a route whose
        // stages are all hidden from this view — and a rule after the only
        // column divides nothing.
        dividerAfter: kept.length > 1 ? dividerAfter : '',
    }
}
