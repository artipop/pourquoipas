package boardadapter

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/artipop/pourquoipas/server/app"
	"github.com/artipop/pourquoipas/server/model"
	"github.com/artipop/pourquoipas/server/utils"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/author"
	"github.com/artipop/pourquoipas/internal/boardmodel"
)

// Writer implements acp.BoardWriter over the server's app layer. All writes
// pass disableNotify=true: notify backends (incl. EventsBackend) are skipped
// so our own writes never re-trigger the agent, while the WebSocket broadcast
// still updates the UI live.
type Writer struct {
	app *app.App
	// log is for the writes that must not fail the caller — the bookkeeping
	// this integration keeps on a card beside what a person filled in. The
	// board's own logger is not reachable from here, and slog is what the rest
	// of this app uses anyway.
	log *slog.Logger
}

var _ acp.BoardWriter = (*Writer)(nil)

func NewWriter(a *app.App) *Writer { return &Writer{app: a, log: slog.Default()} }

// cardBlock is the card every write here starts from. It reads the block rather
// than the board's own Card view on purpose: Block2Card refuses a card whose
// contentOrder is not a list — which a card created with none is, stored as JSON
// null — and a card that cannot be read is a card that cannot be commented on or
// moved. Nothing below needs anything from a card but the board it stands on.
func (w *Writer) cardBlock(cardID string) (*model.Block, error) {
	block, err := w.app.GetBlockByID(cardID)
	if err != nil {
		return nil, fmt.Errorf("get card %s: %w", cardID, err)
	}
	if block == nil || block.Type != model.TypeCard {
		return nil, fmt.Errorf("block %s is not a card", cardID)
	}
	return block, nil
}

// AddComment posts a comment block on the card.
func (w *Writer) AddComment(ctx context.Context, cardID, text string) error {
	card, err := w.cardBlock(cardID)
	if err != nil {
		return err
	}
	now := utils.GetMillis()
	block := &model.Block{
		ID:       utils.NewID(utils.IDTypeBlock),
		BoardID:  card.BoardID,
		ParentID: cardID,
		Type:     model.TypeComment,
		Title:    text,
		// The card draws an agent's comment differently from a person's — one
		// is a report, the other is somebody talking — and this is what it
		// reads to know which. The author says *which* agent; this says that
		// it is one, which is still the question on a board whose agents have
		// no accounts yet.
		Fields:    map[string]any{"agent": true},
		CreatedBy: w.author(ctx),
		CreateAt:  now,
		UpdateAt:  now,
	}
	_, err = w.app.InsertBlocksAndNotify([]*model.Block{block}, w.author(ctx), true)
	return err
}

// author is whose this write is. An agent's run says so on the context
// (internal/author), and without that the board takes the write as the
// single user it runs as — which is what every write was before the card kept
// a history worth reading.
func (w *Writer) author(ctx context.Context) string {
	if who := author.From(ctx); who != "" {
		return who
	}
	return model.SingleUser
}

// MoveCard sets the card's select property to optionID (post-MVP: used to
// advance the card after a successful session).
func (w *Writer) MoveCard(ctx context.Context, cardID, optionID string) error {
	card, err := w.cardBlock(cardID)
	if err != nil {
		return err
	}
	board, err := w.app.GetBoard(card.BoardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", card.BoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
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
		return fmt.Errorf("no select property on board %s has option %s", board.ID, optionID)
	}
	return w.patchCard(ctx, cardID, &model.CardPatch{UpdatedProperties: map[string]any{propID: optionID}}, true)
}

// MoveCardByOptionName moves a card to a column named in the config rather than
// identified by id — "Tested", not "a7f3…". Property and option are matched
// case-insensitively, as the trigger columns are.
func (w *Writer) MoveCardByOptionName(ctx context.Context, cardID, propertyName, optionName string) error {
	card, err := w.cardBlock(cardID)
	if err != nil {
		return err
	}
	board, err := w.app.GetBoard(card.BoardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", card.BoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return err
	}
	propID, optionID, ok := boardmodel.FindSelectOption(schema, propertyName, optionName)
	if !ok {
		return fmt.Errorf("board %s has no column %q in the property %q", board.ID, optionName, propertyName)
	}
	return w.patchCard(ctx, cardID, &model.CardPatch{UpdatedProperties: map[string]any{propID: optionID}}, true)
}

// UpdateCard changes an existing card the way a person editing it would: its
// title, the column it stands in, its other select values — all named, never
// identified by id.
//
// It is the one write here that lets the board notify: a card moved because an
// agent asked for it has to set off the column's automation, or asking was
// pointless. Everything else in this file stays silent so the integration's own
// writes cannot re-trigger the agent that produced them.
//
// A name the board does not have is refused rather than dropped. CreateCard
// takes the opposite bargain, and for a reason that does not hold here: there a
// plan of five cards must not be lost to one wrong guess, while here one card
// was asked to change one way, and half of that change is not it.
func (w *Writer) UpdateCard(ctx context.Context, cardID string, edit acp.CardEdit) error {
	card, err := w.cardBlock(cardID)
	if err != nil {
		return err
	}
	board, err := w.app.GetBoard(card.BoardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", card.BoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return err
	}

	patch, err := boardmodel.CardPatchFor(schema, edit)
	if err != nil {
		return err
	}
	return w.patchCard(ctx, cardID, patch, false)
}

// SetCardText writes one text property of a card, by id. Silent: it is the
// machine recording where a card's work lives, and it must not set the column's
// automation off the way a person's edit does.
func (w *Writer) SetCardText(ctx context.Context, cardID, propertyID, value string) error {
	if strings.TrimSpace(propertyID) == "" {
		return nil
	}
	return w.patchCard(ctx, cardID, &model.CardPatch{UpdatedProperties: map[string]any{propertyID: value}}, true)
}

// SetCardOption writes one select property by id, choosing the option whose
// name this is. Silent, like SetCardText and for the same reason: it is the
// machine recording a fact about the card, not a person editing it, and a
// notify here would set the column's automation off.
//
// An option the board does not have is not invented: adding options is a
// deliberate act elsewhere, and a stage outcome arriving at a board whose
// property somebody has edited must not quietly grow it back.
func (w *Writer) SetCardOption(ctx context.Context, cardID, propertyID, optionName string) error {
	if strings.TrimSpace(propertyID) == "" || strings.TrimSpace(optionName) == "" {
		return nil
	}
	card, err := w.cardBlock(cardID)
	if err != nil {
		return err
	}
	board, err := w.app.GetBoard(card.BoardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", card.BoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return err
	}
	def, ok := schema[propertyID]
	if !ok {
		return fmt.Errorf("the board has no property with id %q", propertyID)
	}
	for oid, opt := range def.Options {
		if strings.EqualFold(opt.Value, optionName) {
			return w.patchCard(ctx, cardID, &model.CardPatch{UpdatedProperties: map[string]any{propertyID: oid}}, true)
		}
	}
	return fmt.Errorf("the property %q has no value %q", def.Name, optionName)
}

// SetCardFields writes named properties of a card: a select property gets the
// option whose name the value is, anything else keeps the value as text. It is
// the write behind a stage's declared outputs (acp.PropertyWrite) — silent,
// because the stage's own outcome is the event the route acts on, and the
// refusals name what a person can fix: a property the board does not have, or
// an option a select does not carry.
func (w *Writer) SetCardFields(ctx context.Context, cardID string, fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}
	card, err := w.cardBlock(cardID)
	if err != nil {
		return err
	}
	board, err := w.app.GetBoard(card.BoardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", card.BoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
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
	return w.patchCard(ctx, cardID, &model.CardPatch{UpdatedProperties: properties}, true)
}

// AddCardProperty adds a text property to the card's board, unless one of that
// name is already there.
func (w *Writer) AddCardProperty(ctx context.Context, cardID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("a field needs a name")
	}
	card, err := w.cardBlock(cardID)
	if err != nil {
		return err
	}
	board, err := w.app.GetBoard(card.BoardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", card.BoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return err
	}
	if _, _, ok := boardmodel.FindPropertyByName(schema, name); ok {
		return nil
	}
	patch := &model.BoardPatch{UpdatedCardProperties: []map[string]any{textProperty(name)}}
	if _, err := w.app.PatchBoard(patch, card.BoardID, w.author(ctx)); err != nil {
		return fmt.Errorf("add the property %q to board %s: %w", name, card.BoardID, err)
	}
	return nil
}

// textProperty is a new text property of the board's cards.
func textProperty(name string) map[string]any {
	return map[string]any{
		"id":      utils.NewID(utils.IDTypeBlock),
		"name":    name,
		"type":    "text",
		"options": []any{},
	}
}

// patchCard applies a card patch as a block patch. app.PatchCard would do the
// same and then convert the result back into a Card, which fails for a card
// whose contentOrder is not a list — and a write that landed must not be
// reported as an error because the answer could not be rendered.
func (w *Writer) patchCard(ctx context.Context, cardID string, patch *model.CardPatch, disableNotify bool) error {
	if len(patch.UpdatedProperties) > 0 {
		block, err := w.app.GetBlockByID(cardID)
		if err != nil {
			return fmt.Errorf("get card %s: %w", cardID, err)
		}
		boardmodel.MergeCardProperties(boardmodel.RawProperties(block), patch)
	}
	blockPatch, err := model.CardPatch2BlockPatch(patch)
	if err != nil {
		return err
	}
	_, err = w.app.PatchBlockAndNotify(cardID, blockPatch, w.author(ctx), disableNotify)
	return err
}

// boardViews is every view of a board.
//
// Asked for by board and *not* by parent, which is the obvious way to ask and
// the wrong one: a board made from a template keeps the template's board id in
// its views' parentId, so "the views whose parent is this board" misses every
// view a template brought — which is all of them on most boards here. That is
// what hid the kanban from the inbox code: it looked for views, found none, and
// carried on having done nothing.
func (w *Writer) boardViews(boardID string) ([]*model.Block, error) {
	views, err := w.app.GetBlocks(boardID, "", model.TypeView)
	if err != nil {
		return nil, fmt.Errorf("get views of board %s: %w", boardID, err)
	}
	return views, nil
}

// ColumnProperty is the name of the property whose options are this board's
// columns. It is asked of the board rather than assumed, because a constant
// default can only ever be right for boards in one language: ours say
// the status property, the upstream templates say "Status", and a source pointed at the
// wrong name files nothing anywhere.
//
// The answer is the property the board itself groups by — what a person sees as
// the columns — and a board whose views group by nothing falls back to its
// first select property, which is what a kanban view would have picked too.
func (w *Writer) ColumnProperty(ctx context.Context, boardID string) (string, error) {
	board, err := w.app.GetBoard(boardID)
	if err != nil {
		return "", fmt.Errorf("get board %s: %w", boardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return "", err
	}
	views, err := w.boardViews(boardID)
	if err != nil {
		return "", err
	}
	name, ok := boardmodel.ColumnPropertyName(board, schema, views)
	if !ok {
		return "", fmt.Errorf("board %s has no column property", boardID)
	}
	return name, nil
}

// ensureLinkProperty returns the id of the board's url property, adding one if
// it has none. A card from a source is the only thing that asks for it, so a
// board nothing arrives on never grows the field.
func (w *Writer) ensureLinkProperty(ctx context.Context, boardID string) (string, error) {
	board, err := w.app.GetBoard(boardID)
	if err != nil {
		return "", fmt.Errorf("get board %s: %w", boardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return "", err
	}
	if id, ok := boardmodel.PropertyOfType(board, schema, "url"); ok {
		return id, nil
	}
	propID := utils.NewID(utils.IDTypeBlock)
	prop := map[string]any{
		"id":      propID,
		"name":    namingWords()(boardmodel.LinkPropertyTitle),
		"type":    "url",
		"options": []any{},
	}
	patch := &model.BoardPatch{UpdatedCardProperties: []map[string]any{prop}}
	if _, err := w.app.PatchBoard(patch, boardID, w.author(ctx)); err != nil {
		return "", fmt.Errorf("add the link property to board %s: %w", boardID, err)
	}
	return propID, nil
}

// AttachFile puts a file into the card's content: an image is rendered inline,
// anything else becomes a download. This is how a test run's screenshots end up
// where a human reads the result, instead of in a directory nobody opens.
func (w *Writer) AttachFile(ctx context.Context, cardID, filename, mime string, data []byte) error {
	card, err := w.cardBlock(cardID)
	if err != nil {
		return err
	}
	board, err := w.app.GetBoard(card.BoardID)
	if err != nil {
		return fmt.Errorf("get board %s: %w", card.BoardID, err)
	}
	fileID, err := w.app.SaveFile(bytes.NewReader(data), board.TeamID, card.BoardID, filename, false)
	if err != nil {
		return fmt.Errorf("save file %s: %w", filename, err)
	}

	blockType := model.BlockType(model.TypeAttachment)
	if strings.HasPrefix(mime, "image/") {
		blockType = model.TypeImage
	}
	now := utils.GetMillis()
	block := &model.Block{
		ID:        utils.NewID(model.BlockType2IDType(blockType)),
		BoardID:   card.BoardID,
		ParentID:  cardID,
		Type:      blockType,
		Title:     filename,
		Fields:    map[string]any{"fileId": fileID},
		CreatedBy: w.author(ctx),
		CreateAt:  now,
		UpdateAt:  now,
	}
	if _, err := w.app.InsertBlocksAndNotify([]*model.Block{block}, w.author(ctx), true); err != nil {
		return fmt.Errorf("insert %s block: %w", blockType, err)
	}
	return w.appendContent(ctx, cardID, block.ID)
}

// CreateCard puts a new card on a board, in the column the spec names, with its
// description as the card's first text block — the same shape a card typed by a
// person has, since it is read back by the same code (EventsBackend.cardBody).
//
// Properties are resolved by name and a name the board does not have is
// dropped rather than refused: a plan of five cards must not be lost because
// the agent guessed one option wrong, and what it got is in the report.
func (w *Writer) CreateCard(ctx context.Context, spec acp.NewCard) (string, error) {
	board, err := w.app.GetBoard(spec.BoardID)
	if err != nil {
		return "", fmt.Errorf("get board %s: %w", spec.BoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return "", err
	}

	properties := map[string]any{}
	if spec.Column != "" {
		propID, optionID, ok := boardmodel.FindSelectOption(schema, spec.Property, spec.Column)
		if !ok {
			return "", fmt.Errorf("board %s has no column %q in the property %q", board.ID, spec.Column, spec.Property)
		}
		properties[propID] = optionID
	}
	for _, option := range spec.Options {
		propID, optionID, ok := boardmodel.FindOptionByName(schema, option)
		// Not the column property: the column is the spec's own field, and an
		// option name that happens to match a column must not move the card
		// somewhere the caller did not ask for.
		if ok && properties[propID] == nil {
			properties[propID] = optionID
		}
	}

	// ContentOrder is set even though the card has no content yet: a nil one is
	// stored as JSON null, and the board's own Block2Card refuses to read a card
	// whose contentOrder is neither a list nor absent. A card nobody can read
	// back is a card nobody can comment on or move — which is everything that
	// was supposed to happen to it next.
	card, err := w.app.CreateCard(
		&model.Card{Title: spec.Title, Properties: properties, ContentOrder: []string{}},
		board.ID, w.author(ctx), true,
	)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(spec.Body) == "" {
		return card.ID, nil
	}

	now := utils.GetMillis()
	body := &model.Block{
		ID:        utils.NewID(model.BlockType2IDType(model.TypeText)),
		BoardID:   board.ID,
		ParentID:  card.ID,
		Type:      model.TypeText,
		Title:     spec.Body,
		Fields:    map[string]any{},
		CreatedBy: w.author(ctx),
		CreateAt:  now,
		UpdateAt:  now,
	}
	if _, err := w.app.InsertBlocksAndNotify([]*model.Block{body}, w.author(ctx), true); err != nil {
		// The card exists and is the point; a description that did not land is
		// worth reporting, not worth pretending the card was never made.
		return card.ID, fmt.Errorf("the card was created, but its description was not saved: %w", err)
	}
	if err := w.appendContent(ctx, card.ID, body.ID); err != nil {
		return card.ID, fmt.Errorf("the card was created, but its description did not attach to it: %w", err)
	}
	return card.ID, nil
}

// appendContent adds a block to the end of the card's content. The card's raw
// contentOrder is patched rather than model.CardPatch's flat []string, because
// the field may hold nested arrays (side-by-side rows) that flattening would
// silently rearrange.
func (w *Writer) appendContent(ctx context.Context, cardID, blockID string) error {
	block, err := w.app.GetBlockByID(cardID)
	if err != nil {
		return fmt.Errorf("get card block %s: %w", cardID, err)
	}
	patch := &model.BlockPatch{UpdatedFields: map[string]any{
		"contentOrder": appendContentOrder(block.Fields, blockID),
	}}
	_, err = w.app.PatchBlockAndNotify(cardID, patch, w.author(ctx), true)
	return err
}

// appendContentOrder returns the card's content order with blockID at the end,
// preserving whatever nesting it already had.
func appendContentOrder(fields map[string]any, blockID string) []any {
	order := []any{}
	if existing, ok := fields["contentOrder"].([]any); ok {
		order = append(order, existing...)
	}
	return append(order, blockID)
}

// patchCardFields writes our own keys on a card, silently.
func (w *Writer) patchCardFields(ctx context.Context, cardID string, patch *model.BlockPatch) error {
	if _, err := w.cardBlock(cardID); err != nil {
		return err
	}
	return boardmodel.RetryOnceInTheSameMillisecond(func() error {
		_, err := w.app.PatchBlockAndNotify(cardID, patch, w.author(ctx), true)
		return err
	})
}
