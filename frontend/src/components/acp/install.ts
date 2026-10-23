import {addBoardUsersSource} from '../../extensions'

import {agentBoardUsers} from './agentAssignees'

// Everything the agents add to the board, plugged in through extensions.ts.
// Called once from main.tsx; the board does not import this.
export function installAgentFeatures(): void {
    addBoardUsersSource(agentBoardUsers)
}
