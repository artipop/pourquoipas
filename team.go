package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/services/auth"
	"github.com/artipop/xciii/server/utils"
)

// Team mode: accounts and a login, instead of one person nobody asks about.
// A mode of the install rather than a build, because the board server is
// constructed with the answer — docs/teamwork.md, docs/decisions.md.

// teamSettings is <dataDir>/team/settings.json. No file means no team.
type teamSettings struct {
	Enabled bool `json:"enabled"`
}

func loadTeamSettings(path string) (teamSettings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return teamSettings{}, nil
	}
	if err != nil {
		return teamSettings{}, err
	}
	var cfg teamSettings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return teamSettings{}, fmt.Errorf("team: %s: %w", path, err)
	}
	return cfg, nil
}

func saveTeamSettings(path string, cfg teamSettings) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// teamState is what the settings panel shows. Enabled and Running disagree
// between throwing the switch and restarting, which is what it says out loud.
type teamState struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	// Owner is the username of the account that holds this install's own
	// identity, empty until somebody names themselves.
	Owner string `json:"owner"`
	// Invite is the signup token the second person registers with. Only ever
	// filled while the mode is running: before that there is nobody to invite.
	Invite string `json:"invite"`
}

// teamBoard is the half of the board server this needs, as an interface so the
// decisions below are testable without a database.
type teamBoard interface {
	GetUser(id string) (*model.User, error)
	GetUserByUsername(username string) (*model.User, error)
	RegisterUserWithID(id, username, email, password string) error
	GetRootTeam() (*model.Team, error)
	UpsertTeamSignupToken(team model.Team) error
	GetSession(token string) (*model.Session, error)
}

// teamController owns the settings file and the account behind it. Safe on a
// nil receiver, like the tailnet controller: no data dir costs the feature,
// not the app.
type teamController struct {
	mu sync.Mutex

	path     string
	settings teamSettings
	// running is what the board server was actually started in, which is what
	// decides whether a restart is owed.
	running bool
	app     teamBoard
}

func newTeamController(path string) (*teamController, error) {
	cfg, err := loadTeamSettings(path)
	if err != nil {
		return nil, err
	}
	return &teamController{path: path, settings: cfg, running: cfg.Enabled}, nil
}

// enabled is read at startup to decide whether the board server gets a
// single-user token.
func (c *teamController) enabled() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.settings.Enabled
}

// setApp hands over the board once it is running: the account and the invite
// token are the board's own tables, and the controller is built before it.
func (c *teamController) setApp(a teamBoard) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.app = a
}

func (c *teamController) state() teamState {
	if c == nil {
		return teamState{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateLocked()
}

func (c *teamController) stateLocked() teamState {
	out := teamState{Enabled: c.settings.Enabled, Running: c.running}
	if c.app == nil {
		return out
	}
	if user, err := c.owner(); err == nil && user != nil {
		out.Owner = user.Username
	}
	// An invite is only meaningful while the mode is running: /register refuses
	// every caller while the server holds a single-user token, so handing out a
	// link before the restart would be handing out a dead one.
	if c.running {
		if team, err := c.app.GetRootTeam(); err == nil && team != nil {
			out.Invite = team.SignupToken
		}
	}
	return out
}

// owner is the account holding this install's identity, nil when nobody has
// named themselves. The store reports that absence as an error.
func (c *teamController) owner() (*model.User, error) {
	user, err := c.app.GetUser(model.SingleUser)
	if model.IsErrNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// enable names the person at this machine and turns the mode on, for the next
// launch.
func (c *teamController) enable(username, password string) (teamState, error) {
	if c == nil {
		return teamState{}, errors.New("командный режим недоступен")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.app == nil {
		return teamState{}, errors.New("доска ещё не готова")
	}

	username = strings.TrimSpace(username)
	owner, err := c.owner()
	if err != nil {
		return teamState{}, err
	}
	if owner == nil {
		// The one moment a password is asked for. An account that already
		// exists is not asked again — this is the switch coming back on, and
		// the way to change a password is to change a password.
		if username == "" {
			return teamState{}, errors.New("нужно имя пользователя")
		}
		if err := auth.IsPasswordValid(password, auth.PasswordSettings{MinimumLength: 6}); err != nil {
			return teamState{}, fmt.Errorf("пароль: %w", err)
		}
		if taken, err := c.app.GetUserByUsername(username); err == nil && taken != nil {
			return teamState{}, fmt.Errorf("имя %q уже занято", username)
		}
		// No email: this account is created here rather than through
		// /register, and an address nobody asked for is an address nobody can
		// be reached at. The people invited afterwards give their own.
		if err := c.app.RegisterUserWithID(model.SingleUser, username, "", password); err != nil {
			return teamState{}, err
		}
	}

	next := c.settings
	next.Enabled = true
	if err := saveTeamSettings(c.path, next); err != nil {
		return teamState{}, err
	}
	c.settings = next
	return c.stateLocked(), nil
}

// disable puts the install back to one person. Nothing moves: the synthesized
// session carries the same id the account does.
func (c *teamController) disable() (teamState, error) {
	if c == nil {
		return teamState{}, errors.New("командный режим недоступен")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	next := c.settings
	next.Enabled = false
	if err := saveTeamSettings(c.path, next); err != nil {
		return teamState{}, err
	}
	c.settings = next
	return c.stateLocked(), nil
}

// regenerateInvite retires the old link by minting a new token.
func (c *teamController) regenerateInvite() (teamState, error) {
	if c == nil {
		return teamState{}, errors.New("командный режим недоступен")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.app == nil {
		return teamState{}, errors.New("доска ещё не готова")
	}
	team, err := c.app.GetRootTeam()
	if err != nil {
		return teamState{}, err
	}
	team.SignupToken = utils.NewID(utils.IDTypeToken)
	if err := c.app.UpsertTeamSignupToken(*team); err != nil {
		return teamState{}, err
	}
	return c.stateLocked(), nil
}

// sessionValid answers the front door. Whose session it is stays the board's
// business — see docs/deferred.md, «Терминал видит любой из команды».
func (c *teamController) sessionValid(token string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	a := c.app
	c.mu.Unlock()
	if a == nil || token == "" {
		return false
	}
	session, err := a.GetSession(token)
	return err == nil && session != nil
}

// sessionCookie is how the token reaches the front door: the Wails runtime
// makes its own fetches and a WebSocket takes no header a page can set. Written
// by the page (session.ts), so not HttpOnly.
const sessionCookie = "boardSession"

// requireSession refuses a request with no live board session. Installed only
// in team mode, where sessions exist.
func requireSession(next http.Handler, valid func(string) bool) http.Handler {
	if valid == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if openToEverybody(r) {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || !valid(cookie.Value) {
			http.Error(w, "not logged in", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// openToEverybody: the runtime module carries no authority — the calls made
// through it are the guarded POST — and a login page has to be able to load it
// to stop being a login page. See docs/decisions.md.
func openToEverybody(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return r.URL.Path == "/wails/runtime.js"
}
