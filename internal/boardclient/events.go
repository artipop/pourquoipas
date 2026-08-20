package boardclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/artipop/xciii/server/mlog"
	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardmodel"
)

var _ acp.BoardEvents = (*Client)(nil)

// The board's own websocket, read as the card moves internal/acp acts on.
//
// The socket carries the *new* block and nothing else, where the in-process
// adapter is handed the old one beside it. So this keeps a shadow: the select
// values of every card of every board it has read. The diff against it is what
// makes a move, and it is boardmodel.MovedEvents either way — the shadow is the
// whole of the difference between the two adapters.
//
// Three things follow, and each is a bug if it is not handled.
//
//   - **A card the shadow has never seen records and emits nothing.** Treating
//     an unknown previous value as a move would start a stage for every card on
//     the board at every launch.
//   - **Our own writes come back.** Every bookkeeping write this app makes is
//     broadcast, because the page has to redraw. What tells them apart is the
//     `silent` flag the board now puts on the message; without it this client
//     would read its own flow_state write as somebody dragging the card and
//     start the stage it has just finished.
//   - **A reconnect loses whatever happened while it was down.** The socket
//     carries no cursor, so every subscribed board is re-read and diffed
//     against the shadow: a missed move becomes a late move rather than a lost
//     one.

const (
	// How long to wait before dialling again. The board's own page uses a
	// backoff; this is a fixed pause because a desktop is one client and the
	// server it is attached to is not being protected from it.
	reconnectAfter = 5 * time.Second
	// The board expects a ping now and then, and a dead connection is
	// otherwise noticed only when something is sent.
	pingEvery = 30 * time.Second
)

type wsCommand struct {
	Action string `json:"action"`
	TeamID string `json:"teamId,omitempty"`
	Token  string `json:"token,omitempty"`
}

type wsMessage struct {
	Action string       `json:"action"`
	TeamID string       `json:"teamId"`
	Block  *model.Block `json:"block"`
	Board  *model.Board `json:"board"`
	Silent bool         `json:"silent"`
}

// shadow is the last select values seen for each card, by card id.
type shadow struct {
	mu   sync.Mutex
	seen map[string]map[string]any
}

func newShadow() *shadow { return &shadow{seen: make(map[string]map[string]any)} }

// remember records a card's select values and answers what they were before.
// known is false the first time a card is seen, which is what stops a listing
// from reading as a board full of moves.
func (s *shadow) remember(cardID string, props map[string]any) (previous map[string]any, known bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, known = s.seen[cardID]
	kept := make(map[string]any, len(props))
	for k, v := range props {
		kept[k] = v
	}
	s.seen[cardID] = kept
	return previous, known
}

func (s *shadow) forget(cardID string) {
	s.mu.Lock()
	delete(s.seen, cardID)
	s.mu.Unlock()
}

// Subscribe opens the board's websocket and returns the normalized event
// stream. It reconnects by itself until ctx is done.
func (c *Client) Subscribe(ctx context.Context) (<-chan acp.CardMoved, error) {
	c.mu.Lock()
	if c.events != nil {
		ch := c.events
		c.mu.Unlock()
		return ch, nil
	}
	c.events = make(chan acp.CardMoved, 256)
	c.shadow = newShadow()
	ch := c.events
	c.mu.Unlock()

	go c.watch(ctx)
	return ch, nil
}

func (c *Client) watch(ctx context.Context) {
	for {
		if err := c.readSocket(ctx); err != nil && ctx.Err() == nil {
			c.log.Warn("boardclient: board socket dropped", mlog.Err(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectAfter):
		}
	}
}

// readSocket runs one connection: dial, authenticate, subscribe, then read
// until it breaks.
func (c *Client) readSocket(ctx context.Context) error {
	endpoint, err := socketURL(c.api.APIURL)
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint, http.Header{})
	if err != nil {
		return fmt.Errorf("dial %s: %w", endpoint, err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.WriteJSON(wsCommand{Action: "AUTH", Token: c.api.Token}); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	if err := conn.WriteJSON(wsCommand{Action: "SUBSCRIBE_TEAM", TeamID: c.teamID, Token: c.api.Token}); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	// A fresh connection knows nothing about what happened while there was
	// none, so the boards this client has been asked about are re-read and
	// diffed. On the first connection this is what fills the shadow, which is
	// why nothing is emitted for a card seen for the first time.
	c.resync(ctx)

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(pingEvery)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				_ = conn.Close()
				return
			case <-ticker.C:
				_ = conn.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		c.handle(msg)
	}
}

// handle turns one message into card moves, or into nothing.
func (c *Client) handle(msg wsMessage) {
	if msg.Action != "UPDATE_BLOCK" || msg.Block == nil {
		return
	}
	block := msg.Block
	if block.Type != model.TypeCard {
		return
	}

	// A deletion arrives as a block with DeleteAt set and nothing else on it.
	// The card is gone for everybody, so what this client owes it is to stop
	// remembering it.
	if block.DeleteAt != 0 {
		c.shadow.forget(block.ID)
		c.forgetCard(block.ID)
		return
	}
	c.rememberCard(block.ID, block.BoardID)

	props := boardmodel.RawProperties(block)
	previous, known := c.shadow.remember(block.ID, props)
	if !known {
		// First sighting: recorded, announced to nobody.
		return
	}
	// The app's own bookkeeping, marked as such by the board. Recorded above so
	// the next real move diffs against the truth, and emitted to nobody: acting
	// on it would be acting on ourselves.
	if msg.Silent {
		return
	}

	_, schema, err := c.board(block.BoardID)
	if err != nil {
		c.log.Warn("boardclient: cannot read the board a card moved on", mlog.Err(err))
		return
	}
	moves := boardmodel.MovedEvents(block.BoardID, block, schema,
		previous, c.cardBody(block.BoardID, block), c.resolver())
	for _, ev := range moves {
		select {
		case c.events <- ev:
		default:
			c.log.Warn("boardclient: event channel full, dropping card move", mlog.String("card", ev.CardID))
		}
	}
}

// resync re-reads every board this client has seen a card of and records where
// its cards stand now. It emits nothing: a move that happened while the socket
// was down is picked up by the next diff, and announcing every card of every
// board on reconnect is how a reconnect would become a stampede.
func (c *Client) resync(ctx context.Context) {
	c.mu.Lock()
	boards := make(map[string]struct{}, len(c.cardBoard))
	for _, boardID := range c.cardBoard {
		boards[boardID] = struct{}{}
	}
	c.mu.Unlock()

	for boardID := range boards {
		cards, err := c.CardsForBoard(ctx, boardID)
		if err != nil {
			c.log.Warn("boardclient: cannot re-read a board after reconnecting", mlog.Err(err))
			continue
		}
		for _, card := range cards {
			props := map[string]any{}
			for _, opt := range card.SelectedOptions {
				props[opt.PropertyID] = opt.OptionID
			}
			c.shadow.remember(card.CardID, props)
		}
	}
}

// socketURL turns the API base into the socket's address: the board serves it
// beside /api/v0, on the same origin, and it is ws:// or wss:// by whichever
// the API is.
func socketURL(apiURL string) (string, error) {
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", apiURL, err)
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	default:
		parsed.Scheme = "ws"
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/api/v0") + "/ws"
	return parsed.String(), nil
}
