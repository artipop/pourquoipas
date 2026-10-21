package boardclient

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/artipop/pourquoipas/server/model"
	"github.com/artipop/pourquoipas/server/utils"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/boardmodel"
)

var _ acp.BoardWriter = (*Client)(nil)

// AddComment writes what a stage had to say onto the card.
func (c *Client) AddComment(_ context.Context, cardID, text string) error {
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return err
	}
	now := utils.GetMillis()
	block := &model.Block{
		ID:       utils.NewID(model.BlockType2IDType(model.TypeComment)),
		BoardID:  boardID,
		ParentID: cardID,
		Type:     model.TypeComment,
		Title:    text,
		// What tells a session's report from somebody talking. The author
		// cannot: over HTTP every write is the session this desktop logged in
		// with, which is a person's account.
		Fields:   map[string]any{"agent": true},
		CreateAt: now,
		UpdateAt: now,
	}
	_, resp := c.api.InsertBlocks(boardID, []*model.Block{block}, true)
	return failed(resp, "comment on card %s", cardID)
}

// MoveCard sets the card's select property to optionID.
func (c *Client) MoveCard(_ context.Context, cardID, optionID string) error {
	boardID, schema, err := c.cardSchema(cardID)
	if err != nil {
		return err
	}
	propID := ""
	for id, def := range schema {
		if def.Type == "select" {
			if _, ok := def.Options[optionID]; ok {
				propID = id
				break
			}
		}
	}
	if propID == "" {
		return fmt.Errorf("no select property on board %s has option %s", boardID, optionID)
	}
	return c.patchCard(cardID, &model.CardPatch{UpdatedProperties: map[string]any{propID: optionID}}, true)
}

// MoveCardByOptionName moves a card to a column named in the config rather than
// identified by id — "Tested", not "a7f3…".
func (c *Client) MoveCardByOptionName(_ context.Context, cardID, propertyName, optionName string) error {
	boardID, schema, err := c.cardSchema(cardID)
	if err != nil {
		return err
	}
	propID, optionID, ok := boardmodel.FindSelectOption(schema, propertyName, optionName)
	if !ok {
		return fmt.Errorf("board %s has no column %q in the property %q", boardID, optionName, propertyName)
	}
	return c.patchCard(cardID, &model.CardPatch{UpdatedProperties: map[string]any{propID: optionID}}, true)
}

// UpdateCard changes an existing card the way a person editing it would. It is
// the one write here that lets the board notify: a card moved because an agent
// asked for it has to set off the column's automation, or asking was pointless.
func (c *Client) UpdateCard(_ context.Context, cardID string, edit acp.CardEdit) error {
	_, schema, err := c.cardSchema(cardID)
	if err != nil {
		return err
	}
	patch, err := boardmodel.CardPatchFor(schema, edit)
	if err != nil {
		return err
	}
	return c.patchCard(cardID, patch, false)
}

// SetCardText writes one text property of a card, by id. Silent: it is the
// machine recording where a card's work lives.
func (c *Client) SetCardText(_ context.Context, cardID, propertyID, value string) error {
	if strings.TrimSpace(propertyID) == "" {
		return nil
	}
	return c.patchCard(cardID, &model.CardPatch{UpdatedProperties: map[string]any{propertyID: value}}, true)
}

// SetCardOption writes one select property by id, choosing the option of this
// name — see the adapter's own for why by id and why silent.
func (c *Client) SetCardOption(_ context.Context, cardID, propertyID, optionName string) error {
	if strings.TrimSpace(propertyID) == "" || strings.TrimSpace(optionName) == "" {
		return nil
	}
	_, schema, err := c.cardSchema(cardID)
	if err != nil {
		return err
	}
	def, ok := schema[propertyID]
	if !ok {
		return fmt.Errorf("the board has no property with id %q", propertyID)
	}
	for oid, opt := range def.Options {
		if strings.EqualFold(opt.Value, optionName) {
			return c.patchCard(cardID, &model.CardPatch{UpdatedProperties: map[string]any{propertyID: oid}}, true)
		}
	}
	return fmt.Errorf("the property %q has no value %q", def.Name, optionName)
}

// SetCardFields writes named properties: a select gets the option whose name
// the value is, anything else keeps the value as text.
func (c *Client) SetCardFields(_ context.Context, cardID string, fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}
	_, schema, err := c.cardSchema(cardID)
	if err != nil {
		return err
	}
	properties := map[string]any{}
	for name, value := range fields {
		propID, def, ok := boardmodel.FindPropertyByName(schema, name)
		if !ok {
			return fmt.Errorf("the board has no property %q: see list_card_fields", name)
		}
		if def.Type == "select" {
			optionID := ""
			for oid, opt := range def.Options {
				if strings.EqualFold(opt.Value, value) {
					optionID = oid
					break
				}
			}
			if optionID == "" {
				return fmt.Errorf("the property %q has no value %q: list_card_fields lists the allowed ones", name, value)
			}
			properties[propID] = optionID
			continue
		}
		properties[propID] = value
	}
	return c.patchCard(cardID, &model.CardPatch{UpdatedProperties: properties}, true)
}

// AddCardProperty adds a text property to the card's board, unless one of that
// name is already there.
func (c *Client) AddCardProperty(_ context.Context, cardID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("a field needs a name")
	}
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return err
	}
	_, schema, err := c.board(boardID)
	if err != nil {
		return err
	}
	if _, _, ok := boardmodel.FindPropertyByName(schema, name); ok {
		return nil
	}
	prop := map[string]any{
		"id":      utils.NewID(utils.IDTypeBlock),
		"name":    name,
		"type":    "text",
		"options": []any{},
	}
	patch := &model.BoardPatch{UpdatedCardProperties: []map[string]any{prop}}
	if _, resp := c.api.PatchBoard(boardID, patch); resp != nil && resp.Error != nil {
		return fmt.Errorf("add the property %q to board %s: %w", name, boardID, resp.Error)
	}
	return nil
}

// AttachFile puts a file on the card and adds the block that shows it. The type
// and id prefix follow the file's kind as the in-process adapter's do: a card
// must not look different for having been written from here.
func (c *Client) AttachFile(_ context.Context, cardID, filename, mime string, data []byte) error {
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return err
	}
	upload, resp := c.api.TeamUploadFile(c.teamID, boardID, bytes.NewReader(data))
	if err := failed(resp, "upload %s", filename); err != nil {
		return err
	}
	if upload == nil || upload.FileID == "" {
		return fmt.Errorf("upload %s: the server named no file", filename)
	}

	blockType := model.BlockType(model.TypeAttachment)
	if strings.HasPrefix(mime, "image/") {
		blockType = model.TypeImage
	}
	now := utils.GetMillis()
	block := &model.Block{
		ID:       utils.NewID(model.BlockType2IDType(blockType)),
		BoardID:  boardID,
		ParentID: cardID,
		Type:     blockType,
		Title:    filename,
		Fields:   map[string]any{"fileId": upload.FileID},
		CreateAt: now,
		UpdateAt: now,
	}
	if _, resp := c.api.InsertBlocks(boardID, []*model.Block{block}, true); resp != nil && resp.Error != nil {
		return fmt.Errorf("insert %s block: %w", blockType, resp.Error)
	}
	return c.appendContent(cardID, block.ID)
}

// appendContent puts a block at the end of the card's content.
func (c *Client) appendContent(cardID, blockID string) error {
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return err
	}
	card, err := c.cardBlock(cardID)
	if err != nil {
		return err
	}
	order := []any{}
	if existing, ok := card.Fields["contentOrder"].([]any); ok {
		order = append(order, existing...)
	}
	patch := &model.BlockPatch{UpdatedFields: map[string]any{"contentOrder": append(order, blockID)}}
	_, resp := c.api.PatchBlock(boardID, cardID, patch, true)
	return failed(resp, "order the content of card %s", cardID)
}

// CreateCard is a card asked for from outside the board — by an agent through
// the board MCP server. A name the board does not have is dropped rather than
// refused: a plan of five cards must not be lost to one wrong guess.
func (c *Client) CreateCard(_ context.Context, spec acp.NewCard) (string, error) {
	_, schema, err := c.board(spec.BoardID)
	if err != nil {
		return "", err
	}

	properties := map[string]any{}
	if spec.Column != "" {
		if propID, optionID, ok := boardmodel.FindSelectOption(schema, spec.Property, spec.Column); ok {
			properties[propID] = optionID
		}
	}
	for _, option := range spec.Options {
		if propID, optionID, ok := boardmodel.FindOptionByName(schema, option); ok {
			if properties[propID] == nil {
				properties[propID] = optionID
			}
		}
	}

	now := utils.GetMillis()
	card := &model.Block{
		ID:       utils.NewID(utils.IDTypeCard),
		BoardID:  spec.BoardID,
		ParentID: spec.BoardID,
		Type:     model.TypeCard,
		Title:    spec.Title,
		Fields:   map[string]any{"properties": properties, "contentOrder": []any{}},
		CreateAt: now,
		UpdateAt: now,
	}
	if _, resp := c.api.InsertBlocks(spec.BoardID, []*model.Block{card}, true); resp != nil && resp.Error != nil {
		return "", fmt.Errorf("create card on board %s: %w", spec.BoardID, resp.Error)
	}
	c.rememberCard(card.ID, spec.BoardID)

	if body := strings.TrimSpace(spec.Body); body != "" {
		text := &model.Block{
			ID:       utils.NewID(model.BlockType2IDType(model.TypeText)),
			BoardID:  spec.BoardID,
			ParentID: card.ID,
			Type:     model.TypeText,
			Title:    body,
			Fields:   map[string]any{},
			CreateAt: now,
			UpdateAt: now,
		}
		if _, resp := c.api.InsertBlocks(spec.BoardID, []*model.Block{text}, true); resp != nil && resp.Error != nil {
			// The card exists and is the thing that was asked for; a body that
			// did not land is worth an id back rather than an error.
			c.log.Warn("boardclient: card created without its body")
			return card.ID, nil
		}
		// contentOrder is what makes the text show as the card's body rather
		// than as a stray block.
		if err := c.appendContent(card.ID, text.ID); err != nil {
			c.log.Warn("boardclient: card body is not in its contentOrder")
		}
	}
	return card.ID, nil
}

// cardSchema is the pair almost every write needs.
func (c *Client) cardSchema(cardID string) (string, model.PropSchema, error) {
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return "", nil, err
	}
	_, schema, err := c.board(boardID)
	if err != nil {
		return "", nil, err
	}
	return boardID, schema, nil
}

// patchCard applies a card patch as a block patch.
//
// The read-then-write is where this adapter is weaker than the in-process one:
// another writer can land between them. Only the property being set is at
// stake, and the alternative is worse — a block patch replaces `properties`
// whole, so a one-key patch would erase the rest.
func (c *Client) patchCard(cardID string, patch *model.CardPatch, disableNotify bool) error {
	boardID, err := c.boardOfCard(cardID)
	if err != nil {
		return err
	}
	if len(patch.UpdatedProperties) > 0 {
		block, err := c.cardBlock(cardID)
		if err != nil {
			return err
		}
		boardmodel.MergeCardProperties(boardmodel.RawProperties(block), patch)
	}
	blockPatch, err := model.CardPatch2BlockPatch(patch)
	if err != nil {
		return err
	}
	return boardmodel.RetryOnceInTheSameMillisecond(func() error {
		_, resp := c.api.PatchBlock(boardID, cardID, blockPatch, disableNotify)
		return failed(resp, "patch card %s", cardID)
	})
}
