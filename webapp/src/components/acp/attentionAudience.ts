import {Board, IPropertyTemplate} from '../../blocks/board'
import {Card} from '../../blocks/card'

// Who an agent's wait is announced to: the card says, through the assignee field
// (docs/teamwork.md).

// personProperty is found by *type*, never by name: the name is the board's.
export function personProperty(board?: Board): IPropertyTemplate | undefined {
    if (!board?.cardProperties) {
        return undefined
    }
    return board.cardProperties.find((p) => p.type === 'person' || p.type === 'multiPerson')
}

// waitAudience is the user ids the card names; empty means everybody.
export function waitAudience(card?: Card, board?: Board): string[] {
    const property = personProperty(board)
    if (!card || !property) {
        return []
    }
    const value = card.fields?.properties?.[property.id]
    if (!value) {
        return []
    }
    return (Array.isArray(value) ? value : [value]).filter(Boolean)
}

// waitIsMine: on an install of one person every wait is theirs.
export function waitIsMine(args: {
    teamMode: boolean
    myId?: string
    audience: string[]
    following: boolean
}): boolean {
    if (!args.teamMode || args.audience.length === 0 || args.following) {
        return true
    }
    return Boolean(args.myId) && args.audience.includes(args.myId!)
}
