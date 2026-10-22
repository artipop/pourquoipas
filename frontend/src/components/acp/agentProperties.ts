import {Board, IPropertyTemplate} from '../../blocks/board'
import {aiEnabled} from '../../aiSwitch'
import {isHosted} from '../../buildKind'

import {BOARD_PROP_PROJECT_PROPERTY, boardBranchProperty} from './automation'

// The card fields that are the agents' own: where the work is (the folder) and
// what it is on (the branch). The board records each by id, so they are found
// the way Go finds them.
export function agentPropertyIds(board: Board): Set<string> {
    const folder = board.properties?.[BOARD_PROP_PROJECT_PROPERTY]
    return new Set([typeof folder === 'string' ? folder : '', boardBranchProperty(board)].filter(Boolean))
}

// withoutAgentProperties is what a card shows while the AI is switched off,
// and on a hosted server, where there is no folder to work in and no branch to
// work on: the board's own fields, and none of the agents'. Only hidden — the
// values stay on the card.
export function withoutAgentProperties(board: Board, templates: IPropertyTemplate[]): IPropertyTemplate[] {
    if (aiEnabled() && !isHosted()) {
        return templates
    }
    const agents = agentPropertyIds(board)
    return agents.size ? templates.filter((t) => !agents.has(t.id)) : templates
}
