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
}

// attachState is what the settings panel draws. Running is the answer the app
// actually started with, so Running disagreeing with URL is the app saying a
// restart is owed — the same shape the tailnet's status has, and for a harder
// reason: the board a card lives on is decided when the seam is built.
type attachState struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Running  string `json:"running"`
	Error    string `json:"error,omitempty"`
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
	if c == nil {
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
	return attachState{URL: c.cfg.URL, Username: c.cfg.Username, Running: c.running}
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
	next := attachSettings{URL: origin, Username: username}
	if err := saveAttachSettings(c.path, next); err != nil {
		return attachState{}, err
	}
	c.cfg = next
	return attachState{URL: next.URL, Username: next.Username, Running: c.running}, nil
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
	return attachState{Running: c.running}, nil
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
