//go:build server

package main

// A server has no ui-settings.json worth reading (uisettings_migrate.go).
func oldUISettingsPath() (string, error) { return "", nil }

func migrateUISettings(string, userPreferences) error { return nil }

type userPreferences any
