import {forgetMovedHere, movedHere, wasMovedHere} from './movedHere'

beforeEach(forgetMovedHere)

// The window remembers what its own hand moved, because the route cannot tell:
// it sees one board event whether a person dragged the card or the route
// advanced it.
it('remembers a card this window moved', () => {
    movedHere('card1')
    expect(wasMovedHere('card1')).toBe(true)
})

// Asked once. The question that move raised is asked in one place, and a second
// wait about the same card is a second thing to look at rather than a repeat.
it('answers once and forgets', () => {
    movedHere('card1')
    expect(wasMovedHere('card1')).toBe(true)
    expect(wasMovedHere('card1')).toBe(false)
})

// A move long past is not the gesture somebody is still making, so the wait it
// belongs to goes where every other wait goes.
it('forgets a move that is no longer the thing somebody just did', () => {
    movedHere('card1', 1_000)
    expect(wasMovedHere('card1', 1_000 + 60_000)).toBe(false)
})

// And the map does not grow across a session: a stale entry is dropped by the
// next move rather than kept for ever.
it('drops what has gone stale when the next card is moved', () => {
    movedHere('old', 1_000)
    movedHere('fresh', 1_000 + 60_000)
    expect(wasMovedHere('old', 1_000 + 60_000)).toBe(false)
    expect(wasMovedHere('fresh', 1_000 + 60_000)).toBe(true)
})
