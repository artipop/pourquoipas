import {BoardView, createBoardView} from '../blocks/boardView'
import {Block} from '../blocks/block'
import {Utils} from '../utils'

import {getCurrentBoard} from './boards'

import type {StoreContext} from './context'

import type {RootState} from './index'

export type ViewsState = {
    current: string
    views: {[key: string]: BoardView}
}

export const initialViewsState = (): ViewsState => ({views: {}, current: ''})

// isEqual compares what a view's fields are made of: arrays, plain objects,
// strings and numbers as they came out of JSON. Written here rather than taken
// from a library because this is the only place the page compares by value, and
// like lodash's it counts a key that is present but undefined as a difference.
function isEqual(a: unknown, b: unknown): boolean {
    if (a === b) {
        return true
    }
    if (Array.isArray(a) && Array.isArray(b)) {
        return a.length === b.length && a.every((item, i) => isEqual(item, b[i]))
    }
    if (typeof a !== 'object' || typeof b !== 'object' || a === null || b === null) {
        return false
    }
    const keys = Object.keys(a)
    if (keys.length !== Object.keys(b).length) {
        return false
    }

    // hasOwnProperty through call, not Object.hasOwn: the page still supports a
    // Safari that predates it, and this file is not the place to find that out.
    return keys.every((key) =>
        Object.prototype.hasOwnProperty.call(b, key) &&
        isEqual((a as Record<string, unknown>)[key], (b as Record<string, unknown>)[key]))
}

// This update ensure that we are not regenerating that fields all the time
const smartViewUpdate = (oldView: BoardView, newView: BoardView) => {
    if (!oldView) {
        return newView
    }

    if (isEqual(newView.fields.sortOptions, oldView.fields.sortOptions)) {
        newView.fields.sortOptions = oldView.fields.sortOptions
    }
    if (isEqual(newView.fields.visiblePropertyIds, oldView.fields.visiblePropertyIds)) {
        newView.fields.visiblePropertyIds = oldView.fields.visiblePropertyIds
    }
    if (isEqual(newView.fields.visibleOptionIds, oldView.fields.visibleOptionIds)) {
        newView.fields.visibleOptionIds = oldView.fields.visibleOptionIds
    }
    if (isEqual(newView.fields.hiddenOptionIds, oldView.fields.hiddenOptionIds)) {
        newView.fields.hiddenOptionIds = oldView.fields.hiddenOptionIds
    }
    if (isEqual(newView.fields.collapsedOptionIds, oldView.fields.collapsedOptionIds)) {
        newView.fields.collapsedOptionIds = oldView.fields.collapsedOptionIds
    }
    if (isEqual(newView.fields.filter, oldView.fields.filter)) {
        newView.fields.filter = oldView.fields.filter
    }
    if (isEqual(newView.fields.cardOrder, oldView.fields.cardOrder)) {
        newView.fields.cardOrder = oldView.fields.cardOrder
    }
    if (isEqual(newView.fields.columnWidths, oldView.fields.columnWidths)) {
        newView.fields.columnWidths = oldView.fields.columnWidths
    }
    if (isEqual(newView.fields.columnCalculations, oldView.fields.columnCalculations)) {
        newView.fields.columnCalculations = oldView.fields.columnCalculations
    }
    if (isEqual(newView.fields.kanbanCalculations, oldView.fields.kanbanCalculations)) {
        newView.fields.kanbanCalculations = oldView.fields.kanbanCalculations
    }
    return newView
}

// The views a fresh board load carries: every full (re)load rebuilds the map
// from the block list.
export const viewsFromBlocks = (blocks: Block[]): {[key: string]: BoardView} => {
    const views: {[key: string]: BoardView} = {}
    for (const block of blocks) {
        if (block.type === 'view') {
            views[block.id] = block as BoardView
        }
    }
    return views
}

export const createViewsActions = ({setState}: StoreContext) => ({
    setCurrent(viewId: string) {
        setState((draft) => {
            draft.views.current = viewId
        })
    },
    updateViews(views: BoardView[]) {
        setState(({views: {views: stateViews}}) => {
            for (const view of views) {
                if (view.deleteAt === 0) {
                    stateViews[view.id] = smartViewUpdate(stateViews[view.id], view)
                } else {
                    delete stateViews[view.id]
                }
            }
        })
    },
    updateView(view: BoardView) {
        setState((draft) => {
            draft.views.views[view.id] = view
        })
    },
    setViews(views: {[key: string]: BoardView}) {
        setState((draft) => {
            draft.views.views = views
        })
    },
})

export const getViews = (state: RootState): {[key: string]: BoardView} => state.views.views

export const getSortedViews = (state: RootState): BoardView[] =>
    Object.values(getViews(state)).sort((a, b) => a.title.localeCompare(b.title)).map((v) => createBoardView(v))

export const getViewsByBoard = (state: RootState): {[key: string]: BoardView[]} => {
    const result: {[key: string]: BoardView[]} = {}
    Object.values(getViews(state)).forEach((view) => {
        if (result[view.parentId]) {
            result[view.parentId].push(view)
        } else {
            result[view.parentId] = [view]
        }
    })
    return result
}

export function getView(viewId: string): (state: RootState) => BoardView|null {
    return (state: RootState): BoardView|null => {
        return state.views.views[viewId] || null
    }
}

export const getCurrentBoardViews = (state: RootState): BoardView[] => {
    const boardId = state.boards.current
    const views = getViews(state)
    Utils.log(`getCurrentBoardViews boardId: ${boardId} views: ${views.length}`)
    return Object.values(views).filter((v) => v.boardId === boardId).sort((a, b) => a.title.localeCompare(b.title)).map((v) => createBoardView(v))
}

export const getCurrentViewId = (state: RootState): string => state.views.current

export const getCurrentView = (state: RootState): BoardView => getViews(state)[getCurrentViewId(state)]

export const getCurrentViewGroupBy = (state: RootState) => {
    const currentBoard = getCurrentBoard(state)
    const currentView = getCurrentView(state)
    if (!currentBoard) {
        return undefined
    }
    if (!currentView) {
        return undefined
    }
    return currentBoard.cardProperties.find((o) => o.id === currentView.fields.groupById)
}

export const getCurrentViewDisplayBy = (state: RootState) => {
    const currentBoard = getCurrentBoard(state)
    const currentView = getCurrentView(state)
    if (!currentBoard) {
        return undefined
    }
    if (!currentView) {
        return undefined
    }
    return currentBoard.cardProperties.find((o) => o.id === currentView.fields.dateDisplayPropertyId)
}

// oldestView is the view a board was made with: the ties are broken by id so
// two views written in the same millisecond still resolve to the same one every
// time, rather than to whichever the store happened to list first.
//
// It is what decides which view a board — or a template's preview — opens on.
// Sorting by title, which is right for the sidebar, is wrong for that: a view
// added later and named the inbox view took the board over from a chores board.
export const oldestView = (views: BoardView[]): BoardView =>
    views.reduce((oldest, view) => {
        if (view.createAt !== oldest.createAt) {
            return view.createAt < oldest.createAt ? view : oldest
        }
        return view.id < oldest.id ? view : oldest
    })
