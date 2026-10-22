//go:build !server

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/artipop/pourquoipas/server/model"
)

type preferenceRows map[string]string

func (p preferenceRows) GetUserPreferences(userID string) ([]model.Preference, error) {
	var out []model.Preference
	for name, value := range p {
		out = append(out, model.Preference{UserId: userID, Category: model.PreferencesCategoryFocalboard, Name: name, Value: value})
	}
	return out, nil
}

func (p preferenceRows) UpdateUserConfig(_ string, patch model.UserPreferencesPatch) ([]model.Preference, error) {
	for k, v := range patch.UpdatedFields {
		p[k] = v
	}
	return nil, nil
}

func writeOldSettings(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ui-settings.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Somebody updating keeps what they picked: the person's settings become the
// single user's, the machine's switches the install's, and the file goes.
func TestAnUpdateCarriesTheOldSettingsFileIntoTheDatabase(t *testing.T) {
	table := settingsTable{}
	freshMachine(t, table)
	prefs := preferenceRows{}
	path := writeOldSettings(t, `{
		"preferences": {"language": "ru", "themeMode": "dark", "lastBoardId": "{\"0\":\"b1\"}",
			"aiEnabled": "false", "mobileWarningClosed": "true"},
		"resolvedLanguage": "ru"
	}`)

	if err := migrateUISettings(path, prefs); err != nil {
		t.Fatal(err)
	}
	if prefs["language"] != "ru" || prefs["themeMode"] != "dark" || prefs["lastBoardId"] != `{"0":"b1"}` {
		t.Errorf("the person's settings came over as %v", prefs)
	}
	if _, there := prefs["aiEnabled"]; there {
		t.Error("a machine switch became the person's")
	}
	if _, there := prefs["mobileWarningClosed"]; there {
		t.Error("a browser's own key became the person's")
	}
	if table[settingAIEnabled] != "false" || table[settingShownLanguage] != "ru" {
		t.Errorf("the machine's came over as %v", table)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the file is still there: %v", err)
	}
}

// The first release kept only the language, in a field of its own.
func TestTheOldestSettingsFileStillCarriesItsLanguage(t *testing.T) {
	freshMachine(t, settingsTable{})
	prefs := preferenceRows{}
	if err := migrateUISettings(writeOldSettings(t, `{"language":"de"}`), prefs); err != nil {
		t.Fatal(err)
	}
	if prefs["language"] != "de" {
		t.Errorf("came over as %v", prefs)
	}
}

// What the database already says is newer than the file: a file that outlived
// a failed delete must not put old answers back.
func TestTheDatabaseWinsOverTheOldFile(t *testing.T) {
	table := settingsTable{settingAIEnabled: "true"}
	freshMachine(t, table)
	prefs := preferenceRows{"language": "en"}
	path := writeOldSettings(t, `{"preferences": {"language": "ru", "aiEnabled": "false"}}`)

	if err := migrateUISettings(path, prefs); err != nil {
		t.Fatal(err)
	}
	if prefs["language"] != "en" || table[settingAIEnabled] != "true" {
		t.Errorf("the file overwrote the database: %v, %v", prefs, table)
	}
}

// A fresh install has no file, and that is not an error.
func TestAFreshInstallHasNothingToCarry(t *testing.T) {
	freshMachine(t, settingsTable{})
	if err := migrateUISettings(filepath.Join(t.TempDir(), "ui-settings.json"), preferenceRows{}); err != nil {
		t.Error(err)
	}
}
