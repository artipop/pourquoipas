// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Board, IPropertyOption, IPropertyTemplate} from '../../blocks/board'
import {Card, createCard} from '../../blocks/card'
import {goApp} from '../../goBridge'

import {personProperty} from './attentionAudience'
import {columnProperty} from './automation'

// The first task the setup wizard offers (docs/onboarding.md): one sentence
// from a person, made into a card assigned to an agent and moved from the
// board's first column into its second.
//
// Moved, not made there: a card starts when it is *moved* into a column
// (internal/acp/trigger.go), and a card created in a column is a card created,
// which starts nothing. So the card is born in the first column and patched
// into the second — the same two writes a person makes by adding a card and
// dragging it — and no configuration is added for it.

export type FirstTaskColumns = {
    property: IPropertyTemplate
    from: IPropertyOption
    to: IPropertyOption
}

// firstTaskColumns is where the card is born and where it is moved to: the
// first two columns as the view shows them, or, off a grouped view, the first
// two options of the board's column property. No two columns, no first task.
export function firstTaskColumns(board: Board, groupBy?: IPropertyTemplate, shown?: IPropertyOption[]): FirstTaskColumns | undefined {
    const property = groupBy?.type === 'select' ? groupBy : columnProperty(board)
    if (!property) {
        return undefined
    }
    const options = (property.id === groupBy?.id && shown?.length ? shown : property.options).filter((o) => o.id)
    if (options.length < 2) {
        return undefined
    }
    return {property, from: options[0], to: options[1]}
}

// agentAccountId is the board account of the agent with this name, made if the
// board has none yet: the wizard may have registered the agent a moment ago,
// before any board knew it.
export async function agentAccountId(boardId: string, agentName: string): Promise<string> {
    const bindings = goApp()
    if (!bindings?.SyncAgentUsers) {
        return ''
    }
    const users = (JSON.parse(await bindings.SyncAgentUsers(boardId)) || []) as Array<{name: string, userId?: string}>
    return users.find((u) => u.name === agentName)?.userId || ''
}

// firstTaskCard is the card as it is born: its title, the agent in the board's
// person field and the first column. extra is what every new card on the board
// gets besides (the view's filter, the board's only folder).
export function firstTaskCard(board: Board, columns: FirstTaskColumns, title: string, agentUserId: string, extra: Record<string, string>): Card | undefined {
    const assignee = personProperty(board)
    if (!assignee) {
        return undefined
    }
    const card = createCard()
    card.parentId = board.id
    card.boardId = board.id
    card.title = title
    card.fields.properties = {
        ...extra,
        [columns.property.id]: columns.from.id,
        [assignee.id]: assignee.type === 'multiPerson' ? [agentUserId] : agentUserId,
    }
    return card
}
