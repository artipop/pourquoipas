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

// entryAskingFlow is sampleFlow with the question on transition 0 — the way onto
// the route — rather than on an arrow between two of its stages.
func entryAskingFlow(ask ...EdgeAsk) FlowEntry {
	f := sampleFlow()
	f.Edges = append(f.Edges, FlowEdge{From: EntryNode, To: "work", Ask: ask})
	return f
}

// The way onto the route is a transition like any other, and it is the one a
// card crosses when it arrives from the backlog: nothing about the route has
// been decided yet, so this is where the machine may ask how the work is to be
// arranged.
func TestArrivingOnTheRouteCrossesTheEntryTransition(t *testing.T) {
	m, _, events, project := flowManager(t, fakeClaudeHappy,
		entryAskingFlow(EdgeAsk{ID: "q1", Kind: AskWorkMode}))
	m.SetBoardCardState(&fakeCardState{})

	events.ch <- flowEvent("card1", project, "Backlog", "To Agent")

	waitFor(t, 20*time.Second, "the card is held on the way in", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.Ask != nil
	})
	m.mu.Lock()
	_, running := m.byCard["card1"]
	m.mu.Unlock()
	if running {
		t.Fatal("the first stage started under an unanswered question")
	}

	if err := m.AnswerCardAsk(context.Background(), "card1", "q1", WorkModeWorktree); err != nil {
		t.Fatalf("answering: %v", err)
	}
	waitFor(t, 20*time.Second, "the card entered the first stage once it was answered", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.NodeID == "work" && st.Ask == nil
	})
	if st, _, _ := m.flowState("card1"); st.WorkMode != WorkModeWorktree {
		t.Fatalf("the answer about the workspace was not kept: %q", st.WorkMode)
	}
}

// Asked once, on the way in. A card sent back to the first stage from a real
// stage of the route has crossed a different arrow, and how its work is
// arranged was settled the first time — asking again would offer to rebuild a
// workspace that already has the work in it.
func TestComingBackToTheFirstStageDoesNotAskAgain(t *testing.T) {
	m, _, events, project := flowManager(t, fakeClaudeHappy,
		entryAskingFlow(EdgeAsk{ID: "q1", Kind: AskWorkMode}))
	m.SetBoardCardState(&fakeCardState{})
	m.saveFlowState(FlowState{CardID: "card1", BoardID: "board1", FlowID: m.Flows()[0].ID, NodeID: "review"})

	events.ch <- flowEvent("card1", project, "Review", "To Agent")

	waitFor(t, 20*time.Second, "the card entered the stage", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.NodeID == "work"
	})
	if st, _, _ := m.flowState("card1"); st.Ask != nil {
		t.Fatalf("the way in asked a card that was already on the route: %+v", st.Ask)
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

// The wait a held card raises carries the answers, so a surface can put the
// question in front of whoever is looking instead of pointing them at the card.
// Not the question's id, though: withKey would key the wait by it, and an ask
// id is the arrow's own word rather than something unique — two cards held on
// one route would collapse into one wait.
func TestTheWaitRaisedByAHeldCardOffersItsAnswers(t *testing.T) {
	m, _, events, project := flowManager(t, fakeClaudeHappy,
		entryAskingFlow(EdgeAsk{ID: "q1", Kind: AskWorkMode}))
	m.SetBoardCardState(&fakeCardState{})

	events.ch <- flowEvent("card1", project, "Backlog", "To Agent")

	var wait Attention
	waitFor(t, 20*time.Second, "the card's wait is announced", func() bool {
		for _, a := range m.Attention() {
			if a.CardID == "card1" && a.Reason == AttentionCard {
				wait = a
				return true
			}
		}
		return false
	})
	if len(wait.Options) != 2 || wait.Options[0].ID != WorkModeWorktree {
		t.Fatalf("the wait does not carry the answers: %+v", wait.Options)
	}
	if wait.QuestionID != "" || wait.Key != "card:card1" {
		t.Fatalf("the wait is keyed by the question rather than by the card: %q %q", wait.QuestionID, wait.Key)
	}
}

// The crossing the whole mechanism was built for: a card in the backlog, which
// has usually named no folder, dragged into work. It is asked for the folder
// first, because until there is one the question about how to work it is about
// nothing — and that is exactly how it used to be skipped, silently, on the one
// transition it was for.
func TestACardWithNoFolderIsAskedForOneOnTheWayIn(t *testing.T) {
	flow := sampleFlow()
	flow.Edges = append(flow.Edges, entryTransition("work"))
	m, _, events, project := flowManager(t, fakeClaudeHappy, flow)
	m.SetBoardCardState(&fakeCardState{})
	m.SetBoardMeta(&fakeBoardMeta{props: map[string]any{BoardPropProject: "p-folder"}})

	ev := flowEvent("card1", project, "Backlog", "To Agent")
	ev.OptionNames, ev.Values, ev.SelectedOptions = nil, nil, nil
	events.ch <- ev

	waitFor(t, 20*time.Second, "the card is held at the way in", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.Ask != nil
	})
	st, _, _ := m.flowState("card1")
	first := st.Ask.Next()
	if first.Kind != AskWorkdir || len(first.Options) == 0 {
		t.Fatalf("the first question is not an answerable folder one: %+v", first)
	}

	// Answering writes the folder onto the card, and the board hands the card
	// back naming it — which is what makes the second question answerable.
	m.SetBoardReader(&fakeReader{ev: CardMoved{
		BoardID: "board1", Title: "Test task",
		Props:           map[string]string{"branch": flowTestBranch},
		SelectedOptions: []Column{{PropertyID: "p-folder", Name: testWorkdirName}},
	}})
	if err := m.AnswerCardAsk(context.Background(), "card1", first.ID, first.Options[0].Value); err != nil {
		t.Fatalf("answering the folder: %v", err)
	}

	st, _, _ = m.flowState("card1")
	if st.Ask == nil || st.Ask.Next() == nil {
		t.Fatal("the card was let through without being asked how to work the folder")
	}
	if got := st.Ask.Next().Kind; got != AskWorkMode {
		t.Fatalf("the second question is %q, want %q", got, AskWorkMode)
	}
}

// A card that already names a folder is not asked about it again: the question
// applies or it does not, and it is decided when it comes up.
func TestACardThatNamesAFolderIsNotAskedForOne(t *testing.T) {
	flow := sampleFlow()
	flow.Edges = append(flow.Edges, entryTransition("work"))
	m, _, events, project := flowManager(t, fakeClaudeHappy, flow)
	m.SetBoardCardState(&fakeCardState{})

	events.ch <- flowEvent("card1", project, "Backlog", "To Agent")

	waitFor(t, 20*time.Second, "the card is held at the way in", func() bool {
		st, ok, err := m.flowState("card1")
		return err == nil && ok && st.Ask != nil
	})
	st, _, _ := m.flowState("card1")
	if got := st.Ask.Next().Kind; got != AskWorkMode {
		t.Fatalf("a card naming a folder was asked %q, want the work mode", got)
	}
}

// A way in written before the folder question existed asks how to work a folder
// the card has not chosen — the one crossing where it cannot be answered. The
// board is brought up to date where it is read, and a hand-edited arrow is left
// alone.
func TestAWayInWrittenBeforeTheFolderQuestionGainsIt(t *testing.T) {
	old := FlowEntry{
		Name:  "Фича",
		Nodes: []FlowNode{{ID: "work", Column: "To Agent"}},
		Edges: []FlowEdge{{From: EntryNode, To: "work", Ask: []EdgeAsk{{ID: "workmode", Kind: AskWorkMode}}}},
	}
	flows := []FlowEntry{old}
	if added := addEntryTransitions(flows); added != 1 {
		t.Fatalf("the way in was not brought up to date: %d", added)
	}
	got := flows[0].Edges[0].Ask
	if len(got) != 2 || got[0].Kind != AskWorkdir || got[1].Kind != AskWorkMode {
		t.Fatalf("the folder question did not go in front of the work mode: %+v", got)
	}

	// Twice changes nothing, and an arrow somebody wrote themselves is not
	// touched: the repair is matched on the ids this app mints.
	if added := addEntryTransitions(flows); added != 0 {
		t.Fatalf("the repair repeated itself: %d", added)
	}
	mine := []FlowEntry{{
		Name:  "Своё",
		Nodes: []FlowNode{{ID: "work", Column: "To Agent"}},
		Edges: []FlowEdge{{From: EntryNode, To: "work", Ask: []EdgeAsk{{ID: "q1", Kind: AskWorkMode}}}},
	}}
	if added := addEntryTransitions(mine); added != 0 {
		t.Fatalf("a hand-written way in was edited: %d", added)
	}
}
