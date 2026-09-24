package acp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Where a card stands, and what moves it.
//
// A board's columns are options of one of its select properties and mean
// whatever the people using the board mean by them. Nothing here configures
// them: the agent is told which column the card was dragged into and which
// columns the board has, and it moves the card on by naming one (move_card).
// The board is asked for them every time rather than remembered, because a
// column added this morning has to be offered this afternoon.

// boardStanding is the one paragraph a stage is told about the board: where the
// card is now and where it can be put. It is part of the task rather than of
// the tools' instructions, because whether a CLI has loaded its tools is its
// own business and the task text always arrives.
func (m *Manager) boardStanding(ev CardMoved) string {
	var b strings.Builder
	if column := strings.TrimSpace(ev.ToColumn.Name); column != "" {
		fmt.Fprintf(&b, "The card has just been moved into the «%s» column of the board.", column)
	}
	columns := m.boardColumnNames(ev.BoardID)
	if len(columns) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "The board's columns, in order: %s.", quoteAll(columns))
	}
	if b.Len() == 0 {
		return ""
	}
	b.WriteString("\nWhen you are done, move the card to the column it belongs in with move_card — " +
		"the one whose name says what has happened to it. Leave it where it is if none of them does.")
	return b.String()
}

// boardColumnNames is what the board's column property offers today.
func (m *Manager) boardColumnNames(boardID string) []string {
	if m.meta == nil || boardID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(m.rootOr(), 10*time.Second)
	defer cancel()
	options, err := m.forBoard(boardID).Meta.BoardColumnOptions(ctx, boardID)
	if err != nil {
		m.log.Warn("acp: cannot read the board's columns", "board", boardID, "err", err)
		return nil
	}
	names := make([]string, 0, len(options))
	for _, o := range options {
		if name := strings.TrimSpace(o.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// agentMoves remembers the cards an agent has just moved itself.
//
// A move is what starts a run, and an agent ends its run by moving the card
// (move_card). Without this the board would hand the card straight back to the
// agent that had just finished with it, for ever. The window is short and the
// entry is used once: what it has to survive is the round trip from the write
// to the board event, not the rest of the day.
type agentMoves struct {
	mu   sync.Mutex
	when map[string]time.Time
}

// agentMoveWindow is how long a move stays the agent's own. Generously longer
// than the board's own notification takes, and far shorter than anything a
// person would do next.
const agentMoveWindow = 2 * time.Minute

// noteAgentMove records that the agent moved this card itself.
func (m *Manager) noteAgentMove(cardID string) {
	if cardID == "" {
		return
	}
	m.moved.mu.Lock()
	defer m.moved.mu.Unlock()
	if m.moved.when == nil {
		m.moved.when = map[string]time.Time{}
	}
	m.moved.when[cardID] = time.Now()
}

// movedByAgent reports — and forgets — that this card's move was the agent's
// own. Forgetting on the way out is what keeps one move from silencing the
// person's next one.
func (m *Manager) movedByAgent(cardID string) bool {
	m.moved.mu.Lock()
	defer m.moved.mu.Unlock()
	at, ok := m.moved.when[cardID]
	if !ok {
		return false
	}
	delete(m.moved.when, cardID)
	return time.Since(at) < agentMoveWindow
}
