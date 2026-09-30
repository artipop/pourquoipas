package acp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/artipop/pourquoipas/internal/secrets"
)

// A proxy's password is the one credential of the registries, and it lives in
// the secret store — the keychain where there is one — under the entry's id.
// The table and the settings file never hold it. The working copy in
// Config.Proxies does, because a session reads it at spawn; nothing that leaves
// the process (Proxies, the return of Add/Update) carries it.

// proxySecretKey is the id and not the name, so renaming a configuration moves
// nothing.
func proxySecretKey(id string) string { return "proxy:" + id }

// withoutPassword is an entry as the window may see it.
func withoutPassword(p ProxyEntry) ProxyEntry {
	p.PasswordSet = p.Password != ""
	p.Password = ""
	return p
}

// keepProxyPassword makes the secret store say what the entry says. It runs
// before the row is written: a failure leaves a row naming a password that was
// never stored, the other order would leave a stored one nobody asked for.
func (m *Manager) keepProxyPassword(p ProxyEntry) error {
	if p.ID == "" {
		return nil
	}
	key := proxySecretKey(p.ID)
	if p.Password == "" {
		// Best effort: a machine without a working keychain must still be
		// able to edit the proxies that have no password.
		if m.vault != nil {
			if _, err := m.vault.Get(key); err == nil {
				return m.vault.Delete(key)
			}
		}
		return nil
	}
	if m.vault == nil {
		return fmt.Errorf("пароль прокси %q некуда сохранить: нет хранилища секретов", p.Name)
	}
	if current, err := m.vault.Get(key); err == nil && current == p.Password {
		return nil
	}
	if err := m.vault.Set(key, p.Password); err != nil {
		return fmt.Errorf("пароль прокси %q не сохранён в связке ключей: %w", p.Name, err)
	}
	return nil
}

// forgetProxyPassword drops the secret of an entry that is gone. A stray secret
// is harmless and a failure here must not undo the removal.
func (m *Manager) forgetProxyPassword(id string) {
	if m.vault == nil {
		return
	}
	if err := m.vault.Delete(proxySecretKey(id)); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		m.log.Warn("acp: a removed proxy's password is still in the secret store", "proxy", id, "err", err)
	}
}

// adoptProxySecrets brings the working copy in line with the secret store, once
// the store is known (SetSecrets) and whenever the registries are read.
//
// A row that still holds a password is from before it had a place to go: it is
// written to the store, read back, and only then cleared from the row. Anything
// that goes wrong leaves the row as it was and the password in the working copy,
// so the agent still runs and the next start tries again.
//
// Called with cfgMu held.
func (m *Manager) adoptProxySecrets() {
	if m.vault == nil || m.store == nil {
		return
	}
	for i := range m.cfg.Proxies {
		p := &m.cfg.Proxies[i]
		key := proxySecretKey(p.ID)
		if p.Password == "" {
			switch pw, err := m.vault.Get(key); {
			case err == nil:
				p.Password = pw
			case !errors.Is(err, secrets.ErrNotFound):
				m.log.Warn("acp: a proxy's password could not be read from the secret store", "proxy", p.Name, "err", err)
			}
			continue
		}
		if err := m.vault.Set(key, p.Password); err != nil {
			m.log.Warn("acp: a proxy password stays in the database: the secret store refused it", "proxy", p.Name, "err", err)
			continue
		}
		if back, err := m.vault.Get(key); err != nil || back != p.Password {
			m.log.Warn("acp: a proxy password stays in the database: the secret store does not return it", "proxy", p.Name, "err", err)
			continue
		}
		if err := m.store.clearProxyPassword(p.ID); err != nil {
			m.log.Warn("acp: a proxy password is in the secret store but its column could not be cleared", "proxy", p.Name, "err", err)
			continue
		}
		m.log.Info("acp: a proxy password moved to the secret store", "proxy", p.Name)
	}
}

// keptPassword is what an edit that left the field empty means: the same
// password, as long as the login it belongs to is the same.
func keptPassword(existing, edit ProxyEntry) string {
	if edit.Password == "" && strings.TrimSpace(edit.Username) == strings.TrimSpace(existing.Username) {
		return existing.Password
	}
	return edit.Password
}
