// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Board, IPropertyOption} from '../../blocks/board'
import {goApp} from '../../goBridge'

import {FieldNames, Workdir, syncWorkdirsToBoard, useWorkdirHere} from './workdirSync'

// A folder is asked on the card, the first time a card wants one, rather than
// when the board is made: a card without one is worked in drafts of its own,
// so the board has no reason to insist, and the person making a card is the
// one who knows where its work lives. Added once, a folder is an option of the
// board's field and every other card picks it from the list.

// canAddFolder says this build can show a folder picker at all. A browser
// served by a headless server has no disk of the person's to pick from.
export function canAddFolder(): boolean {
    const bindings = goApp()
    return typeof bindings?.PickDirectory === 'function' && typeof bindings.AddAgentWorkdir === 'function'
}

export function folderBasename(path: string): string {
    return path.split(/[/\\]/).filter(Boolean).pop() || path
}

// A folder's name is unique on the machine, and two projects called `app` are
// common enough. The second is numbered rather than refused: nobody asked for
// a name here, so there is nobody to ask for another one.
const NAME_ATTEMPTS = 20

function nameTaken(e: unknown): boolean {
    return (e as {cause?: {code?: string}} | null)?.cause?.code === 'folders.name-taken'
}

async function register(path: string, boardId: string): Promise<Workdir> {
    const bindings = goApp()!
    const base = folderBasename(path)
    for (let n = 1; ; n++) {
        const name = n === 1 ? base : `${base} ${n}`
        try {
            // One at a time on purpose: the next name is only worth trying
            // once this one was refused.
            // eslint-disable-next-line no-await-in-loop
            return JSON.parse(await bindings.AddAgentWorkdir(name, path, boardId, '', false)) as Workdir
        } catch (e) {
            if (!nameTaken(e) || n >= NAME_ATTEMPTS) {
                throw e
            }
        }
    }
}

// The option of the board's folder field that stands for this entry: created
// under the entry's id, or — on a board that listed the folder before options
// carried ids — the one with its name.
function optionFor(entry: Workdir, options: IPropertyOption[]): IPropertyOption | undefined {
    return options.find((o) => entry.id && o.id === entry.id) ||
        options.find((o) => o.value.trim().toLowerCase() === entry.name.trim().toLowerCase())
}

export type AddedFolder = {propertyId: string, optionId: string}

// pickFolderForBoard asks for a folder and makes it one this board offers,
// returning the field and the option a card should be set to. Undefined when
// the person closed the picker.
//
// A folder the machine already knows is used, not added again: attached to
// this board, or made every board's when another board has it — the same
// answer the folders panel gives.
export async function pickFolderForBoard(board: Board, title: string, names: FieldNames): Promise<AddedFolder | undefined> {
    const bindings = goApp()
    if (!bindings || !canAddFolder()) {
        return undefined
    }
    const path = await bindings.PickDirectory(title)
    if (!path) {
        return undefined
    }

    const known = JSON.parse(await bindings.FindAgentWorkdir?.(path) || 'null') as Workdir | null
    let entry: Workdir
    if (known) {
        await useWorkdirHere(known, board.id)
        entry = known
    } else {
        entry = await register(path, board.id)
    }

    // The registry as this board sees it, with the chosen folder counted as
    // the board's own: a folder every board shares is otherwise kept off a
    // board that has no folder field yet, and this one is here because a
    // person asked for it on this board.
    const registry = (JSON.parse(await bindings.ListAgentWorkdirs(board.id)) || []) as Workdir[]
    const chosen = registry.find((r) => (entry.id && r.id === entry.id) || r.path === entry.path) || entry
    const mirrored = [...registry.filter((r) => r !== chosen), {...chosen, global: false}]

    const {property} = await syncWorkdirsToBoard(board, mirrored, names)
    const option = property && optionFor(chosen, property.options)
    if (!property || !option) {
        return undefined
    }
    return {propertyId: property.id, optionId: option.id}
}
