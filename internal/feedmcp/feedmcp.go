// Package feedmcp is the board read from outside: one MCP tool that lists the
// cards standing in a column, for another application to take as its own
// inbox. XXVI reads it as a source (its manifest xciii.json names
// `xciii mcp feed`).
//
// It is not boardmcp. That one is an agent's, authorized by one run's grant and
// written for a model to read — lines of text. This one is a program's: a
// token a person issued in the settings, and JSON with names on it, because
// what reads it maps fields rather than reads prose.
//
// Read only. Another application may learn what is in a column; moving a card
// or writing on one is this board's business.
package feedmcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/artipop/xciii/internal/boardmodel"
)

// ServerName is the name the tools appear under and the subcommand that
// bridges to them: `xciii mcp feed`.
const ServerName = "feed"

// Path is where the feed is served on the front door.
const Path = "/acp/feed/mcp"

const version = "0.1.0"

// Environment the bridge is configured through.
const (
	EnvToken = "XCIII_FEED_TOKEN"
	// EnvURL is the board's address when it is not this machine's running
	// app: a server build, reached at an address that does not change. It is
	// also what makes a card's link worth giving — see BaseHeader.
	EnvURL = "XCIII_URL"
)

// BaseHeader carries the address the caller reached the board at, when that
// address is stable. The feed answers with links only then: a desktop's front
// door is a random loopback port per launch, and a link to it is dead by the
// next start.
const BaseHeader = "X-Xciii-Base"

// Lists is the part of the board the feed reads.
type Lists interface {
	Boards(ctx context.Context) ([]boardmodel.BoardSummary, error)
	BoardCards(ctx context.Context, boardID string) ([]boardmodel.CardSummary, error)
}

// Card is one row of the feed.
type Card struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Board   string `json:"board"`
	Column  string `json:"column,omitempty"`
	Updated string `json:"updated"`
	URL     string `json:"url,omitempty"`
	// Properties are the card's own values by the names a person reads, so a
	// manifest can put what matters of them into the text of what it makes.
	Properties map[string]string `json:"properties,omitempty"`
}

type listInput struct {
	Board  string `json:"board" jsonschema:"the board, by its title or id"`
	Column string `json:"column,omitempty" jsonschema:"only the cards in this column; every card of the board without it"`
}

// NewServer builds the feed over a board. base is the stable address the
// caller reached the board at, or empty when there is none.
func NewServer(lists Lists, base string) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: ServerName, Title: "XCIII board feed", Version: version}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_column_cards",
		Description: "The cards standing in one column of a board, as JSON: {cards: [{id, title, board, column, updated, url, properties}]}.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
		cards, err := ColumnCards(ctx, lists, in.Board, in.Column, base)
		if err != nil {
			return errorResult(err), nil, nil
		}
		out, err := json.Marshal(map[string]any{"cards": cards})
		if err != nil {
			return errorResult(err), nil, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(out)}}}, nil, nil
	})
	return srv
}

// ColumnCards lists a column. A board or a column that is not there is an
// error naming what is: the names are typed into another application's form,
// and "no such column, there are Inbox, Today, Done" is the answer to the typo.
func ColumnCards(ctx context.Context, lists Lists, board, column, base string) ([]Card, error) {
	board, column = strings.TrimSpace(board), strings.TrimSpace(column)
	if board == "" {
		return nil, errors.New("не сказано, какая доска")
	}
	boards, err := lists.Boards(ctx)
	if err != nil {
		return nil, err
	}
	var found *boardmodel.BoardSummary
	titles := make([]string, 0, len(boards))
	for i := range boards {
		titles = append(titles, boards[i].Title)
		if boards[i].ID == board || strings.EqualFold(boards[i].Title, board) {
			found = &boards[i]
		}
	}
	if found == nil {
		return nil, fmt.Errorf("доски «%s» нет (есть: %s)", board, strings.Join(titles, ", "))
	}
	if column != "" && len(found.Columns) > 0 {
		names := make([]string, 0, len(found.Columns))
		known := false
		for _, c := range found.Columns {
			names = append(names, c.Value)
			if strings.EqualFold(c.Value, column) {
				known = true
			}
		}
		if !known {
			return nil, fmt.Errorf("на доске «%s» нет колонки «%s» (есть: %s)", found.Title, column, strings.Join(names, ", "))
		}
	}
	summaries, err := lists.BoardCards(ctx, found.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Card, 0, len(summaries))
	for _, s := range summaries {
		if column != "" && !strings.EqualFold(s.Column, column) {
			continue
		}
		card := Card{
			ID: s.ID, Title: s.Title, Board: found.Title, Column: s.Column,
			// The card's last change is its version: a card renamed or moved
			// on the board is a card the other side brings up to date.
			Updated:    strconv.FormatInt(s.UpdateAt, 10),
			Properties: s.Properties,
		}
		if base != "" {
			card.URL = base + "/team/0/" + found.ID + "/0/" + s.ID
		}
		out = append(out, card)
	}
	return out, nil
}

// NewHandler serves the feed to whoever holds a token check accepts.
func NewHandler(lists Lists, check func(token string) bool) http.Handler {
	inner := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return NewServer(lists, stableBase(r.Header.Get(BaseHeader)))
	}, &mcp.StreamableHTTPOptions{Stateless: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !check(bearer(r)) {
			http.Error(w, "нет доступа к фиду доски: токен не выдан или отозван", http.StatusForbidden)
			return
		}
		inner.ServeHTTP(w, r)
	})
}

// stableBase is the base a link is built on, or empty. Only an absolute http
// address is taken: the value comes from the caller and ends up in a link.
func stableBase(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return strings.TrimSuffix(u.Scheme+"://"+u.Host+u.Path, "/")
}

func bearer(r *http.Request) string {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
}

// How the bridge finds a desktop app: its front door is a random loopback port
// per launch, so the running app leaves the address beside its data, and
// `xciii mcp feed` reads it there. The token is not in the file — it is the
// caller's, issued in the settings and kept by whoever reads the feed.

// HandoffFile is where a running app says where its feed is.
const HandoffFile = "feed.json"

// Handoff is that file.
type Handoff struct {
	URL string `json:"url"`
	PID int    `json:"pid"`
}

// WriteHandoff leaves the address for the bridge.
func WriteHandoff(dataDir, base string) error {
	body, err := json.MarshalIndent(Handoff{URL: base + Path, PID: os.Getpid()}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, HandoffFile), append(body, '\n'), 0o600)
}

// RemoveHandoff takes the address away when the app stops, so a bridge started
// afterwards says the app is not running instead of failing to connect.
func RemoveHandoff(dataDir string) {
	_ = os.Remove(filepath.Join(dataDir, HandoffFile))
}

// Endpoint is where the bridge connects, and the stable base to announce with
// it: XCIII_URL when it is given, else the address the running app left.
func Endpoint(dataDir string) (endpoint, base string, err error) {
	if raw := strings.TrimSpace(os.Getenv(EnvURL)); raw != "" {
		base = stableBase(raw)
		if base == "" {
			return "", "", fmt.Errorf("%s не похож на адрес: %q", EnvURL, raw)
		}
		return base + Path, base, nil
	}
	body, err := os.ReadFile(filepath.Join(dataDir, HandoffFile))
	if os.IsNotExist(err) {
		return "", "", errors.New("XCIII не запущен: фид доски отдаёт само приложение, откройте его")
	}
	if err != nil {
		return "", "", err
	}
	var h Handoff
	if err := json.Unmarshal(body, &h); err != nil || h.URL == "" {
		return "", "", fmt.Errorf("не удалось прочитать %s — перезапустите XCIII", HandoffFile)
	}
	return h.URL, "", nil
}

// ServeStdio is `xciii mcp feed`: one stdio session piped through to the feed.
// The bridge carries no tool of its own — it asks the feed what it has and
// forwards the calls — so the tool is described in one place.
func ServeStdio(ctx context.Context, dataDir string, in io.ReadCloser, out io.WriteCloser) error {
	token := strings.TrimSpace(os.Getenv(EnvToken))
	if token == "" {
		return fmt.Errorf("не задан токен фида (%s): его выдают в настройках XCIII", EnvToken)
	}
	endpoint, base, err := Endpoint(dataDir)
	if err != nil {
		return err
	}
	httpClient := &http.Client{Transport: headers{token: token, base: base}}
	if err := checkAccess(ctx, httpClient, endpoint); err != nil {
		return err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: ServerName + "-bridge", Version: version}, nil)
	upstream, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: httpClient,
	}, nil)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к XCIII: %w", err)
	}
	defer upstream.Close()

	tools, err := upstream.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("XCIII не отдал инструменты фида: %w", err)
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: ServerName, Title: "XCIII board feed", Version: version}, nil)
	for _, tool := range tools.Tools {
		name := tool.Name
		srv.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return upstream.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: req.Params.Arguments})
		})
	}
	return srv.Run(ctx, &mcp.IOTransport{Reader: in, Writer: out})
}

// checkAccess asks once, before the MCP session, whether the feed answers and
// takes the token. The session would find out too, but only as "Forbidden" or
// a dial error three wrappers deep, and the other application shows whatever
// this process says last — so it had better be what to do about it.
func checkAccess(ctx context.Context, client *http.Client, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("XCIII не отвечает по адресу %s: %w", endpoint, err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return errors.New("токен фида не подходит: выдайте новый в настройках XCIII (Прочее → Доступ для других приложений)")
	}
	return nil
}

type headers struct{ token, base string }

func (h headers) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+h.token)
	if h.base != "" {
		r.Header.Set(BaseHeader, h.base)
	}
	return http.DefaultTransport.RoundTrip(r)
}
