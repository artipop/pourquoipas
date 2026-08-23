package acp

import (
	"context"
	"testing"
	"time"
)

// askingFlow is sampleFlow with a question written on the way out of the agent
// stage: the card is not let into the review until somebody has answered.
func askingFlow(ask ...EdgeAsk) FlowEntry {
	f := sampleFlow()
	for i := range f.Edges {
		if f.Edges[i].From == "work" && f.Edges[i].On == TriggerSuccess {
			f.Edges[i].Ask = ask
		}
	}
	return f
}

// A route that asks stops rather than guesses: the card keeps its place, the
// next stage does not start, and what is being asked is on the card.
func TestATransitionThatAsksHoldsTheCardUntilSomebodyAnswers(t *testing.T) {
	m, writer, events, project := flowManager(t, fakeClaudeHappy,
		askingFlow(EdgeAsk{ID: "q1", Kind: AskWorkMode}))
	m.SetBoardCardState(&fakeCardState{})

	events.ch <- flowEvent("card1", project, "Backlog", "To Agent")

	waitFor(t, 20*time.Second, "the card is held at the transition", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.Ask != nil
	})
	if moves := writer.cardMoves(); len(moves) != 0 {
		t.Fatalf("the card moved while a question about it was unanswered: %+v", moves)
	}

	// The card draws the question itself, which is the only place it is
	// answered — one at a time, named by the stage it leads to.
	flow, err := m.CardFlowFor("card1")
	if err != nil || flow == nil || flow.Ask == nil {
		t.Fatalf("card flow: %+v, %v", flow, err)
	}
	if flow.Ask.QuestionID != "q1" || flow.Ask.Column != "Review" || len(flow.Ask.Options) != 2 {
		t.Fatalf("the question the card shows is wrong: %+v", flow.Ask)
	}

	// And the answer lets it across.
	if err := m.AnswerCardAsk(context.Background(), "card1", "q1", WorkModeBranch); err != nil {
		t.Fatalf("answering: %v", err)
	}
	waitFor(t, 20*time.Second, "the card moved on once it was answered", func() bool {
		moves := writer.cardMoves()
		return len(moves) == 1 && moves[0].option == "Review"
	})

	st, ok, err := m.flowState("card1")
	if err != nil || !ok || st.Ask != nil {
		t.Fatalf("the question outlived its answer: %+v, %v, %v", st, ok, err)
	}
	if st.WorkMode != WorkModeBranch {
		t.Fatalf("the answer about the workspace was not kept: %q", st.WorkMode)
	}
}

// The answer about the workspace is what every later stage reads, because it
// is what the card's checkout is made of — the review's diff and the deploy
// both ask the checkout and nothing else.
func TestTheWorkspaceAnswerDecidesHowTheCardsWorkIsMade(t *testing.T) {
	m, _, _, project := testManager(t, fakeClaudeHappy, nil)
	m.SetBoardCardState(&fakeCardState{})

	// The folder works in a copy per card unless something says otherwise.
	if got := m.WorkModeFor("board1", project); got != WorkModeWorktree {
		t.Fatalf("the folder works %q, want %q", got, WorkModeWorktree)
	}
	m.saveFlowState(FlowState{CardID: "card1", BoardID: "board1", NodeID: "work", WorkMode: WorkModeBranch})

	ws, err := m.ClaimWorkspace(WorkSpec{Workdir: project, Owner: "card1", BoardID: "board1", Title: "Test task"})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if ws.Mode != WorkModeBranch || ws.Cwd != project {
		t.Fatalf("the card's answer did not reach its workspace: %+v", ws)
	}

	// And it is the checkout that carries it on: the second claim asks nobody.
	again, err := m.ClaimWorkspace(WorkSpec{Workdir: project, Owner: "card1", BoardID: "board1", Title: "Test task"})
	if err != nil || again.Cwd != ws.Cwd || again.Branch != ws.Branch {
		t.Fatalf("the workspace was not handed back: %+v, %v", again, err)
	}
}

// A hand crossing the arrow crosses it whole. The card is already in the column
// somebody dropped it in, so what is held back is the stage: nothing starts
// until the question on that arrow is answered.
func TestDraggingACardAcrossAnArrowThatAsksHoldsTheStage(t *testing.T) {
	m, _, events, project := flowManager(t, fakeClaudeHappy,
		askingFlow(EdgeAsk{ID: "q1", Kind: AskWorkMode}))
	m.SetBoardCardState(&fakeCardState{})
	m.saveFlowState(FlowState{CardID: "card1", BoardID: "board1", FlowID: m.Flows()[0].ID, NodeID: "work"})

	events.ch <- flowEvent("card1", project, "To Agent", "Review")

	waitFor(t, 20*time.Second, "the card is held at the transition", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.Ask != nil
	})
	st, _, _ := m.flowState("card1")
	if st.Ask.Move {
		t.Fatal("the route would move a card a person had already dropped")
	}
	if st.NodeID != "work" {
		t.Fatalf("the card entered the stage before it was answered: %q", st.NodeID)
	}
	m.mu.Lock()
	_, running := m.byCard["card1"]
	m.mu.Unlock()
	if running {
		t.Fatal("the stage started under an unanswered question")
	}
}

// A question the board cannot answer is not a card parked for ever. The route
// carried cards across this arrow before anybody wrote a question on it, and an
// unanswerable one must not be the thing that stops it.
func TestAQuestionTheBoardCannotAnswerDoesNotHoldTheCard(t *testing.T) {
	m, writer, events, project := flowManager(t, fakeClaudeHappy,
		askingFlow(EdgeAsk{ID: "q1", Kind: AskProperty, Property: "Вердикт"}))
	m.SetBoardMeta(&fakeBoardMeta{cardProps: []CardProperty{{Name: "Превью", Type: "url"}}})

	events.ch <- flowEvent("card1", project, "Backlog", "To Agent")

	waitFor(t, 20*time.Second, "card advanced to Review", func() bool {
		moves := writer.cardMoves()
		return len(moves) == 1 && moves[0].option == "Review"
	})
}

// A question about a board property is written onto the card, which is what
// puts it in the route's dataflow: a later stage reads it, an edge branches on
// it.
func TestAnAnswerAboutABoardPropertyLandsOnTheCard(t *testing.T) {
	m, writer, events, project := flowManager(t, fakeClaudeHappy,
		askingFlow(EdgeAsk{ID: "q1", Kind: AskProperty, Property: "Вердикт"}))
	m.SetBoardMeta(&fakeBoardMeta{cardProps: []CardProperty{
		{Name: "Вердикт", Type: "select", Values: []string{"прошло", "не прошло"}},
	}})
	m.SetBoardCardState(&fakeCardState{})

	events.ch <- flowEvent("card1", project, "Backlog", "To Agent")

	waitFor(t, 20*time.Second, "the card is held at the transition", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.Ask != nil
	})
	if err := m.AnswerCardAsk(context.Background(), "card1", "q1", "прошло"); err != nil {
		t.Fatalf("answering: %v", err)
	}
	waitFor(t, 20*time.Second, "the answer is on the card", func() bool {
		return writer.cardFields("card1")["Вердикт"] == "прошло"
	})
}

// An answer nobody offered is refused: the options are the question, and a
// value from somewhere else would be written onto the card unchecked.
func TestAnAnswerOutsideTheOfferedOnesIsRefused(t *testing.T) {
	m, _, events, project := flowManager(t, fakeClaudeHappy,
		askingFlow(EdgeAsk{ID: "q1", Kind: AskWorkMode}))
	m.SetBoardCardState(&fakeCardState{})

	events.ch <- flowEvent("card1", project, "Backlog", "To Agent")
	waitFor(t, 20*time.Second, "the card is held at the transition", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.Ask != nil
	})

	if err := m.AnswerCardAsk(context.Background(), "card1", "q1", "как-нибудь"); err == nil {
		t.Fatal("an answer that was never offered was accepted")
	}
}

// The editor may leave a question unnamed; an answer has to have something to
// name, so saving the route mints one.
func TestAQuestionWithoutAnIdentifierGetsOne(t *testing.T) {
	f := askingFlow(EdgeAsk{Kind: AskWorkMode})
	out, err := validateFlow(f, []WorkdirEntry{}, []AgentEntry{}, []DeployEntry{})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	for _, e := range out.Edges {
		if len(e.Ask) > 0 && e.Ask[0].ID == "" {
			t.Fatal("a question was saved with nothing to name it by")
		}
	}

	if _, err := validateFlow(askingFlow(EdgeAsk{Kind: AskProperty}), nil, nil, nil); err == nil {
		t.Fatal("a property question that names no property was accepted")
	}
	if _, err := validateFlow(askingFlow(EdgeAsk{Kind: "погода"}), nil, nil, nil); err == nil {
		t.Fatal("an unknown kind of question was accepted")
	}
}

// The questions live on the route, and the route lives on the board — so they
// travel with it, into a template made from it and out of one made into a
// board. Nothing here spells them out: the whole route goes through JSON, which
// is why a new field on it costs nothing. This is the guard on that.
func TestTheQuestionsOnARouteTravelWithTheBoard(t *testing.T) {
	m, meta, _ := storeManager(t)

	if _, err := m.AddFlow(FlowEntry{
		BoardID: "board1",
		Name:    "Фича",
		Nodes: []FlowNode{
			{ID: "agent", Column: "В работе"},
			{ID: "review", Column: "На ревью"},
		},
		Edges: []FlowEdge{{From: "agent", To: "review", On: TriggerSuccess,
			Ask: []EdgeAsk{{Kind: AskWorkMode, Text: "Как работаем?"}}}},
	}); err != nil {
		t.Fatal(err)
	}

	_, flows, err := parseBoardAutomation(meta.written["board1"])
	if err != nil {
		t.Fatalf("read the board back: %v", err)
	}
	if len(flows) != 1 || len(flows[0].Edges) != 1 {
		t.Fatalf("the board kept: %+v", flows)
	}
	ask := flows[0].Edges[0].Ask
	if len(ask) != 1 || ask[0].Kind != AskWorkMode || ask[0].Text != "Как работаем?" || ask[0].ID == "" {
		t.Fatalf("the question did not survive the board: %+v", ask)
	}
}
