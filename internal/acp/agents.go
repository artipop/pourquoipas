package acp

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/modelapi"
)

// Agent registry: named coding agents (claude/codex with their own prompt,
// model and env), edited from the desktop UI and persisted into the config
// file. A card goes to the agent it is assigned to: an agent is a member of the
// board under its own name, so the assignee field is the whole of the question.

// Agents returns a snapshot of the registry.
func (m *Manager) Agents() []AgentEntry {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return append([]AgentEntry(nil), m.cfg.Agents...)
}

// validateAgent normalizes and checks a registry entry.
func validateAgent(a AgentEntry) (AgentEntry, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return AgentEntry{}, apperr.New("agents.no-name", "The agent needs a name")
	}
	// An empty element (a stray space in the UI input, a hand-edited config)
	// would become an empty argv[0] and a confusing exec error.
	command := a.Command[:0:0]
	for _, arg := range a.Command {
		if arg = strings.TrimSpace(arg); arg != "" {
			command = append(command, arg)
		}
	}
	a.Command = command
	a.Kind = strings.TrimSpace(strings.ToLower(a.Kind))
	switch {
	case a.Kind == AgentKindModel:
		a.Provider = strings.TrimSpace(strings.ToLower(a.Provider))
		if !knownProvider(a.Provider) {
			return AgentEntry{}, apperr.New("agents.unknown-provider", "Unknown model provider {provider} (one of: {providers})", "provider", a.Provider, "providers", strings.Join(modelapi.Providers, ", "))
		}
		a.Model = strings.TrimSpace(a.Model)
		if a.Model == "" {
			return AgentEntry{}, apperr.New("agents.no-model", "An agent that is a model needs the model's name")
		}
		// None of the process's fields mean anything without a process.
		a.BinPath, a.Command, a.Args, a.CLIArgs, a.Env = "", nil, nil, nil, nil
	case a.Kind == AgentKindACP:
		if len(a.Command) == 0 {
			return AgentEntry{}, apperr.New("agents.no-command", "An agent of kind {kind} needs a launch command (the ACP agent's argv)", "kind", AgentKindACP)
		}
	case knownAdapter(a.Kind):
	default:
		return AgentEntry{}, apperr.New("agents.unknown-kind", "Unknown agent kind {kind} (one of: {kinds})", "kind", a.Kind, "kinds", strings.Join(AgentKinds, ", "))
	}
	cliArgs, err := validateCLIArgs(a)
	if err != nil {
		return AgentEntry{}, err
	}
	a.CLIArgs = cliArgs
	a.Options = normalizeOptions(a.Options)
	servers, err := validateMCPServers(a.MCPServers, a.IsModel())
	if err != nil {
		return AgentEntry{}, fmt.Errorf("agent %q: %w", a.Name, err)
	}
	a.MCPServers = servers
	a.ProxyName = strings.TrimSpace(a.ProxyName)
	return a, nil
}

// validateMCPServers normalizes the agent's own MCP servers. A name has to be
// usable as a tool prefix and must not shadow one we spawn ourselves.
func validateMCPServers(servers map[string]AgentMCPServer, remoteAllowed bool) (map[string]AgentMCPServer, error) {
	if len(servers) == 0 {
		return nil, nil
	}
	out := make(map[string]AgentMCPServer, len(servers))
	for name, srv := range servers {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, apperr.New("mcp.no-name", "An MCP server has no name")
		}
		if !validMCPName(name) {
			return nil, apperr.New("mcp.bad-name", "The MCP server name {name} may contain only Latin letters, digits, hyphens and underscores", "name", name)
		}
		for _, builtin := range builtinMCPNames {
			if strings.EqualFold(name, builtin) {
				return nil, apperr.New("mcp.name-builtin", "The name {name} belongs to a built-in server", "name", name)
			}
		}
		for existing := range out {
			if strings.EqualFold(existing, name) {
				return nil, apperr.New("mcp.duplicate", "There is already an MCP server named {name}", "name", name)
			}
		}

		// A model agent is an MCP client of ours and reaches a remote server
		// itself; a CLI is handed servers in session/new, which carries stdio.
		if remoteAllowed && strings.TrimSpace(srv.URL) != "" {
			srv.URL = strings.TrimSpace(srv.URL)
			srv.Type = strings.ToLower(strings.TrimSpace(srv.Type))
			if srv.Type != "sse" {
				srv.Type = "http"
			}
			srv.Command, srv.Args = "", nil
			out[name] = srv
			continue
		}
		// A remote server is a normal thing to paste from a README, and it
		// would otherwise fail as "no command" — which explains nothing.
		if strings.TrimSpace(srv.URL) != "" || strings.EqualFold(strings.TrimSpace(srv.Type), "http") || strings.EqualFold(strings.TrimSpace(srv.Type), "sse") {
			return nil, apperr.New("mcp.remote", "The server {name} is remote (url/type), and only local servers are supported so far: command + args", "name", name)
		}
		srv.Command = strings.TrimSpace(srv.Command)
		if srv.Command == "" {
			return nil, apperr.New("mcp.no-command", "The MCP server {name} has no launch command (command)", "name", name)
		}
		args := srv.Args[:0:0]
		for _, arg := range srv.Args {
			if arg = strings.TrimSpace(arg); arg != "" {
				args = append(args, arg)
			}
		}
		srv.Args = args
		srv.Type = ""
		out[name] = srv
	}
	return out, nil
}

// normalizeOptions drops the empties. An option left blank means "whatever the
// agent starts with", which is what leaving it out says, and saying it twice
// only leaves the config file harder to read. Values are kept verbatim: what a
// value id may look like is the agent's business, not ours.
func normalizeOptions(options map[string]string) map[string]string {
	if len(options) == 0 {
		return nil
	}
	out := make(map[string]string, len(options))
	for id, value := range options {
		id, value = strings.TrimSpace(id), strings.TrimSpace(value)
		if id == "" || value == "" {
			continue
		}
		out[id] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// validMCPName mirrors what a tool name may carry: the server name becomes the
// mcp__<name>__<tool> prefix the agent reports calls under.
func validMCPName(name string) bool {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// AddAgent registers a new agent and persists the config.
func (m *Manager) AddAgent(a AgentEntry) (AgentEntry, error) {
	a, err := validateAgent(a)
	if err != nil {
		return AgentEntry{}, err
	}
	if err := m.agentFitsHere(a); err != nil {
		return AgentEntry{}, err
	}
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	for _, e := range m.cfg.Agents {
		if strings.EqualFold(e.Name, a.Name) {
			return AgentEntry{}, apperr.New("agents.duplicate", "An agent named {name} already exists", "name", e.Name)
		}
	}
	if _, err := resolveNetworkIn(m.cfg.Proxies, a); err != nil {
		return AgentEntry{}, err
	}
	m.cfg.Agents = append(m.cfg.Agents, a)
	if err := m.persistConfigLocked(); err != nil {
		return AgentEntry{}, err
	}
	// What is handed back is the entry as it was stored, not the copy that went
	// in: saving is what gives it its id, and a caller holding an entry without
	// one holds something it cannot point anything at.
	a = m.cfg.Agents[len(m.cfg.Agents)-1]

	// The account is made here, at the one moment there is something new: a
	// registered agent is a name a card can be assigned to, and it can be
	// assigned to it as soon as it exists. Nothing catches this up later.
	//
	// It does not hold up the registration if it fails — the registry is the
	// answer to "which agents are there", and a board that is not ready yet
	// must not turn adding one into an error.
	m.recordAgentAccountsLocked(m.ensureAgentAccount(a.Name))
	return a, nil
}

// ensureAgentAccount provisions one agent's board account and hands back what
// the board made, for the caller to record. Failure is logged rather than
// returned: see AddAgent.
//
// It does not record the account itself, because its two callers stand on
// different sides of cfgMu — AddAgent is already inside it, and taking it again
// would deadlock.
func (m *Manager) ensureAgentAccount(name string) []AgentUser {
	username := AgentUsername(name)
	if m.users == nil || username == "" {
		return nil
	}
	made, err := m.primary().Users.EnsureAgentAccounts(context.Background(), []AgentUser{{Name: name, Username: username}})
	if err != nil {
		m.log.Warn("acp: the agent has no board account yet", "agent", name, "err", err)
		return nil
	}
	return made
}

// recordAgentAccounts writes which board account belongs to which agent, so an
// assigned card finds its agent by key rather than by their names happening to
// match — which is what renaming an agent used to break.
func (m *Manager) recordAgentAccounts(accounts []AgentUser) {
	if len(accounts) == 0 {
		return
	}
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	m.recordAgentAccountsLocked(accounts)
}

func (m *Manager) recordAgentAccountsLocked(accounts []AgentUser) {
	for _, acc := range accounts {
		if acc.UserID == "" {
			continue
		}
		for i := range m.cfg.Agents {
			if !sameAgentName(m.cfg.Agents[i].Name, acc.Name) || m.cfg.Agents[i].UserID == acc.UserID {
				continue
			}
			m.cfg.Agents[i].UserID = acc.UserID
			if m.store != nil && m.cfg.Agents[i].ID != "" {
				if err := m.store.SetAgentAccount(m.cfg.Agents[i].ID, acc.UserID); err != nil {
					m.log.Warn("acp: cannot record the agent's board account", "agent", acc.Name, "err", err)
				}
			}
		}
	}
}

// assignCardAgent writes a stage's worker into the card's assignee field, in
// the background and best-effort: the field is a mirror of a decision already
// made, and a board that cannot take the write must not stop the session that
// decision started.
func (m *Manager) assignCardAgent(ev CardMoved, agent AgentEntry) {
	if m.users == nil {
		return
	}
	username := AgentUsername(agent.Name)
	if username == "" {
		return
	}
	// Already says so — by username, which is the name the board shows.
	for _, person := range ev.PersonNames {
		if strings.EqualFold(strings.TrimSpace(person), username) {
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.forCard(ev.CardID).Users.AssignCardAgent(ctx, ev.CardID, AgentUser{Name: agent.Name, Username: username}); err != nil {
		m.log.Warn("acp: cannot write the stage's agent into the assignee", "card", ev.CardID, "agent", agent.Name, "err", err)
	}
}

// ensureAgentAccounts is the same for the whole registry, at startup.
func (m *Manager) ensureAgentAccounts() {
	agents := m.AgentUsers()
	if m.users == nil || len(agents) == 0 {
		return
	}
	made, err := m.primary().Users.EnsureAgentAccounts(m.rootCtx, agents)
	if err != nil {
		m.log.Warn("acp: some agents have no board account", "err", err)
	}
	m.recordAgentAccounts(made)

	// An agent is its owner's, not a board's: any of them may be assigned a
	// card on any board, so none is a member of one. Installs from before that
	// made every agent a member of every board it was synced to; those
	// memberships go here, once per launch, and cost nothing when there are none.
	for _, agent := range agents {
		if _, err := m.primary().Users.RetireAgentUser(m.rootCtx, agent); err != nil {
			m.log.Warn("acp: an agent is still a member of a board", "agent", agent.Name, "err", err)
		}
	}
}

// UpdateAgent replaces an existing agent and persists.
//
// Matched by id where the caller has one, which is what makes renaming an agent
// possible: matched by name, the lookup used the new name and answered that
// there is no such agent.
func (m *Manager) UpdateAgent(a AgentEntry) (AgentEntry, error) {
	a, err := validateAgent(a)
	if err != nil {
		return AgentEntry{}, err
	}
	if err := m.agentFitsHere(a); err != nil {
		return AgentEntry{}, err
	}
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	for i, e := range m.cfg.Agents {
		if sameAgentEntry(e, a) {
			if _, err := resolveNetworkIn(m.cfg.Proxies, a); err != nil {
				return AgentEntry{}, err
			}
			if name := strings.TrimSpace(a.Name); !strings.EqualFold(e.Name, name) {
				if _, taken := agentByName(m.cfg.Agents, name); taken {
					return AgentEntry{}, apperr.New("agents.duplicate", "An agent named {name} already exists", "name", name)
				}
			}
			// The account outlives the rename: it is the agent's identity on the
			// board, and every card assigned to it points at it by that id.
			a.UserID = e.UserID
			// So is the owner: whose agent it is does not change by editing it.
			a.OwnerID = e.OwnerID
			// The identity is the entry's, not the caller's. A caller that read
			// the agent, did something else that saved, and then handed the copy
			// back would otherwise replace a registered agent with one that has
			// no id — and the next save would insert it a second time under the
			// same name. Which is contradiction 2 in miniature: while the name
			// was the key, losing the id cost nothing and nobody noticed.
			a.ID = e.ID
			m.cfg.Agents[i] = a
			return a, m.persistConfigLocked()
		}
	}
	return AgentEntry{}, apperr.New("agents.not-found", "Agent {name} was not found", "name", a.Name)
}

// AgentUsers returns the registry as board accounts, in registry order, with
// the account id where one has been recorded.
func (m *Manager) AgentUsers() []AgentUser {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	out := make([]AgentUser, 0, len(m.cfg.Agents))
	for _, a := range m.cfg.Agents {
		if username := AgentUsername(a.Name); username != "" {
			out = append(out, AgentUser{Name: a.Name, Username: username, UserID: a.UserID})
		}
	}
	return out
}

// AgentAccounts is AgentUsers with every account made: the page offers these
// as assignees on any board, and an agent with no account cannot be offered.
func (m *Manager) AgentAccounts(ctx context.Context) ([]AgentUser, error) {
	agents := m.AgentUsers()
	missing := []AgentUser{}
	for _, a := range agents {
		if a.UserID == "" {
			missing = append(missing, a)
		}
	}
	if len(missing) == 0 || m.users == nil {
		return agents, nil
	}
	made, err := m.primary().Users.EnsureAgentAccounts(ctx, missing)
	m.recordAgentAccounts(made)
	if err != nil {
		return nil, err
	}
	return m.AgentUsers(), nil
}

// PlanningPrompt returns the instructions a planning terminal is opened with.
func (m *Manager) PlanningPrompt() string {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	if p := strings.TrimSpace(m.cfg.PlanningPrompt); p != "" {
		return m.cfg.PlanningPrompt
	}
	// A config written before this key existed, or emptied by hand: the
	// planning terminal has no other instructions, so it gets the default
	// rather than nothing.
	return DefaultPlanningPrompt
}

// SetPlanningPrompt stores the planning instructions and persists.
func (m *Manager) SetPlanningPrompt(text string) error {
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	m.cfg.PlanningPrompt = text
	return m.persistConfigLocked()
}

// RemoveAgent deletes a registry entry by name and persists the config. The
// account survives (cards may still name it), so re-registering the agent
// restores the same identity; it stops being offered because the page offers
// the registry, not the accounts.
func (m *Manager) RemoveAgent(name string) error {
	_, err := m.removeAgentEntry(name)
	return err
}

// removeAgentEntry drops the entry from the registry and returns it as an
// account, so the caller can retire it outside the config lock.
func (m *Manager) removeAgentEntry(name string) (AgentUser, error) {
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	for i, e := range m.cfg.Agents {
		if strings.EqualFold(e.Name, name) {
			m.cfg.Agents = append(m.cfg.Agents[:i], m.cfg.Agents[i+1:]...)
			return AgentUser{Name: e.Name, Username: AgentUsername(e.Name)}, m.persistConfigLocked()
		}
	}
	return AgentUser{}, apperr.New("agents.not-found", "Agent {name} was not found", "name", name)
}

// AgentUsername is the board username an agent is provisioned under: the
// registry name folded to what a username can carry, so the entry "My Agent"
// and the account "my-agent" are the same agent. Also used for matching, which
// is why it must stay deterministic.
//
// A letter is any letter, not only a Latin one: folded to `a-z0-9`, an agent
// whose name is written in Cyrillic becomes the empty string and can never be
// put in the assignee field at all. Nothing here needs ASCII — the username is
// stored in the board's own table and read back by the same fold.
func AgentUsername(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// sameAgentName reports whether a name from the board (a select option, an
// assignee's username) refers to the registry entry named entryName.
func sameAgentName(name, entryName string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.EqualFold(name, entryName) {
		return true
	}
	// The account carries the folded form of the name, so "My Agent" on the
	// board and the assignee "my-agent" both reach the same entry.
	folded := AgentUsername(name)
	return folded != "" && folded == AgentUsername(entryName)
}

// resolveAgent maps a trigger event to an agent, with no stage to speak for
// itself. See resolveSessionAgent for the whole rule.
func (m *Manager) resolveAgent(ev CardMoved) (AgentEntry, error) {
	a, _, err := m.resolveSessionAgent(ev, nil)
	return a, err
}

// resolveSessionAgent picks who runs the session.
//
// roster is the stage's own crew — the flow node's, else the column's. It is
// not a pin but a membership list: it says who may work this column at all, and
// the card chooses among them. So an assignee who is on the crew gets the card,
// an assignee who is not is passed over (the *Deploy* column is not worked by
// the developer the card is assigned to), and a column with no crew leaves the
// choice to the card exactly as before:
//
//  1. the card's own choice — its assignee — narrowed to the crew when there
//     is one;
//  2. the crew itself: the first member with nothing else running;
//  3. the single registered agent, when exactly one exists;
//  4. a synthesized entry from the global AgentMode (backward compat: the
//     built-in claude bridge, or an external acp-command agent).
//
// busy reports that a crew exists and every one of its members is already
// working: the caller parks the card instead of piling a second session onto
// the same agent.
func (m *Manager) resolveSessionAgent(ev CardMoved, roster []string) (AgentEntry, bool, error) {
	m.cfgMu.RLock()
	agents := append([]AgentEntry(nil), m.cfg.Agents...)
	mode := m.cfg.AgentMode
	m.cfgMu.RUnlock()

	crew, err := crewOf(roster, agents)
	if err != nil {
		return AgentEntry{}, false, err
	}
	// By the account the card actually stores, then by the username it shows.
	// The id is the answer that survives a rename; the username still answers
	// for a card assigned before the agent's account was recorded, and for a
	// board carried here from a machine where the agent has an account of its
	// own under the same name.
	byAccount := func(userID string) (AgentEntry, bool) {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return AgentEntry{}, false
		}
		for _, a := range crew {
			if a.UserID == userID {
				return a, true
			}
		}
		return AgentEntry{}, false
	}
	find := func(name string) (AgentEntry, bool) {
		for _, a := range crew {
			if sameAgentName(name, a.Name) {
				return a, true
			}
		}
		return AgentEntry{}, false
	}

	// The assignee is how a card names its agent, and the only way it does.
	// There were three ways once — a property named `agent`, an option of the
	// board's «Agent» field, and the assignee — which is three answers to one
	// question and a rule about which of them wins. The two that went were the
	// two nobody could see: a property nothing in this app creates, and any
	// select option anywhere on the board that happened to be spelled like an
	// agent.
	for _, id := range ev.PersonIDs {
		if a, ok := byAccount(id); ok {
			return a, false, nil
		}
	}
	for _, person := range ev.PersonNames {
		if a, ok := find(person); ok {
			return a, false, nil
		}
	}

	if len(roster) > 0 {
		if a, ok := m.freeAgent(crew); ok {
			return a, false, nil
		}
		return AgentEntry{}, true, nil
	}

	if len(agents) == 1 {
		return agents[0], false, nil
	}
	if len(agents) > 1 {
		return AgentEntry{}, false, apperr.New("agents.ambiguous", "Could not choose an agent: assign one to the card (there are: {agents})", "agents", agentNames(agents))
	}

	// Empty registry: fall back to the global AgentMode.
	kind := mode
	if kind == "" {
		kind = AgentKindClaude
	}
	if kind == agentModeCommand {
		// The mode names no kind of its own — the argv in the config is the
		// whole agent, which is exactly what the generic acp kind is.
		m.cfgMu.RLock()
		command := append([]string(nil), m.cfg.AgentCommand...)
		m.cfgMu.RUnlock()
		if len(command) == 0 {
			return AgentEntry{}, false, fmt.Errorf("agentMode = %q, but agentCommand is empty", agentModeCommand)
		}
		return AgentEntry{Name: kind, Kind: AgentKindACP, Command: command}, false, nil
	}
	return AgentEntry{Name: kind, Kind: kind}, false, nil
}

// crewOf resolves the names of a stage's crew against the registry. Unknown
// names are skipped — a registry edited after the column was configured should
// not stop the board — but a crew where nobody is left is a mistake worth
// reporting rather than quietly ignoring.
func crewOf(roster []string, agents []AgentEntry) ([]AgentEntry, error) {
	if len(roster) == 0 {
		return agents, nil
	}
	crew := make([]AgentEntry, 0, len(roster))
	for _, id := range roster {
		if a, ok := agentByID(agents, id); ok {
			crew = append(crew, a)
		}
	}
	if len(crew) == 0 {
		// The roster is ids, so there is nothing readable to name back: what
		// the person can act on is which agents this machine does have.
		return nil, fmt.Errorf("the column's agents are not in the registry (there are: %s)", agentNames(agents))
	}
	return crew, nil
}

// freeAgent is the first member of the crew with no live session, in the order
// the crew was listed — so the choice is repeatable and the first name in the
// list is the one that normally works.
func (m *Manager) freeAgent(crew []AgentEntry) (AgentEntry, bool) {
	m.mu.Lock()
	busy := make(map[string]bool, len(m.active))
	for _, s := range m.active {
		busy[strings.ToLower(s.Agent.Name)] = true
	}
	m.mu.Unlock()

	for _, a := range crew {
		if !busy[strings.ToLower(a.Name)] {
			return a, true
		}
	}
	return AgentEntry{}, false
}

func agentNames(agents []AgentEntry) string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

// AssignedToHumanError is why a stage did not start: the card is somebody's.
// It is not a failure — the work is being done, only not by us — so the card
// waits where it stands rather than taking a failure branch.
type AssignedToHumanError struct {
	Who string
}

func (e AssignedToHumanError) Error() string {
	return fmt.Sprintf("the card is assigned to %s", e.Who)
}

// humanAssignee is who took the card, when that is a person rather than an
// agent. Assigning yourself is how you say "this one is mine": an agent picking
// the same card up would do the same work twice, and — on a route — would move
// the card on the moment it decided it was finished.
//
// An assignee that *is* a registered agent means the opposite (that agent runs
// it), so a card with one is not somebody's in this sense.
func humanAssignee(ev CardMoved, agents []AgentEntry) string {
	human := ""
	for _, person := range ev.PersonNames {
		person = strings.TrimSpace(person)
		if person == "" {
			continue
		}
		for _, a := range agents {
			if sameAgentName(person, a.Name) {
				return "" // an agent was assigned; it works
			}
		}
		if human == "" {
			human = person
		}
	}
	return human
}

// sameAgentEntry is which registry row an edit is about: the id when the caller
// carries one, else the name.
func sameAgentEntry(existing, edit AgentEntry) bool {
	if id := strings.TrimSpace(edit.ID); id != "" {
		return existing.ID == id
	}
	return sameAgentName(existing.Name, edit.Name)
}

// agentFitsHere refuses what this build cannot run, when it is registered
// rather than when a card is moved: a hosted server starts no program, so an
// agent there is a model, and its MCP servers are addresses.
func (m *Manager) agentFitsHere(a AgentEntry) error {
	if !m.hosted {
		return nil
	}
	if !a.IsModel() {
		return apperr.New("agents.cli-not-here", "Agent {name} runs a program on a machine, and this server runs none: give the card an agent that is a model", "name", a.Name)
	}
	for name, srv := range a.MCPServers {
		if srv.URL == "" {
			return apperr.New("mcp.no-processes-here", "The MCP server {name} is a program to start, and this server starts none: give its address (url) instead", "name", name)
		}
	}
	return nil
}
