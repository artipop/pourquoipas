package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/artipop/xciii/internal/acp"
)

// The claim a desktop makes on a route event of a board this process serves.
// Every attached desktop hears the same card move and each keeps its own
// database, so the claim has to be made where all the candidates can see it.
//
// On the front door rather than in the board's REST API because the table
// behind it is this application's, and the board server cannot see
// internal/acp without the import going the wrong way — the same reason the
// board's MCP tools are served here.

// LeasePath is where a desktop posts its claim.
const LeasePath = "/acp/lease"

type leaseRequest struct {
	// Token is the event's key. Two machines hearing one move compute the same
	// one, which is what makes the claim mean anything.
	Token string `json:"token"`
	// WindowSeconds is how long the claim stands: a machine that dies mid-stage
	// costs that stage rather than the card for ever.
	WindowSeconds int `json:"windowSeconds"`
}

type leaseResponse struct {
	Granted bool `json:"granted"`
}

// leaseRoutes serves the claim. valid says whether the caller carries a live
// board session — taking work on a board is something a member does.
type leaseRoutes struct {
	mu    sync.RWMutex
	store *acp.Store
	valid func(string) bool
}

func newLeaseRoutes(valid func(string) bool) *leaseRoutes {
	return &leaseRoutes{valid: valid}
}

// SetStore hands over the store once the board's database is open.
func (l *leaseRoutes) SetStore(s *acp.Store) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.store = s
}

func (l *leaseRoutes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// On a desktop nothing authenticates anybody and there is one candidate
	// anyway; on a hosted server this is what keeps a stranger from deciding
	// who works the board.
	l.mu.RLock()
	store, valid := l.store, l.valid
	l.mu.RUnlock()
	if valid != nil && !valid(bearer(r)) {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}
	if store == nil {
		http.Error(w, "agents are off on this board", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<10))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var want leaseRequest
	if err := json.Unmarshal(body, &want); err != nil || want.Token == "" {
		http.Error(w, "a claim needs a token", http.StatusBadRequest)
		return
	}

	granted, err := store.ClaimIdempotency(want.Token, "", time.Duration(want.WindowSeconds)*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(leaseResponse{Granted: granted})
}
