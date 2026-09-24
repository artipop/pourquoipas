package acp

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// What this application keeps on a board lives on the board: in the board's
// own `properties` map, in the board database, beside everything else that is
// the board's. That is why a board made from a template arrives briefed, and
// why a board carried to another machine takes its instructions with it.
//
// The keys are bare names on purpose: the map is shared with the board server's
// own keys — of which there is exactly one, `trackingTemplateId` — and a prefix
// here would only be the product's name written into stored data, where a
// rename becomes a migration.
//
// BoardPropSetup is not about the board but about this machine: the questions
// it has to be asked before an agent can work a card of it. It is read in
// setup.go.
const (
	BoardPropSetup = "setup"
	// BoardPropPrompt is what this board's agents are told first. It is the
	// board's, for the same reason its columns are: a household board and a
	// code board want different first words, and a board carried to another
	// machine that arrived without them would run its agents unbriefed.
	BoardPropPrompt = "prompt"
	// BoardPropBranch is the id of the text property a card's branch is
	// written into. An id and not a name, for the reason every other field of
	// ours is found by one: the branch field is what this app calls it when it makes
	// one, and a person may call it anything.
	//
	// Written by the page (it owns the board's card properties), read here.
	BoardPropBranch = "branchProperty"
	// BoardPropProject is the id of the select property a card names its folder
	// in — the folder field when this app makes it. An id for the same reason the branch
	// field is one, and load-bearing for a second: it is what lets the folder be
	// read off the card rather than *recognised* among everything the card has
	// selected (resolveWorkdir).
	BoardPropProject = "projectProperty"
	// BoardPropColumnProperty is the id of the select property this board's
	// columns live on — the field a card's column is a value of.
	//
	// It is the last of the three "which field is which" records, and the one
	// that took longest to arrive: which property held the columns was a *name*
	// in the machine's settings, matched against every board this machine ever
	// saw. A board in English, or one
	// where somebody renamed the status property, matched nothing.
	BoardPropColumnProperty = "columnProperty"
)

// boardProp reads one of this app's own keys off a board.
func boardProp(props map[string]any, key string) (any, bool) {
	if v, ok := props[key]; ok && v != nil {
		return v, true
	}
	return nil, false
}

// BoardMeta is the board's own properties — where a board keeps what it tells
// its agents and which of its fields are which — and whether the board is
// itself a template. Optional: without it a board brings nothing and keeps
// nothing.
type BoardMeta interface {
	BoardProperties(ctx context.Context, boardID string) (map[string]any, error)
	// SetBoardProperties writes the named properties onto the board and removes
	// the named ones, leaving everything else alone: the board is where they
	// belong, so the board is what is patched. remove is how a key that has
	// been renamed stops existing — writing the new name without it would leave
	// two answers on the board.
	SetBoardProperties(ctx context.Context, boardID string, props map[string]any, remove []string) error
	// BoardColumnOptions is the board's own columns: the property they live on
	// and every option of it. Asked for every time rather than remembered — a
	// column added this morning has to be offered this afternoon.
	BoardColumnOptions(ctx context.Context, boardID string) ([]Column, error)
	// CardProperties is the board's card fields, for the one thing an agent
	// cannot work out by itself: what a value may be. A select takes one of its
	// option names and nothing else, and until this the agent found that out by
	// being refused.
	CardProperties(ctx context.Context, boardID string) ([]CardProperty, error)
	// IsBoardTemplate says this board is one to copy rather than to work in.
	// Nothing runs in a template — no card moves in it, no session starts from
	// it — so nothing about this machine is asked for on its behalf.
	IsBoardTemplate(ctx context.Context, boardID string) (bool, error)
}

// CardProperty is one field of a board's cards: what it is called, what kind of
// value it takes, and — for a closed set — which values those are. Named rather
// than identified, because that is how an agent is told about it and how
// update_card names one.
type CardProperty struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Values is the closed set, for a select. Empty for everything else.
	Values []string `json:"values,omitempty"`
}

// SetBoardMeta supplies the board's own property store.
func (m *Manager) SetBoardMeta(b BoardMeta) { m.meta = b }

// SeedBoard takes a board's own automation into the registry on demand — what
// the first card move would do anyway, asked for early by the setup wizard.
func (m *Manager) SeedBoard(boardID string) { m.seedFromBoard(boardID) }

// listenBeforeSpeaking makes sure a board has been read before this machine
// writes anything back to it. Every edit is written through in full
// (persistBoardLocked), so an edit made before the board was ever read would
// write the registry's idea of that board — nothing, for a board this machine
// has not looked at — over the board's own columns and routes.
//
// That is not hypothetical: setting a board's instructions is the one edit
// somebody can make without the automation editor having opened first, and it
// emptied a freshly made board's automation.
//
// Called from the edit entry points rather than from persistBoardLocked, which
// runs with cfgMu held, and seeding takes it.
func (m *Manager) listenBeforeSpeaking(boards ...string) {
	for _, boardID := range boards {
		if boardID != "" {
			m.seedFromBoard(boardID)
		}
	}
}

// seedFromBoard imports whatever automation a board carries. It runs once per
// board per run: the import is idempotent anyway (anything already registered
// is left alone), and the point of the flag is to not read the board on every
// card move.
//
// The exception is a board still holding entries this machine could not use.
// Registering the missing agent is what the setup wizard is for, and it seeds
// the board again on the way out — so the answer takes effect there rather
// than at the next launch.
func (m *Manager) seedFromBoard(boardID string) {
	if boardID == "" || m.meta == nil {
		return
	}
	m.seededMu.Lock()
	if m.seeded == nil {
		m.seeded = map[string]bool{}
	}
	if m.seeded[boardID] {
		m.seededMu.Unlock()
		return
	}
	m.seeded[boardID] = true
	m.seededMu.Unlock()

	// The wizard may ask before the trigger loop has started, and a nil parent
	// context is a panic rather than a timeout.
	parent := m.rootCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	props, err := m.forBoard(boardID).Meta.BoardProperties(ctx, boardID)
	cancel()
	if err != nil {
		m.log.Warn("acp: cannot read the board's own settings", "board", boardID, "err", err)
		return
	}
	if len(props) == 0 {
		return
	}
	if m.adoptPrompt(boardID, boardPromptFrom(props)) {
		m.log.Info("acp: the board's own instructions taken from the board itself", "board", boardID)
	}
}

func reinterpret(from any, into any) error {
	// A property may also arrive as the JSON text of itself: the board store
	// keeps free-form properties as strings in some paths.
	if text, ok := from.(string); ok {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return json.Unmarshal([]byte(text), into)
	}
	encoded, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, into)
}

// adoptColumns registers the board's columns, skipping any the registry already
// has an answer for — an edited column is the user's, not the template's. What
// it could not validate comes back as the second result rather than being
// dropped: this machine not knowing the agent a column names is a fact about
// persistBoardLocked writes what this application keeps on a board back onto
// the board: its instructions to agents, and the record of which property its
// columns are. Called from the edit that changes them, with cfgMu already held,
// so what the registry says and what the board says can never drift apart.
//
// A board that cannot be written to is not a lost edit: the value stays in the
// registry for this run, and the next edit tries again.
func (m *Manager) persistBoardLocked(boardID string) {
	if boardID == "" || m.meta == nil {
		return
	}
	props := map[string]any{BoardPropPrompt: m.cfg.BoardPrompts[boardID]}
	ctx, cancel := context.WithTimeout(m.rootOr(), 10*time.Second)
	defer cancel()
	if err := m.forBoard(boardID).Meta.SetBoardProperties(ctx, boardID, props, nil); err != nil {
		m.log.Warn("acp: cannot write the board's own settings", "board", boardID, "err", err)
	}
}

// saveBoardsLocked writes each board once and then the machine's own file.
func (m *Manager) saveBoardsLocked(boards ...string) error {
	done := make(map[string]bool, len(boards))
	for _, boardID := range boards {
		if boardID == "" || done[boardID] {
			continue
		}
		done[boardID] = true
		m.persistBoardLocked(boardID)
	}
	return m.persistConfigLocked()
}
