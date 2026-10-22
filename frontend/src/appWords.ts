import {Board} from './blocks/board'
import {Block} from './blocks/block'
import {Constants} from './constants'
import {goApp} from './goBridge'

// The boards the app made itself — the install's own templates and the inbox —
// are stored in English and translated here, as they arrive, into the language
// this page shows (internal/boardadapter/templatetext.go). On a server one
// board is read in several languages at once, so the words cannot be stored in
// any of them. A board made from a template is translated by Go as it is
// copied, and is the person's data from then on: nothing here touches it.

type Catalogue = {fields: Set<string>, words: Record<string, string | undefined>}

let catalogue: Catalogue = {fields: new Set(), words: {}}
let loadedFor: string | undefined
let asking: string | undefined

// A block arrives without its board, so which boards are the app's is
// remembered as they pass.
const templateBoards = new Set<string>()
const inboxBoards = new Set<string>()

// loadAppWords fetches the catalogue for a language, and says whether it is a
// different one from what boards already on the page were translated with.
export async function loadAppWords(lang: string): Promise<boolean> {
    if (lang === loadedFor && asking === undefined) {
        return false
    }
    asking = lang
    let got: {fields?: string[], words?: Record<string, string>} | undefined
    try {
        // eslint-disable-next-line new-cap
        got = await goApp()?.TemplateWords?.(lang)
    } catch (e) {
        // No Go side yet — a server's page before the login, which guards the
        // bindings — or a binding that threw: the boards are shown in English,
        // and the next call asks again.
    }

    // Overtaken by a later language while this one was being fetched.
    if (asking !== lang) {
        return false
    }
    asking = undefined

    // Nothing answered: what was loaded stays, English when nothing was.
    if (!got) {
        return false
    }
    catalogue = {fields: new Set(got.fields || []), words: got.words || {}}
    const changed = loadedFor !== undefined && loadedFor !== lang
    loadedFor = lang
    return changed
}

// appWordsLanguage is the language the catalogue is in, which is the one the
// page shows: a board copied from a template is asked to be made in it.
export function appWordsLanguage(): string {
    return loadedFor || ''
}

const word = (s: string): string => catalogue.words[s] || s

// inboxTitle is the inbox's name as the phone and the share dialog list it,
// which come from bindings rather than through the board's client.
export function inboxTitle(title: string): string {
    return word(title)
}

// The same walk as Go's translateValue: the key a string arrived under travels
// with it through lists, so an option's `value` is translated and a card's
// `properties`, keyed by property id, is not.
function walk(value: unknown, key: string): unknown {
    if (Array.isArray(value)) {
        return value.map((item) => walk(item, key))
    }
    if (value && typeof value === 'object') {
        const out: Record<string, unknown> = {}
        for (const [k, v] of Object.entries(value)) {
            out[k] = walk(v, k)
        }
        return out
    }
    if (typeof value === 'string' && catalogue.fields.has(key)) {
        return word(value)
    }
    return value
}

function isInstallTemplate(board: Board): boolean {
    return board.isTemplate && board.createdBy === Constants.SystemUserID
}

function isInbox(board: Board): boolean {
    return !board.isTemplate && (board.properties as Record<string, unknown> | undefined)?.inbox === true
}

// translateBoard is a board as this page shows it. The inbox has only its own
// names translated — its title and its fields — because everything else on it
// is what people and sources wrote.
export function translateBoard<T extends Board | undefined>(board: T): T {
    if (!board) {
        return board
    }
    if (isInstallTemplate(board)) {
        templateBoards.add(board.id)
        return walk(board, '') as T
    }
    if (isInbox(board)) {
        inboxBoards.add(board.id)
        return {
            ...board,
            title: word(board.title),
            cardProperties: board.cardProperties?.map((p) => ({...p, name: word(p.name)})),
        }
    }
    return board
}

export function translateBoards<T extends Board>(boards: T[]): T[] {
    return boards.map((b) => translateBoard(b))
}

// translateBlocks does the same for blocks: everything on a template, and on
// the inbox only the title of a view, which the app named.
export function translateBlocks<T extends Block>(blocks: T[]): T[] {
    return blocks.map((block) => {
        if (templateBoards.has(block.boardId)) {
            return walk(block, '') as T
        }
        if (inboxBoards.has(block.boardId) && block.type === 'view') {
            return {...block, title: word(block.title)}
        }
        return block
    })
}

// resetAppWords is for tests, which load catalogues of their own.
export function resetAppWords(): void {
    catalogue = {fields: new Set(), words: {}}
    loadedFor = undefined
    asking = undefined
    templateBoards.clear()
    inboxBoards.clear()
}
