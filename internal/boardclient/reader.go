package boardclient

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardmodel"
)

var (
	_ acp.BoardReader = (*Client)(nil)
	_ acp.BoardMeta   = (*Client)(nil)
)

// cardListLimit is how many cards one listing reads — the same answer the
// in-process adapter gives, and for the same reason: a board is a person's
// working set, not an archive.
const cardListLimit = 500

// CardByID reads one card. Three round trips at worst — which board, the block,
// the board's schema — and the first is cached, since a card changes board only
// through a move and the socket says when.
func (c *Client) CardByID(_ context.Context, cardID string) (acp.CardMoved, error) {
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return acp.CardMoved{}, err
	}
	block, resp := c.api.GetBlockByID(boardID, cardID)
	if err := failed(resp, "read card %s", cardID); err != nil {
		return acp.CardMoved{}, err
	}
	if block == nil || block.Type != model.TypeCard {
		return acp.CardMoved{}, fmt.Errorf("block %s is not a card", cardID)
	}
	_, schema, err := c.board(boardID)
	if err != nil {
		return acp.CardMoved{}, err
	}
	return boardmodel.CardEvent(boardID, block, schema, c.cardBody(boardID, block), c.resolver()), nil
}

// CardsForBoard lists a board's cards. Bodies are left out on purpose (see
// acp.BoardReader): each one is a request of its own here, where in the
// in-process adapter it was merely a query.
func (c *Client) CardsForBoard(_ context.Context, boardID string) ([]acp.CardMoved, error) {
	board, schema, err := c.board(boardID)
	if err != nil {
		return nil, err
	}
	blocks, resp := c.api.GetBlocksOfType(boardID, string(model.TypeCard))
	if err := failed(resp, "read cards of board %s", boardID); err != nil {
		return nil, err
	}

	resolver := c.resolver()
	out := make([]acp.CardMoved, 0, len(blocks))
	for _, block := range blocks {
		if block == nil || block.DeleteAt != 0 {
			continue
		}
		if isTemplate, _ := block.Fields["isTemplate"].(bool); isTemplate {
			continue
		}
		c.rememberCard(block.ID, board.ID)
		ev := boardmodel.CardEvent(board.ID, block, schema, "", resolver)
		ev.At = time.UnixMilli(block.UpdateAt)
		out = append(out, ev)
		if len(out) >= cardListLimit {
			break
		}
	}
	return out, nil
}

// cardBody joins the card's text content blocks in contentOrder order. The
// board has no route for "the text blocks of one card", so this reads the
// board's text blocks and keeps the card's own — which is a listing per body,
// and the reason a card listing carries none.
func (c *Client) cardBody(boardID string, card *model.Block) string {
	blocks, resp := c.api.GetBlocksOfType(boardID, string(model.TypeText))
	if resp != nil && resp.Error != nil {
		return ""
	}

	order := map[string]int{}
	if rawOrder, ok := card.Fields["contentOrder"].([]any); ok {
		for i, v := range rawOrder {
			if id, ok := v.(string); ok {
				order[id] = i
			}
		}
	}

	mine := make([]*model.Block, 0, len(order))
	for _, blk := range blocks {
		if blk != nil && blk.ParentID == card.ID && blk.DeleteAt == 0 {
			mine = append(mine, blk)
		}
	}
	sort.SliceStable(mine, func(i, j int) bool {
		oi, iok := order[mine[i].ID]
		oj, jok := order[mine[j].ID]
		switch {
		case iok && jok:
			return oi < oj
		case iok:
			return true
		default:
			return false
		}
	})

	var parts []string
	for _, blk := range mine {
		if t := strings.TrimSpace(blk.Title); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

// BoardProperties returns the board's own free-form properties, where its
// automation lives (acp.BoardPropColumns/Flows).
func (c *Client) BoardProperties(_ context.Context, boardID string) (map[string]any, error) {
	board, _, err := c.board(boardID)
	if err != nil {
		return nil, err
	}
	return board.Properties, nil
}

// SetBoardProperties writes the named properties and removes the named ones,
// leaving the rest alone.
//
// Who it is written as is the difference from the in-process adapter, and it is
// not a choice: there the app patches as the system user, here every request
// carries the session the desktop logged in with. So a board's automation saved
// from an attached desktop is modified by that person — which is what the board
// would say about any other change they made through a page.
func (c *Client) SetBoardProperties(_ context.Context, boardID string, props map[string]any, remove []string) error {
	patch := &model.BoardPatch{UpdatedProperties: props, DeletedProperties: remove}
	_, resp := c.api.PatchBoard(boardID, patch)
	return failed(resp, "patch board %s", boardID)
}

// IsBoardTemplate says the board is one to copy rather than to work in.
func (c *Client) IsBoardTemplate(_ context.Context, boardID string) (bool, error) {
	board, _, err := c.board(boardID)
	if err != nil {
		return false, err
	}
	return board.IsTemplate, nil
}

// BoardColumnOptions is the board's column property and every option of it.
func (c *Client) BoardColumnOptions(_ context.Context, boardID string) ([]acp.Column, error) {
	board, schema, err := c.board(boardID)
	if err != nil {
		return nil, err
	}
	recorded, _ := board.Properties[acp.BoardPropColumnProperty].(string)
	if _, ok := schema[recorded]; ok {
		return boardmodel.ColumnOptions(board, schema, nil, recorded), nil
	}
	// Only a board that never recorded its column property pays for the views.
	views, resp := c.api.GetBlocksOfType(boardID, model.TypeView)
	if err := failed(resp, "read views of board %s", boardID); err != nil {
		return nil, err
	}
	return boardmodel.ColumnOptions(board, schema, views, recorded), nil
}
