// Package boardclient is the board over HTTP: the same interfaces
// internal/boardadapter hands internal/acp, backed by the board's REST API and
// its websocket instead of by the app layer in this process.
//
// The two are siblings and neither imports the other. What they must not do is
// disagree about what a card says, which is why everything that reads a block
// lives in internal/boardmodel and both call it.
package boardclient

import (
	"fmt"
	"sync"

	"github.com/artipop/pourquoipas/server/client"
	"github.com/artipop/pourquoipas/server/mlog"
	"github.com/artipop/pourquoipas/server/model"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/boardmodel"
)

// Client is one attached board server, seen as everything internal/acp asks of
// a board.
type Client struct {
	api    *client.Client
	teamID string
	log    mlog.LoggerIFace

	mu sync.Mutex
	// cardBoard is which board a card is on: most of the seam carries only a
	// card id and every route here is addressed by board. A card changes board
	// only through MoveCardToBoard, and the socket says so.
	cardBoard map[string]string
	// events is the normalized stream Subscribe hands out, and shadow is what
	// makes a diff possible at all — see events.go.
	events chan acp.CardMoved
	shadow *shadow
	// users is the username behind a user id, asked for on every card read.
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

// rememberCard records which board a card is on. Called from the socket too.
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

// resolver reads usernames through the API, memoized: a board full of assigned
// cards would otherwise be one request per person value per read.
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

// failed turns the REST client's two-value answer into an ordinary error: it
// reports failure in the Response rather than in the error it returns.
func failed(resp *client.Response, what string, args ...any) error {
	if resp == nil {
		return fmt.Errorf(what+": no response", args...)
	}
	if resp.Error != nil {
		return fmt.Errorf(what+": %w", append(args, resp.Error)...)
	}
	return nil
}

// cardBlock reads one card block, refusing anything that is not a card: every
// write of ours names a card, and a block id that is something else is a
// mistake worth saying out loud.
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

// patchCardFields writes our own keys on a card, silently.
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
