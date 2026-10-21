package sources

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/secrets"
)

// Manager owns the registry and runs the pipeline. It is the only thing outside
// this package needs: an entry point delivers items, the manager decides and
// writes.
type Manager struct {
	mu      sync.RWMutex
	cfg     Config
	cfgPath string

	store  *Store
	writer BoardWriter
	// items is the board asked what it already holds from a source. Optional,
	// and separate from writer so that a build with no board, and every test
	// that does not care, keeps working with this machine's table alone.
	items BoardItems
	log   *slog.Logger
	// catalog is the manifests read from <dataDir>/sources/manifests. Kept
	// beside the registry rather than in it: the registry is what a person
	// configured, and this is a directory that is read again on every start.
	catalog []Manifest
	// agents runs an agent source's turn, and ingestURL is where that agent
	// files what it found. Both are the app's to supply: this package keeps
	// working with the agent integration switched off.
	agents    AgentRunner
	ingestURL string

	// The running half: one goroutine per source that names a plugin, under a
	// context the app cancels, and what each of them is currently doing.
	rootCtx context.Context
	stop    context.CancelFunc
	wg      sync.WaitGroup
	dial    dialer
	status  map[string]*Status
	// running is the cancel of each source's loop, so one can be restarted
	// after it was added or edited without taking the others down.
	running map[string]context.CancelFunc
	// secrets is where the credentials a plugin has to present are kept. Not
	// where an inbound ingest token lives: that one is only ever checked, so it
	// is a hash on the entry itself.
	secrets secrets.Store
	// arrived is told about every card this pipeline puts in the inbox — the
	// inbox sorter's queue (internal/inboxsort), handed over by the app so this
	// package names nothing of it. Nil is nobody listening.
	arrived func(cardID string)
}

// NewManager builds a manager over a loaded registry. writer may be nil, which
// disables writing and is what a build with no board does.
func NewManager(cfg Config, cfgPath string, store *Store, writer BoardWriter, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{cfg: cfg, cfgPath: cfgPath, store: store, writer: writer, log: log}
}

// SetBoardItems supplies the board as the record of what a source has already
// brought. Without it that record is this machine's alone, and a board carried
// here from elsewhere has every one of its items brought again.
func (m *Manager) SetBoardItems(b BoardItems) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = b
}

// SetArrivals names who is told when a card lands in the inbox. Not a card the
// share dialog put on a board somebody picked: that one has arrived where it
// belongs.
func (m *Manager) SetArrivals(f func(cardID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.arrived = f
}

// boardWriteTimeout bounds one card write, so a stuck board cannot hold an
// incoming request open.
const boardWriteTimeout = 10 * time.Second

// Sources returns a snapshot of the registry.
func (m *Manager) Sources() []SourceEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]SourceEntry(nil), m.cfg.Sources...)
}

// Source returns one entry by name.
func (m *Manager) Source(name string) (SourceEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return findSource(m.cfg.Sources, name)
}

func findSource(list []SourceEntry, name string) (SourceEntry, bool) {
	for _, s := range list {
		if strings.EqualFold(s.Name, name) {
			return s, true
		}
	}
	return SourceEntry{}, false
}

// AddSource registers a new source.
func (m *Manager) AddSource(entry SourceEntry) (SourceEntry, error) {
	valid, err := entry.Validate()
	if err != nil {
		return SourceEntry{}, err
	}
	if err := m.insertEntry(valid); err != nil {
		return SourceEntry{}, err
	}
	// Outside the lock on purpose: this one writes to the board, and holding
	// the registry while waiting on board I/O would stall every reader of it.
	m.ensureInbox()
	// A source with a plugin is a process, and it starts now rather than at the
	// next launch: "add the source, then restart the app" is not a feature that
	// works. Does nothing for an ingest source, which has no process.
	m.Restart(valid.Name)
	return valid, nil
}

// EnsureSource registers a source the app itself needs and returns whatever
// entry now carries that name — the existing one if there is one, untouched.
//
// It exists for the share sheet, which is a source nobody asked for in the
// dialog: the first thing shared has to land somewhere, and asking a person to
// register the share dialog before they can share anything would be a setup step
// for a feature whose whole point is that there is no setup. Not overwriting is
// the other half: once the entry exists it is the person's, rules and all.
func (m *Manager) EnsureSource(entry SourceEntry) (SourceEntry, error) {
	if existing, ok := m.Source(entry.Name); ok {
		return existing, nil
	}
	valid, err := entry.Validate()
	if err != nil {
		return SourceEntry{}, err
	}
	if err := m.insertEntry(valid); err != nil {
		// Lost the race with another caller: whoever won wrote the same entry,
		// and theirs is as good as ours.
		if existing, ok := m.Source(entry.Name); ok {
			return existing, nil
		}
		return SourceEntry{}, err
	}
	m.ensureInbox()
	return valid, nil
}

func (m *Manager) insertEntry(valid SourceEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := findSource(m.cfg.Sources, valid.Name); exists {
		return apperr.New("sources.duplicate", "There is already a source named {name}", "name", valid.Name)
	}
	m.cfg.Sources = append(m.cfg.Sources, valid)
	return m.persistLocked()
}

// ensureInbox makes the inbox board exist as soon as there is a source, so
// somebody who has just registered one can see where its cards will land
// instead of finding out when the first item arrives.
//
// Best-effort: a board that could not be made now is made again by the first
// delivery, which is what actually has to succeed, and refusing to register
// the source over it would be a worse answer than a line in the log.
func (m *Manager) ensureInbox() {
	if m.writer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), boardWriteTimeout)
	defer cancel()
	if _, err := m.writer.InboxBoard(ctx); err != nil {
		m.log.Warn("sources: could not make the inbox board", "err", err)
	}
}

// UpdateSource replaces an existing source, matched by name.
func (m *Manager) UpdateSource(entry SourceEntry) (SourceEntry, error) {
	valid, err := entry.Validate()
	if err != nil {
		return SourceEntry{}, err
	}
	if err := m.replaceEntry(valid); err != nil {
		return SourceEntry{}, err
	}
	m.ensureInbox()
	// The entry it was running under is gone: whatever changed — the interval,
	// the config a plugin is started with, whether it is enabled at all — takes
	// effect now.
	m.Restart(valid.Name)
	return valid, nil
}

func (m *Manager) replaceEntry(valid SourceEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.cfg.Sources {
		if strings.EqualFold(s.Name, valid.Name) {
			m.cfg.Sources[i] = valid
			return m.persistLocked()
		}
	}
	return apperr.New("sources.not-found", "Source {name} was not found", "name", valid.Name)
}

// IssueToken mints a new ingest token for a source and returns it — the only
// moment it can be read, since only its hash is kept.
//
// It writes the hash without going through UpdateSource on purpose: an entry
// that is saved restarts whatever is running under it, and a token is reissued
// *by* the thing that is running (an agent source mints one per turn), which
// would restart it inside its own poll.
func (m *Manager) IssueToken(name string) (string, error) {
	token, err := GenerateToken()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.cfg.Sources {
		if strings.EqualFold(s.Name, name) {
			m.cfg.Sources[i].TokenHash = HashToken(token)
			if err := m.persistLocked(); err != nil {
				return "", err
			}
			return token, nil
		}
	}
	return "", apperr.New("sources.not-found", "Source {name} was not found", "name", name)
}

// RemoveSource deletes a source and everything it remembered.
func (m *Manager) RemoveSource(name string) error {
	// The process goes with the entry, and it is stopped before the lock: the
	// stop takes the same lock, and a deferred stop under a held one is a
	// deadlock — which is exactly how this was written the first time.
	m.stopSource(name)

	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.cfg.Sources {
		if strings.EqualFold(s.Name, name) {
			m.cfg.Sources = append(m.cfg.Sources[:i], m.cfg.Sources[i+1:]...)
			if err := m.persistLocked(); err != nil {
				return err
			}
			if m.store != nil {
				return m.store.ForgetSource(s.Name)
			}
			return nil
		}
	}
	return apperr.New("sources.not-found", "Source {name} was not found", "name", name)
}

// persistLocked writes the registry. Like the ACP one, it does nothing without
// a path, which is what tests run with.
func (m *Manager) persistLocked() error {
	if m.cfgPath == "" {
		return nil
	}
	return SaveConfig(m.cfgPath, m.cfg)
}

// Events returns a source's log, newest first.
func (m *Manager) Events(source string, limit int) ([]EventRecord, error) {
	if m.store == nil {
		return nil, nil
	}
	return m.store.Events(source, limit)
}

// Result is what one delivery came to, in the terms the caller reports back to
// whoever sent the items.
type Result struct {
	Created   int `json:"created"`
	Commented int `json:"commented"`
	Dropped   int `json:"dropped"`
	Skipped   int `json:"skipped"` // already known, unchanged
	Failed    int `json:"failed"`
}

// Deliver runs items through the pipeline of one source: deduplicate, decide,
// write. It is the single entry point — a pushed item and a polled one take the
// same path, which is what keeps them from growing two sets of bugs.
func (m *Manager) Deliver(ctx context.Context, sourceName string, items []Item) (Result, error) {
	entry, ok := m.Source(sourceName)
	if !ok {
		return Result{}, apperr.New("sources.not-found", "Source {name} was not found", "name", sourceName)
	}
	if !entry.Enabled {
		return Result{}, apperr.New("sources.off", "Source {name} is switched off", "name", entry.Name)
	}
	if m.writer == nil {
		return Result{}, fmt.Errorf("the board is not available")
	}

	var res Result
	scope := newDelivery()
	for _, it := range items {
		if err := m.deliverOne(ctx, scope, entry, it, &res); err != nil {
			res.Failed++
			m.log.Warn("sources: could not handle an item",
				"source", entry.Name, "item", it.ExternalID, "err", err)
			m.record(EventRecord{Source: entry.Name, ExternalID: it.ExternalID,
				Outcome: OutcomeFailed, Detail: said(err)})
		}
	}
	return res, nil
}

// delivery is what stays true for the length of one batch: which board is the
// inbox, asked once rather than once per item.
type delivery struct {
	inbox string
}

func newDelivery() *delivery { return &delivery{} }

// inboxBoard is the inbox's id, asked of the board the first time a batch
// needs it.
func (m *Manager) inboxBoard(ctx context.Context, d *delivery) (string, error) {
	if d.inbox != "" {
		return d.inbox, nil
	}
	bctx, cancel := context.WithTimeout(ctx, boardWriteTimeout)
	defer cancel()
	id, err := m.writer.InboxBoard(bctx)
	if err != nil {
		return "", fmt.Errorf("the inbox board: %w", err)
	}
	d.inbox = id
	return id, nil
}

func (m *Manager) deliverOne(ctx context.Context, d *delivery, entry SourceEntry, it Item, res *Result) error {
	state, cardID, err := m.stateOf(ctx, d, entry, it)
	if err != nil {
		return err
	}
	switch {
	case state == ItemSeen:
		// The item is exactly what was recorded. Nothing is logged: a source
		// reports its whole state on every poll, and logging that would bury
		// the lines that matter.
		res.Skipped++
		return nil
	case state == ItemChanged && cardID != "":
		return m.updateCard(ctx, entry, it, cardID, res)
	}
	return m.createCard(ctx, d, entry, it, res)
}

// updateCard is what a changed item does to the card it already has: a comment,
// and never a rewrite of the description, which a person may have edited.
func (m *Manager) updateCard(ctx context.Context, entry SourceEntry, it Item, cardID string, res *Result) error {
	if entry.Update == UpdateIgnore {
		res.Skipped++
		return m.remember(entry.Name, it, cardID)
	}
	wctx, cancel := context.WithTimeout(ctx, boardWriteTimeout)
	defer cancel()
	if err := m.writer.AddComment(wctx, cardID, updateComment(it)); err != nil {
		return fmt.Errorf("comment on card %s: %w", cardID, err)
	}
	res.Commented++
	m.record(EventRecord{Source: entry.Name, ExternalID: it.ExternalID,
		Outcome: OutcomeCommented, CardID: cardID})
	return m.remember(entry.Name, it, cardID)
}

func (m *Manager) createCard(ctx context.Context, d *delivery, entry SourceEntry, it Item, res *Result) error {
	rule, matched := FirstMatch(entry.Rules, it)
	switch {
	case matched && rule.Then == ActionDrop:
		res.Dropped++
		m.record(EventRecord{Source: entry.Name, ExternalID: it.ExternalID,
			Rule: rule.Name, Outcome: OutcomeDropped})
		// Remembered, so a source that keeps reporting it does not keep
		// deciding it.
		return m.remember(entry.Name, it, "")
	case matched && rule.Then == ActionComment:
		// There is no card to comment on: the item is new. Treated as a drop,
		// and said so, rather than silently doing nothing.
		res.Dropped++
		m.record(EventRecord{Source: entry.Name, ExternalID: it.ExternalID, Rule: rule.Name,
			Outcome: OutcomeDropped, Detail: say(apperr.Say("sources.comment-no-card", "The rule comments, but there is no card yet"))})
		return m.remember(entry.Name, it, "")
	case !matched && entry.Noisy:
		// A stream of notifications is mostly noise: there a rule is a
		// subscription, and everything else is deliberately dropped.
		res.Dropped++
		m.record(EventRecord{Source: entry.Name, ExternalID: it.ExternalID, Outcome: OutcomeDropped})
		return m.remember(entry.Name, it, "")
	}

	spec := CardFor(rule, it)
	outcome := OutcomeCreated
	if !matched {
		// Nothing claimed it, and this source is not noisy: it goes to the
		// inbox rather than being lost. A lost item is what makes an
		// integration impossible to debug.
		spec = CardFor(Rule{}, it)
		outcome = OutcomeInbox
	}
	// Where the card came from is not a property: the card is authored by the
	// source, which is the board's own answer and the one the inbox groups by,
	// and a property saying it again would be a second answer to one question.
	// The way back to the original travels as CardSpec.URL, which the board
	// side puts wherever that board keeps a link — this package knows no
	// property names of its own.
	spec.Source = entry.Name

	boardID, err := m.boardFor(ctx, d, entry, it)
	if err != nil {
		return err
	}

	wctx, cancel := context.WithTimeout(ctx, boardWriteTimeout)
	defer cancel()
	cardID, err := m.writer.CreateCard(wctx, boardID, spec)
	if err != nil {
		return fmt.Errorf("creating the card: %w", err)
	}
	// Remembered at once: the card exists, and losing track of it here would
	// create a second one on the next delivery.
	if err := m.remember(entry.Name, it, cardID); err != nil {
		return err
	}
	res.Created++
	m.record(EventRecord{Source: entry.Name, ExternalID: it.ExternalID,
		Rule: rule.Name, Outcome: outcome, CardID: cardID})
	if strings.TrimSpace(it.BoardID) == "" {
		m.mu.RLock()
		arrived := m.arrived
		m.mu.RUnlock()
		if arrived != nil {
			arrived(cardID)
		}
	}
	return nil
}

// boardFor is where one item's card goes: the inbox, or the board the item
// names when the entry allows an item to name one.
//
// A board named by an item that may not name one is refused rather than
// ignored. Ignoring it would write the card to the inbox and report success,
// and the person who picked a board would find their card somewhere else —
// which is the failure this whole subsystem is meant not to have.
func (m *Manager) boardFor(ctx context.Context, d *delivery, entry SourceEntry, it Item) (string, error) {
	chosen := strings.TrimSpace(it.BoardID)
	if chosen == "" {
		return m.inboxBoard(ctx, d)
	}
	if !entry.PickBoard {
		return "", fmt.Errorf("source %q does not choose a board, and the item named %q", entry.Name, chosen)
	}
	return chosen, nil
}

// stateOf answers whether this item has been brought before, and onto which
// card. This machine's table is asked first because it is the fast answer and
// the usual one; the board is asked only when the table has never heard of the
// item, which is what happens on a board that arrived from somewhere else.
//
// The board's answer is written into the table, so it is asked once per item
// and not once per poll.
func (m *Manager) stateOf(ctx context.Context, d *delivery, entry SourceEntry, it Item) (ItemState, string, error) {
	if m.store != nil {
		state, cardID, err := m.store.StateOf(entry.Name, it.ExternalID, it.Version)
		if err != nil {
			return state, cardID, err
		}
		if state != ItemNew {
			return state, cardID, nil
		}
	}
	m.mu.RLock()
	items := m.items
	m.mu.RUnlock()
	if items == nil || it.ExternalID == "" {
		return ItemNew, "", nil
	}
	boardID, err := m.boardFor(ctx, d, entry, it)
	if err != nil {
		return ItemNew, "", err
	}
	bctx, cancel := context.WithTimeout(ctx, boardWriteTimeout)
	cardID, version, ok, err := items.CardBySourceItem(bctx, boardID, entry.Name, it.ExternalID)
	cancel()
	if err != nil {
		// The board could not be asked, so this machine does not know. Treating
		// that as "never seen" would make a second card; treating it as an
		// error leaves the item for the next poll, which is the poll's job.
		return ItemNew, "", fmt.Errorf("asking the board about item %s: %w", it.ExternalID, err)
	}
	if !ok {
		return ItemNew, "", nil
	}
	if version != it.Version {
		// Changed: what happens next is a comment on that card, and only once
		// it lands is the item remembered — the same ordering the rest of the
		// pipeline keeps, so a failed write replays instead of being lost.
		return ItemChanged, cardID, nil
	}
	if err := m.remember(entry.Name, it, cardID); err != nil {
		m.log.Warn("sources: could not remember an item the board knows", "source", entry.Name, "err", err)
	}
	return ItemSeen, cardID, nil
}

func (m *Manager) remember(source string, it Item, cardID string) error {
	if m.store == nil {
		return nil
	}
	return m.store.RememberItem(source, it.ExternalID, it.Version, cardID)
}

func (m *Manager) record(r EventRecord) {
	if m.store == nil {
		return
	}
	if err := m.store.AppendEvent(r); err != nil {
		m.log.Warn("sources: could not record an event", "source", r.Source, "err", err)
		return
	}
	if err := m.store.PruneEvents(r.Source, eventsKept); err != nil {
		m.log.Warn("sources: cannot prune the log", "source", r.Source, "err", err)
	}
}

// eventsKept is how much of a source's log survives. The log answers "why did
// nothing happen" about recent deliveries; nobody reads it as an archive.
const eventsKept = 500
