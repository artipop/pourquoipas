package acp

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// What a card can say about itself: which route it is on, where along it it
// stands, and what it is waiting for. The card shows this instead of making
// somebody read the comments backwards to work out why it has not moved.

// CardFlowStage is one stage of the route as the card sees it.
type CardFlowStage struct {
	NodeID  string   `json:"nodeId"`
	Column  string   `json:"column"`
	Action  string   `json:"action"`         // resolved: the stage's own, else its column's
	Crew    []string `json:"crew,omitempty"` // who works it
	Current bool     `json:"current"`
	Done    bool     `json:"done"` // the card has already been through it
}

// CardFlowMark is a way a person can move the card from where it stands: the
// value they would put in the outcome field, and the stage that value leads to.
//
// It exists because a wait stage is one where nobody works and nothing runs —
// the card sits until a person says how it went — and saying so was picking a
// value out of a field on the card, three clicks from the card's face. There
// are two answers and the route already knows where each of them leads, so the
// card can offer them as what they are: forward, and back.
type CardFlowMark struct {
	Value  string `json:"value"`
	Column string `json:"column"`
	// Forward is the answer that carries the work on, as against the one that
	// sends it back. It decides which way the arrow points and nothing else.
	Forward bool `json:"forward"`
}

// CardFlow is the whole answer for one card.
type CardFlow struct {
	Flow       string          `json:"flow"`
	Stages     []CardFlowStage `json:"stages"`
	CurrentID  string          `json:"currentNodeId"`
	Since      time.Time       `json:"since"`
	Branch     string          `json:"branch,omitempty"`
	WaitingFor []string        `json:"waitingFor,omitempty"` // human labels of the events the stage waits on
	Queued     bool            `json:"queued,omitempty"`     // waiting for a place in the column
	Running    bool            `json:"running,omitempty"`    // a session of this stage is working now
	// Stalled says why nothing is happening, when the machinery knows: the
	// stage would not start, the route has no edge for what arrived. It used
	// to be a comment on the card; it is state, so it lives here and goes away
	// with the next progress.
	Stalled string `json:"stalled,omitempty"`
	// Marks are the moves a person can make from here — see CardFlowMark. Empty
	// wherever the route does not offer one, which is every stage that decides
	// for itself.
	Marks []CardFlowMark `json:"marks,omitempty"`
}

// CardFlowFor describes where a card stands on its route. It returns nothing —
// and no error — for a card that is not on one, since most cards are not.
func (m *Manager) CardFlowFor(cardID string) (*CardFlow, error) {
	st, ok, err := m.flowState(cardID)
	if err != nil || !ok {
		return nil, err
	}
	flow, found := m.FlowByID(st.FlowID)
	if !found {
		return nil, nil
	}

	out := &CardFlow{Flow: flow.Name, CurrentID: st.NodeID, Since: st.EnteredAt, Branch: st.Branch}

	// "Done" is what the card's own history says it has been through, not what
	// the graph makes possible: a route with a loop has no linear order. The
	// card carries that history itself; the journal answers for a card parked
	// before it did.
	visited := make(map[string]bool)
	for _, id := range st.Visited {
		visited[id] = true
	}
	if len(visited) == 0 && m.store != nil {
		if events, err := m.store.FlowEvents(cardID); err == nil {
			for _, e := range events {
				if e.FromNode != "" {
					visited[e.FromNode] = true
				}
			}
		}
	}

	for _, n := range flow.Nodes {
		action := n.Action
		if action == "" {
			if spec, ok := m.columnOf(n, flow.PropertyOr(m.triggerProperty())); ok {
				action = spec.Action
			}
		}
		crew := n.Crew()
		if len(crew) == 0 {
			if spec, ok := m.columnOf(n, flow.PropertyOr(m.triggerProperty())); ok {
				crew = spec.AgentIDs
			}
		}
		out.Stages = append(out.Stages, CardFlowStage{
			NodeID:  n.ID,
			Column:  n.Column,
			Action:  action,
			Crew:    m.crewNames(crew),
			Current: n.ID == st.NodeID,
			Done:    visited[n.ID] && n.ID != st.NodeID,
		})
	}

	// Conditions included: a condition on a card property is the
	// answer to "what is this card waiting for", the kind alone is not.
	out.WaitingFor = append(out.WaitingFor, flow.WaitDescriptions(st.NodeID)...)
	out.Marks = m.marksFrom(flow, st.NodeID)

	m.mu.Lock()
	_, out.Running = m.byCard[cardID]
	m.mu.Unlock()
	if !out.Running {
		out.Queued = m.cardIsQueued(cardID)
	}
	if stall, ok := m.CardStall(cardID); ok {
		out.Stalled = stall.Reason
	}
	return out, nil
}

// cardIsQueued reports whether the card is waiting for a place in its column.
func (m *Manager) cardIsQueued(cardID string) bool {
	st, ok, err := m.flowState(cardID)
	if err != nil || !ok {
		return false
	}
	flow, found := m.FlowByID(st.FlowID)
	if !found {
		return false
	}
	node, found := flow.Node(st.NodeID)
	if !found {
		return false
	}
	spec, found := m.columnOf(node, flow.PropertyOr(m.triggerProperty()))
	if !found {
		return false
	}
	q, ok, err := m.store.NextQueuedStage(spec.Key())
	return err == nil && ok && q.CardID == cardID
}

// columnOf finds the configured column a route's stage stands on, by the
// stage's option id and by nothing else. Matching one by the column's name is
// what let renaming a column silently rearrange a route; a stage with no option
// id is one the board could not bind (bindToBoardOptions), and no card can
// stand on it.
//
// The property argument is what the callers already had in hand and is no
// longer consulted.
func (m *Manager) columnOf(node FlowNode, _ string) (ColumnSpec, bool) {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	if node.OptionID != "" {
		for _, c := range m.cfg.Columns {
			if c.OptionID != "" && c.OptionID == node.OptionID {
				return c, true
			}
		}
	}
	return ColumnSpec{}, false
}

// FlowStageCount is how busy one stage of a route is right now.
type FlowStageCount struct {
	NodeID  string `json:"nodeId"`
	Cards   int    `json:"cards"`   // cards standing on this stage
	Running int    `json:"running"` // of those, being worked on now
	Queued  int    `json:"queued"`  // of those, waiting for a place in the column
}

// FlowOverview is one route and where the board's cards are along it.
type FlowOverview struct {
	Flow   string           `json:"flow"`
	Stages []FlowStageCount `json:"stages"`
	Cards  int              `json:"cards"`
}

// BoardFlowOverview answers "where is everything right now" for a board: for
// each route it may use, how many cards stand on each of its stages and which
// of them are moving. It is the map the workflow view draws — the same data the
// engine runs on, rather than a second bookkeeping of it.
func (m *Manager) BoardFlowOverview(boardID string) ([]FlowOverview, error) {
	flows := m.BoardFlows(boardID)
	if len(flows) == 0 {
		return nil, nil
	}
	states, err := m.flowStates()
	if err != nil {
		return nil, err
	}

	// One pass over the live sessions, rather than one lookup per card.
	m.mu.Lock()
	running := make(map[string]bool, len(m.active))
	for cardID := range m.byCard {
		running[cardID] = true
	}
	m.mu.Unlock()

	out := make([]FlowOverview, 0, len(flows))
	for _, flow := range flows {
		view := FlowOverview{Flow: flow.Name}
		counts := make(map[string]*FlowStageCount, len(flow.Nodes))
		for _, n := range flow.Nodes {
			counts[n.ID] = &FlowStageCount{NodeID: n.ID}
		}
		for _, st := range states {
			if st.FlowID != flow.ID {
				continue
			}
			if boardID != "" && st.BoardID != "" && st.BoardID != boardID {
				continue
			}
			count, ok := counts[st.NodeID]
			if !ok {
				continue // the card stands on a stage the route no longer has
			}
			count.Cards++
			view.Cards++
			switch {
			case running[st.CardID]:
				count.Running++
			case m.cardIsQueued(st.CardID):
				count.Queued++
			}
		}
		for _, n := range flow.Nodes {
			view.Stages = append(view.Stages, *counts[n.ID])
		}
		out = append(out, view)
	}
	return out, nil
}

// marksFrom is what a person can say from this stage: the route's own
// card.changed edges that ask about the outcome field, named by where they
// lead. Only those — an edge waiting on a merged branch is the world's answer
// and not a person's, and an edge on a stage's own outcome belongs to whatever
// runs there.
func (m *Manager) marksFrom(flow FlowEntry, nodeID string) []CardFlowMark {
	var out []CardFlowMark
	for _, e := range flow.Edges {
		if e.From != nodeID || e.On != TriggerCardChanged || e.If == nil {
			continue
		}
		if !strings.EqualFold(e.If.Property, OutcomePropertyTitle) {
			continue
		}
		node, ok := flow.Node(e.To)
		if !ok {
			continue
		}
		out = append(out, CardFlowMark{
			Value:   e.If.Value,
			Column:  node.Column,
			Forward: strings.EqualFold(e.If.Value, OutcomePassed),
		})
	}
	return out
}

// MarkCardOutcome is a person answering for a stage that runs nothing: «прошло»
// or «не прошло», put on the card so the route sees it and moves.
//
// Through UpdateCard rather than SetCardOption, and that is the whole point:
// SetCardOption is silent, because everything the machine records about a card
// must not set the card's own automation off. This one has to — the board's
// notify is what wakes handleCardChanged, which is what moves the card. It is a
// person editing the card, and it goes the way a person's edit goes.
func (m *Manager) MarkCardOutcome(ctx context.Context, cardID, value string) error {
	if strings.TrimSpace(cardID) == "" {
		return fmt.Errorf("не сказано, какая карточка")
	}
	if value != OutcomePassed && value != OutcomeFailed {
		return fmt.Errorf("исход бывает «%s» или «%s»", OutcomePassed, OutcomeFailed)
	}
	if m.writer == nil {
		return fmt.Errorf("доска недоступна")
	}
	return m.forCard(cardID).Writer.UpdateCard(ctx, cardID, CardEdit{Options: []string{value}})
}
