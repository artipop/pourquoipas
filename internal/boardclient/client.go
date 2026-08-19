// Package boardclient is the board over HTTP: the same interfaces
// internal/acp is handed by internal/boardadapter, backed by the board's own
// REST API and its websocket instead of by the app layer in this process.
//
// The two are siblings and neither imports the other. What they must not do is
// disagree about what a card says, which is why everything that reads a block
// lives in internal/boardmodel and both call it.
//
// What it is for: a desktop attached to a hosted board runs the agents on this
// machine and keeps the cards on somebody else's server.
package boardclient

import (
	"fmt"
	"sync"

	"github.com/artipop/xciii/server/client"
	"github.com/artipop/xciii/server/mlog"
	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardmodel"
)

// Client is one attached board server, seen as everything internal/acp asks of
// a board.
type Client struct {
	api    *client.Client
	teamID string
	log    mlog.LoggerIFace

	mu sync.Mutex
	// cardBoard is which board a card is on. A card id is all most of the seam
	// carries, and every route here is addressed by board — so this is the one
	// index without which each write would cost an extra round trip. A card
	// changes board only through MoveCardToBoard, and the socket says so.
	cardBoard map[string]string
	// events is the normalized stream Subscribe hands out, and shadow is what
	// makes a diff possible at all — see events.go.
	events chan acp.CardMoved
	shadow *shadow
	// users is the username behind a user id. A person property stores ids and
	// an agent is matched by the name, so this is asked for on every card read;
	// the answer changes when somebody is renamed, which is not something this
	// app does — it makes a new account instead.
	users map[string]string
}

// New builds a client for a board server already logged in to: token is a live
// session token, which is what the desktop got when it attached.
func New(serverURL, token, teamID string, log mlog.LoggerIFace) *Client {
	return &Client{
		api:       client.NewClient(serverURL, token),
		teamID:    teamID,
		log:       log,
		cardBoard: make(map[string]string),
		users:     make(map[string]string),
	}
}

// API exposes the underlying REST client for the few callers that need the
// board's own vocabulary rather than the seam's.
func (c *Client) API() *client.Client { return c.api }

// TeamID is the team every board here belongs to. One install, one team, and
// the id is '0' at both ends — see docs/teamwork.md.
func (c *Client) TeamID() string { return c.teamID }

// board reads a board and its property schema, which is what turns a block's
// option ids into names.
func (c *Client) board(boardID string) (*model.Board, model.PropSchema, error) {
	board, resp := c.api.GetBoard(boardID, "")
	if err := failed(resp, "read board %s", boardID); err != nil {
		return nil, nil, err
	}
	if board == nil {
		return nil, nil, fmt.Errorf("board %s not found", boardID)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		return nil, nil, fmt.Errorf("parse board schema %s: %w", boardID, err)
	}
	return board, schema, nil
}

// boardOfCard answers the question the seam does not carry: a card id alone is
// not addressable over a route keyed by board.
func (c *Client) boardOfCard(cardID string) (string, error) {
	c.mu.Lock()
	boardID, ok := c.cardBoard[cardID]
	c.mu.Unlock()
	if ok {
		return boardID, nil
	}

	card, resp := c.api.GetCard(cardID)
	if err := failed(resp, "read card %s", cardID); err != nil {
		return "", err
	}
	if card == nil || card.BoardID == "" {
		return "", fmt.Errorf("card %s is on no board", cardID)
	}
	c.rememberCard(cardID, card.BoardID)
	return card.BoardID, nil
}

// rememberCard records which board a card is on. Called from the socket too, so
// a card carried to another board from somewhere else stops being looked for on
// the old one.
func (c *Client) rememberCard(cardID, boardID string) {
	if cardID == "" || boardID == "" {
		return
	}
	c.mu.Lock()
	c.cardBoard[cardID] = boardID
	c.mu.Unlock()
}

func (c *Client) forgetCard(cardID string) {
	c.mu.Lock()
	delete(c.cardBoard, cardID)
	c.mu.Unlock()
}

// resolver reads usernames through the API, memoized across calls: a board full
// of assigned cards would otherwise be one request per person value per read.
func (c *Client) resolver() *boardmodel.Resolver {
	return boardmodel.NewResolver(func(userID string) string {
		c.mu.Lock()
		name, ok := c.users[userID]
		c.mu.Unlock()
		if ok {
			return name
		}
		user, resp := c.api.GetUser(userID)
		if resp != nil && resp.Error != nil {
			return ""
		}
		if user == nil {
			return ""
		}
		c.mu.Lock()
		c.users[userID] = user.Username
		c.mu.Unlock()
		return user.Username
	})
}

// failed turns the REST client's two-value answer into an ordinary error. The
// client reports failure in a Response rather than in the error it returns, so
// every call site here would otherwise carry the same three lines.
func failed(resp *client.Response, what string, args ...any) error {
	if resp == nil {
		return fmt.Errorf(what+": no response", args...)
	}
	if resp.Error != nil {
		return fmt.Errorf(what+": %w", append(args, resp.Error)...)
	}
	return nil
}
