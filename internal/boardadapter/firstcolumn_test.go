package boardadapter

import (
	"testing"
	"time"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/server/model"
)

// A board's columns are its stages, and a card in none of them stands in a
// column for «no status» that the board does not draw while it is empty. So
// every way a card arrives without a column named puts it in the first one,
// as the sources already did.

func cardColumn(t *testing.T, w *Writer, cardID string) string {
	t.Helper()
	card, err := w.cardBlock(cardID)
	if err != nil {
		t.Fatal(err)
	}
	props, _ := card.Fields["properties"].(map[string]any)
	column, _ := props["prop-status"].(string)
	return column
}

// An agent creating a card — or a planning conversation — need not name a column.
func TestACardMadeWithNoColumnStandsInTheFirst(t *testing.T) {
	a := newTestApp(t)
	board := boardWithAKanban(t, a)
	w := NewWriter(a)

	id, err := w.CreateCard(t.Context(), acp.NewCard{BoardID: board.ID, Title: "Без колонки"})
	if err != nil {
		t.Fatal(err)
	}
	if got := cardColumn(t, w, id); got != "opt-todo" {
		t.Errorf("the card stands in %q", got)
	}
}

// Carried from the inbox, which has no columns, a card brings no column with
// it; moved without one named it stands in the first.
func TestACardCarriedWithNoColumnStandsInTheFirst(t *testing.T) {
	a := newTestApp(t)
	board := boardWithAKanban(t, a)
	inbox, err := EnsureInboxBoard(a)
	if err != nil {
		t.Fatal(err)
	}
	card, err := a.CreateCard(&model.Card{Title: "Письмо", ContentOrder: []string{}}, inbox, model.SingleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	w := NewWriter(a)

	if err := w.MoveCardToBoard(t.Context(), card.ID, board.ID, ""); err != nil {
		t.Fatal(err)
	}
	if got := cardColumn(t, w, card.ID); got != "opt-todo" {
		t.Errorf("the card stands in %q", got)
	}
}
