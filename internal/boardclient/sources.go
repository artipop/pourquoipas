package boardclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/utils"

	"github.com/artipop/xciii/internal/boardmodel"
	"github.com/artipop/xciii/internal/sources"
)

// The board as internal/sources writes to it, over HTTP. A wrapper type for the
// reason the in-process one is: a source's CreateCard and an agent's have
// different signatures, and Go allows one method of a name.
type SourceWriter struct {
	*Client
}

var (
	_ sources.BoardWriter = (*SourceWriter)(nil)
	_ sources.BoardItems  = (*SourceWriter)(nil)
)

func NewSourceWriter(c *Client) *SourceWriter { return &SourceWriter{c} }

// InboxBoard is refused over HTTP: the inbox is the board of the machine the
// sources run on, and a server's board is only ever the one the share dialog
// picked (sourcesByBoard asks the local board).
func (w *SourceWriter) InboxBoard(context.Context) (string, error) {
	return "", errors.New("«Входящие» — доска этой машины, а не сервера")
}

// CreateCard puts a card a source brought on the board, in its first column.
// Silent, like every write here: the trigger fires on a *change* of the column
// property, so the card starts nothing.
func (w *SourceWriter) CreateCard(ctx context.Context, boardID string, spec sources.CardSpec) (string, error) {
	board, schema, err := w.board(boardID)
	if err != nil {
		return "", err
	}

	properties := boardmodel.CardProperties(schema, spec.Properties)
	if properties == nil {
		properties = map[string]any{}
	}
	views, resp := w.api.GetBlocksOfType(boardID, model.TypeView)
	if err := failed(resp, "read views of board %s", boardID); err != nil {
		return "", err
	}
	if propID, optionID, ok := boardmodel.IntakeColumn(board, schema, views); ok {
		if _, taken := properties[propID]; !taken {
			properties[propID] = optionID
		}
	}
	// The link is the pipeline's own doing rather than a rule's, so it is not
	// looked up among the properties above: the board's url property is found
	// by type and made if the board has none. A rule that named a property of
	// its own still wins — it was written against this board by somebody
	// looking at it.
	if url := strings.TrimSpace(spec.URL); url != "" {
		if id, err := w.ensureLinkProperty(boardID, board, schema); err == nil {
			if _, taken := properties[id]; !taken {
				properties[id] = url
			}
		}
	}

	now := utils.GetMillis()
	card := &model.Block{
		ID:       utils.NewID(utils.IDTypeCard),
		BoardID:  boardID,
		ParentID: boardID,
		Type:     model.TypeCard,
		Title:    spec.Title,
		Fields: map[string]any{
			"properties":   properties,
			"contentOrder": []any{},
			"icon":         spec.Icon,
			// Which item of which source this card came from, so the board
			// itself answers "have we brought this one already" once it has
			// left this machine.
			boardmodel.CardFieldSource: cardOrigin(spec),
		},
		CreateAt: now,
		UpdateAt: now,
	}
	if _, resp := w.api.InsertBlocks(boardID, []*model.Block{card}, true); resp != nil && resp.Error != nil {
		return "", fmt.Errorf("create card on board %s: %w", boardID, resp.Error)
	}
	w.rememberCard(card.ID, boardID)

	if body := strings.TrimSpace(spec.Body); body != "" {
		text := &model.Block{
			ID:       utils.NewID(model.BlockType2IDType(model.TypeText)),
			BoardID:  boardID,
			ParentID: card.ID,
			Type:     model.TypeText,
			Title:    body,
			Fields:   map[string]any{},
			CreateAt: now,
			UpdateAt: now,
		}
		if _, resp := w.api.InsertBlocks(boardID, []*model.Block{text}, true); resp == nil || resp.Error == nil {
			if err := w.appendContent(card.ID, text.ID); err != nil {
				w.log.Warn("boardclient: the card's body is not in its contentOrder")
			}
		}
	}
	return card.ID, nil
}

// CardBySourceItem finds the card a source's item became. Not found is not an
// error: most items have never been seen.
func (w *SourceWriter) CardBySourceItem(_ context.Context, boardID, source, externalID string) (string, string, bool, error) {
	blocks, resp := w.api.GetBlocksOfType(boardID, string(model.TypeCard))
	if err := failed(resp, "read cards of board %s", boardID); err != nil {
		return "", "", false, err
	}
	for _, block := range blocks {
		if block == nil || block.DeleteAt != 0 {
			continue
		}
		origin, ok := boardmodel.CardSourceOf(block)
		if !ok {
			continue
		}
		if !strings.EqualFold(origin.Source, source) || origin.ExternalID != externalID {
			continue
		}
		return block.ID, origin.Version, true, nil
	}
	return "", "", false, nil
}

// cardOrigin is the origin as the card stores it. A card with no origin — one a
// person typed — carries no key rather than an empty one.
func cardOrigin(spec sources.CardSpec) map[string]any {
	if strings.TrimSpace(spec.Source) == "" || strings.TrimSpace(spec.Item.ExternalID) == "" {
		return nil
	}
	return map[string]any{
		"source":     spec.Source,
		"externalId": spec.Item.ExternalID,
		"version":    spec.Item.Version,
	}
}

// ensureLinkProperty is the board's url property, made if it has none. The
// board is passed in because the caller has just read it.
func (w *SourceWriter) ensureLinkProperty(boardID string, board *model.Board, schema model.PropSchema) (string, error) {
	if id, ok := boardmodel.PropertyOfType(board, schema, "url"); ok {
		return id, nil
	}
	propID := utils.NewID(utils.IDTypeBlock)
	prop := map[string]any{
		"id":      propID,
		"name":    boardmodel.LinkPropertyTitle,
		"type":    "url",
		"options": []any{},
	}
	patch := &model.BoardPatch{UpdatedCardProperties: []map[string]any{prop}}
	if _, resp := w.api.PatchBoard(boardID, patch); resp != nil && resp.Error != nil {
		return "", fmt.Errorf("add the link property to board %s: %w", boardID, resp.Error)
	}
	return propID, nil
}
