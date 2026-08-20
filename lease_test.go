package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/appschema"
)

func leaseAt(t *testing.T, valid func(string) bool) *leaseRoutes {
	t.Helper()
	db, err := appschema.OpenEnforcing(filepath.Join(t.TempDir(), "xciii.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	routes := newLeaseRoutes(valid)
	routes.SetStore(acp.NewStore(db))
	return routes
}

func claim(t *testing.T, routes *leaseRoutes, token, bearer string) (int, bool) {
	t.Helper()
	body := `{"token":"` + token + `","windowSeconds":60}`
	req := httptest.NewRequest(http.MethodPost, LeasePath, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return rec.Code, false
	}
	var answer leaseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	return rec.Code, answer.Granted
}

// Every desktop attached to a hosted board hears the same card move, and each
// keeps its own database — so the latch that made one drag one stage was a
// latch per machine, and three desktops would have run three stages on one
// card. Claimed on the board's own side, exactly one of them gets it.
func TestOnlyOneDesktopGetsAStage(t *testing.T) {
	routes := leaseAt(t, nil)

	if _, granted := claim(t, routes, "flow|card-1|opt-work|started", ""); !granted {
		t.Fatal("the first machine to ask was refused")
	}
	if _, granted := claim(t, routes, "flow|card-1|opt-work|started", ""); granted {
		t.Fatal("a second machine got the same stage")
	}
	// A different event of the same card is a different claim: two things
	// happening to one card are two stages.
	if _, granted := claim(t, routes, "flow|card-1|opt-review|started", ""); !granted {
		t.Error("another event of the same card was refused")
	}
}

// Taking work on a board is something a member does, and a member is somebody
// who has logged in. The route is reachable from another machine, so this is
// the whole of what stands between it and a stranger.
func TestAClaimNeedsALiveSession(t *testing.T) {
	routes := leaseAt(t, func(token string) bool { return token == "live" })

	if code, _ := claim(t, routes, "flow|card-1|opt-work|started", ""); code != http.StatusUnauthorized {
		t.Errorf("a claim with no session answered %d", code)
	}
	if code, _ := claim(t, routes, "flow|card-1|opt-work|started", "made-up"); code != http.StatusUnauthorized {
		t.Errorf("a claim with a token nobody issued answered %d", code)
	}
	if code, granted := claim(t, routes, "flow|card-1|opt-work|started", "live"); code != http.StatusOK || !granted {
		t.Errorf("a member's claim answered %d, granted=%v", code, granted)
	}
}

// A board whose agents are off has nothing to claim, and says so rather than
// answering that the claim succeeded.
func TestABoardWithNoAgentsGrantsNothing(t *testing.T) {
	routes := newLeaseRoutes(nil)
	if code, granted := claim(t, routes, "flow|card-1|opt-work|started", ""); code != http.StatusServiceUnavailable || granted {
		t.Errorf("answered %d, granted=%v", code, granted)
	}
}
