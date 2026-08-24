// Which cards this window has just moved.
//
// A transition can hold a card until somebody answers, and where that question
// is put depends on who caused it. The route moves cards by itself, and then
// there may be nobody at the screen at all: the card's own strip and the
// notification are the whole of what can be done, and they are what the wait
// has always been drawn as. But a person dragging a card across the board is
// looking at it, and pointing them back at the card they are already holding is
// a worse answer than asking on the spot.
//
// The engine cannot tell the two apart — it sees one board event either way, a
// round trip after the drag — so the window remembers what it moved and for how
// long that is still the same gesture.
//
// Window-scoped and deliberately not stored: a card moved in another window,
// or before a reload, is a card whose mover is no longer looking at this one.

// How long a move stays "the thing I just did". The wait arrives a board round
// trip after the drag — the property write, the websocket, then the engine —
// and a few seconds covers that with room to spare, while a stale entry only
// means one dialog too many rather than a wrong answer.
const graceMs = 15_000

const moved = new Map<string, number>()

// movedHere records that this window moved a card, and forgets what has gone
// stale on the way past, so the map cannot grow across a long session.
export function movedHere(cardId: string, at: number = Date.now()): void {
    if (!cardId) {
        return
    }
    for (const [id, when] of moved) {
        if (at - when > graceMs) {
            moved.delete(id)
        }
    }
    moved.set(cardId, at)
}

// wasMovedHere reports whether the card was moved from this window just now,
// and takes the record with it: the question that move raised is asked once,
// and a second wait about the same card is a second thing to look at.
export function wasMovedHere(cardId: string, now: number = Date.now()): boolean {
    const when = moved.get(cardId)
    if (when === undefined) {
        return false
    }
    moved.delete(cardId)
    return now - when <= graceMs
}

// forgetMovedHere is for tests, which share a module across cases.
export function forgetMovedHere(): void {
    moved.clear()
}
