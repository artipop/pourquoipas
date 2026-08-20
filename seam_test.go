package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/artipop/xciii/internal/sources"
)

// oneBoard is a source writer that serves one board and refuses every other,
// which is what makes "who has this board" answerable by asking.
type oneBoard struct {
	boardID string
	// wrote records what reached this writer, by card.
	wrote map[string]string
}

func newOneBoard(boardID string) *oneBoard {
	return &oneBoard{boardID: boardID, wrote: map[string]string{}}
}

func (b *oneBoard) CreateCard(_ context.Context, boardID string, spec sources.CardSpec) (string, error) {
	if boardID != b.boardID {
		return "", fmt.Errorf("no board %s here", boardID)
	}
	id := b.boardID + "-card"
	b.wrote[id] = "created"
	return id, nil
}

func (b *oneBoard) AddComment(_ context.Context, cardID, text string) error {
	b.wrote[cardID] = "commented"
	return nil
}

func (b *oneBoard) MoveCardByOptionName(_ context.Context, cardID, _, _ string) error {
	b.wrote[cardID] = "moved"
	return nil
}

func (b *oneBoard) ColumnProperty(_ context.Context, boardID string) (string, error) {
	if boardID != b.boardID {
		return "", fmt.Errorf("no board %s here", boardID)
	}
	return "Статус", nil
}

func (b *oneBoard) EnsureInbox(_ context.Context, boardID, _, _ string) (string, error) {
	if boardID != b.boardID {
		return "", fmt.Errorf("no board %s here", boardID)
	}
	return "opt-inbox", nil
}

func (b *oneBoard) CardBySourceItem(_ context.Context, boardID, _, _ string) (string, string, bool, error) {
	if boardID != b.boardID {
		return "", "", false, fmt.Errorf("no board %s here", boardID)
	}
	return b.boardID + "-card", "v1", true, nil
}

// A source belongs to a board and a board belongs to this machine or to the
// server, so where a source writes is not a question anybody has to answer.
func TestASourceWritesToTheBoardItNames(t *testing.T) {
	local, remote := newOneBoard("board-here"), newOneBoard("board-there")
	w := newSourcesByBoard(local, remote, func(id string) bool { return id == "board-there" })
	ctx := context.Background()

	if _, err := w.CreateCard(ctx, "board-there", sources.CardSpec{Title: "письмо"}); err != nil {
		t.Fatalf("a card for the server's board was refused: %v", err)
	}
	if _, ok := remote.wrote["board-there-card"]; !ok {
		t.Error("the card did not reach the server")
	}
	if len(local.wrote) != 0 {
		t.Errorf("the card reached this machine as well: %v", local.wrote)
	}

	if _, err := w.CreateCard(ctx, "board-here", sources.CardSpec{Title: "заметка"}); err != nil {
		t.Fatalf("a card for this machine's board was refused: %v", err)
	}
	if _, ok := local.wrote["board-here-card"]; !ok {
		t.Error("the card did not reach this machine")
	}
}

// The two writes keyed by a card alone go to the board the card is on. The
// pipeline only ever learns a card id by creating the card or by finding it,
// and both name the board — so it is remembered rather than asked for.
func TestACommentFollowsTheCardToItsOwnBoard(t *testing.T) {
	local, remote := newOneBoard("board-here"), newOneBoard("board-there")
	w := newSourcesByBoard(local, remote, func(id string) bool { return id == "board-there" })
	ctx := context.Background()

	cardID, _, ok, err := w.CardBySourceItem(ctx, "board-there", "почта", "letter-1")
	if err != nil || !ok {
		t.Fatalf("the card was not found on the server: %v %v", ok, err)
	}
	if err := w.AddComment(ctx, cardID, "обновилось"); err != nil {
		t.Fatal(err)
	}
	if remote.wrote[cardID] != "commented" {
		t.Errorf("the comment did not follow the card to the server: %v", remote.wrote)
	}

	if err := w.MoveCardByOptionName(ctx, cardID, "Статус", "Готово"); err != nil {
		t.Fatal(err)
	}
	if remote.wrote[cardID] != "moved" {
		t.Errorf("the move did not follow the card to the server: %v", remote.wrote)
	}
}

// A card nobody created or found in this run goes where it would have gone
// before there was a second board.
func TestACardNobodyHasSeenGoesToThisMachine(t *testing.T) {
	local, remote := newOneBoard("board-here"), newOneBoard("board-there")
	w := newSourcesByBoard(local, remote, func(id string) bool { return id == "board-there" })

	if err := w.AddComment(context.Background(), "card-from-nowhere", "что-то"); err != nil {
		t.Fatal(err)
	}
	if local.wrote["card-from-nowhere"] != "commented" {
		t.Errorf("an unknown card did not go to this machine: %v %v", local.wrote, remote.wrote)
	}
}

// With nothing attached the router is the local writer and asks nothing.
func TestWithNoServerEverythingIsLocal(t *testing.T) {
	local := newOneBoard("board-here")
	w := newSourcesByBoard(local, nil, nil)

	if _, err := w.CreateCard(context.Background(), "board-here", sources.CardSpec{Title: "заметка"}); err != nil {
		t.Fatal(err)
	}
	if len(local.wrote) != 1 {
		t.Errorf("wrote %v", local.wrote)
	}
}
