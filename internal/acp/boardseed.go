package acp

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// A board's automation lives on the board. The columns it runs and the routes
// cards take across it are two of the board's own properties, in the board
// database, beside everything else that is the board's — which is why a board
// made from the the developer template template works before anything is configured, and
// why a template can carry automation at all.
//
// The registry in memory is what the engine reads on every card move — a board
// read per move would be absurd — but it is a cache of what the boards say, and
// every edit is written through to the board it belongs to
// (persistBoardLocked).
//
// A template's automation is still a **seed** for a board made from it: it is
// imported once, tagged with the new board's id, and from then on that board
// owns it. A template that changes later does not rewrite a board somebody has
// since adjusted.

// Board properties the template writes its automation into. BoardPropSetup is
// not automation but the questions the board needs answered before any of it
// can run — see setup.go; it is read there rather than adopted here, because it
// is about this machine and nothing about it belongs in the registry.
//
// The prefix is the app's, not the agent integration's, because a route does
// not presuppose an agent: of the twelve triggers an edge can wait on, nine
// come from git, GitHub or the board itself, and a ColumnActionNone stage runs
// nothing at all. A board can carry a working route with no agent in it.
// The keys this application keeps on a board, in the board's own `properties`
// map. Bare names on purpose: the map is shared with the board server's own
// keys — of which there is exactly one, `trackingTemplateId` — and a prefix
// here would only be the product's name written into stored data, where a
// rename becomes a migration.
const (
	BoardPropColumns = "columns"
	BoardPropFlows   = "flows"
	BoardPropSetup   = "setup"
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
	//
	// Written here rather than by the page, unlike the folder and branch
	// records: the columns already carry the property they were bound to, so
	// the board can be told what it is the first time its automation is saved.
	BoardPropColumnProperty = "columnProperty"
	// BoardPropOutcome is the id of the select property this app writes the
	// outcome of a stage into — every stage, without anybody declaring it.
	//
	// It exists because how a stage ended used to be expressible only as *where
	// the card went*: a board grew a «Проверено», a «Не прошло» and a
	// «Заблокировано» column whose whole content was one fact about the stage
	// before them, and that fact then read as a place where work happens — on
	// the board and on the canvas alike. An outcome belongs to the card, like
	// the branch and the folder, and a column is for work.
	//
	// Written by the engine and never declared, which is what makes it
	// "служебное": every node produces one, so no node should have to say so.
	BoardPropOutcome = "outcomeProperty"
)

// The values that property takes, and there are two. A third was tried —
// «заблокировано», for a check that could not run — and taken out: what a card
// carries here is whether the step worked, and every question anybody asks of
// it is that question. «Заблокировано» is not a third answer to it, it is a
// reason, and the reason is on the card's route strip already (stallCardSoft),
// where it can be a sentence instead of a word.
//
// Binary is also what makes it cheap to answer by hand: a reviewer marking a
// card picks between two things and not out of a list.
const (
	OutcomePassed = "прошло"
	OutcomeFailed = "не прошло"
)

// OutcomePropertyTitle is what this app calls that field when it makes one. A
// name and not an id, unlike everything else that finds a field of ours — but
// this one is never *found* by it: it is what a shipped route's condition asks
// about, and a condition names a property the way a person reading the canvas
// does. A board whose owner renamed the field renames the condition with it,
// which is the same bargain every condition on the canvas takes.
const OutcomePropertyTitle = "Исход"

// OutcomeValue is what a stage's outcome is called on the card. Empty for
// anything that is not a stage's own outcome — a merged branch is news about
// the world, not a verdict on the work.
//
// A check that could not run answers «не прошло» like anything else that did
// not pass: the card is asked whether the step worked, and it did not. Why is
// the stall's business.
func OutcomeValue(trigger string) string {
	switch trigger {
	case OutcomeSuccess:
		return OutcomePassed
	case OutcomeFailure:
		return OutcomeFailed
	default:
		return ""
	}
}

// The names these keys had before, still read because every board made until
// now carries them. They are never written: reading a board carries what it
// says over to the current names and drops the old ones in the same patch, so
// a board migrates the first time anything touches it and nothing has to walk
// the database looking for boards to fix.
//
// BoardPropPrompt is the odd one and is listed for honesty rather than for
// history: the prompt was in config.json until this key existed, so no released
// build ever wrote `acpPrompt`. `acp` was also the one defensible prefix here —
// this really is what an agent is told — and it was renamed anyway so that a
// board does not carry four keys under one prefix and a fifth under another.
// boardProp reads one of this app's own keys off a board, under whichever name
// that board happens to carry it.
func boardProp(props map[string]any, key string) (any, bool) {
	if v, ok := props[key]; ok && v != nil {
		return v, true
	}
	return nil, false
}

// BoardMeta is the board's own properties — where a board keeps the automation
// it runs and a template the automation it ships — and whether the board is
// itself a template. Optional: without it a board brings nothing and keeps
// nothing, and the registry falls back to the file it used to live in.
type BoardMeta interface {
	BoardProperties(ctx context.Context, boardID string) (map[string]any, error)
	// SetBoardProperties writes the named properties onto the board and removes
	// the named ones, leaving everything else alone. This is how a board's
	// automation is saved: the board is where it belongs, so the board is what
	// is patched. remove is how a key that has been renamed stops existing —
	// writing the new name without it would leave two answers on the board.
	SetBoardProperties(ctx context.Context, boardID string, props map[string]any, remove []string) error
	// BoardColumnOptions is the board's own columns: the property they live on
	// and every option of it. It is what binds a stage written before stages
	// recorded an option id to the option it always meant, once, so nothing has
	// to go on matching a column by its name (contradiction 5).
	BoardColumnOptions(ctx context.Context, boardID string) ([]Column, error)
	// CardProperties is the board's card fields, for the one thing a stage's
	// declared outputs cannot say by themselves: what a value may be. A select
	// takes one of its option names and nothing else, and until this the agent
	// found that out by being refused.
	CardProperties(ctx context.Context, boardID string) ([]CardProperty, error)
	// IsBoardTemplate says this board is one to copy rather than to work in.
	// Nothing runs in a template — no card moves in it, no session starts from
	// it — so nothing about this machine is asked for on its behalf.
	IsBoardTemplate(ctx context.Context, boardID string) (bool, error)
}

// CardProperty is one field of a board's cards: what it is called, what kind of
// value it takes, and — for a closed set — which values those are. Named rather
// than identified, because that is how a stage declares a write and how
// finish_work delivers one.
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
