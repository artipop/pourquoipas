package boardadapter

import (
	"strings"
	"testing"
	"time"

	"github.com/artipop/pourquoipas/server/app"
	"github.com/artipop/pourquoipas/server/mlog"
	"github.com/artipop/pourquoipas/server/model"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/author"
)

// What an agent does to a card through the board tools ends here, in the real
// board: a patch of a real block, seen by the real trigger. Both halves are
// worth running against it rather than against a fake — the property ids come
// from a schema the board parsed, and whether a write is *heard* is decided by a
// flag two layers down.

// boardWithACard makes a board with a column property and one card standing in
// it, which is the smallest board these tools can be asked about.
func boardWithACard(t *testing.T, a *app.App) (*model.Board, *model.Card) {
	t.Helper()
	board, err := a.CreateBoard(&model.Board{
		TeamID: model.GlobalTeamID,
		Type:   model.BoardTypeOpen,
		Title:  "Доска",
		CardProperties: []map[string]any{
			{
				"id": "prop-status", "name": "Статус", "type": "select",
				"options": []any{
					map[string]any{"id": "opt-ideas", "value": "Идеи", "color": ""},
					map[string]any{"id": "opt-agent", "value": "К агенту", "color": ""},
				},
			},
			{
				"id": "prop-project", "name": "Проекты", "type": "select",
				"options": []any{map[string]any{"id": "opt-pourquoipas", "value": "pourquoipas", "color": ""}},
			},
		},
	}, model.SingleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	card, err := a.CreateCard(&model.Card{
		Title:      "Починить окно",
		Properties: map[string]any{"prop-status": "opt-ideas"},
	}, board.ID, model.SingleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	// The block history is keyed by (id, insert_at) in milliseconds, so a card
	// written and changed inside the same millisecond collides with its own
	// history row. Only a test is ever that fast; a person or an agent taking a
	// second call to move a card never is.
	time.Sleep(2 * time.Millisecond)
	return board, card
}

// A card an agent moves is a card moved: the trigger has to see it, or the
// column it landed in never starts and asking was pointless. This is the one
// write in Writer that lets the board notify, and this is why.
func TestAMoveThroughTheToolsSetsTheTriggerOff(t *testing.T) {
	logger, err := mlog.NewLogger()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Shutdown() })

	events := NewEventsBackend(logger)
	a := newTestAppWith(t, events)
	events.SetApp(a)
	board, card := boardWithACard(t, a)

	writer := NewWriter(a)
	err = writer.UpdateCard(t.Context(), card.ID, acp.CardEdit{
		Property: "Статус",
		Column:   "К агенту",
		Options:  []string{"pourquoipas"},
	})
	if err != nil {
		t.Fatal(err)
	}

	moved, err := events.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// One write that sets two select values is two events — the trigger is per
	// property, and which arrives first is the schema map's business. The one
	// that matters is the column's; the other names a property no column watches
	// and is dropped a layer up.
	deadline := time.After(5 * time.Second)
	var column acp.CardMoved
	for column.CardID == "" {
		select {
		case ev := <-moved:
			if ev.ToColumn.PropertyName == "Статус" {
				column = ev
			}
		case <-deadline:
			t.Fatal("the card moved and nothing heard it, so the column would never start")
		}
	}
	if column.CardID != card.ID || column.ToColumn.Name != "К агенту" || column.FromColumn.Name != "Идеи" {
		t.Errorf("the trigger heard %+v", column)
	}
	// Everything selected on the card travels with the event, so the project it
	// names is resolvable from the move alone.
	if !strings.Contains(strings.Join(column.OptionNames, ","), "pourquoipas") {
		t.Errorf("the values set with the move did not travel with it: %v", column.OptionNames)
	}

	// And the board really holds what the agent asked for, by id.
	after, err := a.GetBlockByID(card.ID)
	if err != nil {
		t.Fatal(err)
	}
	props, _ := after.Fields["properties"].(map[string]any)
	if props["prop-status"] != "opt-agent" || props["prop-project"] != "opt-pourquoipas" {
		t.Errorf("the card stands as %+v", props)
	}
	_ = board
}

// Listing is how an agent finds a card at all, and it has to come back in the
// vocabulary the rest of the tools speak: names, not the ids the board stores.
func TestCardsForBoardReadTheBoardBackByName(t *testing.T) {
	logger, err := mlog.NewLogger()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Shutdown() })

	events := NewEventsBackend(logger)
	a := newTestAppWith(t)
	events.SetApp(a)
	board, card := boardWithACard(t, a)

	cards, err := events.CardsForBoard(t.Context(), board.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("cards listed: %d, want the one on the board — %+v", len(cards), cards)
	}
	got := cards[0]
	if got.CardID != card.ID || got.Title != "Починить окно" {
		t.Errorf("the card reads as %+v", got)
	}
	// The board renders a select value upper-cased, which is what Props carries
	// everywhere; the option's own name — the one an agent has to send back — is
	// in OptionNames, exactly as it is for a card that moved.
	if !strings.EqualFold(got.Props["статус"], "Идеи") {
		t.Errorf("the column reads as %q", got.Props["статус"])
	}
	if len(got.OptionNames) != 1 || got.OptionNames[0] != "Идеи" {
		t.Errorf("selected values read as %v", got.OptionNames)
	}
	// A listing carries no bodies: each one is a query of its own, and a listing
	// is read to pick a card out rather than to work from it.
	if got.Body != "" {
		t.Errorf("a listing carried a body: %q", got.Body)
	}

	// A board with no cards is not an error; it is a board with no cards.
	empty, err := a.CreateBoard(&model.Board{TeamID: model.GlobalTeamID, Type: model.BoardTypeOpen, Title: "Пустая"}, model.SingleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	if cards, err := events.CardsForBoard(t.Context(), empty.ID); err != nil || len(cards) != 0 {
		t.Errorf("an empty board listed %+v, %v", cards, err)
	}
}

// The card's history is only an answer if the writes are signed. On a desktop
// the board runs single-user, so an agent's write used to arrive under the one
// account there is — the person's — and the history could only ever have said
// "you". A run says whose its writes are on the context, and this is where that
// reaches the board.
func TestAnAgentsWriteIsSignedByTheAgent(t *testing.T) {
	a := newTestApp(t)
	_, card := boardWithACard(t, a)
	writer := NewWriter(a)

	if err := writer.AddComment(author.With(t.Context(), "agent-user-1"), card.ID, "Сделал"); err != nil {
		t.Fatal(err)
	}
	if err := writer.AddComment(t.Context(), card.ID, "И это тоже"); err != nil {
		t.Fatal(err)
	}

	comments, err := a.GetBlocks(card.BoardID, card.ID, string(model.TypeComment))
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]string{}
	for _, c := range comments {
		by[c.Title] = c.CreatedBy
	}
	if by["Сделал"] != "agent-user-1" {
		t.Errorf("the agent's comment is signed %q, so the history cannot say who wrote it", by["Сделал"])
	}
	// And a caller that names nobody writes as the app always did, which is
	// what keeps every other path unchanged.
	if by["И это тоже"] != model.SingleUser {
		t.Errorf("a write with no author became %q", by["И это тоже"])
	}
}

// A property an agent fills in is the agent's too — the history line about a
// card moving is the one a person reads first.
func TestAnAgentsFieldWriteIsSignedByTheAgent(t *testing.T) {
	a := newTestApp(t)
	_, card := boardWithACard(t, a)
	writer := NewWriter(a)

	ctx := author.With(t.Context(), "agent-user-1")
	if err := writer.UpdateCard(ctx, card.ID, acp.CardEdit{Title: "Починил вход"}); err != nil {
		t.Fatal(err)
	}

	block, err := a.GetBlockByID(card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if block.ModifiedBy != "agent-user-1" {
		t.Errorf("the card was changed by %q, want the agent", block.ModifiedBy)
	}
}
