package acp

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// A column holds as many sessions at once as it was given crew and permission
// for. A card that arrives at a full column is not refused and not failed — it
// waits, exactly as a card waits for a person, and starts by itself as soon as
// somebody finishes. That is what a WIP limit means on a board.

// runningInColumn counts the live sessions of one column.
//
// Asked of the database rather than of this process's own map, which is what it
// used to be. A column's limit is a fact about the board — how many of its crew
// may work it at once — and two windows of this app each counted their own
// sessions, so a limit of one ran two. A database that cannot be read counts
// nothing, which lets the stage start: refusing work because a count failed is
// the worse of the two wrong answers.
func (m *Manager) runningInColumn(key string) int {
	if key == "" {
		return 0
	}
	n, err := m.store.RunningInColumn(key)
	if err != nil {
		m.log.Error("acp: cannot count what is running in a column", "column", key, "err", err)
		return 0
	}
	return n
}

// columnByKey finds a configured column by the key its sessions carry.
func (m *Manager) columnByKey(key string) (ColumnSpec, bool) {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	for _, c := range m.cfg.Columns {
		if c.Key() == key {
			return c, true
		}
	}
	return ColumnSpec{}, false
}

// enqueueStage parks a card whose stage could not start because the column is
// full. Nothing is said on the card: the route strip already reports that a
// queued card is waiting for a free place, which is this state told live rather
// than as a comment that outlives it.
func (m *Manager) enqueueStage(ev CardMoved, spec ColumnSpec, nodeID string) {
	fresh, err := m.store.EnqueueStage(QueuedStage{
		CardID:    ev.CardID,
		BoardID:   ev.BoardID,
		ColumnKey: spec.Key(),
		NodeID:    nodeID,
	})
	if err != nil {
		m.log.Error("acp: cannot queue the card", "card", ev.CardID, "err", err)
		return
	}
	// Queued is progress of a kind: whatever stalled the card before is not
	// what it is waiting for now.
	m.clearStall(ev.CardID)
	m.log.Info("acp: card waiting for a place in the column",
		"card", ev.CardID, "column", spec.Column, "fresh", fresh)
}

// dequeueStage forgets a card that no longer waits: it started, or somebody
// moved it somewhere else.
func (m *Manager) dequeueStage(cardID string) {
	if err := m.store.DequeueStage(cardID); err != nil {
		m.log.Warn("acp: cannot clear the queue entry", "card", cardID, "err", err)
	}
}

// drainColumn starts the card that has waited longest for this column, if the
// column has room again. Called where a session releases its resources, so the
// place a session frees is filled by the next card rather than by nothing.
func (m *Manager) drainColumn(key string) {
	if key == "" || m.reader == nil {
		return
	}
	q, ok, err := m.store.NextQueuedStage(key)
	if err != nil {
		m.log.Error("acp: cannot read the stage queue", "err", err)
		return
	}
	if !ok {
		return
	}
	spec, ok := m.columnByKey(key)
	if !ok {
		// The column was reconfigured out of existence while the card waited.
		m.dequeueStage(q.CardID)
		return
	}

	ctx, cancel := context.WithTimeout(m.rootCtx, 10*time.Second)
	ev, err := m.forCard(q.CardID).Reader.CardByID(ctx, q.CardID)
	cancel()
	if err != nil {
		m.log.Warn("acp: cannot read a queued card", "card", q.CardID, "err", err)
		return
	}

	opts := startOptions{column: spec}
	switch spec.Action {
	case ColumnActionDeploy:
		opts.deploy = true
	case ColumnActionTest:
		opts.test = true
	case ColumnActionNone, "":
		m.dequeueStage(q.CardID)
		return
	}

	s, err := m.startSession(ev, opts)
	if errors.Is(err, errStageBusy) {
		return // somebody else took the place; the card keeps its turn
	}
	m.dequeueStage(q.CardID)
	// Somebody took the card while it waited: it leaves the queue and stays
	// where it is, rather than failing its stage.
	var mine AssignedToHumanError
	if errors.As(err, &mine) {
		m.sayCardIsTaken(q.CardID, q.NodeID, mine)
		return
	}
	if err != nil {
		m.log.Warn("acp: queued stage not started", "card", q.CardID, "column", spec.Column, "err", err)
		m.stallCard(q.CardID, q.NodeID, fmt.Sprintf("стадия «%s» не запустилась: %v", spec.Column, err))
		return
	}
	m.log.Info("acp: queued stage started", "session", s.ID, "card", q.CardID, "column", spec.Column)
}

// cardIsQueued reports that the card is waiting for a place in its column.
func (m *Manager) cardIsQueued(cardID string) bool {
	if m.store == nil || cardID == "" {
		return false
	}
	queued, err := m.store.StageQueued(cardID)
	if err != nil {
		m.log.Warn("acp: cannot read the card's place in the queue", "card", cardID, "err", err)
		return false
	}
	return queued
}
