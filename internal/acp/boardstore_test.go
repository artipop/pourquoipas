package acp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// storeManager is a manager with a real config file and a board store that
// records what it was told to keep.
func storeManager(t *testing.T) (*Manager, *fakeBoardMeta, string) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	m := agentManager(t, cfgPath)
	m.rootCtx = context.Background()
	meta := &fakeBoardMeta{}
	m.SetBoardMeta(meta)
	return m, meta, cfgPath
}

// storedConfig is what the machine's own file ended up holding.
func storedConfig(t *testing.T, path string) Config {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the config: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse the config: %v", err)
	}
	return cfg
}

// What a board's agents are told first is about that board, not about this
// machine, so it lives on the board and travels with it — and stops being
// written to the machine's own file once the board has taken it.
func TestTheBoardsInstructionsAreSavedOnTheBoard(t *testing.T) {
	m, meta, cfgPath := storeManager(t)

	if err := m.SetBoardPrompt("board1", "Отвечай по-русски."); err != nil {
		t.Fatal(err)
	}
	if got := meta.written["board1"][BoardPropPrompt]; got != "Отвечай по-русски." {
		t.Fatalf("the board was told %q", got)
	}
	if got := storedConfig(t, cfgPath).BoardPrompts["board1"]; got != "" {
		t.Errorf("the machine's file still carries the board's instructions: %q", got)
	}

	// A board that unsaid it says so on the board, or the next launch would
	// hand the old text straight back.
	if err := m.SetBoardPrompt("board1", ""); err != nil {
		t.Fatal(err)
	}
	if got := meta.written["board1"][BoardPropPrompt]; got != "" {
		t.Errorf("the board still holds %q after it was cleared", got)
	}
}

// The other direction: a board that arrives carrying instructions briefs its
// agents here without anybody retyping them.
func TestTheBoardsInstructionsAreTakenFromTheBoard(t *testing.T) {
	m, meta, _ := storeManager(t)
	meta.props = map[string]any{BoardPropPrompt: "Отвечай по-русски."}

	m.seedFromBoard("board1")

	if got := m.BoardPrompt("board1"); got != "Отвечай по-русски." {
		t.Fatalf("the machine took %q from the board", got)
	}
}
