package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/artipop/xciii/internal/boardmodel"
	"github.com/artipop/xciii/internal/feedmcp"
	"github.com/artipop/xciii/internal/sources"
)

// Access for other applications: the feed of a column (internal/feedmcp), read
// by whoever holds a token a person issued here. One token for the install
// rather than one per reader, because the question a person answers is "may
// something else read my board", and revoking it answers it for everything.
//
// Only the hash is kept, the way a source's ingest token is: the token is shown
// once, when it is issued, and goes into the other application's keychain.

type feedAccess struct {
	TokenHash string `json:"tokenHash,omitempty"`
}

var feedAccessMu sync.Mutex

func feedAccessPath() (string, error) {
	dir, err := appDataDir("", 0o755)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "feed-access.json"), nil
}

func readFeedAccess() feedAccess {
	var f feedAccess
	path, err := feedAccessPath()
	if err != nil {
		return f
	}
	if body, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(body, &f)
	}
	return f
}

func writeFeedAccess(f feedAccess) error {
	path, err := feedAccessPath()
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o600)
}

// checkFeedToken reports whether a token opens the feed. The file is read on
// every request rather than cached: revoking has to take effect on the next
// poll, and a small file read is cheaper than a cache to keep honest.
func checkFeedToken(token string) bool {
	if token == "" {
		return false
	}
	if secret := localFeedSecret(); secret != "" && subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1 {
		return true
	}
	hash := readFeedAccess().TokenHash
	return hash != "" && subtle.ConstantTimeCompare([]byte(sources.HashToken(token)), []byte(hash)) == 1
}

// The secret of this run, for the reader that is on this machine. It is never
// stored: it exists in memory and in the handoff file, and a restart makes
// another, so it opens nothing that outlives the app.
var (
	localSecretMu sync.RWMutex
	localSecret   string
)

func localFeedSecret() string {
	localSecretMu.RLock()
	defer localSecretMu.RUnlock()
	return localSecret
}

// newLocalFeedSecret makes the secret of this run, or returns the one made.
func newLocalFeedSecret() (string, error) {
	localSecretMu.Lock()
	defer localSecretMu.Unlock()
	if localSecret != "" {
		return localSecret, nil
	}
	secret, err := sources.GenerateToken()
	if err != nil {
		return "", err
	}
	localSecret = secret
	return secret, nil
}

func dropLocalFeedSecret() {
	localSecretMu.Lock()
	defer localSecretMu.Unlock()
	localSecret = ""
}

// IssueFeedToken makes a new token for other applications and returns it —
// the one time it is ever shown. The previous one stops working.
func (a *App) IssueFeedToken() (string, error) {
	token, err := sources.GenerateToken()
	if err != nil {
		return "", err
	}
	feedAccessMu.Lock()
	defer feedAccessMu.Unlock()
	if err := writeFeedAccess(feedAccess{TokenHash: sources.HashToken(token)}); err != nil {
		return "", err
	}
	return token, nil
}

// RevokeFeedToken closes the feed to everything that reads it.
func (a *App) RevokeFeedToken() error {
	feedAccessMu.Lock()
	defer feedAccessMu.Unlock()
	return writeFeedAccess(feedAccess{})
}

// FeedTokenIssued says whether a token is out, never what it is.
func (a *App) FeedTokenIssued() bool {
	return readFeedAccess().TokenHash != ""
}

// feedRoutes serves the feed. Like the board tools, it is built before the
// board it reads exists and is given it afterwards.
type feedRoutes struct {
	mu    sync.RWMutex
	lists feedmcp.Lists
}

func (f *feedRoutes) SetLists(lists feedmcp.Lists) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lists = lists
}

func (f *feedRoutes) Handler() http.Handler {
	return feedmcp.NewHandler(listsFunc(func() feedmcp.Lists {
		f.mu.RLock()
		defer f.mu.RUnlock()
		return f.lists
	}), checkFeedToken)
}

// listsFunc reads whatever board is set by the time a request arrives.
type listsFunc func() feedmcp.Lists

func (l listsFunc) Boards(ctx context.Context) ([]boardmodel.BoardSummary, error) {
	lists := l()
	if lists == nil {
		return nil, errors.New("доска ещё не открыта")
	}
	return lists.Boards(ctx)
}

func (l listsFunc) BoardCards(ctx context.Context, boardID string) ([]boardmodel.CardSummary, error) {
	lists := l()
	if lists == nil {
		return nil, errors.New("доска ещё не открыта")
	}
	return lists.BoardCards(ctx, boardID)
}
