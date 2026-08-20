package boardclient

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/boardmodel"
)

// The board as a list shows it — what the phone at /m and the share dialog
// read, over HTTP rather than through the app layer. The shapes and everything
// that reads a block are internal/boardmodel's, so the two ways to reach a
// board cannot come to describe one differently.

// Boards lists the boards this session can see, by title.
func (c *Client) Boards(_ context.Context) ([]boardmodel.BoardSummary, error) {
	boards, resp := c.api.GetBoardsForTeam(c.teamID)
	if err := failed(resp, "list boards"); err != nil {
		return nil, err
	}
	out := make([]boardmodel.BoardSummary, 0, len(boards))
	for _, board := range boards {
		if board == nil || board.IsTemplate || board.DeleteAt != 0 {
			continue
		}
		summary := boardmodel.BoardSummary{ID: board.ID, Title: board.Title, Icon: board.Icon}
		if property, columns, err := c.columnsOf(board); err == nil {
			summary.Property = property
			summary.Columns = columns
		}
		out = append(out, summary)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

// columnsOf is the board's column property and its options, in the board's own
// order — which is the order a person sees on the kanban.
func (c *Client) columnsOf(board *model.Board) (string, []boardmodel.ColumnOption, error) {
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return "", nil, err
	}
	views, resp := c.api.GetBlocksOfType(board.ID, model.TypeView)
	if err := failed(resp, "read views of board %s", board.ID); err != nil {
		return "", nil, err
	}
	name, ok := boardmodel.ColumnPropertyName(board, schema, views)
	if !ok {
		return "", nil, fmt.Errorf("на доске %s нет свойства-колонок", board.ID)
	}
	prop, ok := boardmodel.FindCardProperty(board.CardProperties, name)
	if !ok {
		return name, nil, nil
	}
	options, _ := prop["options"].([]any)
	columns := make([]boardmodel.ColumnOption, 0, len(options))
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
		columns = append(columns, boardmodel.ColumnOption{Value: value, Color: color})
	}
	return name, columns, nil
}

// BoardCards lists a board's cards with the column each one stands in. The
// card's text is deliberately not among them: it lives in blocks of its own,
// which over HTTP is a request per card.
func (c *Client) BoardCards(_ context.Context, boardID string) ([]boardmodel.CardSummary, error) {
	board, schema, err := c.board(boardID)
	if err != nil {
		return nil, err
	}
	views, resp := c.api.GetBlocksOfType(boardID, model.TypeView)
	if err := failed(resp, "read views of board %s", boardID); err != nil {
		return nil, err
	}
	property, _ := boardmodel.ColumnPropertyName(board, schema, views)
	linkID, _ := boardmodel.PropertyOfType(board, schema, "url")
	authors := c.boardAuthors(boardID)

	blocks, resp := c.api.GetBlocksOfType(boardID, string(model.TypeCard))
	if err := failed(resp, "list cards of board %s", boardID); err != nil {
		return nil, err
	}
	out := make([]boardmodel.CardSummary, 0, len(blocks))
	for _, block := range blocks {
		if block == nil || block.DeleteAt != 0 {
			continue
		}
		if isTemplate, _ := block.Fields["isTemplate"].(bool); isTemplate {
			continue
		}
		icon, _ := block.Fields["icon"].(string)
		card := boardmodel.CardSummary{
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
		c.rememberCard(block.ID, boardID)
		out = append(out, card)
	}
	// Newest first: a list on a phone is read from the top.
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdateAt > out[j].UpdateAt })
	return out, nil
}

// boardAuthors maps the board's members to the names a person reads. Read once
// per board rather than per card — here that is the difference between a
// handful of requests and one per row.
func (c *Client) boardAuthors(boardID string) map[string]string {
	members, resp := c.api.GetMembersForBoard(boardID)
	if resp != nil && resp.Error != nil {
		return nil
	}
	out := make(map[string]string, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		user, resp := c.api.GetUser(member.UserID)
		if resp != nil && resp.Error != nil || user == nil {
			continue
		}
		out[user.ID] = user.Username
	}
	return out
}

// MoveCardToBoard carries a card to another board and, if a column is named,
// puts it in that column.
//
// The move into the column is the one write here that lets the board notify,
// and it has to: a person moved this card, so a column that runs an agent has
// to start one, exactly as it would if the card had been dragged.
func (c *Client) MoveCardToBoard(_ context.Context, cardID, toBoardID, column string) error {
	// A move that reports failure may still have landed — the board records
	// every card write in a history keyed by (id, insert_at) in whole
	// milliseconds — so the card is asked where it is now rather than retried:
	// a second attempt finds it already there and fails differently.
	if _, resp := c.api.MoveCardToBoard(cardID, toBoardID); resp != nil && resp.Error != nil {
		c.forgetCard(cardID)
		block, readErr := c.cardBlock(cardID)
		if readErr != nil || block.BoardID != toBoardID {
			return fmt.Errorf("перенос карточки %s на доску %s: %w", cardID, toBoardID, resp.Error)
		}
	}
	c.rememberCard(cardID, toBoardID)

	if strings.TrimSpace(column) == "" {
		return nil
	}
	board, schema, err := c.board(toBoardID)
	if err != nil {
		return err
	}
	views, resp := c.api.GetBlocksOfType(toBoardID, model.TypeView)
	if err := failed(resp, "read views of board %s", toBoardID); err != nil {
		return err
	}
	property, ok := boardmodel.ColumnPropertyName(board, schema, views)
	if !ok {
		return fmt.Errorf("на доске %s нет свойства-колонок", toBoardID)
	}
	propID, optionID, ok := boardmodel.FindSelectOption(schema, property, column)
	if !ok {
		return fmt.Errorf("на доске %s нет колонки %q", toBoardID, column)
	}
	return c.patchCard(cardID, &model.CardPatch{UpdatedProperties: map[string]any{propID: optionID}}, false)
}
