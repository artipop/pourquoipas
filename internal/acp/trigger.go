package acp

import (
	"errors"
	"fmt"
)

// errSwitchedOff is what a start refused by the switch says.
var errSwitchedOff = errors.New("ИИ в приложении выключен")

// triggerLoop consumes normalized card-move events and applies the trigger
// policy: enter the trigger column → start a session (idempotently); leave it
// while a session is live → cancel.
func (m *Manager) triggerLoop(ch <-chan CardMoved) {
	defer m.wg.Done()
	for {
		select {
		case <-m.rootCtx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			m.handleEvent(ev)
		}
	}
}

func (m *Manager) handleEvent(ev CardMoved) {
	if m.aiSwitchedOff() {
		return
	}
	// A board may ship its own columns and routes (the template does); take
	// them once, before anything asks what this column does.
	m.seedFromBoard(ev.BoardID)

	to, entered := m.columnFor(ev.BoardID, ev.ToColumn)
	if entered && to.Action != ColumnActionNone {
		m.handleEnter(ev, to)
		return
	}
	if from, left := m.columnFor(ev.BoardID, ev.FromColumn); left && from.Action != ColumnActionNone {
		// A card that leaves stops waiting for the column it left — and stops
		// being stalled by it: whatever could not start there is moot now.
		m.dequeueStage(ev.CardID)
		m.clearStall(ev.CardID)
		if m.CancelSessionForCard(ev.CardID, "карточка убрана из триггерной колонки") {
			m.log.Info("acp: session cancelled by card move", "card", ev.CardID)
		}
	}
}

// handleEnter starts whatever the column does. The three kinds differ only in
// the options they start the session with and in what they say when they fail,
// so the guards, the idempotency key and the logging are shared.
func (m *Manager) handleEnter(ev CardMoved, spec ColumnSpec) {
	opts := startOptions{column: spec}
	kind, failed := "agent", "агент не запущен"
	switch spec.Action {
	case ColumnActionDeploy:
		opts.deploy, kind, failed = true, "deploy", "деплой не запущен"
	case ColumnActionTest:
		opts.test, kind, failed = true, "test", "тестирование не запущено"
	}
	if !m.claimMove(ev, kind) {
		return
	}
	s, err := m.startSession(ev, opts)
	if errors.Is(err, errStageBusy) {
		m.enqueueStage(ev, spec, "")
		return
	}
	var mine AssignedToHumanError
	if errors.As(err, &mine) {
		m.sayCardIsTaken(ev.CardID, "", mine)
		return
	}
	if err != nil {
		m.log.Warn("acp: session not started", "card", ev.CardID, "kind", kind, "err", err)
		m.stallCard(ev.CardID, "", fmt.Sprintf("%s: %v", failed, err))
		return
	}
	m.log.Info("acp: session started", "session", s.ID, "card", ev.CardID, "kind", kind, "workdir", s.WorkdirPath)
}

// claimIdempotent collapses the burst of patches one drag-and-drop produces
// into a single move. kind namespaces the key, so the agent, deploy, test and
// flow paths cannot suppress each other's events.
func (m *Manager) claimIdempotent(ev CardMoved, kind string) bool {
	key := fmt.Sprintf("%s|%s|%s|%s", kind, ev.CardID, ev.FromColumn.OptionID, ev.ToColumn.OptionID)
	if !m.claimStage(ev.CardID, key) {
		m.log.Debug("acp: duplicate move suppressed", "card", ev.CardID, "kind", kind)
		return false
	}
	return true
}

// claimMove adds the guard the standalone trigger columns need on top: a card
// with a live session is left alone.
func (m *Manager) claimMove(ev CardMoved, kind string) bool {
	if !m.claimIdempotent(ev, kind) {
		return false
	}

	m.mu.Lock()
	_, live := m.byCard[ev.CardID]
	m.mu.Unlock()
	if live {
		m.log.Info("acp: card already has a live session, skipping", "card", ev.CardID, "kind", kind)
		return false
	}
	return true
}

// sayCardIsTaken explains why nothing started: somebody has the card. Said on
// the card's own state rather than in a comment — the card is being worked,
// just not by us, and that is a state of affairs, not an event worth keeping.
func (m *Manager) sayCardIsTaken(cardID, nodeID string, err AssignedToHumanError) {
	m.log.Info("acp: card is assigned to a person, no agent started", "card", cardID, "assignee", err.Who)
	m.stallCard(cardID, nodeID, fmt.Sprintf(
		"%s — агент не запускается. Снимите исполнителя или назначьте агента, если работу должен взять он.",
		err.Error()))
}
