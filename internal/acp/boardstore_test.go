package acp

import (
	"context"
	"encoding/json"
	"errors"
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
	m.cfg.Columns = nil
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

// A board's automation belongs to the board, so editing it writes to the board
// and leaves nothing about that board in the machine's own file — which is what
// makes it travel with the board and disappear with it.
func TestABoardsAutomationIsSavedOnTheBoard(t *testing.T) {
	m, meta, cfgPath := storeManager(t)

	if _, err := m.SaveColumn(ColumnSpec{BoardID: "board1", Property: "Статус", Column: "В работе", Action: ColumnActionAgent}); err != nil {
		t.Fatal(err)
	}
	board := meta.written["board1"]
	if board == nil {
		t.Fatal("the board was never told about its own automation")
	}
	columns, err := parseBoardAutomation(board)
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 1 || columns[0].Column != "В работе" {
		t.Errorf("the board holds columns %+v", columns)
	}

	stored := storedConfig(t, cfgPath)
	if len(stored.Columns) != 0 {
		t.Errorf("the machine's file still carries the board's automation: %+v", stored.Columns)
	}
}

// A board that cannot be written to right now must not lose what somebody drew.
// It is not in config.json any more, so what has to survive is the registry
// this run reads — and the next edit has to reach the board.
func TestARefusedBoardKeepsTheEditForTheRun(t *testing.T) {
	m, meta, _ := storeManager(t)
	meta.fail = errors.New("board store is not ready")

	if _, err := m.SaveColumn(ColumnSpec{BoardID: "board1", Property: "Статус", Column: "В работе", Action: ColumnActionAgent}); err != nil {
		t.Fatal(err)
	}
	if len(m.cfg.Columns) != 1 {
		t.Fatalf("the column was dropped with no board to hold it: %+v", m.cfg.Columns)
	}

	// And it reaches the board the moment the board can take it.
	meta.fail = nil
	if _, err := m.SaveColumn(ColumnSpec{BoardID: "board1", Property: "Статус", Column: "В работе", Action: ColumnActionDeploy}); err != nil {
		t.Fatal(err)
	}
	columns, err := parseBoardAutomation(meta.written["board1"])
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 1 || columns[0].Action != ColumnActionDeploy {
		t.Errorf("the board did not get the column after it could take it: %+v", columns)
	}
}

// A board imported from another machine names agents and deploy targets that
// were registered there. Reading it here cannot use those columns — and must
// not take them off the board, which is what "reading a board cannot shrink
// it" means. Before this, opening an imported board once emptied its
// its columns and there was nothing left to register the agent for.
func TestReadingABoardNeverTakesAutomationOffIt(t *testing.T) {
	m, meta, _ := storeManager(t)
	mine := ColumnSpec{Property: "Статус", Column: "Готово", Action: ColumnActionNone}
	theirs := ColumnSpec{Property: "Статус", Column: "В работе", Action: ColumnActionAgent, Agents: []string{"Клод"}}
	meta.props = map[string]any{
		BoardPropColumns: []ColumnSpec{mine, theirs},
	}

	m.SeedBoard("board1")

	columns, err := parseBoardAutomation(meta.written["board1"])
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 {
		t.Fatalf("the board kept %d of its 2 columns: %+v", len(columns), columns)
	}

	// The usable half did reach the registry; the other half did not, because
	// there is no agent here to run it.
	if len(m.cfg.Columns) != 1 || m.cfg.Columns[0].Column != "Готово" {
		t.Fatalf("the registry took %+v", m.cfg.Columns)
	}

	// Registering the agent and asking again — which is what the setup wizard
	// does — makes the column live without a restart, and still leaves the
	// board with both.
	if _, err := m.AddAgent(AgentEntry{Name: "Клод", Kind: "claude"}); err != nil {
		t.Fatal(err)
	}
	m.SeedBoard("board1")
	if len(m.cfg.Columns) != 2 {
		t.Fatalf("the registry took %+v after the agent was registered", m.cfg.Columns)
	}
	columns, err = parseBoardAutomation(meta.written["board1"])
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 {
		t.Fatalf("the board ended up with %d columns: %+v", len(columns), columns)
	}
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

	m.SeedBoard("board1")

	if got := m.BoardPrompt("board1"); got != "Отвечай по-русски." {
		t.Fatalf("the machine took %q from the board", got)
	}
}


// Setting a board's instructions can be the first thing that ever happens to a
// board — the automation editor need not have been opened. Every edit writes
// the whole of that board's automation back, so an edit made before the board
// was read once emptied a freshly made board's columns.
func TestAnEditBeforeTheBoardWasEverReadKeepsItsAutomation(t *testing.T) {
	m, meta, _ := storeManager(t)
	meta.props = map[string]any{
		BoardPropColumns: []ColumnSpec{{Property: "Статус", Column: "В работе", Action: ColumnActionAgent, Agents: []string{"Клод"}}},
	}

	if err := m.SetBoardPrompt("board1", "Отвечай по-русски."); err != nil {
		t.Fatal(err)
	}

	columns, err := parseBoardAutomation(meta.written["board1"])
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 1 {
		t.Fatalf("the board was left with %d columns", len(columns))
	}
	if meta.written["board1"][BoardPropPrompt] != "Отвечай по-русски." {
		t.Errorf("the instruction did not land: %v", meta.written["board1"][BoardPropPrompt])
	}
}
