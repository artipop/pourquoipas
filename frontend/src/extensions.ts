import {IUser} from './user'

// Where features built on top of the board plug into it. The board calls what
// is registered here and imports nothing that fills it, so it builds and runs
// with nothing registered at all; the features register themselves from
// main.tsx (docs/architecture.md, «The board knows nothing of the agents»).

// People a board knows without them being its members: the agents a person
// has, which may be assigned a card on any board (docs/modes.md).
type BoardUsersSource = (boardId: string) => Promise<IUser[]>

const boardUsersSources: BoardUsersSource[] = []
const boardUsersListeners: Array<() => void> = []

export function addBoardUsersSource(source: BoardUsersSource): void {
    boardUsersSources.push(source)
}

export async function extraBoardUsers(boardId: string): Promise<IUser[]> {
    const found = await Promise.all(boardUsersSources.map((source) => source(boardId).catch(() => [] as IUser[])))
    return found.flat()
}

// A source says its answer changed (an agent was connected); the board that is
// open asks again.
export function boardUsersChanged(): void {
    boardUsersListeners.forEach((listener) => listener())
}

export function onBoardUsersChanged(listener: () => void): () => void {
    boardUsersListeners.push(listener)
    return () => {
        const at = boardUsersListeners.indexOf(listener)
        if (at >= 0) {
            boardUsersListeners.splice(at, 1)
        }
    }
}

// For tests: forget everything registered.
export function resetExtensions(): void {
    boardUsersSources.length = 0
    boardUsersListeners.length = 0
}
