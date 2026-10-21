package acp

import (
	"context"

	"github.com/artipop/pourquoipas/internal/author"
)

// Whose the writes of a run are.
//
// Every registered agent has a board account (`AgentEntry.UserID`), and the
// card's history is read off who made each write. Without this every write an
// agent makes arrives under the single account a desktop runs as — the same one
// the person uses — so the card could say what changed and never who changed
// it, which is the whole question.
//
// The id travels on the context (internal/author), because a context is what
// every write already carries and because this package must go on knowing
// nothing about the board.

// asAgent says the writes under this context are this session's agent's own.
// A session that has no account — an agent registered before accounts existed,
// or a board that could not make one — leaves the context alone and the write
// lands as it always did.
func (m *Manager) asAgent(ctx context.Context, s *Session) context.Context {
	if s == nil {
		return ctx
	}
	return author.With(ctx, s.Agent.UserID)
}

// asGrantedAgent is the same for a call arriving through the board tools, where
// the run is named by the grant rather than held in hand.
func (m *Manager) asGrantedAgent(ctx context.Context, g BoardGrant) context.Context {
	return m.asAgent(ctx, m.session(g.SessionID))
}
