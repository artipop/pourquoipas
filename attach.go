package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/artipop/xciii/internal/secrets"
	"github.com/artipop/xciii/server/client"
	"github.com/artipop/xciii/server/model"
)

// Attaching this desktop to a hosted board.
//
// The board moves and the agents do not: terminals, folders, worktrees and the
// CLIs stay on this machine, and the cards live on somebody else's server. What
// makes that cheap is that the front door already owns the origin — the page
// has always been served through a reverse proxy to a board, and which board
// that is was simply never a question anybody asked.
//
// So this holds the answer: an address in a settings file beside the tailnet's,
// and the session token in the credential store, because a token is a
// credential and a settings file is not the place for one.

// attachSettings is <dataDir>/server/settings.json. No file means this desktop
// works its own board, which is what every install starts as.
type attachSettings struct {
	// URL is the server's origin as a person typed it — scheme and host, no
	// path. Empty means not attached.
	URL string `json:"url,omitempty"`
	// Username is shown so a person can tell whose session this is without
	// being asked to log in again.
	Username string `json:"username,omitempty"`

	// Selected is which board the page shows: empty is this machine's own, and
	// the server's origin is the server's. A separate question from URL —
	// attached and looked at are not the same thing, which is the whole reason
	// the sidebar has a switcher rather than a setting.
	Selected string `json:"selected,omitempty"`
}

// attachState is what the settings panel draws. Running is the answer the app
// actually started with, so Running disagreeing with URL is the app saying a
// restart is owed — the same shape the tailnet's status has, and for a harder
// reason: the board a card lives on is decided when the seam is built.
type attachState struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Running  string `json:"running"`
	// Selected is the board the page is showing, and Sources is what it could
	// be showing — this machine first, then the server.
	Selected string        `json:"selected"`
	Sources  []boardChoice `json:"sources"`
	Error    string        `json:"error,omitempty"`
}

// boardChoice is one entry of the sidebar's switcher.
type boardChoice struct {
	// ID is the server's origin, or empty for this machine's own board.
	ID    string `json:"id"`
	Title string `json:"title"`
}

const attachTokenKey = "server.session"

func loadAttachSettings(path string) (attachSettings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return attachSettings{}, nil
	}
	if err != nil {
		return attachSettings{}, err
	}
	var cfg attachSettings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return attachSettings{}, fmt.Errorf("attach: %s: %w", path, err)
	}
	return cfg, nil
}

func saveAttachSettings(path string, cfg attachSettings) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// attachableBuild says this build can work somebody else's board.
//
// A desktop can; a hosted server cannot, and refusing is better than the
// half-working thing it would otherwise be. Its own front door guards
// /wails/ and both sockets against *its* session table, while the page it
// serves carries the attached board's sessions — so every bound call, every
// terminal and every event socket would be refused, and the screen would say
// nothing about why.
//
// It is also not a configuration anybody wants: a server that is a client of
// another server is two installs where one was asked for.

// attachController owns the settings file and the token behind it. Safe on a
// nil receiver, like the tailnet controller: no data dir costs the feature, not
// the app.
type attachController struct {
	mu sync.Mutex

	path  string
	vault secrets.Store
	cfg   attachSettings
	// running is the address the app was started against, empty for this
	// machine's own board.
	running string
}

func newAttachController(path string, vault secrets.Store) (*attachController, error) {
	cfg, err := loadAttachSettings(path)
	if err != nil {
		return nil, err
	}
	return &attachController{path: path, vault: vault, cfg: cfg, running: cfg.URL}, nil
}

// target is the board this launch talks to: the attached server, or nothing,
// meaning the board in this process.
func (c *attachController) target() string {
	if c == nil || !attachableBuild {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.URL
}

// token is the live session for the attached server. Empty when this desktop
// works its own board — where the token is the single-user one instead.
func (c *attachController) token() string {
	if c == nil || c.vault == nil {
		return ""
	}
	c.mu.Lock()
	attached := c.cfg.URL != ""
	c.mu.Unlock()
	if !attached {
		return ""
	}
	token, err := c.vault.Get(attachTokenKey)
	if err != nil {
		return ""
	}
	return token
}

func (c *attachController) state() attachState {
	if c == nil {
		return attachState{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := attachState{
		URL:      c.cfg.URL,
		Username: c.cfg.Username,
		Running:  c.running,
		Selected: c.cfg.Selected,
		Sources:  []boardChoice{{ID: "", Title: thisMachine}},
	}
	if c.cfg.URL != "" {
		out.Sources = append(out.Sources, boardChoice{ID: c.cfg.URL, Title: c.cfg.URL})
	}
	return out
}

// thisMachine is what the switcher calls the board this process serves. Not the
// machine's name: there is one of it, and what a person is choosing between is
// «here» and «the server».
const thisMachine = "Эта машина"

// selected is the board the page shows. A server that has been detached, or one
// selected before the app was restarted without it, falls back to this
// machine's own rather than to a blank screen.
func (c *attachController) selected() string {
	if c == nil || !attachableBuild {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Selected != "" && c.cfg.Selected == c.cfg.URL {
		return c.cfg.Selected
	}
	return ""
}

// selectBoard remembers which board to show. It changes nothing about which
// boards the agents work: both are theirs, whichever one is on screen.
func (c *attachController) selectBoard(id string) (attachState, error) {
	if c == nil {
		return attachState{}, errors.New("подключение недоступно")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if id != "" && id != c.cfg.URL {
		return attachState{}, fmt.Errorf("нет подключения к %s", id)
	}
	next := c.cfg
	next.Selected = id
	if err := saveAttachSettings(c.path, next); err != nil {
		return attachState{}, err
	}
	c.cfg = next
	out := attachState{URL: c.cfg.URL, Username: c.cfg.Username, Running: c.running, Selected: id,
		Sources: []boardChoice{{ID: "", Title: thisMachine}}}
	if c.cfg.URL != "" {
		out.Sources = append(out.Sources, boardChoice{ID: c.cfg.URL, Title: c.cfg.URL})
	}
	return out, nil
}

// attach logs in to a server and remembers it for the next launch.
//
// The login happens here rather than on the page, and that is not a detail of
// where the form is: the Go side needs the token for its own board client, and
// the page is served by a proxy that cannot point anywhere until there is an
// address. One login, both halves.
func (c *attachController) attach(rawURL, username, password string) (attachState, error) {
	if c == nil {
		return attachState{}, errors.New("подключение недоступно")
	}
	origin, err := serverOrigin(rawURL)
	if err != nil {
		return attachState{}, err
	}
	if strings.TrimSpace(username) == "" {
		return attachState{}, errors.New("нужно имя пользователя")
	}

	api := client.NewClient(origin, "")
	login, resp := api.Login(&model.LoginRequest{Type: "normal", Username: username, Password: password})
	if resp != nil && resp.Error != nil {
		return attachState{}, fmt.Errorf("вход на %s: %w", origin, resp.Error)
	}
	if login == nil || login.Token == "" {
		return attachState{}, fmt.Errorf("сервер %s не выдал сессию", origin)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.vault == nil {
		return attachState{}, errors.New("негде хранить сессию")
	}
	if err := c.vault.Set(attachTokenKey, login.Token); err != nil {
		return attachState{}, fmt.Errorf("сохранение сессии: %w", err)
	}
	// Attaching selects it: showing this machine's own board after somebody
	// just typed a server's address would be answering a different question.
	next := attachSettings{URL: origin, Username: username, Selected: origin}
	if err := saveAttachSettings(c.path, next); err != nil {
		return attachState{}, err
	}
	c.cfg = next
	return attachState{URL: next.URL, Username: next.Username, Running: c.running, Selected: next.Selected,
		Sources: []boardChoice{{ID: "", Title: thisMachine}, {ID: origin, Title: origin}}}, nil
}

// detach puts this desktop back on its own board. The server keeps everything
// that was made there; what goes is this machine's way in.
func (c *attachController) detach() (attachState, error) {
	if c == nil {
		return attachState{}, errors.New("подключение недоступно")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := saveAttachSettings(c.path, attachSettings{}); err != nil {
		return attachState{}, err
	}
	c.cfg = attachSettings{}
	if c.vault != nil {
		// A token that opens somebody's board is not worth keeping for a
		// server this install no longer talks to.
		_ = c.vault.Delete(attachTokenKey)
	}
	return attachState{Running: c.running, Sources: []boardChoice{{ID: "", Title: thisMachine}}}, nil
}

// serverOrigin normalizes what a person typed into the origin every request
// will be sent to. A path is dropped rather than kept: the board is served at
// the root of its own host, and a trailing "/board" would make every route a
// 404 that reads as the server being down.
func serverOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("нужен адрес сервера")
	}
	if !strings.Contains(raw, "://") {
		// Typing a bare host is the ordinary case, and https is the answer that
		// is safe to be wrong about: a server that only speaks http says so by
		// refusing, where defaulting to http would quietly send a password in
		// the clear.
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("адрес %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("адрес %q: нужен http или https", raw)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("адрес %q: не указан хост", raw)
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}
