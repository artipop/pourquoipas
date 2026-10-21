package boardadapter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/artipop/pourquoipas/server/model"

	"github.com/artipop/pourquoipas/internal/boardmodel"
)

// The board as a phone reads it.
//
// The page at /m has no board API of its own — it is served the bindings and
// the event socket and nothing else — so what it needs from the board comes
// through here. It is a small, flat surface on purpose: a phone shows a list of cards
// and moves one, and everything else about a board belongs on a screen with
// room for it.
//
// Everything is named rather than identified, for the reason the agent's tools
// are: a column is what a person typed, and an id is what the board's own REST
// API would have made the page learn.

// The types a list is read as live in internal/boardmodel, so a board read over
// HTTP is read into the same shapes. Aliased rather than moved wholesale
// because this package's callers name them.
type (
	BoardSummary = boardmodel.BoardSummary
	CardSummary  = boardmodel.CardSummary
	Column       = boardmodel.ColumnOption
)

// Boards lists the boards of the single user this app runs as, newest first —
// the same order and the same set the sidebar shows.
func (w *Writer) Boards(ctx context.Context) ([]BoardSummary, error) {
	boards, err := w.app.GetBoardsForUserAndTeam(model.SingleUser, model.GlobalTeamID, false)
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	out := make([]BoardSummary, 0, len(boards))
	for _, board := range boards {
		if board.IsTemplate {
			continue
		}
		summary := BoardSummary{ID: board.ID, Title: board.Title, Icon: board.Icon, Inbox: IsInboxBoard(board)}
		if property, columns, err := w.columnsOf(board); err == nil {
			summary.Property = property
			summary.Columns = columns
		}
		out = append(out, summary)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

// columnsOf is the board's column property and its options, in the order the
// board keeps them — which is the order a person sees on the kanban.
func (w *Writer) columnsOf(board *model.Board) (string, []Column, error) {
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return "", nil, err
	}
	views, err := w.app.GetBlocks(board.ID, board.ID, model.TypeView)
	if err != nil {
		return "", nil, err
	}
	name, ok := boardmodel.ColumnPropertyName(board, schema, views)
	if !ok {
		return "", nil, fmt.Errorf("board %s has no column property", board.ID)
	}
	prop, ok := boardmodel.FindCardProperty(board.CardProperties, name)
	if !ok {
		return name, nil, nil
	}
	options, _ := prop["options"].([]any)
	columns := make([]Column, 0, len(options))
	for _, option := range options {
		fields, ok := option.(map[string]any)
		if !ok {
			continue
		}
		value, ok := fields["value"].(string)
		if !ok {
			continue
		}
		color, _ := fields["color"].(string)
		columns = append(columns, Column{Value: value, Color: color})
	}
	return name, columns, nil
}

// BoardCards lists a board's cards with the column each one stands in.
//
// The card's text is deliberately not among them: it lives in blocks of its
// own, one query per card, and a list that costs a query per row is a list
// that stops working on the board where it matters most. A title and where the
// card came from is what a phone decides on.
func (w *Writer) BoardCards(ctx context.Context, boardID string) ([]CardSummary, error) {
	board, err := w.app.GetBoard(boardID)
	if err != nil {
		return nil, fmt.Errorf("get board %s: %w", boardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return nil, err
	}
	views, err := w.app.GetBlocks(boardID, boardID, model.TypeView)
	if err != nil {
		return nil, fmt.Errorf("get views of board %s: %w", boardID, err)
	}
	property, _ := boardmodel.ColumnPropertyName(board, schema, views)
	linkID, _ := boardmodel.PropertyOfType(board, schema, "url")
	authors := w.boardAuthors(boardID)
	blocks, err := w.app.GetBlocks(boardID, "", model.TypeCard)
	if err != nil {
		return nil, fmt.Errorf("list cards of board %s: %w", boardID, err)
	}

	out := make([]CardSummary, 0, len(blocks))
	for _, block := range blocks {
		if isTemplate, _ := block.Fields["isTemplate"].(bool); isTemplate {
			continue
		}
		icon, _ := block.Fields["icon"].(string)
		card := CardSummary{
			ID:         block.ID,
			BoardID:    boardID,
			Title:      block.Title,
			Icon:       icon,
			Properties: boardmodel.DisplayProperties(schema, block),
			Author:     authors[block.CreatedBy],
			UpdateAt:   block.UpdateAt,
		}
		card.Column = card.Properties[property]
		if linkID != "" {
			if def, ok := schema[linkID]; ok {
				card.Link = card.Properties[def.Name]
			}
		}
		out = append(out, card)
	}
	// Newest first: a list on a phone is read from the top, and what changed
	// last is what a person is looking for.
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdateAt > out[j].UpdateAt })
	return out, nil
}

// boardAuthors maps the board's members to the names a person reads. Read once
// per board rather than per card: a list of a hundred cards is a handful of
// authors, and asking for each of them would be a query a row.
func (w *Writer) boardAuthors(boardID string) map[string]string {
	members, err := w.app.GetMembersForBoard(boardID)
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(members))
	for _, member := range members {
		user, err := w.app.GetUser(member.UserID)
		if err != nil || user == nil {
			continue
		}
		out[user.ID] = user.Username
	}
	return out
}

// MoveCardToBoard carries a card to another board and, if a column is named,
// puts it there.
//
// The move into the column is the one write here that lets the board notify,
// and it has to: a person moved this card, so a column that runs an agent has
// to start one, exactly as it would if the card had been dragged. Everything
// else in this file is the integration's own bookkeeping and must stay quiet.
func (w *Writer) MoveCardToBoard(ctx context.Context, cardID, toBoardID, column string) error {
	// A move that reports failure may still have landed: the board records
	// every card write in a history keyed by (id, insert_at) in whole
	// milliseconds, so a move a millisecond after any other write to the same
	// card fails on that key after the card has already changed hands. Asking
	// the card where it is now is the only way to tell the two apart — and
	// retrying is not, because the second attempt finds the card already there
	// and answers with a different error entirely.
	if _, err := w.app.MoveCardToBoard(cardID, toBoardID, model.SingleUser); err != nil {
		card, readErr := w.cardBlock(cardID)
		if readErr != nil || card.BoardID != toBoardID {
			return fmt.Errorf("moving card %s to board %s: %w", cardID, toBoardID, err)
		}
	}
	if strings.TrimSpace(column) == "" {
		return nil
	}
	board, err := w.app.GetBoard(toBoardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", toBoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return err
	}
	views, err := w.app.GetBlocks(toBoardID, toBoardID, model.TypeView)
	if err != nil {
		return fmt.Errorf("get views of board %s: %w", toBoardID, err)
	}
	property, ok := boardmodel.ColumnPropertyName(board, schema, views)
	if !ok {
		return fmt.Errorf("board %s has no column property", toBoardID)
	}
	propID, optionID, ok := boardmodel.FindSelectOption(schema, property, column)
	if !ok {
		return fmt.Errorf("board %s has no column %q", toBoardID, column)
	}
	patch := &model.CardPatch{UpdatedProperties: map[string]any{propID: optionID}}
	_, err = w.app.PatchCard(patch, cardID, model.SingleUser, false)
	return err
}
