package boardadapter

import (
	"testing"
	"time"

	"github.com/artipop/pourquoipas/server/mlog"

	"github.com/artipop/pourquoipas/internal/sources"
)

// A card the sorter files stands in the board's first column, where a person
// will find it, and wakes nothing: the column's automation is for a card
// somebody put there, and nobody has read this one yet. Its link comes with it
// even though the board calls its url property something else.
func TestASortedCardStandsInTheFirstColumnAndStartsNothing(t *testing.T) {
	logger, err := mlog.NewLogger()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Shutdown() })
	events := NewEventsBackend(logger)
	a := newTestAppWith(t, events)
	events.SetApp(a)

	work := boardWithAKanban(t, a)
	w := NewSourceWriter(a)
	inbox, err := w.InboxBoard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cardID, err := w.CreateCard(t.Context(), inbox, sources.CardSpec{
		Title: "Письмо", Source: "почта", URL: "https://example.com/letter", Body: "Текст письма",
	})
	if err != nil {
		t.Fatal(err)
	}
	moved, err := events.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	sorter := NewSortBoard(w.Writer)
	if text, err := sorter.CardText(t.Context(), cardID); err != nil || text != "Текст письма" {
		t.Fatalf("the card's text reads %q (%v)", text, err)
	}
	if err := sorter.FileCard(t.Context(), cardID, work.ID); err != nil {
		t.Fatal(err)
	}

	card, err := w.cardBlock(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if card.BoardID != work.ID {
		t.Fatalf("the card is on %s, not on the board it was filed on", card.BoardID)
	}
	props, _ := card.Fields["properties"].(map[string]any)
	if props["prop-status"] != "opt-todo" {
		t.Errorf("the card stands in %v, not in the first column", props["prop-status"])
	}
	cards, err := w.BoardCards(t.Context(), work.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].Link != "https://example.com/letter" {
		t.Errorf("the link did not travel with the card: %+v", cards)
	}
	select {
	case ev := <-moved:
		t.Fatalf("filing the card set the trigger off: %+v", ev)
	case <-time.After(300 * time.Millisecond):
	}
}
