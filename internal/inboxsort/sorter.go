// Package inboxsort carries cards out of the inbox to the boards they belong
// on, by asking an agent.
//
// The inbox is collect-first: everything a source brings lands on one board
// and a person carries each card on (docs/sources.md, §14). Sorting is the
// same carrying done by an agent, and it is switched on by a person, because
// until then the decision is theirs. What the agent decides from is what a
// person would: each board's title and description — asked when the board is
// made — and what its cards are about, which the agent itself writes down per
// board (board_summary) and rewrites when the cards change.
//
// Like internal/sources it knows no board and no agent: both are interfaces the
// app hands in, so the package is tested with fakes and the board server is
// imported by boardadapter alone.
package inboxsort

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/artipop/pourquoipas/internal/apperr"
)

// BoardInfo is a board as the sorter reads it.
type BoardInfo struct {
	ID          string
	Title       string
	Icon        string
	Description string
	// Inbox marks the board cards are sorted out of.
	Inbox bool
}

// CardInfo is a card as the sorter reads it. Its text is read separately
// (Board.CardText): one query per card, asked only of the cards being sorted.
type CardInfo struct {
	ID     string
	Title  string
	Author string
	Link   string
}

// Board is everything the sorter does to the board.
type Board interface {
	// Boards lists every board a person works, the inbox among them and no
	// templates.
	Boards(ctx context.Context) ([]BoardInfo, error)
	// Cards lists a board's cards, newest first.
	Cards(ctx context.Context, boardID string) ([]CardInfo, error)
	CardText(ctx context.Context, cardID string) (string, error)
	// FileCard carries a card to a board and stands it in that board's first
	// column, without waking the column's automation: the card was put there
	// by the sorter, and nobody has read it yet.
	FileCard(ctx context.Context, cardID, boardID string) error
	SetDescription(ctx context.Context, boardID, text string) error
}

// Agent runs one turn of a named agent and returns its last message.
type Agent interface {
	Ask(ctx context.Context, agent, prompt string) (string, error)
}

// Sorter is the inbox's sorter: settings, the queue of arrivals and the one
// worker that drains it.
type Sorter struct {
	board Board
	store *Store
	path  string
	log   *slog.Logger

	mu       sync.Mutex
	settings Settings
	agent    Agent

	// work keeps two sorts from running at once: an arrival and the button
	// would otherwise both carry the same card, and both ask the agent.
	work     sync.Mutex
	arrivals chan string
}

// Tuning.
const (
	// arrivalWindow is how long the worker waits after an arrival for more:
	// a source delivers in batches, and one turn for the batch is one agent
	// start instead of ten.
	arrivalWindow = 3 * time.Second
	// turnTimeout bounds one agent turn.
	turnTimeout = 10 * time.Minute
	// cardsPerTurn keeps one prompt readable; a longer inbox is sorted in
	// several turns.
	cardsPerTurn = 20
	// cardTextLimit is how much of a card's text the agent is shown.
	cardTextLimit = 1500
	// summaryTitles is how many card titles a summary is written from, newest
	// first, and recentTitles how many of them the sorting prompt repeats.
	summaryTitles = 80
	recentTitles  = 8
	// summaryMinAge is how long a summary stands after its board's cards
	// changed. Every card the sorter files changes a board, and rewriting the
	// summary on every arrival would double the turns for a sentence that
	// barely moves.
	summaryMinAge = 30 * time.Minute
)

var (
	errNoAgent       = apperr.New("inboxsort.no-agents", "Agents are not available, so nobody can sort the inbox")
	errNoAgentChosen = apperr.New("inboxsort.no-agent", "No agent is chosen to sort the inbox")
	errNoInbox       = apperr.New("inboxsort.no-inbox", "There is no inbox board")
)

// New builds a sorter over a board and a store, reading its settings from
// path. The agent arrives later (SetAgent): the agent integration may be
// switched off, and then the sorter still shows its log and its summaries.
func New(board Board, store *Store, path string, log *slog.Logger) (*Sorter, error) {
	if log == nil {
		log = slog.Default()
	}
	settings, err := LoadSettings(path)
	if err != nil {
		return nil, fmt.Errorf("inbox sorting settings: %w", err)
	}
	return &Sorter{
		board:    board,
		store:    store,
		path:     path,
		log:      log,
		settings: settings,
		arrivals: make(chan string, 256),
	}, nil
}

// SetAgent supplies what runs a turn.
func (s *Sorter) SetAgent(a Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agent = a
}

// Settings returns what a person chose.
func (s *Sorter) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// SetSettings saves a person's choice. Switching sorting on with no agent named
// is refused here rather than accepted and silently doing nothing.
func (s *Sorter) SetSettings(next Settings) (Settings, error) {
	next.Agent = strings.TrimSpace(next.Agent)
	if next.Enabled && next.Agent == "" {
		return Settings{}, errNoAgentChosen
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := SaveSettings(s.path, next); err != nil {
		return Settings{}, err
	}
	s.settings = next
	return next, nil
}

// runner is the agent to ask and its name, or an error saying why there is none.
func (s *Sorter) runner() (Agent, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agent == nil {
		return nil, "", errNoAgent
	}
	if s.settings.Agent == "" {
		return nil, "", errNoAgentChosen
	}
	return s.agent, s.settings.Agent, nil
}

// Arrived tells the sorter a card has landed in the inbox. It never blocks and
// never fails: with sorting switched off the card simply waits for a person,
// and a full queue leaves it for the next sort — which is what the inbox is
// for anyway.
func (s *Sorter) Arrived(cardID string) {
	s.mu.Lock()
	on := s.settings.Enabled && s.settings.Agent != "" && s.agent != nil
	s.mu.Unlock()
	if !on || cardID == "" {
		return
	}
	select {
	case s.arrivals <- cardID:
	default:
		s.log.Warn("inboxsort: queue full, the card stays in the inbox", "card", cardID)
	}
}

// Start runs the worker until ctx is done.
func (s *Sorter) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *Sorter) run(ctx context.Context) {
	for {
		var first string
		select {
		case <-ctx.Done():
			return
		case first = <-s.arrivals:
		}
		batch := map[string]bool{first: true}
		timer := time.NewTimer(arrivalWindow)
	collect:
		for {
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case id := <-s.arrivals:
				batch[id] = true
			case <-timer.C:
				break collect
			}
		}
		if _, err := s.sort(ctx, batch); err != nil {
			s.log.Warn("inboxsort: arrivals were not sorted", "cards", len(batch), "err", err)
		}
	}
}

// Result is what one sort came to.
type Result struct {
	Moved  int `json:"moved"`
	Kept   int `json:"kept"`
	Failed int `json:"failed"`
}

// SortInbox sorts every card in the inbox now, whether or not sorting is
// switched on: it is the button, and pressing it is the person deciding.
func (s *Sorter) SortInbox(ctx context.Context) (Result, error) {
	return s.sort(ctx, nil)
}

// boardView is a candidate board with what the prompts say about it.
type boardView struct {
	BoardInfo
	cards   []CardInfo
	summary string
}

// sort carries the inbox's cards — only those in `only`, when it is given —
// to the boards the agent picks.
func (s *Sorter) sort(ctx context.Context, only map[string]bool) (Result, error) {
	s.work.Lock()
	defer s.work.Unlock()

	agent, agentName, err := s.runner()
	if err != nil {
		return Result{}, err
	}
	inbox, targets, err := s.targets(ctx)
	if err != nil {
		return Result{}, err
	}
	cards, err := s.board.Cards(ctx, inbox)
	if err != nil {
		return Result{}, fmt.Errorf("the inbox's cards: %w", err)
	}
	// Only what is still in the inbox: a person may have carried a card away
	// while it waited in the queue, and a card the sorter no longer sees is
	// not the sorter's to move.
	var pending []CardInfo
	for _, c := range cards {
		if only == nil || only[c.ID] {
			pending = append(pending, c)
		}
	}
	if len(pending) == 0 {
		return Result{}, nil
	}
	if len(targets) == 0 {
		return Result{}, apperr.New("inboxsort.no-boards", "There are no boards besides the inbox, so there is nowhere to carry cards")
	}

	views := s.readBoards(ctx, targets)
	s.refreshSummaries(ctx, agent, agentName, views, false)

	var res Result
	for start := 0; start < len(pending); start += cardsPerTurn {
		end := min(start+cardsPerTurn, len(pending))
		s.sortBatch(ctx, agent, agentName, views, pending[start:end], &res)
	}
	return res, nil
}

// targets is the inbox and every board a card may be carried to.
func (s *Sorter) targets(ctx context.Context) (string, []BoardInfo, error) {
	boards, err := s.board.Boards(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("the list of boards: %w", err)
	}
	var inbox string
	var targets []BoardInfo
	for _, b := range boards {
		if b.Inbox {
			inbox = b.ID
			continue
		}
		targets = append(targets, b)
	}
	if inbox == "" {
		return "", nil, errNoInbox
	}
	return inbox, targets, nil
}

// readBoards reads each candidate's cards, with the summary this machine last
// wrote for it.
func (s *Sorter) readBoards(ctx context.Context, targets []BoardInfo) []*boardView {
	have, err := s.store.Summaries()
	if err != nil {
		s.log.Warn("inboxsort: cannot read the summaries", "err", err)
	}
	views := make([]*boardView, 0, len(targets))
	for _, b := range targets {
		cards, err := s.board.Cards(ctx, b.ID)
		if err != nil {
			s.log.Warn("inboxsort: cannot read a board's cards", "board", b.ID, "err", err)
		}
		views = append(views, &boardView{BoardInfo: b, cards: cards, summary: have[b.ID].Text})
	}
	return views
}

// refreshSummaries rewrites the summary of every board whose cards changed, in
// one turn for all of them. A failure keeps the summaries there were: sorting
// by a summary an hour old is better than not sorting.
func (s *Sorter) refreshSummaries(ctx context.Context, agent Agent, agentName string, views []*boardView, force bool) error {
	have, err := s.store.Summaries()
	if err != nil {
		return err
	}
	now := time.Now()
	var stale []*boardView
	for _, v := range views {
		fp := fingerprint(v.cards)
		old, ok := have[v.ID]
		switch {
		case ok && old.Fingerprint == fp && !force:
			continue
		case ok && !force && now.Sub(old.UpdatedAt) < summaryMinAge:
			continue
		case len(v.cards) == 0:
			// Nothing to read: the board is its description alone, and saying
			// so costs no turn.
			v.summary = ""
			if err := s.store.SaveSummary(Summary{BoardID: v.ID, Fingerprint: fp, UpdatedAt: now}); err != nil {
				s.log.Warn("inboxsort: cannot save a summary", "board", v.ID, "err", err)
			}
			continue
		}
		stale = append(stale, v)
	}
	if len(stale) == 0 {
		return nil
	}

	keys := map[string]*boardView{}
	for i, v := range stale {
		keys[fmt.Sprintf("b%d", i+1)] = v
	}
	tctx, cancel := context.WithTimeout(ctx, turnTimeout)
	defer cancel()
	answer, err := agent.Ask(tctx, agentName, summaryPrompt(stale))
	if err != nil {
		s.log.Warn("inboxsort: summaries were not written", "err", err)
		return err
	}
	var written map[string]string
	if err := decodeJSON(answer, '{', &written); err != nil {
		s.log.Warn("inboxsort: the summaries answer is unreadable", "err", err)
		return err
	}
	for key, text := range written {
		v, ok := keys[strings.TrimSpace(key)]
		if !ok {
			continue
		}
		v.summary = strings.TrimSpace(text)
		if err := s.store.SaveSummary(Summary{
			BoardID: v.ID, Text: v.summary, CardCount: len(v.cards),
			Fingerprint: fingerprint(v.cards), UpdatedAt: now,
		}); err != nil {
			s.log.Warn("inboxsort: cannot save a summary", "board", v.ID, "err", err)
		}
	}
	return nil
}

// RefreshSummaries rewrites every board's summary now — the dialog's button,
// for a person who wants to read what the sorter thinks a board is about.
func (s *Sorter) RefreshSummaries(ctx context.Context) error {
	s.work.Lock()
	defer s.work.Unlock()
	agent, agentName, err := s.runner()
	if err != nil {
		return err
	}
	_, targets, err := s.targets(ctx)
	if err != nil {
		return err
	}
	return s.refreshSummaries(ctx, agent, agentName, s.readBoards(ctx, targets), true)
}

func said(m apperr.Message) *apperr.Message { return &m }

// agentSaid is the agent's reason, which is already in the conversation's
// language, or nothing.
func agentSaid(reason string) *apperr.Message {
	if reason == "" {
		return nil
	}
	return said(apperr.Plain(reason))
}

// decision is one entry of the agent's answer.
type decision struct {
	Card   string `json:"card"`
	Board  string `json:"board"`
	Reason string `json:"reason"`
}

// sortBatch asks about a handful of cards and carries each where the answer
// says. Cards and boards are shown to the agent under short keys (c1, b1)
// rather than ids: a 27-character id copied wrong is a card moved nowhere, and
// a key copied wrong is simply not found.
func (s *Sorter) sortBatch(ctx context.Context, agent Agent, agentName string, views []*boardView, cards []CardInfo, res *Result) {
	type shown struct {
		CardInfo
		text string
	}
	batch := make([]shown, 0, len(cards))
	for _, c := range cards {
		text, err := s.board.CardText(ctx, c.ID)
		if err != nil {
			s.log.Warn("inboxsort: cannot read a card's text", "card", c.ID, "err", err)
		}
		batch = append(batch, shown{CardInfo: c, text: text})
	}
	boardKeys := map[string]*boardView{}
	for i, v := range views {
		boardKeys[fmt.Sprintf("b%d", i+1)] = v
	}
	cardKeys := map[string]int{}
	prompted := make([]promptCard, 0, len(batch))
	for i, c := range batch {
		key := fmt.Sprintf("c%d", i+1)
		cardKeys[key] = i
		prompted = append(prompted, promptCard{Key: key, Title: c.Title, Author: c.Author, Link: c.Link, Text: c.text})
	}

	fail := func(err error) {
		for _, c := range batch {
			res.Failed++
			s.record(Decision{CardID: c.ID, CardTitle: c.Title, Outcome: OutcomeFailed, Reason: said(apperr.MessageOf(err))})
		}
	}
	tctx, cancel := context.WithTimeout(ctx, turnTimeout)
	defer cancel()
	answer, err := agent.Ask(tctx, agentName, sortPrompt(views, prompted))
	if err != nil {
		fail(err)
		return
	}
	var decisions []decision
	if err := decodeJSON(answer, '[', &decisions); err != nil {
		fail(apperr.Wrap(err, "inboxsort.bad-answer", "The agent's answer could not be read"))
		return
	}

	answered := map[int]bool{}
	for _, d := range decisions {
		i, ok := cardKeys[strings.TrimSpace(d.Card)]
		if !ok || answered[i] {
			continue
		}
		answered[i] = true
		c := batch[i]
		reason := strings.TrimSpace(d.Reason)
		target, ok := boardKeys[strings.TrimSpace(d.Board)]
		if !ok {
			res.Kept++
			s.record(Decision{CardID: c.ID, CardTitle: c.Title, Outcome: OutcomeKept, Reason: agentSaid(reason)})
			continue
		}
		if err := s.board.FileCard(ctx, c.ID, target.ID); err != nil {
			res.Failed++
			s.record(Decision{CardID: c.ID, CardTitle: c.Title, BoardID: target.ID, Outcome: OutcomeFailed,
				Reason: said(apperr.Say("inboxsort.move-failed", "The move failed: {error}", "error", err.Error()))})
			continue
		}
		res.Moved++
		s.record(Decision{CardID: c.ID, CardTitle: c.Title, BoardID: target.ID, Outcome: OutcomeMoved, Reason: agentSaid(reason)})
	}
	for i, c := range batch {
		if !answered[i] {
			res.Kept++
			s.record(Decision{CardID: c.ID, CardTitle: c.Title, Outcome: OutcomeKept, Reason: said(apperr.Say("inboxsort.no-answer", "The agent said nothing about this card"))})
		}
	}
}

func (s *Sorter) record(d Decision) {
	if err := s.store.Record(d); err != nil {
		s.log.Warn("inboxsort: cannot record a decision", "card", d.CardID, "err", err)
	}
}

// fingerprint names a board's cards as the summary saw them: which cards, and
// what they were called. A card retitled is a different board to summarise; a
// card whose text was edited is not, because the summary never read the text.
func fingerprint(cards []CardInfo) string {
	lines := make([]string, 0, len(cards))
	for _, c := range cards {
		lines = append(lines, c.ID+"\x00"+c.Title)
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// BoardState is a board as the sorting dialog shows it.
type BoardState struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Icon        string    `json:"icon,omitempty"`
	Description string    `json:"description"`
	Summary     string    `json:"summary"`
	SummaryAt   time.Time `json:"summaryAt,omitempty"`
	CardCount   int       `json:"cardCount"`
}

// State is everything the sorting dialog draws.
type State struct {
	Settings
	// Available says there is an agent integration to sort with at all.
	Available bool         `json:"available"`
	Boards    []BoardState `json:"boards"`
	Decisions []Decision   `json:"decisions"`
}

// State reads the dialog's contents.
func (s *Sorter) State(ctx context.Context) (State, error) {
	s.mu.Lock()
	state := State{Settings: s.settings, Available: s.agent != nil}
	s.mu.Unlock()

	_, targets, err := s.targets(ctx)
	if err != nil && !errors.Is(err, errNoInbox) {
		return State{}, err
	}
	have, err := s.store.Summaries()
	if err != nil {
		return State{}, err
	}
	state.Boards = make([]BoardState, 0, len(targets))
	for _, b := range targets {
		sum := have[b.ID]
		state.Boards = append(state.Boards, BoardState{
			ID: b.ID, Title: b.Title, Icon: b.Icon, Description: b.Description,
			Summary: sum.Text, SummaryAt: sum.UpdatedAt, CardCount: sum.CardCount,
		})
	}
	state.Decisions, err = s.store.Decisions(50)
	if err != nil {
		return State{}, err
	}
	if state.Decisions == nil {
		state.Decisions = []Decision{}
	}
	return state, nil
}

// SetBoardDescription writes a board's description — the dialog edits it
// beside the summary, because that is where somebody tuning the sorter reads
// both.
func (s *Sorter) SetBoardDescription(ctx context.Context, boardID, text string) error {
	if strings.TrimSpace(boardID) == "" {
		return errors.New("no board given")
	}
	return s.board.SetDescription(ctx, boardID, strings.TrimSpace(text))
}
