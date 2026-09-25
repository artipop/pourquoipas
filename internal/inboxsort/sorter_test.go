package inboxsort

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/artipop/xciii/internal/appschema"
)

// fakeBoard is a board of a few boards and their cards, in memory.
type fakeBoard struct {
	mu     sync.Mutex
	boards []BoardInfo
	cards  map[string][]CardInfo // by board
	texts  map[string]string
	filed  map[string]string // card → board it was filed on
}

func newFakeBoard() *fakeBoard {
	return &fakeBoard{
		boards: []BoardInfo{
			{ID: "inbox", Title: "Входящие", Inbox: true},
			{ID: "home", Title: "Дом", Description: "Покупки, ремонт, счета за квартиру"},
			{ID: "work", Title: "Работа", Description: "Задачи по проекту XCIII"},
		},
		cards: map[string][]CardInfo{
			"home": {{ID: "h1", Title: "Купить лампочки"}},
			"work": {{ID: "w1", Title: "Починить сборку под Windows"}},
		},
		texts: map[string]string{},
		filed: map[string]string{},
	}
}

func (f *fakeBoard) Boards(context.Context) ([]BoardInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]BoardInfo(nil), f.boards...), nil
}

func (f *fakeBoard) Cards(_ context.Context, boardID string) ([]CardInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]CardInfo(nil), f.cards[boardID]...), nil
}

func (f *fakeBoard) CardText(_ context.Context, cardID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.texts[cardID], nil
}

func (f *fakeBoard) FileCard(_ context.Context, cardID, boardID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for from, cards := range f.cards {
		for i, c := range cards {
			if c.ID == cardID {
				f.cards[from] = append(cards[:i:i], cards[i+1:]...)
				f.cards[boardID] = append([]CardInfo{c}, f.cards[boardID]...)
				f.filed[cardID] = boardID
				return nil
			}
		}
	}
	return nil
}

func (f *fakeBoard) SetDescription(_ context.Context, boardID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.boards {
		if f.boards[i].ID == boardID {
			f.boards[i].Description = text
		}
	}
	return nil
}

func (f *fakeBoard) arrive(c CardInfo, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cards["inbox"] = append([]CardInfo{c}, f.cards["inbox"]...)
	f.texts[c.ID] = text
}

// fakeAgent answers a summary prompt with summaries and a sorting prompt with
// whatever sort says, and remembers what it was asked.
type fakeAgent struct {
	mu        sync.Mutex
	summaries int
	sorts     []string
	sort      func(prompt string) string
}

func (a *fakeAgent) Ask(_ context.Context, _ string, prompt string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.Contains(prompt, "mapping each board key to its summary") {
		a.summaries++
		// Wrapped the way agents wrap JSON, to prove it is read anyway.
		return "Here you go:\n```json\n{\"b1\":\"Бытовые дела по квартире.\",\"b2\":\"Разработка XCIII.\"}\n```", nil
	}
	a.sorts = append(a.sorts, prompt)
	return a.sort(prompt), nil
}

func newSorter(t *testing.T, board Board, agent Agent) *Sorter {
	t.Helper()
	db, err := appschema.Open(filepath.Join(t.TempDir(), "xciii.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s, err := New(board, NewStore(db), filepath.Join(t.TempDir(), "sorting.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	s.SetAgent(agent)
	if _, err := s.SetSettings(Settings{Enabled: true, Agent: "claude"}); err != nil {
		t.Fatal(err)
	}
	return s
}

// The whole feature: a card from the inbox ends up on the board the agent
// chose, and the log says where and why, so a person who finds it there can
// tell who put it there.
func TestACardIsCarriedToTheBoardTheAgentChose(t *testing.T) {
	board := newFakeBoard()
	board.arrive(CardInfo{ID: "c-1", Title: "Счёт за электричество"}, "Оплатить до 25-го")
	agent := &fakeAgent{sort: func(string) string {
		return `[{"card":"c1","board":"b1","reason":"Счёт за квартиру"}]`
	}}
	s := newSorter(t, board, agent)

	res, err := s.SortInbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Moved != 1 || board.filed["c-1"] != "home" {
		t.Fatalf("the card was not filed on the home board: %+v, filed %v", res, board.filed)
	}
	log, err := s.store.Decisions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 || log[0].Outcome != OutcomeMoved || log[0].BoardID != "home" || log[0].Reason == nil || log[0].Reason.Text != "Счёт за квартиру" {
		t.Fatalf("the decision is not in the log as it was made: %+v", log)
	}
}

// What the agent decides from is what a person would: the board's description
// and what its cards are about, and the card's own text.
func TestTheAgentIsShownTheDescriptionsTheSummariesAndTheCardText(t *testing.T) {
	board := newFakeBoard()
	board.arrive(CardInfo{ID: "c-1", Title: "Письмо от управляющей компании"}, "Отключат воду в четверг")
	agent := &fakeAgent{sort: func(string) string { return `[]` }}
	s := newSorter(t, board, agent)

	if _, err := s.SortInbox(context.Background()); err != nil {
		t.Fatal(err)
	}
	prompt := agent.sorts[0]
	for _, want := range []string{"Покупки, ремонт, счета за квартиру", "Бытовые дела по квартире.", "Отключат воду в четверг", "Купить лампочки"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the sorting prompt does not say %q", want)
		}
	}
	if strings.Contains(prompt, "[b3]") || strings.Contains(prompt, "] Входящие") {
		t.Error("the inbox is offered as a board to sort onto")
	}
}

// A card the agent is unsure about stays for a person: that is the safe
// answer, and it is logged so "why is this still here" has one too.
func TestACardTheAgentLeavesStaysInTheInbox(t *testing.T) {
	board := newFakeBoard()
	board.arrive(CardInfo{ID: "c-1", Title: "???"}, "")
	board.arrive(CardInfo{ID: "c-2", Title: "Ещё что-то"}, "")
	agent := &fakeAgent{sort: func(string) string {
		// c2 is not mentioned at all.
		return `[{"card":"c1","board":"","reason":"Непонятно, о чём это"}]`
	}}
	s := newSorter(t, board, agent)

	res, err := s.SortInbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Kept != 2 || res.Moved != 0 || len(board.filed) != 0 {
		t.Fatalf("a card moved without an answer that names a board: %+v, filed %v", res, board.filed)
	}
}

// A board key the agent made up is a card left in the inbox, never a card moved
// somewhere else.
func TestABoardTheAgentInventedMovesNothing(t *testing.T) {
	board := newFakeBoard()
	board.arrive(CardInfo{ID: "c-1", Title: "Счёт"}, "")
	agent := &fakeAgent{sort: func(string) string { return `[{"card":"c1","board":"b9","reason":"x"}]` }}
	s := newSorter(t, board, agent)

	res, err := s.SortInbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Kept != 1 || len(board.filed) != 0 {
		t.Fatalf("an unknown board moved the card: %+v", res)
	}
}

// The summary is written once and read back: a sort that finds the boards'
// cards unchanged asks the agent about the inbox and nothing else.
func TestASummaryIsNotRewrittenWhileTheCardsAreTheSame(t *testing.T) {
	board := newFakeBoard()
	agent := &fakeAgent{sort: func(string) string { return `[]` }}
	s := newSorter(t, board, agent)

	board.arrive(CardInfo{ID: "c-1", Title: "Первое"}, "")
	if _, err := s.SortInbox(context.Background()); err != nil {
		t.Fatal(err)
	}
	board.arrive(CardInfo{ID: "c-2", Title: "Второе"}, "")
	if _, err := s.SortInbox(context.Background()); err != nil {
		t.Fatal(err)
	}
	if agent.summaries != 1 {
		t.Fatalf("summaries were written %d times for boards whose cards did not change", agent.summaries)
	}
	state, err := s.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range state.Boards {
		if b.Summary == "" {
			t.Errorf("board %s has no summary in the dialog", b.ID)
		}
	}
}

// Refreshing from the dialog rewrites summaries even when nothing changed:
// that is what the button is for.
func TestRefreshingRewritesTheSummaries(t *testing.T) {
	board := newFakeBoard()
	agent := &fakeAgent{sort: func(string) string { return `[]` }}
	s := newSorter(t, board, agent)

	for range 2 {
		if err := s.RefreshSummaries(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if agent.summaries != 2 {
		t.Fatalf("the button wrote summaries %d times out of 2", agent.summaries)
	}
}

// An arrival while sorting is switched off leaves the card for a person — the
// inbox's default, and what an install that never opened the dialog gets.
func TestNothingIsQueuedWhileSortingIsOff(t *testing.T) {
	board := newFakeBoard()
	agent := &fakeAgent{sort: func(string) string { return `[]` }}
	s := newSorter(t, board, agent)
	if _, err := s.SetSettings(Settings{Enabled: false, Agent: "claude"}); err != nil {
		t.Fatal(err)
	}
	s.Arrived("c-1")
	if len(s.arrivals) != 0 {
		t.Fatal("an arrival was queued with sorting switched off")
	}
}

// Switching sorting on without saying who sorts would be a switch that does
// nothing; it is refused where it is flipped.
func TestSortingCannotBeSwitchedOnWithoutAnAgent(t *testing.T) {
	s := newSorter(t, newFakeBoard(), &fakeAgent{})
	if _, err := s.SetSettings(Settings{Enabled: true}); err == nil {
		t.Fatal("sorting was switched on with no agent named")
	}
}

// A card the person carried away while it waited in the queue is not the
// sorter's to move any more.
func TestACardNoLongerInTheInboxIsLeftAlone(t *testing.T) {
	board := newFakeBoard()
	agent := &fakeAgent{sort: func(string) string { return `[{"card":"c1","board":"b1","reason":"x"}]` }}
	s := newSorter(t, board, agent)

	res, err := s.sort(context.Background(), map[string]bool{"gone": true})
	if err != nil {
		t.Fatal(err)
	}
	if res != (Result{}) || len(agent.sorts) != 0 {
		t.Fatalf("a card that is not in the inbox was sorted: %+v", res)
	}
}

// The settings survive a restart: they are a file, read when the sorter is made.
func TestSettingsAreReadBackFromTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sorting.json")
	if err := SaveSettings(path, Settings{Enabled: true, Agent: "codex"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != (Settings{Enabled: true, Agent: "codex"}) {
		t.Fatalf("read back %+v", got)
	}
}

func TestJSONIsFoundInsideProse(t *testing.T) {
	var got []decision
	if err := decodeJSON("Sure! [not json] here:\n[{\"card\":\"c1\",\"board\":\"b2\"}]\nDone.", '[', &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Board != "b2" {
		t.Fatalf("read %+v", got)
	}
	if err := decodeJSON("I could not decide.", '[', &got); err == nil {
		t.Fatal("an answer without JSON was read as one")
	}
}
