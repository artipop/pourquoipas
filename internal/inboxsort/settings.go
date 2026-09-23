package inboxsort

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Settings is what a person chose in the inbox's sorting dialog. A file of its
// own beside the sources registry rather than a key in it: sorting is not a
// source, and a source works with no agents while this does nothing without
// one.
type Settings struct {
	// Enabled says a card a source brings is sorted as it arrives. Off, the
	// inbox waits for a person — or for the button that sorts it now.
	Enabled bool `json:"enabled"`
	// Agent names the registry entry that reads the cards and the boards. Kept
	// when sorting is switched off, so switching it back on asks nothing.
	Agent string `json:"agent,omitempty"`
}

// LoadSettings reads the file. A file that is not there is sorting switched
// off, which is what an install that never opened the dialog has.
func LoadSettings(path string) (Settings, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, err
	}
	s.Agent = strings.TrimSpace(s.Agent)
	return s, nil
}

// SaveSettings writes the file whole, through a temporary one, so a crash in
// the middle leaves the old settings rather than half of the new ones.
func SaveSettings(path string, s Settings) error {
	if path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
