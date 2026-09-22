import {Board} from '../../blocks/board'

// Which board is the inbox.
//
// The inbox is one board the app makes (boardadapter.EnsureInboxBoard) and
// marks with a board property, and it is found by that mark — never by its
// title, which a person may change, and never by a view's title, which a person
// may give to a view of any board.
export const InboxBoardProperty = 'xciiiInbox'

// What the person's own column on the inbox is headed. The inbox view is
// grouped by author, so the cards a person typed there form a column headed by
// their username; that column *is* their unprocessed tasks, and it is headed by
// this name rather than by the "you" name. A given name, not a translation, like
// the inbox's own.
export const MineColumnTitle = 'Мои задачи'

export function isInboxBoard(board?: Board): boolean {
    // The mark is a JSON boolean, which the board's property type does not
    // describe: it was written for strings.
    return (board?.properties as Record<string, unknown> | undefined)?.[InboxBoardProperty] === true
}
