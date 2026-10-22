package acp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

// The plan is what the wizard asks and what the board menu offers, so a wrong
// one is a question nobody can answer or a setting that does nothing. It is
// resolved from three places at once — what the board asks for, what its
// automation implies, what this machine already has — and these fix each.

func setupManager(t *testing.T, props map[string]any) *Manager {
	t.Helper()
	dir := t.TempDir()
	store, err := newTestStore(t, filepath.Join(dir, "acp.db"))
	if err != nil {
		t.Fatal(err)
	}

	m := NewManager(DefaultConfig(dir), "", store, newFakeWriter(), &fakeEmitter{}, nil)
	m.rootCtx = context.Background()
	m.SetBoardMeta(&fakeBoardMeta{props: props})
	return m
}

// boardProps encodes what a template writes into the board, the way the board
// store hands it back: JSON decoded into `any`.
func boardProps(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var props map[string]any
	if err := json.Unmarshal(encoded, &props); err != nil {
		t.Fatal(err)
	}
	return props
}

func kinds(plan SetupPlan) []string {
	out := make([]string, 0, len(plan.Steps))
	for _, s := range plan.Steps {
		out = append(out, s.Kind)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A template is a board being written, not one being run: whatever a copy of it
// will need is the copy's business, and a wizard on top of one asks about a
// board that does not exist yet. This came up as the setup screen opening on
// «Create new template», where there is nothing to set up at all.
func TestATemplateIsAskedForNothing(t *testing.T) {
	asks := boardProps(t, map[string]any{
		BoardPropSetup: BoardSetup{Steps: []BoardSetupStep{
			{Kind: SetupStepAgent},
			{Kind: SetupStepDone},
		}},
	})
	m := setupManager(t, asks)

	// The same board, asking for the same two things, is asked them as a board
	// and not as a template.
	if len(m.SetupPlanFor("board1").Steps) != 3 {
		t.Fatalf("the board itself was asked for %v", kinds(m.SetupPlanFor("board1")))
	}

	m.SetBoardMeta(&fakeBoardMeta{props: asks, template: true})
	plan := m.SetupPlanFor("board1")
	if len(plan.Steps) != 0 {
		t.Errorf("a template was asked for %v", kinds(plan))
	}
	if plan.Automated {
		t.Error("a template counts as automated, so the wizard would open itself on it")
	}
}

// The one question that is about the board rather than about the machine, and
// the one nobody can be let past: a board arrives called what its template is
// called, so the second one made from the developer template is a second the developer template in
// the sidebar until somebody says what this one is for.
func TestEveryBoardIsAskedItsNameFirst(t *testing.T) {
	boards := map[string]*Manager{
		"a board that declares its steps": setupManager(t, boardProps(t, map[string]any{
			BoardPropSetup: BoardSetup{Steps: []BoardSetupStep{{Kind: SetupStepAgent}, {Kind: SetupStepDone}}},
		})),
		"a board that carries its own instructions": setupManager(t, boardProps(t, map[string]any{
			BoardPropPrompt: "Отвечай по-русски.",
		})),
		"a board with nothing at all": setupManager(t, nil),
	}
	for what, m := range boards {
		plan := m.SetupPlanFor("board1")
		if len(plan.Steps) == 0 || plan.Steps[0].Kind != SetupStepName {
			t.Fatalf("%s: opens on %v", what, kinds(plan))
		}
		if plan.Steps[0].Optional {
			t.Errorf("%s: the name is a question that may be passed over", what)
		}
	}
}

// A board that says what it needs is asked for exactly that, in its own order.
func TestABoardAsksForTheStepsItNames(t *testing.T) {
	m := setupManager(t, boardProps(t, map[string]any{
		BoardPropSetup: BoardSetup{Steps: []BoardSetupStep{
			{Kind: SetupStepAgent, Hint: "The agent that writes the newsletter"},
			{Kind: SetupStepDone},
		}},
	}))

	plan := m.SetupPlanFor("board1")
	if !plan.Declared {
		t.Error("the plan does not say the board asked for it")
	}
	if want := []string{SetupStepName, SetupStepAgent, SetupStepDone}; !equal(kinds(plan), want) {
		t.Fatalf("steps %v, expected %v", kinds(plan), want)
	}
	if plan.Steps[1].Hint == "" {
		t.Error("the board's own sentence was dropped")
	}
}

// A step this build has never heard of is skipped rather than fatal: the board
// may have been made by a newer app, and the rest of the plan still stands.
func TestAStepThisBuildCannotDoIsLeftOut(t *testing.T) {
	m := setupManager(t, boardProps(t, map[string]any{
		BoardPropSetup: BoardSetup{Steps: []BoardSetupStep{
			{Kind: SetupStepAgent},
			{Kind: "telepathy"},
			{Kind: SetupStepDone},
		}},
	}))

	if want := []string{SetupStepName, SetupStepAgent, SetupStepDone}; !equal(kinds(m.SetupPlanFor("board1")), want) {
		t.Fatalf("steps %v, expected %v", kinds(m.SetupPlanFor("board1")), want)
	}
}

// The folder is asked on the card now, and every board made before carries it
// in its own declaration. Such a board must stop asking it, and must not be
// logged as a board from a newer app on every read of its plan.
func TestABoardThatStillDeclaresTheFolderIsNotAskedIt(t *testing.T) {
	m := setupManager(t, boardProps(t, map[string]any{
		BoardPropSetup: BoardSetup{Steps: []BoardSetupStep{
			{Kind: "project", Hint: "A folder for the agent's notes"},
			{Kind: SetupStepAgent},
			{Kind: SetupStepDone},
		}},
	}))

	if want := []string{SetupStepName, SetupStepAgent, SetupStepDone}; !equal(kinds(m.SetupPlanFor("board1")), want) {
		t.Fatalf("steps %v, expected %v", kinds(m.SetupPlanFor("board1")), want)
	}
	if want := []string{SetupStepName, SetupStepAgent, SetupStepDone}; !equal(kinds(setupManager(t, nil).SetupPlanFor("board1")), want) {
		t.Errorf("a board that said nothing is asked %v, expected %v", kinds(setupManager(t, nil).SetupPlanFor("board1")), want)
	}
}

// A machine that can already answer a step says so — but the step is still this
// board's to answer. Setting up one board used to mark every board that came
// after it as set up, which meant a second board was created in silence: no
// wizard, no reminder, nothing asked.
func TestAFilledRegistryOffersAnAnswerRatherThanBeingOne(t *testing.T) {
	m := setupManager(t, nil)
	m.cfg.Workdirs = []WorkdirEntry{{Name: "notes", Path: "/tmp/notes"}}
	m.cfg.Agents = []AgentEntry{{ID: newID(), Name: "claude", Kind: "claude"}}

	byKind := func(boardID string) map[string]SetupStep {
		out := map[string]SetupStep{}
		for _, s := range m.SetupPlanFor(boardID).Steps {
			out[s.Kind] = s
		}
		return out
	}

	first := byKind("board1")

	// The name step is not in this list: nothing on the machine can answer
	// "what is this board called", so it has no registry and is never ready.
	for _, kind := range []string{SetupStepAgent} {
		if first[kind].Status != SetupPending {
			t.Errorf("%s: %q — a board nobody set up counts as set up", kind, first[kind].Status)
		}
		if !first[kind].Ready {
			t.Errorf("%s: the machine can answer it, and the plan does not say so", kind)
		}
	}
	// A board made while publishing to Dokku existed still declares the
	// step; it is dropped in silence, as the folder step was.
	declaring := setupManager(t, boardProps(t, map[string]any{
		BoardPropSetup: BoardSetup{Steps: []BoardSetupStep{{Kind: "deploy"}, {Kind: SetupStepAgent}}},
	}))
	for _, s := range declaring.SetupPlanFor("board9").Steps {
		if s.Kind == "deploy" {
			t.Error("deploy: a retired step is still asked")
		}
	}

	// Answering it for one board leaves the next one asking.
	for _, kind := range []string{SetupStepName, SetupStepAgent} {
		if err := m.RecordSetupStep("board1", kind, SetupDone); err != nil {
			t.Fatal(err)
		}
	}
	if got := byKind("board1")[SetupStepAgent].Status; got != SetupDone {
		t.Errorf("board1 agent after answering: %q", got)
	}
	if got := byKind("board2")[SetupStepAgent].Status; got != SetupPending {
		t.Errorf("board2 agent: %q — one board's answer stood in for another's", got)
	}
}

// Skipping is the one answer nothing else can be read for, so it is the one
// that has to survive a restart — and it is remembered per board.
func TestASkippedStepIsRememberedForThatBoard(t *testing.T) {
	m := setupManager(t, nil)

	if err := m.RecordSetupStep("board1", SetupStepAgent, SetupSkipped); err != nil {
		t.Fatal(err)
	}
	for _, s := range m.SetupPlanFor("board1").Steps {
		if s.Kind == SetupStepAgent && s.Status != SetupSkipped {
			t.Errorf("board1 agent: %q", s.Status)
		}
	}
	for _, s := range m.SetupPlanFor("board2").Steps {
		if s.Kind == SetupStepAgent && s.Status != SetupPending {
			t.Errorf("board2 agent: %q — another board's answer leaked", s.Status)
		}
	}

	// And what does not exist cannot be recorded: the wizard is the only
	// caller, but it reaches this through a binding anything can call.
	if err := m.RecordSetupStep("board1", "telepathy", SetupSkipped); err == nil {
		t.Error("an unknown step was recorded")
	}
	if err := m.RecordSetupStep("board1", SetupStepAgent, "postponed"); err == nil {
		t.Error("an unknown status was recorded")
	}
}

// Having been offered the wizard has to outlive the run it happened in. It used
// to live in the page's localStorage, which is keyed by origin — and the desktop
// app publishes itself on a fresh port, and therefore a fresh origin, every
// launch, so "do not show me this again" lasted exactly one launch.
func TestTheOfferIsRememberedAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	store, err := newTestStore(t, filepath.Join(dir, "acp.db"))
	if err != nil {
		t.Fatal(err)
	}

	manager := func(store *Store) *Manager {
		m := NewManager(DefaultConfig(dir), "", store, newFakeWriter(), &fakeEmitter{}, nil)
		m.rootCtx = context.Background()
		m.SetBoardMeta(&fakeBoardMeta{})
		return m
	}

	m := manager(store)
	if m.SetupPlanFor("board1").Offered {
		t.Fatal("a board nobody has seen the wizard for says it has")
	}
	if err := m.MarkSetupOffered("board1"); err != nil {
		t.Fatal(err)
	}
	if !m.SetupPlanFor("board1").Offered {
		t.Fatal("the offer was not remembered at all")
	}
	if m.SetupPlanFor("board2").Offered {
		t.Error("one board's offer stood in for another's")
	}
	// The next launch: a new store on the same file, and nothing else carried
	// over — no page, no memory.
	reopened, err := newTestStore(t, filepath.Join(dir, "acp.db"))
	if err != nil {
		t.Fatal(err)
	}
	if !manager(reopened).SetupPlanFor("board1").Offered {
		t.Fatal("the offer did not survive the restart")
	}
}
