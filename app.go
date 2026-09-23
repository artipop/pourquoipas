package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardadapter"
	"github.com/artipop/xciii/internal/inboxsort"
	"github.com/artipop/xciii/internal/sources"
)

// errACPDisabled is returned by bindings when the integration is off.
var errACPDisabled = errors.New("интеграция агента выключена (см. конфиг acp)")

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
	// tailnet is the door a phone comes through; nil when its settings could
	// not be read at all. Its own methods are safe on a nil receiver, so the
	// bindings do not each have to check.
	tailnet *tailnetController
	// attach is which board this desktop works: its own, or a hosted server it
	// has logged in to. Nil when its settings could not be read; safe on a nil
	// receiver like the tailnet's.
	attach *attachController
	// boards is the page's own door: which board it is served. Switching aims
	// it somewhere else, and the page reloads onto what it now answers.
	boards *boardRouter
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
	// on a nil receiver, as the tailnet controller's are.
	updates *updateController
	// retemplate reinstalls the shipped board templates in a language. Nil when
	// the templates are not this process's business — an attached desktop works
	// a hosted board and must not fight the server over them. Set in main,
	// where the board server is.
	retemplate func(lang string) error
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
	return a.mgr.CancelSessionForCard(cardID, "отменено пользователем")
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

// ListAgents returns the agent registry as JSON: [{"name","kind",…}, …].
func (a *App) ListAgents() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.Agents())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListAgentAccounts returns the registry as the board knows it — the name a
// person typed and the username it was provisioned under — as JSON:
// [{"name","username"}, …].
//
// The page needs the username to recognise an agent among the board's people:
// the fold from one to the other is this side's (AgentUsername), and writing it
// a second time in TypeScript would be two answers to "is this an agent".
func (a *App) ListAgentAccounts() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.AgentUsers())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AddAgent registers a new agent from a JSON-encoded AgentEntry and returns the
// created entry as JSON.
func (a *App) AddAgent(entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.AgentEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	saved, err := a.mgr.AddAgent(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// UpdateAgent replaces an existing agent (matched by name) from a JSON-encoded
// AgentEntry and returns the saved entry as JSON.
func (a *App) UpdateAgent(entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.AgentEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
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
func (a *App) RemoveAgent(name string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.RemoveAgent(name)
}

// SyncAgentUsers gives every registered agent a board account and adds it to
// the board's members, so cards can be assigned to an agent in a person
// property. Returns the accounts as JSON: [{"name","username","userId",
// "created"}, …].
func (a *App) SyncAgentUsers(boardID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	users, err := a.mgr.SyncAgentUsers(context.Background(), boardID)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(users)
	return string(out), nil
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

// ListDeployTargets returns the deploy registry as JSON:
// [{"name","sshHost","baseDomain",…}, …].
func (a *App) ListDeployTargets() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.Deploys())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AddDeployTarget registers a Dokku destination from a JSON-encoded DeployEntry
// and returns the created entry as JSON.
func (a *App) AddDeployTarget(entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.DeployEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	saved, err := a.mgr.AddDeploy(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// UpdateDeployTarget replaces an existing destination (matched by name) from a
// JSON-encoded DeployEntry and returns the saved entry as JSON.
func (a *App) UpdateDeployTarget(entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.DeployEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	saved, err := a.mgr.UpdateDeploy(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// RemoveDeployTarget deletes a Dokku destination by name.
func (a *App) RemoveDeployTarget(name string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.RemoveDeploy(name)
}

// ListFlows returns the routes a board may use as JSON — its own, plus any tied
// to no board in particular — each a graph of nodes (a column) and edges (an
// event and where it leads).
//
// It reads the board first (SeedBoard), because the registry is filled lazily —
// by a card being moved, by an edit, by the automation editor opening — and the
// question this answers is asked by a screen that is opened before any of that
// happens. A board carrying three routes of its own answered "none" until
// somebody touched it, and the route picker above the kanban then drew nothing
// on exactly the boards it is for. Idempotent and once per board per run.
func (a *App) ListFlows(boardID string) (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	a.mgr.SeedBoard(boardID)
	out, err := json.Marshal(a.mgr.BoardFlows(boardID))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// BoardSetupPlan returns what this board needs answered before its automation
// can run: which steps, in which order, which of them may be skipped and which
// are already answered by this machine. It is one answer to one question — the
// wizard walks it and the board menu reads it to know which registries are
// worth opening at all.
func (a *App) BoardSetupPlan(boardID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	out, err := json.Marshal(a.mgr.SetupPlanFor(boardID))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// MarkBoardSetupOffered remembers that the setup wizard has opened itself for
// this board. It is stored beside the rest of the board's setup rather than in
// the page: the desktop app serves itself on a fresh port every launch, so the
// page's own storage is a fresh one too.
func (a *App) MarkBoardSetupOffered(boardID string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.MarkSetupOffered(boardID)
}

// CheckBoardSetupAnswer says whether an answer fits the step it answers on this
// board — a folder under git for a board that publishes a branch or waits for
// one, and any requirement a step grows later. Called before the answer is
// filed, so the question can refuse it where it was asked.
func (a *App) CheckBoardSetupAnswer(boardID, step, value string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.CheckSetupAnswer(boardID, step, value)
}

// SetBoardTestAgent answers the wizard's QA step: which agent tests this board,
// and what it drives a browser with — a JSON-encoded mcpServers block, the same
// one the agents dialog takes. The agent is also written into the board's test
// column as its crew, so the column runs the agent the browser was given to.
func (a *App) SetBoardTestAgent(boardID, agentName, serversJSON string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	var servers acp.MCPServerSet
	if strings.TrimSpace(serversJSON) != "" {
		if err := json.Unmarshal([]byte(serversJSON), &servers); err != nil {
			return err
		}
	}
	return a.mgr.SetTestAgent(boardID, agentName, servers)
}

// SetBoardWorkAgent answers the wizard's agent step: the agents chosen become
// the crew of this board's agent columns, which is the only place that choice
// can be recorded — the registry is the machine's and says nothing about who
// works which board. An empty list takes the crew off again.
func (a *App) SetBoardWorkAgent(boardID, namesJSON string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	var names []string
	if strings.TrimSpace(namesJSON) != "" {
		if err := json.Unmarshal([]byte(namesJSON), &names); err != nil {
			return err
		}
	}
	return a.mgr.SetWorkAgents(boardID, names)
}

// BoardAgentUsers says which of this machine's agents this board names, and
// which agents there are at all — as board usernames, because that is what a
// person property holds. The page needs both to narrow the assignee list: an
// agent the board does not name is dropped, and anybody who is not an agent is
// left alone. A board that names nobody narrows nothing, or a board nobody has
// crewed would have no agent to assign at all.
func (a *App) BoardAgentUsers(boardID string) (string, error) {
	answer := struct {
		Board []string `json:"board"`
		All   []string `json:"all"`
	}{Board: []string{}, All: []string{}}
	if a.mgr == nil {
		out, err := json.Marshal(answer)
		return string(out), err
	}
	for _, name := range a.mgr.BoardAgentNames(boardID) {
		answer.Board = append(answer.Board, acp.AgentUsername(name))
	}
	for _, user := range a.mgr.AgentUsers() {
		answer.All = append(answer.All, user.Username)
	}
	out, err := json.Marshal(answer)
	return string(out), err
}

// RecordBoardSetupStep remembers what was done with a step — skipping above
// all, which is the one answer no registry can be read for.
func (a *App) RecordBoardSetupStep(boardID, step, status string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.RecordSetupStep(boardID, step, status)
}

// ListSetupSteps returns the closed set of setup steps this build can carry
// out, so a board can only ask for one that exists.
func (a *App) ListSetupSteps() (string, error) {
	out, err := json.Marshal(acp.SetupStepDefs)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListFlowTriggers returns the closed set of edge triggers the engine
// implements, so the editor can only offer transitions that actually work.
func (a *App) ListFlowTriggers() (string, error) {
	out, err := json.Marshal(acp.FlowTriggers)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListFlowTemplates returns the routes a fresh install is seeded with, rebuilt
// from the current column names. An install whose registry predates them (or
// whose routes were deleted) can add the ones it is missing from the editor
// instead of retyping a graph.
func (a *App) ListFlowTemplates() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	// In the language the templates were installed in: these routes stand on
	// that board's columns, and a route naming "In progress" on a Russian
	// board is a route with nowhere to run.
	flows := a.mgr.FlowTemplates()
	word := boardadapter.TemplateWords(uiLanguage())
	for i := range flows {
		flows[i].Name = word(flows[i].Name)
		flows[i].Property = word(flows[i].Property)
		for j := range flows[i].Nodes {
			flows[i].Nodes[j].Column = word(flows[i].Nodes[j].Column)
		}
		for j := range flows[i].Edges {
			if cond := flows[i].Edges[j].If; cond != nil {
				cond.Property = word(cond.Property)
				cond.Value = word(cond.Value)
			}
		}
	}
	out, err := json.Marshal(flows)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AddFlow registers a route from a JSON-encoded FlowEntry and returns it.
func (a *App) AddFlow(entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.FlowEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	saved, err := a.mgr.AddFlow(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// UpdateFlow replaces an existing route (matched by name) and returns it.
func (a *App) UpdateFlow(entryJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var entry acp.FlowEntry
	if err := json.Unmarshal([]byte(entryJSON), &entry); err != nil {
		return "", err
	}
	saved, err := a.mgr.UpdateFlow(entry)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// RemoveFlow deletes a board's route by id. Cards standing on it simply stop
// moving by themselves.
func (a *App) RemoveFlow(boardID, flowID string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.RemoveFlow(boardID, flowID)
}

// ExportBoardAutomation returns what a board runs — its columns and its routes
// — in the shape a template carries them in its own properties. It is how a
// board somebody has built by hand becomes a template: the registry is Go's, so
// only this side can read it out.
func (a *App) ExportBoardAutomation(boardID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	out, err := json.Marshal(a.mgr.BoardAutomation(boardID))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListBoardColumns returns what each configured column of a board does: the
// action, the crew that works it and how many of them at once.
func (a *App) ListBoardColumns(boardID string) (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.BoardColumns(boardID))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SaveBoardColumn stores the settings of one column from a JSON-encoded
// ColumnSpec and returns what was saved.
func (a *App) SaveBoardColumn(specJSON string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	var spec acp.ColumnSpec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		return "", err
	}
	saved, err := a.mgr.SaveColumn(spec)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(saved)
	return string(out), nil
}

// RemoveBoardColumn forgets a column's settings. The column stays on the board.
func (a *App) RemoveBoardColumn(boardID, optionID, column string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.RemoveColumn(boardID, optionID, column)
}

// GetCardFlow describes where a card stands on its route: the stages, the one
// it is on, what that stage waits for. Returns "null" for a card with no route.
func (a *App) GetCardFlow(cardID string) (string, error) {
	if a.mgr == nil {
		return "null", nil
	}
	flow, err := a.mgr.CardFlowFor(cardID)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(flow)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListCardFlows is which route each card of a board is on, in one call: a view
// filtered by route asks it of every card it draws.
func (a *App) ListCardFlows(boardID string) (string, error) {
	if a.mgr == nil {
		return "{}", nil
	}
	// The board is read first for the reason ListFlows reads it: which route a
	// card is on is answered against the registry, and the registry is filled
	// lazily.
	a.mgr.SeedBoard(boardID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := json.Marshal(a.mgr.CardFlowsForBoard(ctx, boardID))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// MarkCardOutcome is the card's own «прошло»/«не прошло» — the two buttons on a
// stage where nobody works and nothing runs, and the way a review says no.
func (a *App) MarkCardOutcome(cardID, value string) error {
	if a.mgr == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.mgr.MarkCardOutcome(ctx, cardID, value)
}

// AnswerCardAsk is a person answering the question a transition is holding the
// card at. The last answer lets the card across.
func (a *App) AnswerCardAsk(cardID, questionID, value string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.mgr.AnswerCardAsk(ctx, cardID, questionID, value)
}

// SeedBoardAutomation takes the columns and routes a board carries of its own
// into the registry now, rather than waiting for the first card to be moved.
// The setup wizard calls it, so what the board can do is visible as soon as it
// is configured. Idempotent: anything already registered is left alone.
func (a *App) SeedBoardAutomation(boardID string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	a.mgr.SeedBoard(boardID)
	return nil
}

// GetBoardFlowOverview returns where the board's cards stand on each route:
// per stage, how many are there, how many are working and how many wait.
func (a *App) GetBoardFlowOverview(boardID string) (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	overview, err := a.mgr.BoardFlowOverview(boardID)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(overview)
	if err != nil {
		return "", err
	}
	return string(out), nil
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

// GetTailnetAccess reports whether the board is published on the user's tailnet
// and, when it is, the address to open on a phone. JSON:
// {"enabled":…,"hostname":…,"status":"off|joining|login|on|error","url":…,
//
//	"loginUrl":…,"error":…,"path":…}.
func (a *App) GetTailnetAccess() (string, error) {
	out, err := json.Marshal(a.tailnet.status())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SetTailnetAccess turns the tailnet door on or off and renames the node,
// returning the new state. It takes effect immediately — the node is brought up
// or closed here, not at the next launch — which is the whole point of having a
// switch rather than a settings file.
//
// Only the two fields the panel owns are read: an auth key and the list of
// allowed logins stay whatever the file says, since the panel has no way to ask
// for either.
func (a *App) SetTailnetAccess(entryJSON string) (string, error) {
	if a.tailnet == nil {
		return "", errors.New("доступ по tailnet недоступен")
	}
	var want struct {
		Enabled  bool   `json:"enabled"`
		Hostname string `json:"hostname"`
	}
	if err := json.Unmarshal([]byte(entryJSON), &want); err != nil {
		return "", err
	}

	next := a.tailnet.settingsCopy()
	next.Enabled = want.Enabled
	next.Hostname = strings.TrimSpace(want.Hostname)

	state, err := a.tailnet.update(next)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(out), nil
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

// GetPlanningPrompt returns the instructions a planning terminal is opened
// with, falling back to the default when the config has none.
func (a *App) GetPlanningPrompt() (string, error) {
	if a.mgr == nil {
		return acp.DefaultPlanningPrompt, nil
	}
	return a.mgr.PlanningPrompt(), nil
}

// SetPlanningPrompt stores the instructions a planning terminal is opened with.
func (a *App) SetPlanningPrompt(text string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.SetPlanningPrompt(text)
}

// StartCardDeploy publishes a card's branch to its Dokku target without moving
// the card into the deploy column, and returns the deploy session's id. branch
// is the one the card is working on (its session's worktree branch); empty lets
// the card property or the checked-out branch decide.
func (a *App) StartCardDeploy(cardID, branch string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	s, err := a.mgr.StartDeployForCard(cardID, branch)
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

// ---- terminal windows ----

// A terminal is the agent's own CLI in a window of ours: the same folder,
// worktree, branch and environment a session would get, with a human at the
// keyboard instead of a prompt loop. These bindings start one and hand back
// where to draw it; the drawing is xterm.js on a WebSocket (terminalws.go).

// terminalHandle is what the UI needs to show a terminal: which one, and where.
type terminalHandle struct {
	acp.TerminalInfo
	// URL is the page that draws it. A desktop build has already opened a
	// window there by the time this is returned; a server build has no windows,
	// so the browser opens a tab itself.
	URL string `json:"url"`
	// Windowed says which of those two happened.
	Windowed bool `json:"windowed"`
}

// OpenCardTerminal opens (or focuses) the agent CLI for a card and returns the
// terminal as JSON. workdirName/agentName may be empty, in which case the card
// decides exactly as it does for a session.
//
// window says whether it gets a window of its own. The card draws the terminal
// inside itself now — the panel its chevron opens is the terminal — so a window
// would be a second view of the same pty opening behind the one being looked
// at. It is still what the ⤢ beside the panel asks for, and a screen of its own
// is worth having.
func (a *App) OpenCardTerminal(cardID, workdirName, agentName string, window bool) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	t, err := a.mgr.StartCardTerminal(cardID, workdirName, agentName)
	if err != nil {
		return "", err
	}
	if !window {
		return terminalHandleJSON(a.terminalURL(t), false, t)
	}
	return a.terminalWindow(t)
}

// OpenCardTalk opens the card's own conversation rather than the work on it —
// the wording, the plan, the brief. A door of its own rather than a flag on the
// one above, because which of the two a person wants is the thing the app kept
// having to guess (nodeTalk).
func (a *App) OpenCardTalk(cardID, workdirName, agentName string, window bool) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	t, err := a.mgr.StartCardTalk(cardID, workdirName, agentName)
	if err != nil {
		return "", err
	}
	if !window {
		return terminalHandleJSON(a.terminalURL(t), false, t)
	}
	return a.terminalWindow(t)
}

// OpenPlanningTerminal opens the CLI with no card behind it — the terminal half
// of "Plan a task". boardID is the board the dialog was opened from: the
// conversation has no card, but it may leave cards, and that is the only board
// it may leave them on.
func (a *App) OpenPlanningTerminal(workdirName, agentName, boardID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	t, err := a.mgr.StartPlanningTerminal(workdirName, agentName, boardID)
	if err != nil {
		return "", err
	}
	return a.terminalWindow(t)
}

// terminalWindow opens the window for a terminal session and describes it.
func (a *App) terminalWindow(t *acp.TerminalSession) (string, error) {
	url := a.terminalURL(t)
	windowed := false
	if wapp := a.app(); wapp != nil {
		windowed = openTerminalWindow(wapp, t.Info(), url)
	}
	return terminalHandleJSON(url, windowed, t)
}

// terminalURL is the page that draws a terminal, wherever it is drawn.
func (a *App) terminalURL(t *acp.TerminalSession) string {
	return a.originURL() + "acp/terminal/" + t.Info().ID
}

func terminalHandleJSON(url string, windowed bool, t *acp.TerminalSession) (string, error) {
	out, err := json.Marshal(terminalHandle{TerminalInfo: t.Info(), URL: url, Windowed: windowed})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetTerminalInfo describes a live terminal, for the page that draws it.
func (a *App) GetTerminalInfo(terminalID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	t := a.mgr.Terminal(terminalID)
	if t == nil {
		return "", errors.New("терминал не активен")
	}
	out, err := json.Marshal(t.Info())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetCardAgent is everything a card shows about its agent in one call: the
// terminal it already has running or could continue, and the state of the
// automation's own session — the branch it works on, so the card can offer to
// deploy it, and whether something is running, so it can be cancelled.
//
// There is no transcript here. A session reports itself in the card's comments,
// and a conversation with an agent happens in a terminal window.
// JSON: {"running":{…}|null,"resume":{…},"session":{…}}.
func (a *App) GetCardAgent(cardID string) (string, error) {
	if a.mgr == nil {
		return "{}", nil
	}
	session, err := a.mgr.CardAgentState(cardID)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"resume":  a.mgr.TerminalHistoryForCard(cardID),
		"session": session,
	}
	// An agent waiting on an answer is the one thing on a card that is
	// addressed to the person reading it.
	if q := a.mgr.QuestionForCard(cardID); q != nil {
		payload["question"] = q
	}
	if live := a.mgr.TerminalForCard(cardID); live != nil {
		payload["running"] = live.Info()
	}
	// Why nothing is happening, for a card outside any route — the route strip
	// says the same thing for cards on one.
	if stall, ok := a.mgr.CardStall(cardID); ok {
		payload["stall"] = stall
	}
	// The card's conversations, one per stage of its route: what the terminal
	// panel lists, with the current stage's being the one it opens.
	if conversations := a.mgr.CardConversations(cardID); len(conversations) > 0 {
		payload["conversations"] = conversations
	}
	// Where a conversation on this card would run. The panel asks the person
	// before starting a folderless one, and this is how it knows to ask.
	if folder, ok := a.mgr.CardFolder(cardID); ok {
		payload["folder"] = folder
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

// ListTerminals returns every terminal currently running as JSON, newest first.
// A window can be closed while its CLI keeps working, and a planning terminal
// has no card to be found through, so this is how the UI offers it back.
func (a *App) ListTerminals() (string, error) {
	if a.mgr == nil {
		return "[]", nil
	}
	out, err := json.Marshal(a.mgr.LiveTerminals())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ListCutOffConversations returns, as a JSON object of card id → reason, the
// cards whose conversation stopped without a verdict: the CLI was closed, or
// the app was, and the stage was never reported. The board draws a paused
// terminal button on each of them, so that work nobody picked back up is
// visible from the board rather than only from inside the card.
//
// One call for the page, indexed by card, exactly as ListTerminals is: a board
// has as many cards as it likes.
func (a *App) ListCutOffConversations() (string, error) {
	if a.mgr == nil {
		return "{}", nil
	}
	out, err := json.Marshal(a.mgr.CutOffConversations())
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

// ListAttention returns everything waiting for a person as JSON: the terminals
// whose agent has gone quiet, oldest wait first. The page keeps itself current
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
// The wait itself stands — the card keeps its amber button until the agent is
// answered — and the announcement comes back if the agent does something and
// stops again (internal/acp/attentionack.go).
func (a *App) AckAttention(key string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	a.mgr.AckAttention(key)
	return nil
}

// ShowTerminal reopens the window of a terminal that is already running and
// returns it as JSON, the same shape OpenCardTerminal returns.
func (a *App) ShowTerminal(terminalID string) (string, error) {
	if a.mgr == nil {
		return "", errACPDisabled
	}
	t := a.mgr.Terminal(terminalID)
	if t == nil {
		return "", errors.New("терминал уже завершён")
	}
	return a.terminalWindow(t)
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

// OpenCardDiff gives the diff a window of its own, the way a terminal gets one:
// reading a review is the thing a panel beside the card is least able to be,
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
// a window in the dock is picked out by its title, and "Изменения" three times
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

// CloseTerminal ends a terminal session and everything its CLI started.
func (a *App) CloseTerminal(terminalID string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.CloseTerminal(terminalID)
}

// RenameTerminal is a person calling a conversation what it is to them. The
// title a terminal starts with says which card it is on, and a list of open
// terminals is read by what each one is about.
func (a *App) RenameTerminal(terminalID, title string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.RenameTerminal(terminalID, title)
}

// AskTerminalName asks the agent to name the conversation it is having. The
// request is typed into that conversation and the answer comes back through the
// board tools, so the row in the list says what is going on in it rather than
// who is talking and where.
func (a *App) AskTerminalName(terminalID string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.AskTerminalName(terminalID)
}

// DeleteCardConversation throws away one conversation of a card: the CLI in it
// ends and the record goes with it, so the next one starts on a blank screen.
// It is how the card's own conversation is closed for good — everything else
// about a terminal is kept, which is what makes continue possible.
func (a *App) DeleteCardConversation(cardID, nodeID string) error {
	if a.mgr == nil {
		return errACPDisabled
	}
	return a.mgr.DeleteCardConversation(cardID, nodeID)
}

// errSourcesDisabled is returned by the source bindings when the subsystem
// could not be started at all — no data directory, an unreadable registry.
var errSourcesDisabled = errors.New("источники недоступны")

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
		return "", errors.New("источник не найден")
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
var errNoBoard = errors.New("доска недоступна")

// The board, as the page at /m reads it.
//
// A phone gets the bindings and the event socket and nothing else, which is
// what lets the same page work through the tailnet door — so what it needs
// from the board comes through here rather than through the board's own REST
// API. What it needs is small: which boards there are, what is on one, what is
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
		return "", errors.New("не сказано, какая доска")
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
		return errors.New("не сказано, какую карточку и куда переносить")
	}
	return a.board.MoveCardToBoard(context.Background(), cardID, boardID, column)
}

// Sorting the inbox (internal/inboxsort, docs/sources.md §29): which agent
// carries arrivals on to the boards, what it has read about each board, and
// what it decided.

var errNoSorter = errors.New("разбор входящих недоступен: нет источников")

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
		return "", errors.New("источники недоступны")
	}
	if strings.TrimSpace(title) == "" && strings.TrimSpace(url) == "" {
		return "", errors.New("нечего сохранять: ни ссылки, ни заголовка")
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
