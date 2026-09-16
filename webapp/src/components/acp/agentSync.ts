// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Board} from '../../blocks/board'

import {goApp} from '../../goBridge'

import {invalidateBoardAgents} from './boardAgents'

// Making the machine's agents usable on a board: an account each, so the board
// can name one the same way it names a person.
//
// There used to be a second way — an «Agent» select property this kept in step
// with the registry — and a card therefore had two fields for one question:
// who is doing this. Two answers meant a rule about which of them wins, a
// column crew that had to be checked against both, and a field that said
// nothing on a board where nobody had registered an agent. The assignee is the
// one that survives: it is the field a person already reaches for, it is
// already what the engine prefers (resolveSessionAgent), and an agent is a
// member of the board like anybody else.
//
// This used to live in the agents dialog, which had a board because it was
// opened from one. The registry is the machine's, and it is now edited where
// the machine is — so the sync moved to where a board actually is: the board's
// own automation screen, and the moment somebody adds an agent from a card.

type SyncResult = {

    // How many agent accounts had to be created. Zero is the ordinary case and
    // is worth saying nothing about.
    accounts: number

}

// syncAgentsToBoard is idempotent: a board that already knows every agent is
// left untouched, so this can ride along with an ordinary refresh without
// churning the board or its undo history.
export async function syncAgentsToBoard(board: Board): Promise<SyncResult> {
    const bindings = goApp()
    const result: SyncResult = {accounts: 0}
    if (!bindings?.ListAgents || !board?.id) {
        return result
    }

    const agents = (JSON.parse(await bindings.ListAgents()) || []) as Array<{name: string}>
    if (agents.length > 0 && bindings.SyncAgentUsers) {
        const synced = (JSON.parse(await bindings.SyncAgentUsers(board.id)) || []) as Array<{created?: boolean}>
        result.accounts = synced.filter((u) => u.created).length
    }

    // Opening a board is when the registry may have moved since anybody last
    // looked: an agent added in the settings is one this board's assignee list
    // has to know about, one way or the other.
    invalidateBoardAgents()

    return result
}
