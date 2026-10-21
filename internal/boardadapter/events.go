// Package boardadapter bridges the board server to the agnostic
// internal/acp package. It is the only package allowed to import both sides:
// replacing the board backend later means replacing only this package.
package boardadapter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/artipop/pourquoipas/server/app"
	"github.com/artipop/pourquoipas/server/mlog"
	"github.com/artipop/pourquoipas/server/model"
	"github.com/artipop/pourquoipas/server/services/notify"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/boardmodel"
)

// EventsBackend is a notify.Backend that normalizes card select-property
// changes into acp.CardMoved events.
type EventsBackend struct {
	log mlog.LoggerIFace
	ch  chan acp.CardMoved

	mu  sync.Mutex
	app *app.App // set after server construction; used to fetch card bodies
}

var (
	_ notify.Backend  = (*EventsBackend)(nil)
	_ acp.BoardEvents = (*EventsBackend)(nil)
	_ acp.BoardReader = (*EventsBackend)(nil)
)

// NewEventsBackend creates the backend. Pass it into server.Params.NotifyBackends.
func NewEventsBackend(log mlog.LoggerIFace) *EventsBackend {
	return &EventsBackend{log: log, ch: make(chan acp.CardMoved, 256)}
}

// SetApp late-binds the app layer (available only after server.New returns).
func (b *EventsBackend) SetApp(a *app.App) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.app = a
}

func (b *EventsBackend) Name() string    { return "acpTrigger" }
func (b *EventsBackend) Start() error    { return nil }
func (b *EventsBackend) ShutDown() error { return nil }

// Subscribe returns the normalized event stream.
func (b *EventsBackend) Subscribe(ctx context.Context) (<-chan acp.CardMoved, error) {
	return b.ch, nil
}

// BlockChanged runs on the notify worker goroutine; it must stay fast and
// never block (buffered channel, drop on overflow).
func (b *EventsBackend) BlockChanged(evt notify.BlockChangeEvent) error {
	if evt.Action != notify.Update || evt.Board == nil ||
		evt.BlockChanged == nil || evt.BlockChanged.Type != model.TypeCard || evt.BlockOld == nil {
		return nil
	}

	schema, err := model.ParsePropertySchema(evt.Board)
	if err != nil {
		return fmt.Errorf("parse board schema: %w", err)
	}
	moves := boardmodel.MovedEvents(
		evt.Board.ID, evt.BlockChanged, schema,
		boardmodel.RawProperties(evt.BlockOld),
		b.cardBody(evt.Board.ID, evt.BlockChanged),
		boardmodel.NewResolver(b.appUserLookup()),
	)
	for _, ev := range moves {
		select {
		case b.ch <- ev:
		default:
			b.log.Warn("acpTrigger: event channel full, dropping card move", mlog.String("card", ev.CardID))
		}
	}
	return nil
}

// CardByID reads a card on demand so a console can be opened on it without
// moving it into the trigger column. The returned event carries no columns —
// nothing moved — but everything project/agent resolution needs.
func (b *EventsBackend) CardByID(ctx context.Context, cardID string) (acp.CardMoved, error) {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return acp.CardMoved{}, fmt.Errorf("board app is not ready")
	}
	block, err := a.GetBlockByID(cardID)
	if err != nil {
		return acp.CardMoved{}, fmt.Errorf("get card %s: %w", cardID, err)
	}
	if block == nil || block.Type != model.TypeCard {
		return acp.CardMoved{}, fmt.Errorf("block %s is not a card", cardID)
	}
	board, err := a.GetBoard(block.BoardID)
	if err != nil {
		return acp.CardMoved{}, fmt.Errorf("get board %s: %w", block.BoardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return acp.CardMoved{}, fmt.Errorf("parse board schema: %w", err)
	}
	return boardmodel.CardEvent(board.ID, block, schema, b.cardBody(board.ID, block),
		boardmodel.NewResolver(b.appUserLookup())), nil
}

// cardListLimit is how many cards one listing reads. A board is a person's
// working set, not an archive, and an agent given a thousand cards would spend
// its context on them rather than on the work.
const cardListLimit = 500

// CardsForBoard lists a board's cards, which is how an agent finds the card it
// has to act on: everything else it can do to one takes an id, and an id is not
// something a conversation carries.
//
// Bodies are left out on purpose (see acp.BoardReader): each one is a query of
// its own, and a listing is read to pick a card, not to work from it.
func (b *EventsBackend) CardsForBoard(ctx context.Context, boardID string) ([]acp.CardMoved, error) {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return nil, fmt.Errorf("board app is not ready")
	}
	board, err := a.GetBoard(boardID)
	if err != nil {
		return nil, fmt.Errorf("get board %s: %w", boardID, err)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return nil, fmt.Errorf("parse board schema: %w", err)
	}
	// The card blocks rather than app.GetCardsForBoard, and for the same reason
	// CardByID reads a block: the board's Card type is a lossy view that refuses
	// a card whose fields it does not recognise, and one such card would fail
	// the whole listing. A block is what a card is; everything below reads it
	// exactly as the trigger does.
	blocks, err := a.GetBlocks(boardID, "", string(model.TypeCard))
	if err != nil {
		return nil, fmt.Errorf("get cards of board %s: %w", boardID, err)
	}

	resolver := boardmodel.NewResolver(b.appUserLookup())
	out := make([]acp.CardMoved, 0, len(blocks))
	for _, block := range blocks {
		if block == nil || block.DeleteAt != 0 {
			continue
		}
		if isTemplate, _ := block.Fields["isTemplate"].(bool); isTemplate {
			continue
		}
		ev := boardmodel.CardEvent(board.ID, block, schema, "", resolver)
		// When the card was last touched, not now: a listing is a picture of
		// the board rather than something that just happened to it.
		ev.At = time.UnixMilli(block.UpdateAt)
		out = append(out, ev)
		if len(out) >= cardListLimit {
			break
		}
	}
	return out, nil
}

// BoardProperties returns the board's own free-form properties, where a
// template leaves the automation it ships (see acp.BoardPropColumns/Flows).
func (b *EventsBackend) BoardProperties(_ context.Context, boardID string) (map[string]any, error) {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return nil, fmt.Errorf("board app is not ready")
	}
	board, err := a.GetBoard(boardID)
	if err != nil {
		return nil, fmt.Errorf("get board %s: %w", boardID, err)
	}
	if board == nil {
		return nil, nil
	}
	return board.Properties, nil
}

// SetBoardProperties writes the named properties onto the board and leaves the
// rest alone, which is how a board's own automation is saved (acp.BoardProp*).
//
// The write is the system user's, not the person's at the keyboard: it is the
// app recording what the app was told, and attributing it to whoever happened
// to have the dialog open would put their name on a board they may only be
// looking at.
func (b *EventsBackend) SetBoardProperties(_ context.Context, boardID string, props map[string]any, remove []string) error {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return fmt.Errorf("board app is not ready")
	}
	patch := &model.BoardPatch{UpdatedProperties: props, DeletedProperties: remove}
	if _, err := a.PatchBoard(patch, boardID, model.SystemUserID); err != nil {
		return fmt.Errorf("patch board %s: %w", boardID, err)
	}
	return nil
}

// IsBoardTemplate says the board is one to copy rather than to work in.
func (b *EventsBackend) IsBoardTemplate(_ context.Context, boardID string) (bool, error) {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return false, fmt.Errorf("board app is not ready")
	}
	board, err := a.GetBoard(boardID)
	if err != nil {
		return false, fmt.Errorf("get board %s: %w", boardID, err)
	}
	return board != nil && board.IsTemplate, nil
}

// appUserLookup reads usernames through the app layer.
func (b *EventsBackend) appUserLookup() boardmodel.UserLookup {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return nil
	}
	return func(userID string) string {
		user, err := a.GetUser(userID)
		if err != nil || user == nil {
			return ""
		}
		return user.Username
	}
}

// cardBody joins the card's text content blocks in contentOrder order.
func (b *EventsBackend) cardBody(boardID string, card *model.Block) string {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return ""
	}
	return cardText(a, boardID, card)
}

// cardText is cardBody for whoever holds the app rather than the backend.
func cardText(a *app.App, boardID string, card *model.Block) string {
	blocks, err := a.GetBlocks(boardID, card.ID, string(model.TypeText))
	if err != nil || len(blocks) == 0 {
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
	sort.SliceStable(blocks, func(i, j int) bool {
		oi, iok := order[blocks[i].ID]
		oj, jok := order[blocks[j].ID]
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
	for _, blk := range blocks {
		if t := strings.TrimSpace(blk.Title); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

// CardProperties is the board's card fields for acp.BoardMeta — see
// boardmodel.ReadCardProperties, which is the half boardclient shares.
func (b *EventsBackend) CardProperties(_ context.Context, boardID string) ([]acp.CardProperty, error) {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return nil, fmt.Errorf("board app is not ready")
	}
	board, err := a.GetBoard(boardID)
	if err != nil {
		return nil, fmt.Errorf("get board %s: %w", boardID, err)
	}
	if board == nil {
		return nil, nil
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return nil, err
	}
	return boardmodel.ReadCardProperties(schema), nil
}

// BoardColumnOptions is the board's column property and its options, for
// acp.BoardMeta. The property is the one the board recorded for itself, and
// failing that the one a person reads as the columns — what a view groups by.
func (b *EventsBackend) BoardColumnOptions(_ context.Context, boardID string) ([]acp.Column, error) {
	b.mu.Lock()
	a := b.app
	b.mu.Unlock()
	if a == nil {
		return nil, fmt.Errorf("board app is not ready")
	}
	board, err := a.GetBoard(boardID)
	if err != nil {
		return nil, fmt.Errorf("get board %s: %w", boardID, err)
	}
	if board == nil {
		return nil, nil
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return nil, err
	}

	propID, _ := board.Properties[acp.BoardPropColumnProperty].(string)
	if _, ok := schema[propID]; !ok {
		views, err := a.GetBlocks(boardID, "", model.TypeView)
		if err != nil {
			return nil, fmt.Errorf("get views of board %s: %w", boardID, err)
		}
		name, found := boardmodel.ColumnPropertyName(board, schema, views)
		if !found {
			return nil, nil
		}
		propID = ""
		for id, def := range schema {
			if def.Type == "select" && strings.EqualFold(def.Name, name) {
				propID = id
				break
			}
		}
	}
	def, ok := schema[propID]
	if !ok {
		return nil, nil
	}
	out := make([]acp.Column, 0, len(def.Options))
	for id, opt := range def.Options {
		out = append(out, acp.Column{
			PropertyID: propID, PropertyName: def.Name,
			OptionID: id, Name: opt.Value,
		})
	}
	return out, nil
}
