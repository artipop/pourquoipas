package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// A preference a person set is the install's to keep: the desktop window's
// localStorage is keyed to a loopback origin with a random port, so anything
// left there is gone by the next launch.
func TestUIPreferencesSurviveInTheAppsOwnData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	app := &App{}
	if prefs, err := app.GetUIPreferences(); err != nil || prefs != "{}" {
		t.Fatalf("a fresh install answered %q, %v — want an empty object", prefs, err)
	}

	if err := app.SetUIPreference("language", "ru"); err != nil {
		t.Fatal(err)
	}
	if err := app.SetUIPreference("theme", "dark"); err != nil {
		t.Fatal(err)
	}

	prefs, err := app.GetUIPreferences()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(prefs), &got); err != nil {
		t.Fatal(err)
	}
	if got["language"] != "ru" || got["theme"] != "dark" {
		t.Fatalf("kept %v, want language=ru theme=dark", got)
	}

	// Empty forgets the one preference, not the file.
	if err := app.SetUIPreference("theme", ""); err != nil {
		t.Fatal(err)
	}
	prefs, _ = app.GetUIPreferences()
	got = nil
	if err := json.Unmarshal([]byte(prefs), &got); err != nil {
		t.Fatal(err)
	}
	if _, there := got["theme"]; there || got["language"] != "ru" {
		t.Fatalf("forgetting the theme left %v", got)
	}
}

// The first release of this file kept only the language, in a field of its
// own. It reads back as the preference it since became.
func TestUISettingsFoldTheOldLanguageField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	dir, err := appDataDir("", 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ui-settings.json"), []byte(`{"language":"de"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	prefs, err := app.GetUIPreferences()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(prefs), &got); err != nil {
		t.Fatal(err)
	}
	if got["language"] != "de" {
		t.Fatalf("the old field read back as %v, want language=de", got)
	}
}

// The caps are a backstop, not a schema: a runaway key or value must not grow
// the settings file without bound.
func TestUIPreferenceRefusesTheUnreasonable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	app := &App{}
	if err := app.SetUIPreference("", "x"); err == nil {
		t.Error("an empty key was accepted")
	}
	// A value the product actually produces has to fit. The cap was 16KB while
	// a preference was a word; the themes a person writes are a list of configs
	// and eight of them alone pass that, so it was raised deliberately
	// (uisettings.go) — and this test went on asserting the old number, which
	// is what left the trunk red.
	roomy := make([]byte, 100*1024)
	if err := app.SetUIPreference("themes", string(roomy)); err != nil {
		t.Errorf("a value the size of somebody's themes was refused: %v", err)
	}
	runaway := make([]byte, 600*1024)
	if err := app.SetUIPreference("themes", string(runaway)); err == nil {
		t.Error("a value past the backstop was accepted")
	}
}

// The page is the only side that knows which language the app is being read in
// when nobody has picked one — the webview's own locale. It says so at boot,
// and that answer must not turn into a setting: storing it would pin the app to
// whatever it happened to say and stop it following the system.
func TestTheLanguageThePageResolvedIsNotAPreference(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	activeLanguage = ""
	t.Cleanup(func() { activeLanguage = "" })

	// Guarded, because SetUIPreference reinstalls on its own goroutine: the
	// switch on screen must not wait for five boards.
	var mu sync.Mutex
	var installed []string
	app := &App{retemplate: func(lang string) error {
		mu.Lock()
		defer mu.Unlock()
		installed = append(installed, lang)
		return nil
	}}
	installedSoFar := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), installed...)
	}

	if err := app.UseLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	if prefs, _ := app.GetUIPreferences(); prefs != "{}" {
		t.Errorf("it was written down as %q", prefs)
	}
	if uiLanguage() != "ru" {
		t.Errorf("the language in force is %q", uiLanguage())
	}
	if got := installedSoFar(); len(got) != 1 || got[0] != "ru" {
		t.Errorf("the templates were installed as %v", got)
	}

	// Said again, it costs nothing: this runs at every launch.
	if err := app.UseLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	if got := installedSoFar(); len(got) != 1 {
		t.Errorf("the templates were reinstalled for the same answer: %v", got)
	}

	// And a pick beats it — the person said so, the webview only guessed.
	if err := app.SetUIPreference("language", "en"); err != nil {
		t.Fatal(err)
	}
	if err := app.UseLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	if uiLanguage() != "en" {
		t.Errorf("the language in force is %q", uiLanguage())
	}
}
