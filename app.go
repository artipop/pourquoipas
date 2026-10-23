package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/inboxsort"
	"github.com/artipop/pourquoipas/internal/sources"
)

// errACPDisabled is returned by bindings when the integration is off.
var errACPDisabled = apperr.New("agents.off", "Agents are off in this install")

// App is the bound service: every exported method is callable from the page,
// through the bindings `wails3 task generate:bindings` writes into
// frontend/bindings. Wiring that main does itself stays unexported for that
// reason — an exported setter here is one the page can call. It contains no
// logic of its own; ACP calls delegate to the manager.
type App struct {
	mu      sync.RWMutex
	wapp    *application.App
	origin  string
	emitter *wailsEmitter
	mgr     *acp.Manager // nil when the ACP integration is disabled
	// attach is which board this desktop works: its own, or a hosted server it
	// has logged in to. Nil when its settings could not be read; its methods
	// are safe on a nil receiver, so the bindings do not each have to check.
	attach *attachController
	// boards is the page's own door: which board it is served. Switching aims
	// it somewhere else, and the page reloads onto what it now answers.
	boards *boardRouter
	// sockets carries a desktop page's WebSockets over the bus
	// (sockettunnel.go). Nil in a test that does not care.
	sockets *socketTunnels
	// local is the door other processes are given — a script posting to a
	// source, an agent's tools — which a desktop page, on wails://, cannot
	// read off its own address.
	local string
	// sources turns outside events into cards. Separate from mgr on purpose: a
	// board of household chores wants cards from a phone and no agents at all.
	sources *sources.Manager
	// sorter carries what arrives in the inbox on to the boards it belongs on.
	// Nil when the sources are off, since it keeps its log in their database.
	sorter *inboxsort.Sorter
	// board is how the page at /m reads the board, since it is served the
	// bindings and the event socket and no board API of its own.
	board boardLists
	// updates is how this app replaces itself; nil in a headless build and
	// whenever the release feed could not be configured. Its methods are safe
	// on a nil receiver, as the attach controller's are.
	updates *updateController
	// owners says who may change the machine's settings on a server
	// (identity_server.go); a desktop asks nobody.
	owners installOwners
	// aiKeys are the model providers' keys, by person (aikeys.go).
	aiKeys *aiKeys
}

func NewApp(emitter *wailsEmitter) *App {
	return &App{emitter: emitter}
}

// setApplication hands over the running application, the way to reach the
// runtime in v3. It is set once, before Run.
func (a *App) setApplication(wapp *application.App) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.wapp = wapp
}

// setOrigin records the address the front door serves the app under, which is
// what a second window has to be pointed at.
func (a *App) setOrigin(url string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.origin = url
}

func (a *App) originURL() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.origin
}

func (a *App) app() *application.App {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.wapp
}

// OpenInBrowser opens the given URL in the user's default system browser.
// Called by window.openInNewBrowser and the outward-link catch-all the page
// installs (installGoBridge).
func (a *App) OpenInBrowser(url string) {
	wapp := a.app()
	if wapp == nil || url == "" {
		return
	}
	_ = wapp.Browser.OpenURL(url)
}

// CancelSession cancels the live agent session of a card, if any.
func (a *App) CancelSession(cardID string) bool {
	if a.mgr == nil {
		return false
	}
	return a.mgr.CancelSessionForCard(cardID, "cancelled by the user")
}

// ListAgentWorkdirs returns the folder registry as JSON, each entry carrying
// what git says about it right now ("git", "base", "broken" — see
// acp.WorkdirStatus). boardID is the board asking; "" asks for the whole
// registry, which is what a place with no board behind it (the planning
// dialog) wants.
func (a *App) ListAgentWorkdirs(boardID string) (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.WorkdirStatusesForBoard(boardID))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// PickDirectory opens the native folder picker and returns the chosen
// absolute path ("" when the user cancels). A server build has no native
// dialog to open, so it says so rather than returning an empty path the UI
// would read as a cancellation.
func (a *App) PickDirectory(title string) (string, error) {
	wapp := a.app()
	if wapp == nil {
		return "", nil
	}
	return pickDirectory(wapp, title)
}

// AddAgentWorkdir registers a local folder (name defaults to the directory
// basename when empty) and returns the created entry as JSON. It belongs to
// boardID — the board it was added on and the only one that offers it — unless
// global says every board should. kind is what was asked for: "git" when the
// screen demanded a repository, "" everywhere else (see acp.AddWorkdir).
func (a *App) AddAgentWorkdir(name, path, boardID, kind string, global bool) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	entry, err := a.mgr.AddWorkdir(name, path, boardID, kind, global)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(entry)
	return string(out), nil
}

// SetAgentWorkdirBase changes what work in a folder branches from, and what
// merged into the main branch waits for. Empty falls the folder back to what git says.
func (a *App) SetAgentWorkdirBase(name, branch string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	entry, err := a.mgr.SetWorkdirBase(name, branch)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(entry)
	return string(out), nil
}

// FindAgentWorkdir returns the registry entry for a path — whichever board it
// belongs to — or "" for a path nobody has added. The screens that add a folder
// ask first, so "already added elsewhere" can be offered as a choice instead of
// refused as a mistake.
func (a *App) FindAgentWorkdir(path string) (string, error) {
	if a.mgr == nil {
		return "", nil
	}
	entry, ok := a.mgr.WorkdirAt(path)
	if !ok {
		return "", nil
	}
	out, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ShareAgentWorkdir marks a folder as every board's, which is how a folder
// registered on one board comes to be offered on another.
func (a *App) ShareAgentWorkdir(name string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	entry, err := a.mgr.ShareWorkdir(name)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(entry)
	return string(out), nil
}

// ListUnattachedWorkdirs returns the registry entries no board has claimed, as
// JSON. They are what an install upgrading into board-owned folders is left
// with, and the dialog offers them to the board somebody is on.
func (a *App) ListUnattachedWorkdirs() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.UnattachedWorkdirs())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AttachAgentWorkdir gives an unattached folder to a board.
func (a *App) AttachAgentWorkdir(name, boardID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	entry, err := a.mgr.AttachWorkdir(name, boardID)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(entry)
	return string(out), nil
}

// RemoveAgentWorkdir deletes a folder registry entry by name.
func (a *App) RemoveAgentWorkdir(name string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.RemoveWorkdir(name)
}

// GetAgentNamedBranches reports whether the agent invents branch names — a
// short headless run per card, before its first workspace (acp/naming.go).
func (a *App) GetAgentNamedBranches() (bool, error) {
	if a.mgr == nil {
		return false, nil
	}
	return a.mgr.AgentNamedBranches(), nil
}

// SetAgentNamedBranches flips that setting.
func (a *App) SetAgentNamedBranches(on bool) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.SetAgentNamedBranches(on)
}

// ListAgentAdapters reports, per agent kind, whether it can be started on this
// machine — the adapter is installed, npx would fetch it, or Node.js is missing
// — so the dialog can say it instead of a card failing later.
func (a *App) ListAgentAdapters() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.AdapterStatuses())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// InstallAgentAdapter installs a kind's adapter with npm and returns npm's own
// output, so a failure is readable.
func (a *App) InstallAgentAdapter(kind string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	return a.mgr.InstallAdapter(kind)
}

// ListAgents returns the signed-in person's agents as JSON: [{"name","kind",…}, …].
func (a *App) ListAgents(ctx context.Context) (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(agentsOf(a.mgr.Agents(), person))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListAgentAccounts returns the person's agents as the board knows them — the
// name a person typed, the username and the account id — as JSON:
// [{"name","username","userId"}, …]. These are offered as assignees on every
// board; no agent is a member of one.
//
// The page needs the username to recognise an agent among the board's people:
// the fold from one to the other is this side's (AgentUsername), and writing it
// a second time in TypeScript would be two answers to "is this an agent".
func (a *App) ListAgentAccounts(ctx context.Context) (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return "", err
	}
	mine := map[string]bool{}
	for _, entry := range agentsOf(a.mgr.Agents(), person) {
		mine[entry.Name] = true
	}
	all, err := a.mgr.AgentAccounts(ctx)
	if err != nil {
		return "", err
	}
	accounts := []acp.AgentUser{}
	for _, account := range all {
		if mine[account.Name] {
			accounts = append(accounts, account)
		}
	}
	out, err := json.Marshal(accounts)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AddAgent registers a new agent from a JSON-encoded AgentEntry and returns the
// created entry as JSON.
func (a *App) AddAgent(ctx context.Context, entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return "", err
	}
	var entry acp.AgentEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	// Whose it is is the session's answer, never the page's.
	entry.OwnerID = person
	saved, err := a.mgr.AddAgent(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// UpdateAgent replaces an existing agent (matched by name) from a JSON-encoded
// AgentEntry and returns the saved entry as JSON.
func (a *App) UpdateAgent(ctx context.Context, entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.AgentEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	if err := a.ownAgent(ctx, entry.ID, entry.Name); err != nil {
		return "", err
	}
	saved, err := a.mgr.UpdateAgent(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// AgentOptions asks the agent itself which settings it has — Fast mode, an
// effort level, a permission mode — and returns them as JSON:
// [{"id","name","type","current","values":[…]}, …]. The agent is started the
// way a session would start it and asked nothing, so the dialog offers exactly
// what this agent supports and no toggle for what it does not. refresh skips
// the cached answer.
func (a *App) AgentOptions(entryJSON string, refresh bool) (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	var entry acp.AgentEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	options, err := a.mgr.AgentOptions(entry, refresh)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(options)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// RemoveAgent deletes an agent registry entry by name.
func (a *App) RemoveAgent(ctx context.Context, name string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	if err := a.ownAgent(ctx, "", name); err != nil {
		return err
	}
	return a.mgr.RemoveAgent(name)
}

// ListProxies returns the proxy registry as JSON: [{"name","proxy",…}, …].
func (a *App) ListProxies() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.Proxies())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AddProxy registers a network configuration from a JSON-encoded ProxyEntry and
// returns the created entry as JSON.
func (a *App) AddProxy(entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.ProxyEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	saved, err := a.mgr.AddProxy(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// UpdateProxy replaces an existing network configuration (matched by name) from
// a JSON-encoded ProxyEntry and returns the saved entry as JSON.
func (a *App) UpdateProxy(entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.ProxyEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	saved, err := a.mgr.UpdateProxy(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// RemoveProxy deletes a network configuration by name.
func (a *App) RemoveProxy(name string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.RemoveProxy(name)
}

// SetAgentWorkdirMode records how a folder that is a repository is worked in
// on one board: "worktree" — a copy of its own per card — or "branch" — a
// branch in the folder itself. Asked per (board, folder), because a folder
// belongs to a board anyway and the one that does not — shared with every board — is
// exactly the case where two boards may want different answers.
func (a *App) SetAgentWorkdirMode(name, boardID, mode string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	entry, err := a.mgr.SetWorkdirMode(name, boardID, mode)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(entry)
	return string(out), nil
}

// GetLocalAddress returns the address other processes on this machine reach
// the app at, e.g. "http://127.0.0.1:53114".
func (a *App) GetLocalAddress() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return strings.TrimSuffix(a.local, "/")
}

// OpenSocket opens the socket at path for the page under the id it chose and
// subscribed to as "socket:<id>" (sockettunnel.go).
func (a *App) OpenSocket(id, path string) error {
	if a.sockets == nil {
		return errors.New("no sockets in this build")
	}
	return a.sockets.open(id, path)
}

// SendSocket writes one frame to an open socket.
func (a *App) SendSocket(id, data string) error {
	if a.sockets == nil {
		return errors.New("no sockets in this build")
	}
	return a.sockets.send(id, data)
}

// CloseSocket closes a socket the page is done with.
func (a *App) CloseSocket(id string) {
	if a.sockets != nil {
		a.sockets.close(id)
	}
}

// GetServerAttachment reports which board this desktop works — its own, or a
// hosted server. JSON: {"url":…,"username":…,"running":…}, where url and
// running disagreeing is the app saying a restart is owed: which board the page
// is served, and which one the agents work against, are both decided when the
// app is built up.
func (a *App) GetServerAttachment() (string, error) {
	out, err := json.Marshal(a.attach.state())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AttachServer logs in to a hosted board and remembers it for the next launch.
//
// The password is not kept: what is stored is the session it bought, in the
// credential store, and a person who wants out says so with DetachServer.
func (a *App) AttachServer(entryJSON string) (string, error) {
	var want struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(entryJSON), &want); err != nil {
		return "", err
	}
	state, err := a.attach.attach(want.URL, want.Username, want.Password)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// DetachServer puts this desktop back on its own board.
func (a *App) DetachServer() (string, error) {
	state, err := a.attach.detach()
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SelectBoardSource changes which board the page shows — this machine's own, or
// the server this desktop is attached to.
//
// It changes nothing about which boards the *agents* work: both are theirs,
// whichever is on screen, so a stage running on the server's card goes on
// running while somebody looks at their own board.
//
// The page has to reload, and does not absorb the change: everything it holds —
// the boards, the members, both sockets — came from the board it loaded
// against, and a screen showing one board's cards under another's name is worse
// than a reload.
func (a *App) SelectBoardSource(id string) (string, error) {
	state, err := a.attach.selectBoard(id)
	if err != nil {
		return "", err
	}
	if a.boards != nil {
		if err := a.boards.point(a.attach.selected()); err != nil {
			return "", err
		}
	}
	out, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetBoardPrompt returns what every agent of this board is told first.
func (a *App) GetBoardPrompt(boardID string) (string, error) {
	if a.mgr == nil {
		return "", nil
	}
	return a.mgr.BoardPrompt(boardID), nil
}

// SetBoardPrompt stores that instruction for one board.
func (a *App) SetBoardPrompt(boardID, text string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.SetBoardPrompt(boardID, text)
}

// ---- planning ----

// A planning conversation is talking a task over with an agent before a card
// exists (internal/acp/planning.go). The reply streams in as acp:chunk events
// and each turn's end is an acp:planning event; these calls start one, speak in
// it and end it.

// StartPlanning opens a planning conversation on a board and returns it as
// JSON. folderName may be empty for the board's drafts folder, agentName when
// only one agent is registered.
func (a *App) StartPlanning(boardID, folderName, agentName string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	info, err := a.mgr.StartPlanning(boardID, folderName, agentName)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SayToPlanning is a person's message: the conversation's next turn.
func (a *App) SayToPlanning(id, text string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.SayToPlanning(id, text)
}

// StopPlanningReply interrupts the reply being written; the conversation stays.
func (a *App) StopPlanningReply(id string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	a.mgr.StopPlanningTurn(id)
	return nil
}

// EndPlanning ends a planning conversation and the agent process with it.
func (a *App) EndPlanning(id string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	a.mgr.EndPlanning(id)
	return nil
}

// ListPlanning returns the planning conversations open on a board as JSON,
// with what was said in each: a dialog closed and opened again finds its
// conversation where it left it.
func (a *App) ListPlanning(boardID string) (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.Planning(boardID))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetCardAgent is everything a card shows about its agent in one call: the
// state of the automation's own session — the branch it works on, and whether
// something is running, so it can be cancelled — and the question it is
// waiting on, if any.
//
// There is no transcript here: a stage reports itself in the card's comments.
// JSON: {"session":{…},"question":{…}?,…}.
func (a *App) GetCardAgent(cardID string) (string, error) {
	if a.mgr == nil {
		return "{}", nil
	}
	session, err := a.mgr.CardAgentState(cardID)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"session": session,
	}
	// An agent waiting on an answer is the one thing on a card that is
	// addressed to the person reading it.
	if q := a.mgr.QuestionForCard(cardID); q != nil {
		payload["question"] = q
	}
	// Why nothing is happening, for a card outside any route — the route strip
	// says the same thing for cards on one.
	if stall, ok := a.mgr.CardStall(cardID); ok {
		payload["stall"] = stall
	}
	// Where the card's work is — one answer, merged from the claim on this
	// machine and what the card itself carries (acp.CardWork). The stamp names
	// its line after the mode, two vocabularies for one thing having read as
	// the setting not having worked; the folder field is locked on «started»,
	// which is true of a card that was worked on another machine too.
	if work := a.mgr.CardWork(cardID); work.Started {
		payload["work"] = work
	}
	// How much that work amounts to. Present only when there is something to
	// read, which is what the «Изменения» button is drawn on — a button that
	// opened «ничего не изменено» was a button nobody could learn
	// (internal/acp/carddiff.go).
	if changes, ok := a.mgr.CardChanges(cardID); ok {
		payload["changes"] = map[string]any{
			"branch":  changes.Branch,
			"base":    changes.Base,
			"files":   changes.Files,
			"added":   changes.Added,
			"removed": changes.Removed,
			"dirty":   changes.Dirty,
		}
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AnswerQuestion hands an agent the answer it is waiting for: an option it
// offered, or text typed instead. Both empty means no — the agent hears a
// refusal and carries on without what it asked for.
func (a *App) AnswerQuestion(questionID, optionID, text string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	ans := acp.Answer{OptionID: optionID, Text: text}
	if optionID == "" && strings.TrimSpace(text) == "" {
		ans.Declined = true
	}
	return a.mgr.AnswerQuestion(questionID, ans)
}

// ListAttention returns everything waiting for a person as JSON: the questions
// agents are holding a turn open on, and the cards a route handed to a person,
// oldest wait first. The page keeps itself current
// from acp:attention events; this is what a page that opened later starts from.
func (a *App) ListAttention() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.Attention())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AckAttention says a person has seen this wait, so it stops being announced.
// The wait itself stands — the card keeps its mark until the agent is answered
// (internal/acp/attentionack.go).
func (a *App) AckAttention(key string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	a.mgr.AckAttention(key)
	return nil
}

// GetCardDiff is the card's work read off the disk: the branch, what it was cut
// from, and the changed files with their patches (internal/acp/carddiff.go).
// JSON, and never an error for "nothing to show" — a card nobody has worked on
// answers with a reason a person can act on.
func (a *App) GetCardDiff(cardID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	d, err := a.mgr.CardDiff(cardID)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// OpenCardDiff gives the diff a window of its own: reading a review is the thing a panel beside the card is least able to be,
// and the card is open behind it anyway. Answers with the address, so a build
// with no windows — a browser, a phone — can still go there itself.
func (a *App) OpenCardDiff(cardID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	url := a.originURL() + "acp/diff/" + cardID
	if wapp := a.app(); wapp != nil {
		openDiffWindow(wapp, cardID, a.cardDiffTitle(cardID), url)
	}
	return url, nil
}

// cardDiffTitle names the window after the card, falling back to the card id:
// a window in the dock is picked out by its title, and "Changes" three times
// over is three windows nobody can tell apart.
func (a *App) cardDiffTitle(cardID string) string {
	if a.mgr == nil {
		return cardID
	}
	if title := a.mgr.CardTitle(cardID); title != "" {
		return title
	}
	return cardID
}

// errSourcesDisabled is returned by the source bindings when the subsystem
// could not be started at all — no data directory, an unreadable registry.
var errSourcesDisabled = apperr.New("sources.unavailable", "Sources are not available")

// ListSources returns the source registry as JSON. It is one list and not a
// board's: every source writes to the one inbox.
func (a *App) ListSources() (string, error) {
	if a.sources == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.sources.Sources())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListSourcePlugins is what a source can be made *of*: the manifests this
// machine knows, from the registry and from <dataDir>/sources/manifests. The
// dialog builds its form out of one of these — the fields to ask for, and
// whether the service wants a token.
func (a *App) ListSourcePlugins() (string, error) {
	if a.sources == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.sources.Plugins())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SetSourceCredential stores the token a source has to *present* — an API key
// somebody pasted, which for an MCP server is the whole of its authorization.
//
// Not to be confused with ResetSourceToken below, which is the other direction:
// that one authorizes what is sent *to* a source over the ingest route, and is
// kept as a hash because it is only ever checked.
func (a *App) SetSourceCredential(name, token string) error {
	if a.sources == nil {
		return errSourcesDisabled
	}
	return a.sources.SetToken(name, token)
}

// SourceStatuses is what each source is doing — running, failed, waiting to be
// connected — for the strip of text that makes a silent integration debuggable.
func (a *App) SourceStatuses() (string, error) {
	if a.sources == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.sources.Statuses())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AddSource registers a source from a JSON-encoded entry and returns it with
// its ingest token — the one time the token is ever shown, since only its hash
// is kept.
func (a *App) AddSource(entryJSON string) (string, error) {
	if a.sources == nil {
		return "", errSourcesDisabled
	}
	var entry sources.SourceEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	token, err := sources.GenerateToken()
	if err != nil {
		return "", err
	}
	entry.TokenHash = sources.HashToken(token)
	saved, err := a.sources.AddSource(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(struct {
		sources.SourceEntry
		Token string `json:"token"`
	}{saved, token})
	return string(out), nil
}

// UpdateSource replaces a source, matched by name. The token is deliberately
// carried over rather than taken from the entry: the UI never holds one, and a
// blank field would otherwise lock out whatever is already sending.
func (a *App) UpdateSource(entryJSON string) (string, error) {
	if a.sources == nil {
		return "", errSourcesDisabled
	}
	var entry sources.SourceEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	if existing, ok := a.sources.Source(entry.Name); ok {
		entry.TokenHash = existing.TokenHash
	}
	saved, err := a.sources.UpdateSource(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// ResetSourceToken issues a new token and returns it, which is the only way to
// see one again: the previous token stops working the moment this returns.
func (a *App) ResetSourceToken(name string) (string, error) {
	if a.sources == nil {
		return "", errSourcesDisabled
	}
	entry, ok := a.sources.Source(name)
	if !ok {
		return "", apperr.New("sources.not-found", "Source {name} was not found", "name", name)
	}
	token, err := sources.GenerateToken()
	if err != nil {
		return "", err
	}
	entry.TokenHash = sources.HashToken(token)
	if _, err := a.sources.UpdateSource(entry); err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]string{"name": entry.Name, "token": token})
	return string(out), nil
}

// RemoveSource deletes a source and everything it remembered.
func (a *App) RemoveSource(name string) error {
	if a.sources == nil {
		return errSourcesDisabled
	}
	return a.sources.RemoveSource(name)
}

// SourceEvents returns a source's log, newest first: what arrived, what the
// rules decided and which card it became. This is the answer to the only
// question a source is ever asked — why nothing happened.
func (a *App) SourceEvents(name string, limit int) (string, error) {
	if a.sources == nil {
		return "[]", nil
	}
	events, err := a.sources.Events(name, limit)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(events)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// errNoBoard is returned by the board bindings when this build has no board to
// read — a server that failed to come up, or a test.
var errNoBoard = apperr.New("board.unavailable", "The board is not available")

// The board, as the page at /m reads it.
//
// A phone gets the bindings and the event socket and nothing else, so what it
// needs from the board comes through here rather than through the board's own
// REST API. What it needs is small: which boards there are, what is on one, what is
// waiting in an inbox, and a way to carry a card from the inbox onto a board.

// ListBoards returns the boards with the columns a card can be moved into.
// Both are one call because the two questions are always asked together —
// which board, then which column of it — and a phone should ask once.
func (a *App) ListBoards() (string, error) {
	if a.board == nil {
		return "[]", nil
	}
	boards, err := a.board.Boards(context.Background())
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(boards)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListBoardCards returns one board's cards, newest first.
func (a *App) ListBoardCards(boardID string) (string, error) {
	if a.board == nil {
		return "[]", nil
	}
	if strings.TrimSpace(boardID) == "" {
		return "", errors.New("no board given")
	}
	cards, err := a.board.BoardCards(context.Background(), boardID)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(cards)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListInbox returns what has arrived and nobody has looked at yet: the cards on
// the inbox board, newest first. The board is found by the mark the app gave
// it, never by its title. Empty, not an error, where there is no inbox yet —
// nothing has arrived.
func (a *App) ListInbox() (string, error) {
	if a.board == nil {
		return "[]", nil
	}
	boards, err := a.board.Boards(context.Background())
	if err != nil {
		return "", err
	}
	for _, board := range boards {
		if !board.Inbox {
			continue
		}
		cards, err := a.board.BoardCards(context.Background(), board.ID)
		if err != nil {
			return "", err
		}
		out, err := json.Marshal(cards)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return "[]", nil
}

// MoveCardToBoard carries a card to another board and, when a column is named,
// puts it there. It is the phone's half of the card menu's the move-to-board item, and it is the same move: the card keeps its id.
func (a *App) MoveCardToBoard(cardID, boardID, column string) error {
	if a.board == nil {
		return errNoBoard
	}
	if strings.TrimSpace(cardID) == "" || strings.TrimSpace(boardID) == "" {
		return errors.New("no card or no board to move it to")
	}
	return a.board.MoveCardToBoard(context.Background(), cardID, boardID, column)
}

// Sorting the inbox (internal/inboxsort, docs/sources.md §29): which agent
// carries arrivals on to the boards, what it has read about each board, and
// what it decided.

var errNoSorter = apperr.New("inboxsort.unavailable", "Sorting the inbox is not available: sources are off")

// GetInboxSorting returns everything the sorting dialog draws: the settings,
// every board with its description and summary, and the recent decisions.
func (a *App) GetInboxSorting() (string, error) {
	if a.sorter == nil {
		return "", errNoSorter
	}
	state, err := a.sorter.State(context.Background())
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SetInboxSorting saves {"enabled":true,"agent":"claude"} and returns what was
// saved.
func (a *App) SetInboxSorting(settingsJSON string) (string, error) {
	if a.sorter == nil {
		return "", errNoSorter
	}
	var want inboxsort.Settings
	if err := json.Unmarshal([]byte(settingsJSON), &want); err != nil {
		return "", err
	}
	saved, err := a.sorter.SetSettings(want)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(saved)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SortInboxNow sorts every card in the inbox, sorting switched on or not, and
// returns how many were moved, kept and failed. It waits for the agent, which
// is a minute rather than a moment.
func (a *App) SortInboxNow() (string, error) {
	if a.sorter == nil {
		return "", errNoSorter
	}
	res, err := a.sorter.SortInbox(context.Background())
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(res)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// RefreshBoardSummaries has the agent read every board's cards again and
// rewrite what they are about.
func (a *App) RefreshBoardSummaries() error {
	if a.sorter == nil {
		return errNoSorter
	}
	return a.sorter.RefreshSummaries(context.Background())
}

// SetBoardDescription writes a board's description from the sorting dialog.
func (a *App) SetBoardDescription(boardID, text string) error {
	if a.sorter == nil {
		return errNoSorter
	}
	return a.sorter.SetBoardDescription(context.Background(), boardID, text)
}

// The share sheet.
//
// What the system hands over when somebody presses the share dialog in another app
// is a link and a title, and the only question left is which board — the inbox,
// unless the person already knows. The dialog that asks it is a page of ours
// (/share), so this is all it needs from Go: the boards to offer are ListBoards
// above, and this is the send.
//
// It goes through the sources pipeline rather than writing a card directly,
// because everything a source's card gets is there already — the author, the
// link, the record that keeps a repeat from becoming a second card.

// ShareItem files a shared link on the board the person picked, or in the
// inbox when boardID is empty, and reports what came of it in the same shape a
// delivery does, so the dialog can tell created from already there.
func (a *App) ShareItem(boardID, title, url, note string) (string, error) {
	if a.sources == nil {
		return "", errSourcesDisabled
	}
	if strings.TrimSpace(title) == "" && strings.TrimSpace(url) == "" {
		return "", apperr.New("share.empty", "Nothing to save: there is neither a link nor a title")
	}
	// Registered on first use: a person who shares a link has not been asked to
	// set up a source, and should not have to be.
	entry, err := a.sources.EnsureSource(sources.ShareSource())
	if err != nil {
		return "", err
	}
	res, err := a.sources.Deliver(context.Background(), entry.Name,
		[]sources.Item{sources.ShareItem(boardID, title, url, note)})
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(res)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetUpdateState reports everything the the updates section panel draws: which
// version is running, whether the automatic check is on, what the last check
// found and how far along an install is. JSON, the shape of updateState in
// updates.go.
//
// A build that cannot update itself answers {"supported":false}, and the
// settings dialog leaves the section out — which is also the answer in a plain
// browser and as a Mattermost plugin, where there is no Go side to ask.
func (a *App) GetUpdateState() (string, error) {
	out, err := json.Marshal(a.updates.snapshot())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SetUpdateSettings changes what the panel owns — for now only whether the app
// looks for updates by itself — and returns the new state. {"enabled":true}.
func (a *App) SetUpdateSettings(entryJSON string) (string, error) {
	var want struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(entryJSON), &want); err != nil {
		return "", err
	}
	state, err := a.updates.setEnabled(want.Enabled)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CheckForUpdate asks the release feed. It returns as soon as the request is
// away: what was found arrives as the acp:update event, which is the same way
// the panel hears about a check nobody started.
func (a *App) CheckForUpdate() error {
	return a.updates.check()
}

// InstallUpdate downloads what the last check found, checks its signature
// against the key compiled into this binary, and stages it. Nothing is
// replaced until RestartToUpdate.
func (a *App) InstallUpdate() error {
	return a.updates.install()
}

// SkipUpdateVersion stops offering the release now on offer. It is remembered
// across restarts, which the framework does not do for us.
func (a *App) SkipUpdateVersion() error {
	return a.updates.skip()
}

// RestartToUpdate closes this app and brings back the new one. Everything the
// shutdown hook does — agents given their grace period, plugins stopped, the
// board server closed — happens first, because the helper waits for this
// process to be gone before it touches anything.
func (a *App) RestartToUpdate() error {
	return a.updates.restart()
}

// agentsOf is the registry as one person sees it: their own agents and the
// ones nobody owns — every agent of a desktop, and on a server those made
// before agents had owners.
func agentsOf(all []acp.AgentEntry, person string) []acp.AgentEntry {
	out := []acp.AgentEntry{}
	for _, entry := range all {
		if entry.OwnerID == "" || entry.OwnerID == person {
			out = append(out, entry)
		}
	}
	return out
}

// ownAgent refuses a change to somebody else's agent. An agent nobody can be
// found by is left for the registry itself to say so.
func (a *App) ownAgent(ctx context.Context, id, name string) error {
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return err
	}
	for _, entry := range a.mgr.Agents() {
		if (id != "" && entry.ID == id) || (id == "" && strings.EqualFold(entry.Name, name)) {
			if entry.OwnerID != "" && entry.OwnerID != person {
				return apperr.New("agents.not-yours", "Agent {name} is somebody else's", "name", entry.Name)
			}
			return nil
		}
	}
	return nil
}
