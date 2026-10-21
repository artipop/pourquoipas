package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/artipop/pourquoipas/internal/acp"
)

// What the install remembers about its own UI — the language and theme a
// person picked, where they were last, every a do-not-show-again switch. It lives
// here, in the app's own data, rather than in the browser: the desktop window
// opens on a loopback origin the browser has no lasting memory for (a random
// port per launch — docs/deferred.md, moving off localStorage), and a
// preference a person set is the install's to keep, the way any desktop app
// keeps one. The page hydrates its localStorage from this before the first
// render (userSettings.ts) and writes back through SetUIPreference; in a
// plain browser or as a Mattermost plugin there is no Go side, and
// localStorage stays the whole memory there.
type uiSettings struct {
	// Language predates Preferences by one release; it is folded into the map
	// on read and never written again.
	Language    string            `json:"language,omitempty"`
	Preferences map[string]string `json:"preferences,omitempty"`

	// ResolvedLanguage is the language the page last said it was showing while
	// nobody had picked one (UseLanguage). Not a preference, and never handed
	// back as one: the page resolves it afresh at every launch, so the app goes
	// on following the system. It is kept so that the next launch installs the
	// templates in the language the page is about to ask for — see uiLanguage.
	ResolvedLanguage string `json:"resolvedLanguage,omitempty"`
}

// One writer at a time: two bindings racing the read-modify-write would drop
// one of the edits.
var uiSettingsMu sync.Mutex

func uiSettingsPath() (string, error) {
	dir, err := appDataDir("", 0o755)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui-settings.json"), nil
}

func readUISettings() uiSettings {
	var s uiSettings
	path, err := uiSettingsPath()
	if err != nil {
		return s
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return uiSettings{}
	}
	if s.Language != "" {
		if s.Preferences == nil {
			s.Preferences = map[string]string{}
		}
		if s.Preferences["language"] == "" {
			s.Preferences["language"] = s.Language
		}
		s.Language = ""
	}
	return s
}

func writeUISettings(s uiSettings) error {
	path, err := uiSettingsPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// GetUIPreferences returns everything the install keeps for its UI, as a JSON
// object of key → value. Empty object when nothing was ever picked.
func (a *App) GetUIPreferences() (string, error) {
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	prefs := readUISettings().Preferences
	if prefs == nil {
		prefs = map[string]string{}
	}
	out, err := json.Marshal(prefs)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// agentNotificationsEnabled is the notify-when-an-agent-waits switch, read from the
// same place the page keeps it — there is one switch for one question, and the
// OS notification the Go side posts (alerts.go) is the same "tell me" the page's
// own notification answers. On unless it has been turned off, which is the rule
// UserSettings.agentNotifications states on the other side (userSettings.ts);
// the value stored is JSON, so it is the string "false" that means no.
//
// The dot in the menu bar deliberately does not consult this: it is an
// indicator and it interrupts nobody.
func agentNotificationsEnabled() bool {
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	return readUISettings().Preferences["agentNotifications"] != "false"
}

// aiEnabled is the app-wide AI switch, kept where the page keeps it
// (aiSwitch.ts, the "aiEnabled" preference). On unless it has been turned off,
// the rule UserSettings.aiEnabled states on the other side. Off, the agent
// manager starts nothing by itself (acp.Manager.SetSwitchedOff) and the OS is
// told about no wait.
func aiEnabled() bool {
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	return readUISettings().Preferences["aiEnabled"] != "false"
}

// activeLanguage is the language the page resolved for itself when nobody has
// picked one: it reads the webview's own locale (getCurrentLanguage in
// i18n.tsx) and the Go side has no way to. Reported at boot through
// UseLanguage; guarded by uiSettingsMu with the rest.
var activeLanguage string

// uiLanguage is the language the app is read in, as the page's own setting
// names it («ru», «en», «pt_BR»). The pick comes first, then what the page
// resolved in this run, then what it resolved in the last one, then the
// language the templates are written in.
//
// The last run's answer is what startup installs the templates in, and it has
// to be: activeLanguage is empty until the page reports, so without it every
// launch put the templates into English and the page's report then put them
// back into its own language — two full reinstalls per launch, the second one
// inside the call the first render waits for.
func uiLanguage() string {
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	return languageInForce(readUISettings())
}

// languageInForce is uiLanguage's answer for settings already read; the caller
// holds uiSettingsMu.
func languageInForce(s uiSettings) string {
	if picked := s.Preferences["language"]; picked != "" {
		return picked
	}
	if activeLanguage != "" {
		return activeLanguage
	}
	return s.ResolvedLanguage
}

// UseLanguage is the page telling this side which language it is showing,
// whether or not anybody picked it. It exists because the templates are boards
// rather than catalogue strings: without it a person who never opened the
// settings would read the app in their own language and be offered templates
// in English.
//
// It sets no preference — a resolved default is not a choice, and a pick would
// stop the app following the system. What it does keep is the answer itself,
// as ResolvedLanguage, so that the next launch starts in it (uiLanguage).
func (a *App) UseLanguage(lang string) error {
	lang = strings.TrimSpace(lang)
	if lang == "" || len(lang) > 64 {
		return nil
	}
	uiSettingsMu.Lock()
	s := readUISettings()
	// What the templates are in right now: this run's answer, or — before the
	// page has said anything — the last run's, which startup installed them in.
	was := languageInForce(s)
	picked := s.Preferences["language"]
	activeLanguage = lang
	if s.ResolvedLanguage != lang {
		s.ResolvedLanguage = lang
		if err := writeUISettings(s); err != nil {
			// Costs the next launch a reinstall, which is what every launch
			// cost before this was kept.
			log.Printf("ui settings: %v", err)
		}
	}
	uiSettingsMu.Unlock()

	// Only when it is the answer that counts and it has changed: a pick beats
	// it, and the templates already record which language they were installed
	// in, so a repeat costs nothing anyway.
	//
	// Here it is done before answering, and the page waits for the answer
	// before its first render (main.tsx): replacing five boards is measured in
	// tens of milliseconds, and doing it after the page has drawn means the
	// page holds a list of boards that no longer exist — the template picker
	// drew both languages at once.
	if picked == "" && was != lang {
		a.reinstallTemplates(lang)
	}
	return nil
}

// SetUIPreference remembers one preference; an empty value forgets it. The
// page sends only the keys it deliberately keeps with the install
// (userSettings.ts names them); the caps below are a backstop, not a schema —
// a runaway value must not grow the settings file without bound.
func (a *App) SetUIPreference(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 64 {
		return errors.New("ui settings: bad key")
	}
	// A preference used to be a word — a language, a theme's name, a
	// do-not-show-again. One of them is now a list: the themes a person wrote,
	// each of them two configs of text, and eight of those alone pass 16KB.
	// This is still a runaway backstop rather than a schema; it is just set
	// where a real answer can fit under it.
	if len(value) > 512*1024 {
		return errors.New("ui settings: value too large")
	}
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	s := readUISettings()
	if s.Preferences == nil {
		s.Preferences = map[string]string{}
	}
	changed := s.Preferences[key] != value
	if value == "" {
		delete(s.Preferences, key)
	} else {
		s.Preferences[key] = value
	}
	if err := writeUISettings(s); err != nil {
		return err
	}

	// The shipped templates are boards, not catalogue strings, so a language
	// change has to reinstall them (internal/boardadapter/templatetext.go).
	// After the write and off the caller's goroutine: it deletes and recreates
	// five boards, and the switch on screen must not wait for that. A failure
	// costs the person templates in the previous language, which is what they
	// already had, so it is logged rather than returned.
	// Off the caller's goroutine: the switch on screen must not wait for five
	// boards to be deleted and made again, and the picker is a click away at
	// the earliest. It fetches what it draws when it opens, so by then this is
	// done. A failure costs the person templates in the language they already
	// had, so it is logged rather than returned.
	// The tray's dot and the OS notifications re-read the waits on this event
	// (alerts_desktop.go), and they answer the switch as well: without it the
	// dot switched off would stay lit until the next wait came or went. Off
	// this goroutine, which holds uiSettingsMu, and every listener reads the
	// switch under it.
	if key == "aiEnabled" && changed && a != nil && a.emitter != nil {
		go a.emitter.Emit(acp.EventAttention, nil)
	}

	if key == "language" && changed {
		// Into the language now in force, not into value: a pick taken back is
		// an empty value, and it means "follow the system again" — which is the
		// page's resolved answer, not the templates' source language.
		go a.reinstallTemplates(languageInForce(s))
	}
	return nil
}

// reinstallTemplates puts the templates and the inbox into a language, and does the
// work where it is called from.
func (a *App) reinstallTemplates(lang string) {
	if a == nil || a.relanguage == nil {
		return
	}
	if err := a.relanguage(lang); err != nil {
		log.Printf("language: %v", err)
	}
}
