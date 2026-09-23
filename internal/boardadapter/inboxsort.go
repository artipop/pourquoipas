package boardadapter

import (
	"context"
	"fmt"

	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/boardmodel"
	"github.com/artipop/xciii/internal/inboxsort"
)

// SortBoard is the board as internal/inboxsort reads and writes it. A type of
// its own over the Writer rather than more methods on it: the Writer already
// answers Boards and BoardCards for the phone, in shapes the sorter has no use
// for, and one method name cannot have two answers.
//
// It is this process's board only. The inbox is this machine's (docs/sources.md,
// §14), and a card is sorted onto the boards beside it; a desktop attached to a
// server carries a card there by hand.
type SortBoard struct{ w *Writer }

var _ inboxsort.Board = SortBoard{}

func NewSortBoard(w *Writer) SortBoard { return SortBoard{w: w} }

func (s SortBoard) Boards(_ context.Context) ([]inboxsort.BoardInfo, error) {
	boards, err := s.w.app.GetBoardsForUserAndTeam(model.SingleUser, model.GlobalTeamID, false)
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	out := make([]inboxsort.BoardInfo, 0, len(boards))
	for _, board := range boards {
		if board.IsTemplate || board.DeleteAt != 0 {
			continue
		}
		out = append(out, inboxsort.BoardInfo{
			ID: board.ID, Title: board.Title, Icon: board.Icon,
			Description: board.Description, Inbox: IsInboxBoard(board),
		})
	}
	return out, nil
}

func (s SortBoard) Cards(ctx context.Context, boardID string) ([]inboxsort.CardInfo, error) {
	cards, err := s.w.BoardCards(ctx, boardID)
	if err != nil {
		return nil, err
	}
	out := make([]inboxsort.CardInfo, 0, len(cards))
	for _, c := range cards {
		out = append(out, inboxsort.CardInfo{ID: c.ID, Title: c.Title, Author: c.Author, Link: c.Link})
	}
	return out, nil
}

func (s SortBoard) CardText(_ context.Context, cardID string) (string, error) {
	card, err := s.w.cardBlock(cardID)
	if err != nil {
		return "", err
	}
	return cardText(s.w.app, card.BoardID, card), nil
}

// FileCard carries the card and stands it in the new board's first column,
// silently. It is not MoveCardToBoard with a column: that write notifies,
// because a person who picks a column is asking for what the column does, and
// the sorter picked a board and nothing else — an agent starting on a card
// nobody has read is the decision the inbox exists to leave with a person.
//
// The link is carried by type rather than by name, which is how the move
// carries everything else: the inbox calls its url property one thing and the
// board may call its own another.
func (s SortBoard) FileCard(ctx context.Context, cardID, boardID string) error {
	card, err := s.w.cardBlock(cardID)
	if err != nil {
		return err
	}
	link := s.linkOf(card)

	if _, err := s.w.app.MoveCardToBoard(cardID, boardID, model.SingleUser); err != nil {
		// See Writer.MoveCardToBoard: a move that reports failure may have
		// landed, and only the card can say.
		if moved, readErr := s.w.cardBlock(cardID); readErr != nil || moved.BoardID != boardID {
			return fmt.Errorf("перенос карточки %s на доску %s: %w", cardID, boardID, err)
		}
	}

	board, err := s.w.app.GetBoard(boardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", boardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return err
	}
	patch := &model.CardPatch{UpdatedProperties: map[string]any{}}
	if views, err := s.w.boardViews(boardID); err == nil {
		if propID, optionID, ok := boardmodel.IntakeColumn(board, schema, views); ok {
			patch.UpdatedProperties[propID] = optionID
		}
	}
	if link != "" {
		if propID, err := s.w.ensureLinkProperty(boardID); err == nil {
			patch.UpdatedProperties[propID] = link
		}
	}
	if len(patch.UpdatedProperties) == 0 {
		return nil
	}
	return s.w.patchCard(cardID, patch, true)
}

// linkOf is the card's value of its board's url property, if it has one.
func (s SortBoard) linkOf(card *model.Block) string {
	board, err := s.w.app.GetBoard(card.BoardID)
	if err != nil {
		return ""
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return ""
	}
	propID, ok := boardmodel.PropertyOfType(board, schema, "url")
	if !ok {
		return ""
	}
	value, _ := boardmodel.RawProperties(card)[propID].(string)
	return value
}

func (s SortBoard) SetDescription(_ context.Context, boardID, text string) error {
	patch := &model.BoardPatch{Description: &text}
	if _, err := s.w.app.PatchBoard(patch, boardID, model.SingleUser); err != nil {
		return fmt.Errorf("описание доски %s: %w", boardID, err)
	}
	return nil
}
