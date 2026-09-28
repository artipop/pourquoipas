package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/artipop/xciii/internal/apperr"
	"github.com/artipop/xciii/internal/dokku"
	"github.com/artipop/xciii/internal/secrets"
)

// Manager owns all agent sessions: it consumes board events, enforces limits
// and policies, and reports results back to the board and the UI.
type Manager struct {
	cfg     Config
	cfgMu   sync.RWMutex // guards the UI-mutable parts of cfg (Workdirs, Agents, BoardPrompts)
	cfgPath string       // where registry edits are persisted; empty in tests
	store   *Store
	// vault is where a credential this app presents to somebody else is kept —
	// today the GitHub token for pull-request polling. Nil until SetSecrets,
	// and nil for good on a build with no store; both read as "no token".
	vault  secrets.Store
	writer BoardWriter
	reader BoardReader // optional; enables opening a console on a card
	users  BoardUsers  // optional; enables assigning cards to an agent
	meta   BoardMeta   // optional; lets a board bring its own columns and routes
	// cards is where a card's own route position is kept — on the card, so it
	// travels with it. Optional; without it the position lives in this
	// machine's store alone and stays behind when the board moves.
	// routing is how a board or a card is matched to the source it lives on,
	// once there is more than one (boardsource.go). Empty and unused on every
	// install that works its own board.
	routing boardRouting
	ui      UIEmitter
	log     *slog.Logger
	tr      *Tracer

	mu     sync.Mutex
	active map[string]*Session // session ID → session
	byCard map[string]*Session // card ID → live (non-terminal) session
	// planning are the conversations of the planning dialog (planning.go).
	planning map[string]*planningChat
	// moved remembers the cards an agent has just moved itself (cardplace.go).
	moved agentMoves
	// Waits the route raises on cards where nothing runs (cardwait.go). Their
	// own lock, for the same reason questions have one: an ack arrives on a
	// click and must not queue behind a session starting.
	stageMu   sync.Mutex
	cardWaits map[string]Attention // card ID → the route handing it to a person (cardwait.go)
	// changesWriteFailed keeps the "no card_changes table" warning to one line
	// a run — see saveCardChanges.
	changesWriteFailed sync.Once

	// acked is which waits a person has already been told about, so that being
	// told is a thing that happens once (attentionack.go). Its own lock: it is
	// written from a click and read from every emit.
	acked ackedWaits

	seededMu sync.Mutex
	seeded   map[string]bool // boards whose own settings have been imported

	// switchedOff is the app-wide AI switch (SetSwitchedOff). Nil means on.
	switchedOff func() bool

	// questions are what agents are waiting to hear back on: one entry per open
	// question, keyed by its id (question.go). Its own lock — a question is
	// registered from an agent's inbound request and answered from the UI, and
	// neither should queue behind whatever holds mu.
	questionsMu sync.Mutex
	questions   map[string]*pendingQuestion

	// grants are the open permissions to write to a board through the board
	// tools (boardtools.go), one per agent run, and origin is the address the
	// tool server reaches us at. Their own lock: a tool call arrives on an
	// HTTP handler and must not queue behind a session starting.
	grantsMu sync.RWMutex
	grants   map[string]BoardGrant
	origin   string

	// What an agent says it can be configured with, keyed by how it is
	// launched. Asking costs an agent startup, and the dialog asks whenever a
	// form is opened. See capabilities.go.
	optionsMu    sync.Mutex
	optionsCache map[string][]AgentOption

	sem chan struct{}
	// rootCtx is the running manager's, and it is nil until Start. What reads
	// the board is not always behind Start any more — resolving a card's folder
	// asks the board which field holds it — so derive from it through rootOr
	// rather than from the field.
	rootCtx context.Context
	stop    context.CancelFunc
	wg      sync.WaitGroup
}

// rootOr is the manager's context, or a background one before it is running.
// Deriving a timeout from a nil parent panics, and a read that happens to be
// the first thing a manager does must not take the app down.
func (m *Manager) rootOr() context.Context {
	if m.rootCtx == nil {
		return context.Background()
	}
	return m.rootCtx
}

// SetBoardReader supplies on-demand card reads, which the "open a console on
// this card" path needs. Optional: without it, sessions start only on a move.
func (m *Manager) SetBoardReader(r BoardReader) { m.reader = r }

// SetBoardUsers supplies account provisioning, which "assign a card to an
// agent" needs. Optional: without it a card can only reach an agent through
// its column's crew.
func (m *Manager) SetBoardUsers(u BoardUsers) { m.users = u }

// NewManager wires the manager. cfgPath is where folder-registry edits are
// persisted (may be empty in tests). Call Start to begin consuming events.
func NewManager(cfg Config, cfgPath string, st *Store, w BoardWriter, ui UIEmitter, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	maxConc := cfg.MaxConcurrent
	if maxConc <= 0 {
		maxConc = 1
	}
	tr := newTracer(cfg, log)
	if tr.Enabled() {
		ui = &tracingEmitter{inner: ui, tr: tr}
	}
	m := &Manager{
		cfg:     cfg,
		cfgPath: cfgPath,
		store:   st,
		writer:  w,
		ui:      ui,
		log:     log,
		tr:      tr,
		active:  make(map[string]*Session),
		byCard:  make(map[string]*Session),
		sem:     make(chan struct{}, maxConc),
	}
	return m
}

// SetSecrets supplies the credential store. Optional: without one, a token can
// still arrive through the environment, and most folders need none at all.
func (m *Manager) SetSecrets(v secrets.Store) {
	m.vault = v
}

// SetSwitchedOff supplies the app-wide AI switch. While it answers true
// nothing starts by itself: a card moved between columns is only moved, no
// route advances on what a folder did, and no agent source polls. Whatever
// was already running goes on to its end; a person switching the AI off is
// saying "no more", not "throw away what an agent is halfway through".
func (m *Manager) SetSwitchedOff(off func() bool) { m.switchedOff = off }

func (m *Manager) aiSwitchedOff() bool {
	return m.switchedOff != nil && m.switchedOff()
}

// Start recovers interrupted sessions and launches the trigger loop.
func (m *Manager) Start(ctx context.Context, events BoardEvents) error {
	m.rootCtx, m.stop = context.WithCancel(ctx)

	// Probe the fallback kind's adapter only when the empty registry would use
	// it; registered agents resolve their own at run time.
	if len(m.cfg.Agents) == 0 && m.cfg.AgentMode != agentModeCommand {
		kind := firstNonEmpty(m.cfg.AgentMode, AgentKindClaude)
		if _, err := m.adapterArgv(kind, ""); err != nil {
			m.log.Warn("acp: the fallback agent cannot be started yet", "kind", kind, "err", err)
		}
	}

	// A hand-edited config is never validated, so what the editor would have
	// refused only surfaces when a server fails to start. Say it now instead.
	for _, a := range m.cfg.Agents {
		if _, err := validateAgent(a); err != nil {
			m.log.Warn("acp: agent is configured in a way that will not work", "agent", a.Name, "err", err)
		}
	}

	// Before anything reads a registry: whichever side holds it. A database
	// that already has entries is the truth; an empty one beside a populated
	// settings file is the install that has just upgraded, and its file is
	// carried in (registrymove.go). Failing here is not a reason to refuse to
	// start — the registries are then whatever the file says, which is what
	// every previous version did.
	m.cfgMu.Lock()
	if err := m.loadRegistriesLocked(); err != nil {
		m.log.Error("acp: could not settle the registries", "err", err)
	}
	m.cfgMu.Unlock()

	// Before anything can edit either side: whatever automation the file still
	// carries goes onto the boards that own it (boardseed.go).

	// The accounts the registry is named by. Registering an agent makes its
	// own from now on, so this is only ever the catch-up for a registry that
	// predates that — and for the agents named in Russian, who folded to an
	// empty username and got none at all.
	m.ensureAgentAccounts()

	// The ids a card points its folder at. Given here for a registry written
	// before they existed, and before anything can offer a folder to a board:
	// the board's option is created under the id.
	m.ensureWorkdirIDs()

	m.recover()
	// Worktrees whose directory has gone, pruned in the folders that could have
	// one. The registry is the list now; it used to be the whitelist, which was
	// about a card naming a path and never about where worktrees live.
	PruneStale(m.rootCtx, m.workdirPaths())

	ch, err := events.Subscribe(m.rootCtx)
	if err != nil {
		return fmt.Errorf("subscribe to board events: %w", err)
	}
	m.wg.Add(1)
	go m.triggerLoop(ch)
	return nil
}

// recover marks sessions left non-terminal by a previous run as failed.
func (m *Manager) recover() {
	stale, err := m.store.StaleSessions()
	if err != nil {
		m.log.Error("acp: recovery query failed", "err", err)
		return
	}
	for _, r := range stale {
		if err := m.store.SetSessionStatus(r.ID, StatusFailed, "interrupted by an app restart"); err != nil {
			m.log.Warn("acp: recovery update failed", "session", r.ID, "err", err)
			continue
		}
		m.commentCard(r.CardID, "Сессия агента была прервана перезапуском приложения.")
	}
}

// startOptions are the ways a session can differ from a plain card task.
type startOptions struct {
	// agent is who works it, resolved by the caller from the card's assignee.
	// Empty leaves the resolution to startSession, which is what a deploy
	// started from the card's own button gets.
	agent AgentEntry
	// deploy makes this a deploy session: it resolves a Dokku target, is given
	// the dokku MCP tools and gets the deploy prompt instead of the card task.
	deploy bool
	// deployOverride pins the target by the registry entry's id, because a name
	// is what somebody renames.
	deployOverride string
	// folderName picks a folder explicitly, for a start from the card that does
	// not say which one it is about.
	folderName string
}

// StartSessionForEvent creates and launches a session for a validated trigger
// event. Callers must have passed idempotency/liveness checks.
func (m *Manager) StartSessionForEvent(ev CardMoved) (*Session, error) {
	return m.startSession(ev, startOptions{})
}

// startSession is the shared launch path: every session — a card's task, a
// deploy, a browser test — is started here, runs its one turn and ends.
func (m *Manager) startSession(ev CardMoved, opts startOptions) (*Session, error) {
	// The last line of the switch: a stage queued behind a busy one, or an
	// edge taken on something that was already in flight, stops here.
	if m.aiSwitchedOff() {
		return nil, errSwitchedOff
	}
	// Asked before anything is resolved: a card somebody took for themselves is
	// theirs, and there is no point working out which folder an agent would not
	// be using. A deploy is unaffected — that is machine work, not the
	// assignee's. Somebody who wants an agent to work it assigns the agent,
	// which humanAssignee reads as the opposite answer.
	if !opts.deploy {
		m.cfgMu.RLock()
		known := append([]AgentEntry(nil), m.cfg.Agents...)
		m.cfgMu.RUnlock()
		if who := humanAssignee(ev, known); who != "" {
			return nil, AssignedToHumanError{Who: who}
		}
	}

	workdirPath, err := m.resolveWorkdir(ev)
	if opts.folderName != "" {
		// An explicit choice wins: the console offers one exactly when the card
		// itself does not say which folder it is about.
		workdirPath, err = m.resolveNamedWorkdir(opts.folderName)
	}
	// No folder is an answer, not a failure: the folder is asked on the card,
	// and a card nobody has given one is worked in drafts of its own. Not a
	// deploy — it publishes a branch, and drafts have none.
	var noFolder errNoWorkdir
	if errors.As(err, &noFolder) && !opts.deploy {
		workdirPath, err = m.cardDrafts(ev.BoardID, ev.CardID)
	}
	if err != nil {
		return nil, err
	}
	deploy, deployBranch, err := m.resolveDeploy(ev, workdirPath, opts.deploy, opts.deployOverride)
	if err != nil {
		return nil, err
	}
	sessionID := uuid.NewString()
	// Where a deploy's own outcome is recorded (deployreport.go).
	artifacts, err := m.artifactsDir(sessionID)
	if err != nil {
		return nil, err
	}
	// Who works it: the caller's answer, which is the card's assignee wherever
	// a move started this (assignedAgent). A deploy started from the card's own
	// button names nobody, and falls back to the registry.
	agent := opts.agent
	if agent.Name == "" {
		resolved, busy, resolveErr := m.resolveSessionAgent(ev, nil)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if busy {
			return nil, apperr.New("agents.busy", "The agent is busy with another card")
		}
		agent = resolved
	}

	net, err := m.resolveNetwork(agent)
	if err != nil {
		return nil, err
	}
	// An ordinary folder is one working copy with nothing to hold it, and two
	// agents must never share it (spec §7): reject while another live session
	// uses it. A repository needs none of this — a copy per card cannot
	// collide, and a branch in the folder itself is held by the card that has
	// it (folderIsFree). A deploy session is exempt for the same reason a
	// planning one is: it only pushes an existing branch and never touches the
	// checkout.
	mode := m.WorkModeFor(ev.BoardID, workdirPath)
	worktreeAvailable := mode != WorkModePlain
	if !worktreeAvailable && !opts.deploy {
		m.mu.Lock()
		var busyCard string
		for _, other := range m.active {
			// A planning session only reads, so it neither claims the working
			// copy nor keeps a card's session out of it.
			if other.WorkdirPath == workdirPath && !other.Planning {
				busyCard = other.CardID
				break
			}
		}
		m.mu.Unlock()
		if busyCard != "" {
			return nil, apperr.Kind(errWorkdirBusy, "folders.busy-session", "Another card's session ({card}) is already working in the folder {dir}: wait for it to finish", "card", busyCard, "dir", workdirPath)
		}
	}
	// A branch in the folder itself is held by one card until its branch is
	// merged, so the answer is known before a session is made rather than
	// after — a card that cannot start must not leave a cancelled run behind.
	if mode == WorkModeBranch && !opts.deploy {
		if err := m.folderIsFree(workdirPath, ev.CardID); err != nil {
			return nil, err
		}
	}

	m.cfgMu.RLock()
	systemPrompt, deployPrompt := m.cfg.BoardPrompts[ev.BoardID], m.cfg.DeployPrompt
	m.cfgMu.RUnlock()

	// Where the card stands, said in the words the board uses: the agent reads
	// the column it was dragged into along with the task, and moves the card on
	// by naming another one (move_card).
	prompt := composePrompt(ev, agent, systemPrompt, m.boardStanding(ev), worktreeAvailable)
	if deploy != nil {
		prompt = composeDeployPrompt(ev, agent, systemPrompt, deployPrompt, "", *deploy, deployBranch)
	}
	// The tools of the deploy server are allowed up front: nobody is watching a
	// card-triggered run, and an unanswered prompt is a rejected one. Seeding
	// the session rather than DefaultConfig.AutoAllowTools also reaches installs
	// whose config.json predates the feature.
	allowTools := make(map[string]bool)
	if deploy != nil {
		allowTools = deployTools()
	}
	s := &Session{
		ID:           sessionID,
		CardID:       ev.CardID,
		Title:        ev.Title,
		BoardID:      ev.BoardID,
		WorkdirPath:  workdirPath,
		BaseBranch:   ev.Props["branch"],
		Agent:        agent,
		Net:          net,
		Deploy:       deploy,
		DeployBranch: deployBranch,
		Artifacts:    artifacts,
		ColumnName:   ev.ToColumn.Name,
		NodeID:       ev.ToColumn.OptionID,
		PromptText:   prompt,
		Policy:       agentPolicy(agent),
		status:       StatusQueued,
		allowTools:   allowTools,
	}
	rec := SessionRecord{
		ID:        s.ID,
		CardID:    s.CardID,
		BoardID:   s.BoardID,
		AgentKind: agent.Kind,
		Status:    StatusQueued,
		StartedAt: time.Now(),
	}
	if err := m.store.InsertSession(rec); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}

	m.mu.Lock()
	m.active[s.ID] = s
	// byCard is the card's *own* session — the one its console talks to and the
	// one leaving the column cancels. A deploy started from the card while that
	// session is alive must not take its place.
	if live := m.byCard[s.CardID]; live == nil || !opts.deploy {
		m.byCard[s.CardID] = s
	}
	m.mu.Unlock()

	// A session starting is the progress every stall record was waiting for.
	m.clearStall(s.CardID)
	m.emitSession(s, "")

	m.wg.Add(1)
	go m.runSession(s)
	return s, nil
}

// CancelSessionForCard cancels the live session of a card, if any.
func (m *Manager) CancelSessionForCard(cardID, reason string) bool {
	m.mu.Lock()
	s := m.byCard[cardID]
	m.mu.Unlock()
	if s == nil {
		return false
	}
	m.log.Info("acp: cancelling session", "session", s.ID, "card", cardID, "reason", reason)
	s.mu.Lock()
	s.cancelSent = true
	cancel := s.turnCancel
	running := s.status == StatusRunning || s.status == StatusWaitingPermission
	// The session may still be starting up — connecting to the agent takes a
	// process spawn and a handshake — so there is nothing to cancel yet and the
	// turn about to start has to be told.
	s.cancelPending = !running
	s.mu.Unlock()
	if cancel != nil && running {
		cancel()
	} else {
		// Still queued, or between the connection and the turn: there is no
		// turn to interrupt, so end the session outright.
		m.finishSession(s, StatusCancelled, reason)
	}
	return true
}

// StartDeployForCard publishes a card's branch without moving the card into the
// deploy column — the "Deploy" button next to the branch the card is working on.
// branch overrides the card's own "branch" property, which is how the session's
// worktree branch (the one the agent is actually committing to) gets deployed;
// empty falls back to the card property and then to the checked-out branch.
//
// The branch lives in the folder's shared object store even when it was
// created in a worktree, so pushing it from the folder itself — which is
// where a deploy session always runs — reaches it.
func (m *Manager) StartDeployForCard(cardID, branch string) (*Session, error) {
	if m.reader == nil {
		return nil, fmt.Errorf("reading cards is not available")
	}
	ctx, cancel := context.WithTimeout(m.rootCtx, 10*time.Second)
	defer cancel()
	ev, err := m.forCard(cardID).Reader.CardByID(ctx, cardID)
	if err != nil {
		return nil, fmt.Errorf("could not read the card: %w", err)
	}
	if b := strings.TrimSpace(branch); b != "" {
		if ev.Props == nil {
			ev.Props = map[string]string{}
		}
		ev.Props["branch"] = b
	}
	return m.startSession(ev, startOptions{deploy: true})
}

// planningWorkdir picks the registry entry to plan against. An empty name means
// planning without a folder, which is a valid choice rather than a default:
// the dialog preselects a lone entry, so nothing here has to guess.
func (m *Manager) planningWorkdir(name string) (WorkdirEntry, error) {
	if name == "" {
		return WorkdirEntry{}, nil
	}
	for _, r := range m.Workdirs() {
		if strings.EqualFold(r.Name, name) {
			return r, nil
		}
	}
	return WorkdirEntry{}, apperr.New("folders.not-found", "Folder {name} was not found", "name", name)
}

// planningAgent picks the registry entry that will do the planning.
func (m *Manager) planningAgent(name string) (AgentEntry, error) {
	agents := m.Agents()
	if len(agents) == 0 {
		return AgentEntry{}, apperr.New("agents.none", "No agents are registered (the board's menu, Agents…)")
	}
	if name == "" {
		if len(agents) > 1 {
			return AgentEntry{}, apperr.New("agents.which", "Say which agent: several are registered")
		}
		return agents[0], nil
	}
	for _, a := range agents {
		if strings.EqualFold(a.Name, name) {
			return a, nil
		}
	}
	return AgentEntry{}, apperr.New("agents.not-found", "Agent {name} was not found", "name", name)
}

// session looks up a live session by id.
func (m *Manager) session(sessionID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active[sessionID]
}

// CardSessions returns persisted sessions and events for a card (UI hydration).
// CardAgentState is what a card shows about the agent working it: whether a
// session of the automation is running, and which branch the card's work is on.
// It is deliberately small — what a session did is the comment it wrote.
type CardAgentState struct {
	SessionID string `json:"sessionId,omitempty"`
	Status    string `json:"status,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Worktree  string `json:"worktree,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CardAgentState reports the card's live session, or the last one it had.
func (m *Manager) CardAgentState(cardID string) (CardAgentState, error) {
	m.mu.Lock()
	live := m.byCard[cardID]
	m.mu.Unlock()
	if live != nil {
		return CardAgentState{
			SessionID: live.ID,
			Status:    string(live.Status()),
			Branch:    live.recordedBranch(),
			Worktree:  live.Worktree.Path,
		}, nil
	}

	records, _, err := m.store.SessionsForCard(cardID)
	if err != nil {
		return CardAgentState{}, err
	}
	// Newest first is not guaranteed by the store, so take the latest start.
	var last *SessionRecord
	for i := range records {
		if last == nil || records[i].StartedAt.After(last.StartedAt) {
			last = &records[i]
		}
	}
	if last == nil {
		return CardAgentState{}, nil
	}
	return CardAgentState{
		SessionID: last.ID,
		Status:    string(last.Status),
		Branch:    last.Branch,
		Worktree:  last.WorktreePath,
		Error:     last.ErrorText,
	}, nil
}

// Shutdown cancels everything and kills agent processes within grace.
func (m *Manager) Shutdown(grace time.Duration) {
	if m.stop != nil {
		m.stop()
	}
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(grace):
		m.log.Warn("acp: shutdown grace expired with sessions still winding down")
	}
	// The store is not closed here: the database is the board's, and the board
	// outlives this manager by the length of its own shutdown.
	m.tr.Close()
}

// ---- internals ----

// finishSession transitions to a terminal status exactly once.
func (m *Manager) finishSession(s *Session, status SessionStatus, errText string) {
	// A CLI failing to reach its proxy may echo the proxy URL back at us.
	errText = s.Net.redactProxySecret(errText)
	s.mu.Lock()
	if s.status.Terminal() {
		s.mu.Unlock()
		return
	}
	s.status = status
	s.mu.Unlock()
	m.persistStatus(s, status, errText)
}

func (m *Manager) releaseSession(s *Session) {
	// What the work amounts to now (carddiff.go). Here rather than in
	// finishSession, which is the status transition and stands in front of the
	// card's own comment: a git run there delayed the comment by as long as git
	// took, and the first thing that noticed was a test reading the card the
	// moment the session went to done.
	m.refreshCardChanges(s.CardID, s.BoardID)
	m.mu.Lock()
	delete(m.active, s.ID)
	if m.byCard[s.CardID] == s {
		delete(m.byCard, s.CardID)
	}
	m.mu.Unlock()
}

// setStatus moves a live (non-terminal) session between running states, e.g.
// in and out of a permission prompt. Terminal sessions are left alone.
func (m *Manager) setStatus(s *Session, status SessionStatus) {
	s.mu.Lock()
	if s.status.Terminal() {
		s.mu.Unlock()
		return
	}
	s.status = status
	s.mu.Unlock()
	m.persistStatus(s, status, "")
}

func (m *Manager) persistStatus(s *Session, status SessionStatus, errText string) {
	// A session with no card behind it has no row: nothing is recorded against
	// a conversation that belongs to no card (planning.go, inboxrun.go).
	if s.Planning {
		m.emitSession(s, errText)
		return
	}
	if err := m.store.SetSessionStatus(s.ID, status, errText); err != nil {
		m.log.Warn("acp: failed to persist status", "session", s.ID, "status", status, "err", err)
	}
	m.emitSession(s, errText)
}

func (m *Manager) emitSession(s *Session, errText string) {
	s.mu.Lock()
	status, turn := s.status, s.turnNo
	s.mu.Unlock()
	m.ui.Emit(EventSession, map[string]any{
		"sessionId": s.ID,
		"cardId":    s.CardID,
		"status":    string(status),
		"error":     errText,
		// The branch is what the card displays and what its deploy button
		// publishes; deploy tells a card's own session apart from the deploy
		// it started, which shares its card id.
		"branch":       s.recordedBranch(),
		"worktreePath": s.Worktree.Path,
		"deploy":       s.Deploy != nil,
		"turn":         turn,
	})
}

func (m *Manager) comment(s *Session, text string) {
	// Signed by the agent whose run it reports, not by the app: this comment is
	// the fallback for an agent that did not write its own, and reading it as
	// the owner's would be the card telling a person they said it.
	m.commentAs(s, s.CardID, text)
}

func (m *Manager) commentCard(cardID, text string) {
	m.commentAs(nil, cardID, text)
}

func (m *Manager) commentAs(s *Session, cardID, text string) {
	if cardID == "" {
		return // nothing to report to
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.forCard(cardID).Writer.AddComment(m.asAgent(ctx, s), cardID, text); err != nil {
		m.log.Error("acp: failed to add card comment", "card", cardID, "err", err)
	}
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// lookupBin finds name on PATH or in common install locations. When name is an
// absolute/explicit path (contains a separator) it is stat-checked directly.
// notFoundMsg may be empty, in which case the name itself is the message.
func lookupBin(name, notFoundMsg string) (string, error) {
	if notFoundMsg == "" {
		notFoundMsg = fmt.Sprintf("%s was not found", name)
	}
	if strings.ContainsRune(name, filepath.Separator) {
		if _, err := os.Stat(name); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		return name, nil
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".local", "bin", name),
		"/opt/homebrew/bin/" + name,
		"/usr/local/bin/" + name,
	} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s", notFoundMsg)
}

// resolveArgv0 makes an argv runnable from a GUI process: a bare command name is
// looked up on PATH and in the common install locations. The PATH itself is
// repaired at startup from the user's login shell (internal/userpath), which is
// the only thing that finds a version manager's node; the extra locations here
// are what is left when the shell could not be asked. Left as written when
// nothing matches, so the spawn error names the command the user actually typed.
func resolveArgv0(argv []string) []string {
	if len(argv) == 0 {
		return argv
	}
	out := append([]string(nil), argv...)
	if p, err := lookupBin(argv[0], "not found"); err == nil {
		out[0] = p
	}
	return out
}

// planningPrompt is what a planning conversation opens with, ahead of the
// person's first message: the board's system prompt, the agent's own, the
// planning instructions a person can edit, and the one fact nobody should have
// to type — which folder the agent is standing in.
func planningPrompt(systemPrompt, planning string, agent AgentEntry, workdir WorkdirEntry) string {
	b := []byte(promptLead(systemPrompt, agent))
	if p := strings.TrimSpace(planning); p == "" {
		b = fmt.Appendf(b, "%s\n\n", DefaultPlanningPrompt)
	} else {
		b = fmt.Appendf(b, "%s\n\n", p)
	}
	b = fmt.Appendf(b, "Folder: `%s` (%s).", workdir.Name, workdir.Path)
	return string(b)
}

// cardInputs is the stage's declared reads, valued: what an earlier stage
// wrote onto the card — the preview URL, the reviewer's verdict — handed to
// the agent in its brief instead of hoping it asks get_card.
//
// A read that nobody has filled in is still named, so the agent knows the field
// exists and is empty rather than wondering whether it was told — but only
// beside one that has a value. A block where *every* read is empty is dropped
// whole: it is the ordinary state of a route's first stage (and of every stage
// on a route that loops, where a later stage's output is "upstream" of an
// earlier one), it tells the agent nothing it can act on, and it stands at the
// head of the brief where the task should be. Measured on a real run: the task
// opened «From the card:\nПревью:» and the instructions came after.
func cardInputs(ev CardMoved, reads []string) string {
	if len(reads) == 0 {
		return ""
	}
	var b strings.Builder
	valued := false
	for _, name := range reads {
		value := strings.TrimSpace(ev.Props[strings.ToLower(name)])
		if value == "" {
			fmt.Fprintf(&b, "\n%s: (nobody has filled this in)", name)
			continue
		}
		valued = true
		fmt.Fprintf(&b, "\n%s: %s", name, value)
	}
	if !valued {
		return ""
	}
	return "From the card:" + b.String()
}

// boardCardProperties is the board's fields, or nothing. A brief that cannot
// name a select's values is still a brief that names the properties, and the
// board being unreachable must not stop a stage from starting.
func (m *Manager) boardCardProperties(boardID string) []CardProperty {
	if m.meta == nil || boardID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(m.rootOr(), 5*time.Second)
	defer cancel()
	fields, err := m.meta.CardProperties(ctx, boardID)
	if err != nil {
		m.log.Warn("acp: could not read the board's card properties", "board", boardID, "err", err)
		return nil
	}
	return fields
}

// propertyShape is what a value for this property may be, in the words a model
// needs: the closed set where there is one, and the kind otherwise.
func propertyShape(name string, fields []CardProperty) string {
	for _, f := range fields {
		if !strings.EqualFold(f.Name, name) {
			continue
		}
		if len(f.Values) > 0 {
			return "one of " + strings.Join(f.Values, ", ")
		}
		switch f.Type {
		case "url":
			return "a link"
		case "number":
			return "a number"
		case "email":
			return "an email address"
		case "phone":
			return "a phone number"
		default:
			return "text"
		}
	}
	// The board has no such property. Said as text rather than left out: the
	// stage still declares it, and the refusal SetCardFields answers with names
	// the real problem to the one party that can report it.
	return "text"
}

// composePrompt builds the agent task text from the card: what the board says
// and what the agent carries (promptLead), what the stage wants said
// (stagePrompt — the column's instructions, or its route node's), then the
// card task.
func composePrompt(ev CardMoved, agent AgentEntry, systemPrompt, stagePrompt string, useWorktree bool) string {
	b := []byte(promptLead(systemPrompt, agent))
	if p := strings.TrimSpace(stagePrompt); p != "" {
		b = fmt.Appendf(b, "%s\n\n", p)
	}
	b = fmt.Appendf(b, "Task: %s\n", ev.Title)
	if ev.Body != "" {
		b = fmt.Appendf(b, "\n%s\n", ev.Body)
	}
	if useWorktree {
		b = fmt.Appendf(b, "\nWork in the current directory — it is a git worktree of its own, made for this task. Local commits are fine. Do not run git push.")
	} else {
		b = fmt.Appendf(b, "\nWork in the current directory — it is the person's own working folder. Do not switch branches, do not commit and do not push: leave the changes uncommitted for review.")
	}
	return string(b)
}

// composeDeployPrompt builds the task text of a deploy session: the same brief
// an ordinary task gets, then the deploy instructions, then the concrete
// facts — which branch goes where, and what the resulting address should be.
func composeDeployPrompt(ev CardMoved, agent AgentEntry, systemPrompt, deployPrompt, stagePrompt string, target DeployEntry, branch string) string {
	b := []byte(promptLead(systemPrompt, agent))
	if p := strings.TrimSpace(deployPrompt); p != "" {
		b = fmt.Appendf(b, "%s\n\n", p)
	} else {
		b = fmt.Appendf(b, "%s\n\n", DefaultDeployPrompt)
	}
	if p := strings.TrimSpace(stagePrompt); p != "" {
		b = fmt.Appendf(b, "%s\n\n", p)
	}
	slug := dokku.AppSlug(branch)
	b = fmt.Appendf(b, "Card: %s\nBranch: %s\nTarget: %s\nDokku application: %s\nExpected address: %s\n",
		ev.Title, branch, target.Name, target.AppName(slug), target.URL(slug))
	if ev.Body != "" {
		b = fmt.Appendf(b, "\nThe card's description:\n%s\n", ev.Body)
	}
	return string(b)
}
