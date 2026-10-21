package ws

import (
	"github.com/artipop/pourquoipas/server/model"
)

// UpdateCategoryMessage is sent on block updates.
type UpdateCategoryMessage struct {
	Action          string                              `json:"action"`
	TeamID          string                              `json:"teamId"`
	Category        *model.Category                     `json:"category,omitempty"`
	BoardCategories []*model.BoardCategoryWebsocketData `json:"blockCategories,omitempty"`
}

// UpdateBlockMsg is sent on block updates.
type UpdateBlockMsg struct {
	Action string       `json:"action"`
	TeamID string       `json:"teamId"`
	Block  *model.Block `json:"block"`

	// Silent says this write was the machine's own bookkeeping rather than
	// something a person asked for — the same answer disableNotify already
	// carries to the notification service.
	//
	// The broadcast itself is not suppressed, and must not be: every page
	// looking at the board still has to redraw. What it is for is a client that
	// *acts* on a change, which until now only existed in this process, where
	// disableNotify short-circuits the notify backend and the question never
	// arose. A client reading the board over the socket would otherwise see its
	// own flow_state and branch writes come back indistinguishable from a
	// person's drag, and start the stage it had just finished.
	//
	// Not derivable at the far end: modified_by is no help, since two machines
	// can be logged in as the same person.
	Silent bool `json:"silent,omitempty"`
}

// UpdateBoardMsg is sent on block updates.
type UpdateBoardMsg struct {
	Action string       `json:"action"`
	TeamID string       `json:"teamId"`
	Board  *model.Board `json:"board"`
}

// UpdateMemberMsg is sent on membership updates.
type UpdateMemberMsg struct {
	Action string             `json:"action"`
	TeamID string             `json:"teamId"`
	Member *model.BoardMember `json:"member"`
}

// UpdateSubscription is sent on subscription updates.
type UpdateSubscription struct {
	Action       string              `json:"action"`
	Subscription *model.Subscription `json:"subscription"`
}

// UpdateClientConfig is sent on block updates.
type UpdateClientConfig struct {
	Action       string             `json:"action"`
	ClientConfig model.ClientConfig `json:"clientconfig"`
}

// WebsocketCommand is an incoming command from the client.
type WebsocketCommand struct {
	Action    string   `json:"action"`
	TeamID    string   `json:"teamId"`
	Token     string   `json:"token"`
	ReadToken string   `json:"readToken"`
	BlockIDs  []string `json:"blockIds"`
}

type CategoryReorderMessage struct {
	Action        string   `json:"action"`
	CategoryOrder []string `json:"categoryOrder"`
	TeamID        string   `json:"teamId"`
}

type CategoryBoardReorderMessage struct {
	Action     string   `json:"action"`
	CategoryID string   `json:"CategoryId"`
	BoardOrder []string `json:"BoardOrder"`
	TeamID     string   `json:"teamId"`
}
