//go:build !server

package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/artipop/pourquoipas/server/model"
)

// ui-settings.json is where a desktop kept its UI before the settings moved
// into the database: a person's to preferences, the machine's to
// system_settings (machinesettings.go). It is read once, carried over, and
// deleted. Only a desktop has one worth reading — on a server it was a single
// file for everybody, and whose value was whose cannot be told.

// userPreferenceKeys are the keys that are a person's. The rest of the old
// file was either the machine's or the browser's (mobileWarningClosed) and is
// left behind.
var userPreferenceKeys = []string{
	"language", "theme", "themeMode", "customThemes",
	"customTheme", "customThemeLight", "customThemeDark", "customThemeName",
	"lastTeamId", "lastBoardId", "lastViewId", "nameFormat",
}

var machinePreferenceKeys = []string{settingAIEnabled, settingAgentNotifications, settingPrefillCardFolder}

type oldUISettings struct {
	// Language predates Preferences by one release.
	Language         string            `json:"language"`
	Preferences      map[string]string `json:"preferences"`
	ResolvedLanguage string            `json:"resolvedLanguage"`
}

type userPreferences interface {
	GetUserPreferences(userID string) ([]model.Preference, error)
	UpdateUserConfig(userID string, patch model.UserPreferencesPatch) ([]model.Preference, error)
}

func oldUISettingsPath() (string, error) {
	dir, err := appDataDir("", 0o755)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui-settings.json"), nil
}

// migrateUISettings carries the old file into the database for the single user.
// What the database already has wins, so a file that survived a failed delete
// changes nothing the next time. Left in place on any error, to try again.
func migrateUISettings(path string, prefs userPreferences) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var old oldUISettings
	if err := json.Unmarshal(raw, &old); err != nil {
		// Unreadable is nothing to carry; it is not kept to fail every launch.
		return os.Remove(path)
	}
	if old.Preferences == nil {
		old.Preferences = map[string]string{}
	}
	if old.Preferences["language"] == "" && old.Language != "" {
		old.Preferences["language"] = old.Language
	}

	have, err := prefs.GetUserPreferences(model.SingleUser)
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, p := range have {
		present[p.Name] = true
	}
	patch := model.UserPreferencesPatch{UpdatedFields: map[string]string{}}
	for _, key := range userPreferenceKeys {
		if value := old.Preferences[key]; value != "" && !present[key] {
			patch.UpdatedFields[key] = value
		}
	}
	if len(patch.UpdatedFields) > 0 {
		if _, err := prefs.UpdateUserConfig(model.SingleUser, patch); err != nil {
			return err
		}
	}

	for _, key := range machinePreferenceKeys {
		if value := old.Preferences[key]; value != "" && !machine.has(key) {
			if err := machine.set(key, value); err != nil {
				return err
			}
		}
	}
	if old.ResolvedLanguage != "" && !machine.has(settingShownLanguage) {
		if err := machine.set(settingShownLanguage, old.ResolvedLanguage); err != nil {
			return err
		}
	}
	return os.Remove(path)
}
