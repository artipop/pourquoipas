package acp

import (
	"context"
	"testing"
)

// Which route a card is on has two answers, and a view drawn by route needs
// both: where the card stands once it has entered a stage, and — before that —
// the route the card itself declares by naming it.
func TestWhichRouteEachCardOfABoardIsOn(t *testing.T) {
	feature := sampleFlowOn("board1")
	hotfix := sampleFlowOn("board1")
	hotfix.Name = "hotfix"

	m, _, _, _ := testManager(t, fakeClaudeHappy, func(c *Config) {
		c.Flows = []FlowEntry{feature, hotfix}
	})
	m.SetBoardReader(&fakeReader{cards: []CardMoved{
		{CardID: "declared", BoardID: "board1", OptionNames: []string{"hotfix"}},
		{CardID: "fresh", BoardID: "board1"},
		{CardID: "working", BoardID: "board1"},
		{CardID: "elsewhere", BoardID: "board2", OptionNames: []string{"hotfix"}},
	}})

	ids := map[string]string{}
	for _, f := range m.BoardFlows("board1") {
		ids[f.Name] = f.ID
	}
	if ids["feature"] == "" || ids["hotfix"] == "" {
		t.Fatalf("the routes were not registered: %v", ids)
	}

	// A card standing on a stage: the route it stands on wins, whatever it says
	// about itself.
	m.saveFlowState(FlowState{CardID: "working", BoardID: "board1", FlowID: ids["feature"], NodeID: "work"})

	got := m.CardFlowsForBoard(context.Background(), "board1")
	if got["working"] != ids["feature"] {
		t.Errorf("a card on a stage should name that route: %q", got["working"])
	}
	if got["declared"] != ids["hotfix"] {
		t.Errorf("a card naming its route should name it before it moves: %q", got["declared"])
	}

	// A card on no route belongs to every view of the board, so it is left out
	// rather than guessed at.
	if flow, ok := got["fresh"]; ok {
		t.Errorf("a card that has chosen nothing should be on no route: %q", flow)
	}

	// Another board's cards are another board's business.
	if _, ok := got["elsewhere"]; ok {
		t.Error("a card of another board should not appear")
	}
}

// A board with one route has nothing to choose between, and the engine says
// every card is on it — the view must not disagree with what actually runs.
func TestOneRouteTakesEveryCard(t *testing.T) {
	m, _, _, _ := testManager(t, fakeClaudeHappy, func(c *Config) {
		c.Flows = []FlowEntry{sampleFlowOn("board1")}
	})
	m.SetBoardReader(&fakeReader{cards: []CardMoved{{CardID: "fresh", BoardID: "board1"}}})

	got := m.CardFlowsForBoard(context.Background(), "board1")
	if got["fresh"] == "" {
		t.Errorf("the board's only route should take the card: %v", got)
	}
}
