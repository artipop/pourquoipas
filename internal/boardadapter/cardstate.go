package boardadapter

import (
	"context"
	"fmt"

	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardmodel"
)

var _ acp.BoardCardState = (*Writer)(nil)

// CardFlow reads back where the card stands. A card with nothing under the key
// — every card that is not on a route, which is most of them — is not an error.
func (w *Writer) CardFlow(_ context.Context, cardID string) (acp.FlowState, bool, error) {
	card, err := w.cardBlock(cardID)
	if err != nil {
		return acp.FlowState{}, false, err
	}
	raw, ok := card.Fields[boardmodel.CardFieldFlow]
	if !ok || raw == nil {
		return acp.FlowState{}, false, nil
	}
	st, err := boardmodel.FlowStateFromField(raw)
	if err != nil {
		return acp.FlowState{}, false, fmt.Errorf("card %s: %w", cardID, err)
	}
	// The card knows which board it is on better than whatever wrote the field
	// — after an import it is the only one that does.
	st.CardID, st.BoardID = card.ID, card.BoardID
	return st, true, nil
}

// SetCardFlow records the position on the card. It never notifies: this is the
// integration's own bookkeeping, and a card that announced it had changed would
// set off the very automation that just wrote it.
func (w *Writer) SetCardFlow(_ context.Context, cardID string, st acp.FlowState) error {
	field, err := boardmodel.FlowStateField(st)
	if err != nil {
		return err
	}
	return w.patchCardFields(cardID, &model.BlockPatch{UpdatedFields: map[string]any{boardmodel.CardFieldFlow: field}})
}

// ClearCardFlow is what a card dragged off its route leaves behind: nothing.
func (w *Writer) ClearCardFlow(_ context.Context, cardID string) error {
	return w.patchCardFields(cardID, &model.BlockPatch{DeletedFields: []string{boardmodel.CardFieldFlow}})
}

// BoardCardFlows is every parked card of one board. It reads the card blocks
// for the same reason CardsForBoard does: the board's own Card view refuses a
// card whose fields it does not recognise, and ours is exactly such a field.
func (w *Writer) BoardCardFlows(_ context.Context, boardID string) ([]acp.FlowState, error) {
	blocks, err := w.app.GetBlocks(boardID, "", string(model.TypeCard))
	if err != nil {
		return nil, fmt.Errorf("get cards of board %s: %w", boardID, err)
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

// patchCardFields writes one of our own keys on a card, silently.
func (w *Writer) patchCardFields(cardID string, patch *model.BlockPatch) error {
	if _, err := w.cardBlock(cardID); err != nil {
		return err
	}
	return boardmodel.RetryOnceInTheSameMillisecond(func() error {
		_, err := w.app.PatchBlockAndNotify(cardID, patch, model.SingleUser, true)
		return err
	})
}
