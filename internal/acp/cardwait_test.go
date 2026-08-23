package acp

import (
	"testing"
	"time"
)

// cardWait is the wait a card is standing on, if any.
func cardWait(m *Manager, cardID string) (Attention, bool) {
	for _, a := range m.Attention() {
		if a.CardID == cardID && a.Reason == AttentionCard {
			return a, true
		}
	}
	return Attention{}, false
}

// A route that moves a card onto a stage where nothing runs has handed the work
// to a person — and nobody is looking at the board when it happens, which is the
// whole reason it has to say so.
func TestARouteHandingACardToAPersonSaysSo(t *testing.T) {
	m, _, events, project := flowManager(t, fakeClaudeHappy, sampleFlow())

	events.ch <- flowEvent("card1", project, "Backlog", "To Agent")

	waitFor(t, 30*time.Second, "the card says it is waiting for a person", func() bool {
		_, ok := cardWait(m, "card1")
		return ok
	})
	a, _ := cardWait(m, "card1")
	if a.Text == "" || a.BoardID == "" {
		t.Errorf("the wait says nothing a person can act on: %+v", a)
	}

	// Being told is the whole of it. An agent's wait goes on after somebody has
	// looked and only its notification stops; this one ends, or every card
	// parked at a review would hold the tray icon amber for as long as it stood
	// there and the icon would stop meaning anything.
	m.AckAttention(a.Key)
	if _, still := cardWait(m, "card1"); still {
		t.Error("the card went on waiting after somebody said they had seen it")
	}
}

// A person who drags a card into a column where nothing runs is looking at it
// already, and telling them what they have just done is the noise this is meant
// to be the opposite of.
func TestADragOntoAPersonsStageAnnouncesNothing(t *testing.T) {
	m, _, events, project := flowManager(t, fakeClaudeHappy, sampleFlow())

	events.ch <- flowEvent("card2", project, "To Agent", "Review")

	waitFor(t, 20*time.Second, "the route recorded the card's place", func() bool {
		st, ok, _ := m.flowState("card2")
		return ok && st.NodeID == "review"
	})
	if _, told := cardWait(m, "card2"); told {
		t.Error("a card somebody dragged there themselves announced itself")
	}
}
