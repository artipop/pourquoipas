package acp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artipop/pourquoipas/internal/secrets"
)

func proxyWithPassword(name string) ProxyEntry {
	e := proxyEntry(name, "http://proxy.example.com:8080")
	e.Username = "user"
	e.Password = "s3cret:pw"
	return e
}

// restarted is a second launch over the same database with the given secret
// store, registries read as Start reads them.
func restarted(t *testing.T, m *Manager, vault secrets.Store) *Manager {
	t.Helper()
	next := NewManager(DefaultConfig(t.TempDir()), "", m.store, newFakeWriter(), &fakeEmitter{}, nil)
	next.SetSecrets(vault)
	next.cfgMu.Lock()
	defer next.cfgMu.Unlock()
	if err := next.loadRegistriesLocked(); err != nil {
		t.Fatal(err)
	}
	return next
}

func columnPassword(t *testing.T, m *Manager, id string) string {
	t.Helper()
	var pw *string
	rows, err := m.store.query(`SELECT password FROM proxy WHERE id=?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&pw); err != nil {
			t.Fatal(err)
		}
	}
	if pw == nil {
		return ""
	}
	return *pw
}

// The password is the one credential a person types into the registries, so it
// must not be anywhere a backup, a synced folder or a pasted file would carry
// it: not the row, not config.json, not what the window is sent.
func TestAProxyPasswordIsKeptInTheSecretStoreAndNowhereElse(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	m := agentManager(t, cfgPath)
	saved, err := m.AddProxy(proxyWithPassword("office"))
	if err != nil {
		t.Fatal(err)
	}

	if got, _ := m.vault.Get(proxySecretKey(saved.ID)); got != "s3cret:pw" {
		t.Errorf("the secret store holds %q", got)
	}
	if got := columnPassword(t, m, saved.ID); got != "" {
		t.Errorf("the row holds the password: %q", got)
	}
	file, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(file), "s3cret") {
		t.Errorf("config.json holds the password: %s", file)
	}
	for name, out := range map[string]any{"list": m.Proxies(), "saved": saved} {
		b, _ := json.Marshal(out)
		if strings.Contains(string(b), "s3cret") {
			t.Errorf("the window is sent the password in the %s: %s", name, b)
		}
	}
	if !m.Proxies()[0].PasswordSet {
		t.Error("the window cannot tell there is a password")
	}
}

// A session asks the registry, not the window's copy, so what it gets has to
// carry the password — from the store, after a restart too.
func TestAnAgentStartsWithThePasswordFromTheSecretStore(t *testing.T) {
	m := agentManager(t, "")
	if _, err := m.AddProxy(proxyWithPassword("office")); err != nil {
		t.Fatal(err)
	}
	next := restarted(t, m, m.vault)

	net, err := next.resolveNetwork(AgentEntry{Name: "a", Kind: "codex", ProxyName: "office"})
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for _, kv := range spawnEnv(AgentEntry{}, net) {
		seen = seen || kv == "HTTPS_PROXY=http://user:s3cret%3Apw@proxy.example.com:8080"
	}
	if !seen {
		t.Errorf("HTTPS_PROXY lacks the credentials: %v", spawnEnv(AgentEntry{}, net))
	}
}

// The field is empty when an entry is opened, so empty has to mean "as it was"
// — unless the login changed, when the old password belongs to somebody else.
func TestAnEmptyPasswordFieldKeepsThePasswordOfTheSameLogin(t *testing.T) {
	m := agentManager(t, "")
	saved, err := m.AddProxy(proxyWithPassword("office"))
	if err != nil {
		t.Fatal(err)
	}

	edit := saved
	edit.NoProxy = "localhost"
	if _, err := m.UpdateProxy(edit); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.vault.Get(proxySecretKey(saved.ID)); got != "s3cret:pw" {
		t.Errorf("an edit that left the field empty lost the password: %q", got)
	}

	edit.Username = "someone-else"
	if _, err := m.UpdateProxy(edit); err != nil {
		t.Fatal(err)
	}
	if _, err := m.vault.Get(proxySecretKey(saved.ID)); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("the old password followed a different login: %v", err)
	}
}

func TestRemovingAProxyRemovesItsPassword(t *testing.T) {
	m := agentManager(t, "")
	saved, err := m.AddProxy(proxyWithPassword("office"))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveProxy("office"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.vault.Get(proxySecretKey(saved.ID)); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("the password outlived its proxy: %v", err)
	}
}

// Renaming must not orphan the password: the key is the id.
func TestRenamingAProxyKeepsItsPassword(t *testing.T) {
	m := agentManager(t, "")
	saved, err := m.AddProxy(proxyWithPassword("office"))
	if err != nil {
		t.Fatal(err)
	}
	edit := saved
	edit.Name = "hq"
	if _, err := m.UpdateProxy(edit); err != nil {
		t.Fatal(err)
	}
	net, err := m.resolveNetwork(AgentEntry{Name: "a", Kind: "codex", ProxyID: saved.ID})
	if err != nil || net.Password != "s3cret:pw" {
		t.Errorf("after a rename: %q (%v)", net.Password, err)
	}
}

// Databases written before the secret store held the password in the row. The
// next start moves it, and only clears the row once the store returns it.
func TestAProxyPasswordFromAnOldDatabaseMovesToTheSecretStore(t *testing.T) {
	m := agentManager(t, "")
	saved, err := m.AddProxy(proxyEntry("office", "http://proxy.example.com:8080"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.exec(`UPDATE proxy SET username='user', password='old-pw' WHERE id=?`, saved.ID); err != nil {
		t.Fatal(err)
	}

	next := restarted(t, m, m.vault)

	if got, _ := m.vault.Get(proxySecretKey(saved.ID)); got != "old-pw" {
		t.Errorf("the secret store holds %q", got)
	}
	if got := columnPassword(t, m, saved.ID); got != "" {
		t.Errorf("the row still holds %q", got)
	}
	if net, _ := next.resolveNetwork(AgentEntry{Name: "a", Kind: "codex", ProxyID: saved.ID}); net.Password != "old-pw" {
		t.Errorf("the agent would run without the password: %q", net.Password)
	}
}

type refusingStore struct{ *secrets.MemoryStore }

func (refusingStore) Set(string, string) error { return errors.New("keychain locked") }

// A locked keychain must cost nothing: the row keeps the password, the agent
// still runs with it, and the app starts.
func TestAKeychainThatRefusesLeavesAnOldPasswordWhereItWas(t *testing.T) {
	m := agentManager(t, "")
	saved, err := m.AddProxy(proxyEntry("office", "http://proxy.example.com:8080"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.exec(`UPDATE proxy SET username='user', password='old-pw' WHERE id=?`, saved.ID); err != nil {
		t.Fatal(err)
	}

	next := restarted(t, m, refusingStore{secrets.NewMemoryStore()})

	if got := columnPassword(t, m, saved.ID); got != "old-pw" {
		t.Errorf("the password was erased although nothing kept it: %q", got)
	}
	if net, _ := next.resolveNetwork(AgentEntry{Name: "a", Kind: "codex", ProxyID: saved.ID}); net.Password != "old-pw" {
		t.Errorf("the agent would run without the password: %q", net.Password)
	}
}
