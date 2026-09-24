// What was done to a card, and by whom.
//
// Computed by the server out of `blocks_history` (`server/app/cardhistory.go`),
// where every write already left a copy of the block with the id of whoever
// made it. The page renders it and works nothing out for itself: naming a
// property or an option needs the board's schema, which lives there.

export type CardHistoryKind = 'created' | 'title' | 'property' | 'comment' | 'deleted'

export type CardHistoryEntry = {

    // When, in unix milliseconds.
    at: number

    // Who, as a board account id. An agent has one of its own, which is what
    // makes «кто именно» answerable; empty for a write made before anybody was
    // recorded.
    userId: string
    kind: CardHistoryKind

    // Which property changed, for a property entry, and what kind it is — so a
    // person property can be drawn as a person rather than as an id.
    property?: string
    propertyType?: string
    from?: string
    to?: string

    // What was said, for a comment.
    text?: string

    // Whether that comment came from an agent's run rather than from somebody
    // typing, which is what the card already draws differently.
    agent?: boolean
}
