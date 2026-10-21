import {Board} from '../../blocks/board'

// Which board is the inbox.
//
// The inbox is one board the app makes (boardadapter.EnsureInboxBoard) and
// marks with a board property, and it is found by that mark — never by its
// title, which a person may change, and never by a view's title, which a person
// may give to a view of any board.
export const InboxBoardProperty = 'inbox'

export function isInboxBoard(board?: Board): boolean {
    // The mark is a JSON boolean, which the board's property type does not
    // describe: it was written for strings.
    return (board?.properties as Record<string, unknown> | undefined)?.[InboxBoardProperty] === true
}
