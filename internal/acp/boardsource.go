package acp

import (
	"context"
	"sync"
)

// Where a board lives. The six fields the manager used to hold became the first
// source, and everything reads through forBoard/forCard instead.
//
// With one source registered — every install that has not attached to anything
// — both return it without asking anything at all.
//
// Routing needs no new interface method: the source that has the card is the
// one that answers CardByID, and the answer is remembered.
type BoardSource struct {
	// ID names the source: empty is the board this process serves, and a
	// server's origin is its own name.
	ID     string
	Writer BoardWriter
	Reader BoardReader
	Users  BoardUsers
	Meta   BoardMeta
	Events BoardEvents
}

// boardRouting is what a manager remembers so it does not ask twice.
type boardRouting struct {
	mu sync.Mutex
	// extra are the sources beyond the first. Empty on every install that
	// works its own board.
	extra []BoardSource
	// ofBoard and ofCard are indexes into the sources by id, filled as things
	// are found. A card cannot change source — it would have to change board,
	// and moving a card between servers is not something this app does.
	ofBoard map[string]int
	ofCard  map[string]int
}

// ensureIndexes makes the maps on first use.
func (r *boardRouting) ensureIndexes() {
	if r.ofBoard == nil {
		r.ofBoard = map[string]int{}
	}
	if r.ofCard == nil {
		r.ofCard = map[string]int{}
	}
}

// AddBoardSource registers a board this manager also works. The first source is
// the one the setters configure; this is for the ones after it.
func (m *Manager) AddBoardSource(src BoardSource) {
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	m.routing.extra = append(m.routing.extra, src)
}

// primary is the source the setters configured.
func (m *Manager) primary() BoardSource {
	return BoardSource{
		Writer: m.writer,
		Reader: m.reader,
		Users:  m.users,
		Meta:   m.meta,
	}
}

// sources is every source, primary first.
func (m *Manager) sources() []BoardSource {
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	out := make([]BoardSource, 0, len(m.routing.extra)+1)
	out = append(out, m.primary())
	return append(out, m.routing.extra...)
}

// only says there is nothing to route between, which is the ordinary install.
func (m *Manager) only() bool {
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	return len(m.routing.extra) == 0
}

func (m *Manager) sourceAt(i int) BoardSource {
	if i <= 0 {
		return m.primary()
	}
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	if i-1 < len(m.routing.extra) {
		return m.routing.extra[i-1]
	}
	return m.primary()
}

// Separate rather than one function taking a map: a map read outside the lock
// is a race whatever it is passed as.
func (m *Manager) rememberBoard(boardID string, at int) {
	if boardID == "" {
		return
	}
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	m.routing.ensureIndexes()
	m.routing.ofBoard[boardID] = at
}

func (m *Manager) rememberCard(cardID string, at int) {
	if cardID == "" {
		return
	}
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	m.routing.ensureIndexes()
	m.routing.ofCard[cardID] = at
}

func (m *Manager) recallBoard(boardID string) (int, bool) {
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	at, ok := m.routing.ofBoard[boardID]
	return at, ok
}

func (m *Manager) recallCard(cardID string) (int, bool) {
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	at, ok := m.routing.ofCard[cardID]
	return at, ok
}

// forBoard is the source a board lives on.
func (m *Manager) forBoard(boardID string) BoardSource {
	if m.only() {
		return m.primary()
	}
	if at, ok := m.recallBoard(boardID); ok {
		return m.sourceAt(at)
	}
	all := m.sources()
	for i, src := range all {
		if src.Meta == nil {
			continue
		}
		if _, err := src.Meta.BoardProperties(context.Background(), boardID); err == nil {
			m.rememberBoard(boardID, i)
			return src
		}
	}
	// Nobody claims it: the primary is the honest guess, and its own error is
	// what the caller sees rather than one invented here.
	return m.primary()
}

// forCard is the source a card lives on. Its board is remembered with it.
func (m *Manager) forCard(cardID string) BoardSource {
	if m.only() {
		return m.primary()
	}
	if at, ok := m.recallCard(cardID); ok {
		return m.sourceAt(at)
	}
	all := m.sources()
	for i, src := range all {
		if src.Reader == nil {
			continue
		}
		card, err := src.Reader.CardByID(context.Background(), cardID)
		if err != nil {
			continue
		}
		m.rememberCard(cardID, i)
		m.rememberBoard(card.BoardID, i)
		return src
	}
	return m.primary()
}

// forgetCardRoute drops what was remembered about a card. Called when a card is
// carried to another board, which is the one thing that can move it.
func (m *Manager) forgetCardRoute(cardID string) {
	m.routing.mu.Lock()
	defer m.routing.mu.Unlock()
	delete(m.routing.ofCard, cardID)
}

// claimStage takes a card move for this install, so the burst of patches one
// drag produces starts one session and not four.
func (m *Manager) claimStage(cardID, token string) bool {
	window := m.cfg.IdempotencyWindow()
	fresh, err := m.store.ClaimIdempotency(token, "", window)
	if err != nil {
		m.log.Error("acp: idempotency check failed", "err", err)
		return false
	}
	return fresh
}
