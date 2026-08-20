package boardclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/artipop/xciii/internal/acp"
)

var _ acp.BoardLease = (*Client)(nil)

// LeasePath is where the board's own side answers a claim. It is a route of
// this application on the board's front door rather than one of the board's
// REST API, because the table behind it is ours.
const LeasePath = "/acp/lease"

// ClaimStage asks the board whether this install may take a route event.
//
// The one call in this package that does not go to /api/v2: a claim is about
// which desktop works the card, which is this application's business and not
// the board's. Everything else about it is the same — the same origin, the same
// session.
func (c *Client) ClaimStage(ctx context.Context, token string, window time.Duration) (bool, error) {
	body, err := json.Marshal(map[string]any{
		"token":         token,
		"windowSeconds": int(window.Seconds()),
	})
	if err != nil {
		return false, err
	}
	url := strings.TrimSuffix(c.api.APIURL, "/api/v2") + LeasePath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.api.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("claim %s: %w", token, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("claim %s: the board answered %d", token, resp.StatusCode)
	}
	var answer struct {
		Granted bool `json:"granted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return false, fmt.Errorf("claim %s: %w", token, err)
	}
	return answer.Granted, nil
}
