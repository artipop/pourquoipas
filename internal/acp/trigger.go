package acp

import (
	"fmt"
	"time"

	"github.com/artipop/xciii/internal/apperr"
)

// errSwitchedOff is what a start refused by the switch says.
var errSwitchedOff = apperr.New("ai.off", "AI is switched off in the app")

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

// handleEvent is the whole of what the app does about a card on its own: a
// card moved into a column whose assignee is a registered agent is a card that
// agent starts working on.
//
// There is no configuration behind that. A column is an option of the board's
// own select and means whatever the people using the board mean by it; which
// column somebody drags a card into is their sentence, and the agent reads it
// along with the card. What used to be here — a registry saying that this
// column runs an agent with this crew, that one deploys, and a route saying
// what follows what — is gone, and so is every question it made a person
// answer before the board would do anything.
//
// A move the agent itself made (move_card) starts nothing: that move *is* the
// result of a run, and acting on it would be an agent handing itself its own
// work back for ever.
func (m *Manager) handleEvent(ev CardMoved) {
	if m.aiSwitchedOff() {
		return
	}
	if m.movedByAgent(ev.CardID) {
		m.log.Debug("acp: the move was the agent's own", "card", ev.CardID)
		return
	}
	// A board may carry its own instructions; take them once, before anything
	// asks what this board tells its agents.
	m.seedFromBoard(ev.BoardID)

	if ev.ToColumn.OptionID == "" && ev.ToColumn.Name == "" {
		return // not a move between columns
	}
	agent, ok := m.assignedAgent(ev)
	if !ok {
		// Nobody's to work — but a card dragged out from under a running agent
		// is a card taken back, whoever it now belongs to.
		m.cancelForMove(ev)
		return
	}
	if !m.claimIdempotent(ev, "agent") {
		return
	}
	// The card moved while its agent was working it: that work was about where
	// the card was, and the move is a new instruction. The old run goes first,
	// and the folder it held goes with it.
	m.cancelForMove(ev)
	s, err := m.startSession(ev, startOptions{agent: agent})
	if err != nil {
		m.log.Warn("acp: session not started", "card", ev.CardID, "err", err)
		m.stallCard(ev.CardID, ev.ToColumn.OptionID, apperr.MessageOf(err))
		return
	}
	m.log.Info("acp: session started", "session", s.ID, "card", ev.CardID,
		"agent", agent.Name, "column", ev.ToColumn.Name, "workdir", s.WorkdirPath)
}

// assignedAgent is the agent a card names as its assignee, if it names one at
// all. A card assigned to nobody, or to a person, is nobody's to start: «кто
// занимается» is the one place a card says who works it.
func (m *Manager) assignedAgent(ev CardMoved) (AgentEntry, bool) {
	m.cfgMu.RLock()
	agents := append([]AgentEntry(nil), m.cfg.Agents...)
	m.cfgMu.RUnlock()

	// By the account the card actually stores, then by the username it shows.
	// The id is the answer that survives a rename; the username still answers
	// for a card assigned before the agent's account was recorded, and for a
	// board carried here from a machine where the agent has an account of its
	// own under the same name.
	for _, id := range ev.PersonIDs {
		for _, a := range agents {
			if id != "" && a.UserID == id {
				return a, true
			}
		}
	}
	for _, person := range ev.PersonNames {
		for _, a := range agents {
			if sameAgentName(person, a.Name) {
				return a, true
			}
		}
	}
	return AgentEntry{}, false
}

// claimIdempotent collapses the burst of patches one drag-and-drop produces
// into a single move. kind namespaces the key, so a card's own run and a deploy
// started from it cannot suppress each other's events.
func (m *Manager) claimIdempotent(ev CardMoved, kind string) bool {
	key := fmt.Sprintf("%s|%s|%s|%s", kind, ev.CardID, ev.FromColumn.OptionID, ev.ToColumn.OptionID)
	if !m.claimStage(ev.CardID, key) {
		m.log.Debug("acp: duplicate move suppressed", "card", ev.CardID, "kind", kind)
		return false
	}
	return true
}

// cancelForMove stops the run the card was in the middle of and waits for it to
// let go of its folder, so the next one does not collide with it over a working
// copy. A card with nothing running costs nothing here.
func (m *Manager) cancelForMove(ev CardMoved) {
	if !m.CancelSessionForCard(ev.CardID, "the card was moved to another column") {
		return
	}
	m.log.Info("acp: session cancelled by card move", "card", ev.CardID)
	deadline := time.Now().Add(cardIdleWait)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		_, live := m.byCard[ev.CardID]
		m.mu.Unlock()
		if !live {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	m.log.Warn("acp: the card's previous session is still holding on", "card", ev.CardID)
}

// cardIdleWait bounds how long a move waits for the card's previous session to
// let go of its folder before starting the next one.
const cardIdleWait = 20 * time.Second

// sayCardIsTaken explains why nothing started: somebody has the card. Said on
// the card's own state rather than in a comment — the card is being worked,
// just not by us, and that is a state of affairs, not an event worth keeping.
func (m *Manager) sayCardIsTaken(cardID, nodeID string, err AssignedToHumanError) {
	m.log.Info("acp: card is assigned to a person, no agent started", "card", cardID, "assignee", err.Who)
	m.stallCard(cardID, nodeID, apperr.Say("stall.assigned-to-person",
		"The card is assigned to {who}, so no agent starts. Remove the assignee, or assign an agent if one should take the work",
		"who", err.Who))
}
