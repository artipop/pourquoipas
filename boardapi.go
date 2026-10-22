package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/boardmcp"
)

// The board tools an agent calls, served by the app itself on the front door
// (internal/boardmcp says why there is no subprocess in between).
//
// They are here rather than on the board's own API because what they offer is
// ours, not Focalboard's: a column that means something, a project by name, a
// card placed where the automation will pick it up. Through the board API an
// agent would have had to learn property ids to do the same thing.
//
// Nothing here trusts the caller for anything but its token. The token is one
// agent run's grant (acp/boardtools.go), and the board it names is the only
// board these calls can touch.

// boardToolRoutes serves the tools. Like the event socket, it is built before
// the manager exists and is given it afterwards.
type boardToolRoutes struct {
	mu  sync.RWMutex
	mgr *acp.Manager
}

func newBoardToolRoutes() *boardToolRoutes { return &boardToolRoutes{} }

// SetManager hands over the manager once the ACP integration is up.
func (b *boardToolRoutes) SetManager(m *acp.Manager) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mgr = m
}

func (b *boardToolRoutes) manager() *acp.Manager {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.mgr
}

// open resolves a request to the board its caller holds a grant for. It is the
// whole of the authentication: no grant, no board, no tools.
func (b *boardToolRoutes) open(r *http.Request) (boardmcp.Board, error) {
	mgr := b.manager()
	if mgr == nil {
		return nil, fmt.Errorf("agents are off")
	}
	token := bearer(r)
	if err := mgr.CheckBoardTools(token); err != nil {
		return nil, err
	}
	return mgr.BoardForGrant(token), nil
}

// Handler is the /acp/board/ subtree.
func (b *boardToolRoutes) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(boardmcp.Path, boardmcp.NewHandler(b.open))
	return mux
}

// bearer is the grant the call carries, and the whole of its identity.
func bearer(r *http.Request) string {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}
