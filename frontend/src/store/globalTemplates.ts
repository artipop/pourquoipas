import {Board} from '../blocks/board'
import {Constants} from '../constants'

import type {StoreContext} from './context'

import type {RootState} from './index'

export type GlobalTemplatesState = {value: Board[]}

export const initialGlobalTemplatesState = (): GlobalTemplatesState => ({value: []})

// Who the install's own templates are made by. The app replaces them whole on
// an upgrade, so a copy the page is still holding after that is a board that no
// longer exists.
const SYSTEM_USER = 'system'

export const createGlobalTemplatesActions = ({setState, deps}: StoreContext) => ({
    async fetchGlobalTemplates(): Promise<void> {
        // The fallback is before the sort, not after it: a request that came
        // back with nothing used to throw here, and `|| []` on the far side of
        // `.sort` could never have caught it.
        const templates = await deps.client.getTeamTemplates(Constants.globalTeamId) || []
        setState((draft) => {
            draft.globalTemplates.value = templates.sort((a, b) => a.title.localeCompare(b.title))
        })

        // And the board list is pruned of the install's templates this answer
        // no longer names. The selector reads both sources as one pool, and a
        // template board is deleted without the page hearing of it: nobody is a
        // member of an open template, so the board's own deletion never reaches
        // this client, while the replacement arrives as a new board on the
        // team. Reinstalling them drew both copies at once until somebody
        // reloaded.
        //
        // Only what the install made (`SYSTEM_USER`): somebody's own template
        // lives wherever they made it, and this answer is about one team.
        const live = new Set(templates.map((board) => board.id))
        setState((draft) => {
            const held = draft.boards.templates
            const kept: {[key: string]: Board} = {}
            for (const [id, board] of Object.entries(held)) {
                if (board.createdBy !== SYSTEM_USER || live.has(id)) {
                    kept[id] = board
                }
            }
            draft.boards.templates = kept
        })
    },
})

export function getGlobalTemplates(state: RootState): Board[] {
    return state.globalTemplates.value
}
