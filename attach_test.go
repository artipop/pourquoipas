package main

import (
	"path/filepath"
	"testing"

	"github.com/artipop/pourquoipas/internal/secrets"
)

// fakeVault is the credential store as the attachment sees it: one key, set,
// read and deleted.
type fakeVault map[string]string

func (v fakeVault) Get(key string) (string, error) {
	value, ok := v[key]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return value, nil
}
func (v fakeVault) Set(key, value string) error { v[key] = value; return nil }
func (v fakeVault) Delete(key string) error     { delete(v, key); return nil }

func attachAt(t *testing.T) (*attachController, fakeVault) {
	t.Helper()
	vault := fakeVault{}
	c, err := newAttachController(filepath.Join(t.TempDir(), "settings.json"), vault)
	if err != nil {
		t.Fatal(err)
	}
	return c, vault
}

// Every install starts working its own board, and a desktop that has never
// been attached must read exactly like one that has been detached.
func TestADesktopWithNoSettingsWorksItsOwnBoard(t *testing.T) {
	c, _ := attachAt(t)

	if target := c.target(); target != "" {
		t.Fatalf("target = %q, want this machine's own board", target)
	}
	if token := c.token(); token != "" {
		t.Fatalf("token = %q, want none: the single-user token is the answer here", token)
	}
}

// What a person types is not what a request can be addressed to, and the two
// wrong answers are worth naming: a bare host, and a path that would make every
// board route a 404 reading as the server being down.
func TestTheAddressATypedServerBecomes(t *testing.T) {
	for _, tc := range []struct {
		typed string
		want  string
	}{
		{"https://board.example.com", "https://board.example.com"},
		{"http://192.168.1.10:8080", "http://192.168.1.10:8080"},
		{"board.example.com", "https://board.example.com"},
		{"https://board.example.com/", "https://board.example.com"},
		{"https://board.example.com/boards/x", "https://board.example.com"},
		{"  board.example.com  ", "https://board.example.com"},
	} {
		got, err := serverOrigin(tc.typed)
		if err != nil {
			t.Errorf("%q: %v", tc.typed, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q became %q, want %q", tc.typed, got, tc.want)
		}
	}

	for _, bad := range []string{"", "   ", "ftp://board.example.com", "https://"} {
		if _, err := serverOrigin(bad); err == nil {
			t.Errorf("%q was accepted as an address", bad)
		}
	}
}

// A bare host defaults to https rather than http, and the reason is which way
// it is safe to be wrong: a server that speaks only http refuses and says so,
// where the other default sends a password in the clear and works.
func TestABareHostIsAssumedToSpeakHTTPS(t *testing.T) {
	got, err := serverOrigin("board.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://board.example.com" {
		t.Fatalf("got %q", got)
	}
}

// Detaching takes the way in with it. The server keeps everything made there;
// what must not stay is a live session to somebody's board on a machine that
// no longer talks to it.
func TestDetachingForgetsTheSession(t *testing.T) {
	c, vault := attachAt(t)
	vault[attachTokenKey] = "a-live-session"
	if err := saveAttachSettings(c.path, attachSettings{URL: "https://board.example.com", Username: "anna"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := newAttachController(c.path, vault)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.token() != "a-live-session" {
		t.Fatal("an attached desktop cannot read its own session")
	}

	state, err := reloaded.detach()
	if err != nil {
		t.Fatal(err)
	}
	if state.URL != "" {
		t.Errorf("state.URL = %q after detaching", state.URL)
	}
	if _, ok := vault[attachTokenKey]; ok {
		t.Error("the session survived detaching")
	}
}

// enabled and running disagreeing is how the panel knows to say a restart is
// owed: the board the page is served and the board the agents work against are
// both decided when the app is built up.
func TestAFreshAttachmentOwesARestart(t *testing.T) {
	c, vault := attachAt(t)
	if err := saveAttachSettings(c.path, attachSettings{URL: "https://board.example.com", Username: "anna"}); err != nil {
		t.Fatal(err)
	}
	c.cfg.URL, c.cfg.Username = "https://board.example.com", "anna"
	_ = vault

	state := c.state()
	if state.URL == state.Running {
		t.Fatal("a desktop that attached without restarting must not read as attached")
	}
	if state.Running != "" {
		t.Fatalf("running = %q, want this machine's own board", state.Running)
	}
}

// Attached and looked at are different questions, which is the whole reason the
// sidebar has a switcher rather than the settings having a second toggle: a
// desktop can be working a server's cards while its owner reads their own board.
func TestWhatIsShownIsNotWhatIsAttached(t *testing.T) {
	c, vault := attachAt(t)
	vault[attachTokenKey] = "a-live-session"
	if err := saveAttachSettings(c.path, attachSettings{
		URL: "https://board.example.com", Username: "anna", Selected: "https://board.example.com",
	}); err != nil {
		t.Fatal(err)
	}
	c, err := newAttachController(c.path, vault)
	if err != nil {
		t.Fatal(err)
	}

	if c.selected() != "https://board.example.com" {
		t.Fatalf("selected = %q", c.selected())
	}

	state, err := c.selectBoard("")
	if err != nil {
		t.Fatal(err)
	}
	if state.Selected != "" {
		t.Errorf("state.Selected = %q after choosing this machine", state.Selected)
	}
	if c.selected() != "" {
		t.Errorf("selected = %q after choosing this machine", c.selected())
	}
	// Still attached: the agents go on working the server's cards.
	if c.target() != "https://board.example.com" {
		t.Errorf("looking at this machine detached the server: target = %q", c.target())
	}
	if c.token() == "" {
		t.Error("the server's session was dropped by looking away from it")
	}
}

// A board nobody is attached to cannot be shown, and a selection left over from
// an attachment that is gone falls back rather than showing nothing.
func TestAServerThatIsGoneCannotBeShown(t *testing.T) {
	c, _ := attachAt(t)

	if _, err := c.selectBoard("https://board.example.com"); err == nil {
		t.Error("a server this desktop has never logged in to was selected")
	}

	if err := saveAttachSettings(c.path, attachSettings{Selected: "https://board.example.com"}); err != nil {
		t.Fatal(err)
	}
	stale, err := newAttachController(c.path, fakeVault{})
	if err != nil {
		t.Fatal(err)
	}
	if stale.selected() != "" {
		t.Errorf("selected = %q, want this machine's own board", stale.selected())
	}
}

// The switcher's own list: this machine always, the server when there is one.
func TestTheSwitcherOffersWhatThereIs(t *testing.T) {
	c, _ := attachAt(t)
	if got := c.state().Sources; len(got) != 1 || got[0].ID != "" {
		t.Fatalf("sources = %v, want this machine alone", got)
	}

	if err := saveAttachSettings(c.path, attachSettings{URL: "https://board.example.com", Username: "anna"}); err != nil {
		t.Fatal(err)
	}
	c.cfg.URL, c.cfg.Username = "https://board.example.com", "anna"
	got := c.state().Sources
	if len(got) != 2 || got[0].ID != "" || got[1].ID != "https://board.example.com" {
		t.Fatalf("sources = %v, want this machine and the server", got)
	}
}
