package main

import (
	"path/filepath"
	"testing"
)

// isolateAppData points this install's own directory — everything appDataDir
// hands out — at a scratch one, for the length of one test.
//
// All three platforms have to be named, because os.UserConfigDir reads a
// different variable on each: %AppData% on Windows, HOME on macOS,
// XDG_CONFIG_HOME (else HOME) on Linux. Naming the POSIX pair alone is what let
// the settings tests write into the real install on Windows — the cap test put
// 100 KiB of zeros under `themes` in a running dev build's ui-settings.json,
// and the next launch read them back. The test passed either way, which is why
// it went unnoticed; docs/deferred.md, «Тесты на Windows», records the rest.
func isolateAppData(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	return home
}
