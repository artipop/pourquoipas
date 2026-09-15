package boardclient

import (
	"context"
	"fmt"

	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardmodel"
)

var _ acp.BoardCardState = (*Client)(nil)

// CardFlow reads back where the card stands. A card with nothing under the key
// — every card that is not on a route, which is most of them — is not an error.
func (c *Client) CardFlow(_ context.Context, cardID string) (acp.FlowState, bool, error) {
	block, err := c.cardBlock(cardID)
	if err != nil {
		return acp.FlowState{}, false, err
	}
	raw, ok := block.Fields[boardmodel.CardFieldFlow]
	if !ok || raw == nil {
		return acp.FlowState{}, false, nil
	}
	st, err := boardmodel.FlowStateFromField(raw)
	if err != nil {
		return acp.FlowState{}, false, fmt.Errorf("card %s: %w", cardID, err)
	}
	// The card knows which board it is on better than whatever wrote the field.
	st.CardID, st.BoardID = block.ID, block.BoardID
	return st, true, nil
}

// SetCardFlow records the position on the card, silently: this is the
// integration's own bookkeeping, and a card that announced it had changed would
// set off the very automation that just wrote it.
//
// Over HTTP "silently" now means two things, and the second is new. The write
// carries disable_notify, as it always did — and the board marks the broadcast
// it makes anyway as silent (server/ws.UpdateBlockMsg.Silent), which is what
// keeps this client from hearing its own bookkeeping come back and reading it
// as a person's drag.
func (c *Client) SetCardFlow(_ context.Context, cardID string, st acp.FlowState) error {
	field, err := boardmodel.FlowStateField(st)
	if err != nil {
		return err
	}
	return c.patchCardFields(cardID, &model.BlockPatch{UpdatedFields: map[string]any{boardmodel.CardFieldFlow: field}})
}

// ClearCardFlow is what a card dragged off its route leaves behind: nothing.
func (c *Client) ClearCardFlow(_ context.Context, cardID string) error {
	return c.patchCardFields(cardID, &model.BlockPatch{DeletedFields: []string{boardmodel.CardFieldFlow}})
}

// BoardCardFlows is every parked card of one board.
func (c *Client) BoardCardFlows(_ context.Context, boardID string) ([]acp.FlowState, error) {
	blocks, resp := c.api.GetBlocksOfType(boardID, string(model.TypeCard))
	if err := failed(resp, "read cards of board %s", boardID); err != nil {
		return nil, err
	}
	out := make([]acp.FlowState, 0, 8)
	for _, block := range blocks {
		if block == nil || block.DeleteAt != 0 {
			continue
		}
		raw, ok := block.Fields[boardmodel.CardFieldFlow]
		if !ok || raw == nil {
			continue
		}
		st, err := boardmodel.FlowStateFromField(raw)
		if err != nil {
			// One unreadable card must not hide every other parked card on the
			// board — the answer here is a poll list, and a short one is worse
			// than a wrong entry.
			continue
		}
		st.CardID, st.BoardID = block.ID, block.BoardID
		out = append(out, st)
	}
	return out, nil
}

// cardBlock reads a card as a block. Never model.Card: that view refuses a card
// whose fields it does not recognise, and the flow key is exactly such a field.
func (c *Client) cardBlock(cardID string) (*model.Block, error) {
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return nil, err
	}
	block, resp := c.api.GetBlockByID(boardID, cardID)
	if err := failed(resp, "read card %s", cardID); err != nil {
		return nil, err
	}
	if block == nil || block.Type != model.TypeCard {
		return nil, fmt.Errorf("block %s is not a card", cardID)
	}
	return block, nil
}

// patchCardFields writes one of our own keys on a card, silently.
func (c *Client) patchCardFields(cardID string, patch *model.BlockPatch) error {
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return err
	}
	return boardmodel.RetryOnceInTheSameMillisecond(func() error {
		_, resp := c.api.PatchBlock(boardID, cardID, patch, true)
		return failed(resp, "patch card %s", cardID)
	})
}
