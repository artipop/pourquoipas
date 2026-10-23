package acp

import (
	"context"
	"fmt"
	"testing"
)

// knownCards answers only for the cards it was given, which is what makes it
// possible to ask "whose card is this" by asking.
type knownCards struct {
	boardID string
	cards   map[string]bool
	asked   int
}

func (k *knownCards) CardByID(_ context.Context, cardID string) (CardMoved, error) {
	k.asked++
	if !k.cards[cardID] {
		return CardMoved{}, fmt.Errorf("no card %s here", cardID)
	}
	return CardMoved{CardID: cardID, BoardID: k.boardID}, nil
}

func (k *knownCards) CardsForBoard(_ context.Context, boardID string) ([]CardMoved, error) {
	if boardID != k.boardID {
		return nil, fmt.Errorf("no board %s here", boardID)
	}
	return nil, nil
}

func (k *knownCards) BoardProperties(_ context.Context, boardID string) (map[string]any, error) {
	if boardID != k.boardID {
		return nil, fmt.Errorf("no board %s here", boardID)
	}
	return map[string]any{}, nil
}

func (k *knownCards) SetBoardProperties(context.Context, string, map[string]any, []string) error {
	return nil
}
func (k *knownCards) IsBoardTemplate(context.Context, string) (bool, error) { return false, nil }
func (k *knownCards) BoardAbout(context.Context, string) (BoardAbout, error) {
	return BoardAbout{}, nil
}
func (k *knownCards) BoardColumnOptions(context.Context, string) ([]Column, error) {
	return nil, nil
}
func (k *knownCards) CardProperties(context.Context, string) ([]CardProperty, error) {
	return nil, nil
}

// An install that works its own board must not pay for the possibility of a
// second one, and must not behave differently for it existing in the code.
func TestOneBoardIsAskedNothingAboutWhereItIs(t *testing.T) {
	own := &knownCards{boardID: "board-1", cards: map[string]bool{"card-1": true}}
	m := &Manager{reader: own, meta: own}

	if src := m.forCard("card-1"); src.Reader != own {
		t.Fatal("the only source was not used for a card")
	}
	if src := m.forBoard("board-1"); src.Meta != own {
		t.Fatal("the only source was not used for a board")
	}
	if own.asked != 0 {
		t.Errorf("the only source was asked %d times where a card lives", own.asked)
	}
}

// With two, the source that has the card is the one that answers — which is
// the whole of the routing, and it needs no interface method of its own.
func TestACardIsAskedOfTheSourceThatHasIt(t *testing.T) {
	own := &knownCards{boardID: "board-1", cards: map[string]bool{"card-1": true}}
	server := &knownCards{boardID: "board-2", cards: map[string]bool{"card-2": true}}

	m := &Manager{reader: own, meta: own}
	m.AddBoardSource(BoardSource{ID: "https://board.example.com", Reader: server, Meta: server})

	if src := m.forCard("card-2"); src.Reader != server {
		t.Fatal("a card on the server was asked of this machine's own board")
	}
	if src := m.forCard("card-1"); src.Reader != own {
		t.Fatal("a card of this machine was asked of the server")
	}
	if src := m.forBoard("board-2"); src.Meta != server {
		t.Fatal("a board on the server was asked of this machine")
	}
}

// Asked once, remembered: routing costs a request the first time and nothing
// afterwards, which is what makes asking an acceptable way to route at all.
func TestWhereACardLivesIsAskedOnce(t *testing.T) {
	own := &knownCards{boardID: "board-1", cards: map[string]bool{}}
	server := &knownCards{boardID: "board-2", cards: map[string]bool{"card-2": true}}

	m := &Manager{reader: own, meta: own}
	m.AddBoardSource(BoardSource{ID: "server", Reader: server, Meta: server})

	for i := 0; i < 5; i++ {
		if src := m.forCard("card-2"); src.Reader != server {
			t.Fatal("the card moved between sources, which cannot happen")
		}
	}
	if own.asked != 1 || server.asked != 1 {
		t.Errorf("asked own %d times and the server %d, want once each", own.asked, server.asked)
	}

	// And the card's board came with it, so a question about the board is free.
	if src := m.forBoard("board-2"); src.Meta != server {
		t.Fatal("the board the card named was not remembered with it")
	}
	if server.asked != 1 {
		t.Errorf("the board was asked for separately: %d", server.asked)
	}
}
