package acp

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// A board arrives knowing how the work on it is organised. What it cannot know
// is the machine: which agent runs, in which folder, where it publishes, what
// it drives a browser with. The setup wizard asks for that — and *what* it asks
// is the board's business, because a board of household chores has nowhere to
// deploy to and nothing to test, and an unanswerable question reads as a broken
// feature.
//
// So the steps are a plan, resolved here, out of three things:
//
//   - what the board asks for, in a property of its own (BoardPropSetup), the
//     way it already carries its columns and its routes;
//   - failing that, what its automation implies — a board that runs a deploy
//     needs somewhere to deploy to;
//   - and what this machine already has, so a question already answered is not
//     asked again.
//
// The steps themselves are a closed set implemented here, exactly like
// FlowTriggers: a board picks from it and cannot invent one. A board that could
// declare arbitrary fields would be a program, and nothing on this side could
// promise the answer fits the registry it is going into.

// Setup step kinds.
const (
	// SetupStepName is the board's own name, and the one step that asks about
	// the board rather than about this machine. It is first and it is not
	// optional: a board arrives called what its template is called, so the
	// second one made from the developer template is a second the developer template in the
	// sidebar, and the moment to say what this one is for is the moment it is
	// made.
	SetupStepName   = "name"
	SetupStepAgent  = "agent"  // the agent that picks a card up
	SetupStepDeploy = "deploy" // a Dokku host to publish a branch to
	SetupStepDone   = "done"   // no question: how to use what was set up
)

// retiredSetupSteps are kinds this app used to ask and no longer does. Every
// board made before carries them in its declaration, so they are dropped in
// silence rather than warned about as a step from a newer app.
//
// "project" was the folder. A card without one works in the board's drafts
// folder, so the question stopped being one the board could not do without,
// and it is asked where it has an answer: on the card, whose folder field adds
// a folder the first time one is wanted (docs/decisions.md).
var retiredSetupSteps = map[string]bool{"project": true}

// Setup step status.
const (
	SetupPending = "pending" // still to be answered
	SetupDone    = "done"    // answered, or already true of this machine
	SetupSkipped = "skipped" // deliberately passed over
)

// setupWizardStep is not a step: it is the row that remembers the wizard itself
// was offered for a board. It lives in the same table because it is the same
// kind of fact — something this machine knows about this board — and it cannot
// live in the page, where localStorage is keyed by origin and the desktop app
// takes a fresh port (and therefore a fresh origin) on every launch, so a
// refusal lasted exactly one run.
const setupWizardStep = "wizard"

// SetupStepDef describes one step the app can actually carry out. The list
// doubles as the wizard's shape, so the UI can never offer a step there is no
// registry behind.
type SetupStepDef struct {
	Kind string `json:"kind"`
	// Registry the step fills, and therefore what makes it already answered.
	// Empty for a step that asks nothing.
	Registry string `json:"registry,omitempty"`
	// Optional says the step may be passed over — everything else on the board
	// still works without it.
	Optional bool `json:"optional"`
}

// SetupStepDefs is the closed set, in the order a wizard walks them. The order
// is the order the work needs them in: publishing means nothing without an
// agent.
var SetupStepDefs = []SetupStepDef{
	// No registry: what answers it is the board itself, so there is nothing on
	// this machine that could have answered it already.
	{Kind: SetupStepName},
	{Kind: SetupStepAgent, Registry: "agents"},
	{Kind: SetupStepDeploy, Registry: "deploys", Optional: true},
	{Kind: SetupStepDone},
}

// SetupStepDefinition returns the definition of a kind, if the app has one.
func SetupStepDefinition(kind string) (SetupStepDef, bool) {
	for _, def := range SetupStepDefs {
		if strings.EqualFold(def.Kind, kind) {
			return def, true
		}
	}
	return SetupStepDef{}, false
}

// BoardSetup is what a board asks to be asked, in its own property. Only the
// kinds and their order are the board's to choose; what each one does is this
// side's.
type BoardSetup struct {
	Steps []BoardSetupStep `json:"steps"`
}

// BoardSetupStep is one entry of that list.
type BoardSetupStep struct {
	Kind string `json:"kind"`
	// Hint is a sentence of the board's own, shown beside the step's usual
	// explanation — "the agent that writes your newsletter", say. It is text and
	// nothing else: a board cannot describe a field, only comment on one.
	Hint string `json:"hint,omitempty"`
	// Required overrides an optional step for this board: a board whose route
	// deploys cannot be set up without somewhere to deploy to.
	Required bool `json:"required,omitempty"`
}

// SetupStep is one resolved step: what to ask, whether it may be skipped, and
// whether it still needs asking at all.
type SetupStep struct {
	Kind     string `json:"kind"`
	Optional bool   `json:"optional"`
	Hint     string `json:"hint,omitempty"`
	Status   string `json:"status"`
	// Ready says this machine can already answer the step, however this board
	// has answered it — see registryFilled.
	Ready bool `json:"ready"`
}

// SetupPlan is the whole answer to "what should this board ask for".
type SetupPlan struct {
	BoardID string      `json:"boardId"`
	Steps   []SetupStep `json:"steps"`
	// Declared says the board named these steps itself. When it did not, the
	// plan is the ordinary one — which is a guess, and the difference is worth
	// keeping visible.
	Declared bool `json:"declared"`
	// Automated says there is anything to set up at all: an agent registered on
	// this machine, or a board that asks for steps of its own. A board on a
	// machine with no agent is a kanban board, and the wizard does not open
	// itself for one.
	Automated bool `json:"automated"`
	// Offered says the wizard has already opened itself for this board once.
	// Closing it half-way answers nothing, but it is still an answer to "have
	// you seen this?", and asking again on every launch is how a dialog becomes
	// something people close on reflex.
	Offered bool `json:"offered"`
}

// SetupStepState is what a person did with a step, remembered per board.
type SetupStepState struct {
	BoardID string
	Step    string
	Status  string
	At      time.Time
}

// SetupPlanFor resolves the plan for a board. It reads the board (for what it
// asks and what it runs), the registries (for what is already answered) and the
// store (for what was skipped).
func (m *Manager) SetupPlanFor(boardID string) SetupPlan {
	plan := SetupPlan{BoardID: boardID}

	// A template is not a board being run, it is a board being written: what a
	// copy of it will need depends on the copy, and the wizard would be asking
	// about a board that does not exist yet. Nothing to set up, so nothing is
	// asked and nothing opens by itself.
	if m.isBoardTemplate(boardID) {
		return plan
	}

	declared := m.boardSetup(boardID)
	steps := setupSteps(declared)
	plan.Declared = len(declared.Steps) > 0
	plan.Automated = plan.Declared || len(m.Agents()) > 0

	states := m.setupStates(boardID)
	plan.Offered = states[setupWizardStep] != ""
	for _, step := range steps {
		if retiredSetupSteps[strings.ToLower(strings.TrimSpace(step.Kind))] {
			continue
		}
		def, ok := SetupStepDefinition(step.Kind)
		if !ok {
			// A board asking for something this build cannot do is not an
			// error: the app is older (or newer) than the board, and the rest
			// of the plan is still worth walking.
			m.log.Warn("acp: the board asks for a setup step that does not exist", "board", boardID, "step", step.Kind)
			continue
		}
		optional := def.Optional && !step.Required
		plan.Steps = append(plan.Steps, SetupStep{
			Kind:     def.Kind,
			Optional: optional,
			Hint:     strings.TrimSpace(step.Hint),
			Status:   m.setupStatus(def, states),
			Ready:    def.Registry != "" && m.registryFilled(def.Registry),
		})
	}
	return plan
}

// boardSetup is what a board asks to be asked, in its own property. A board
// that says nothing gets the ordinary plan.
func (m *Manager) boardSetup(boardID string) BoardSetup {
	var declared BoardSetup
	props := m.boardProperties(boardID)
	if raw, ok := boardProp(props, BoardPropSetup); ok {
		if err := reinterpret(raw, &declared); err != nil {
			m.log.Warn("acp: the board's setup steps are unreadable", "board", boardID, "err", err)
			return BoardSetup{}
		}
	}
	return declared
}

// agentNamed finds a registry entry by name, the way every other lookup does:
// case-insensitively, because the name is what a person typed.
func (m *Manager) agentNamed(name string) (AgentEntry, bool) {
	for _, a := range m.Agents() {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return AgentEntry{}, false
}

// setupSteps is what this board asks to be asked: what the stages it has now
// need, with what the board said about them laid over it.
//
// The stages decide *which* questions there are, because they are the board as
// it is now; a declaration is written once, before the board exists, and would
// run stale in both directions — a board that grew a deploy stage later never
// asked for a host, and one whose deploy column was deleted asked for ever.
// The declaration decides what the questions *say*: a hint, and a step the
// board insists on.
//
// None of it makes a question an obligation. An inferred step keeps the closed
// set's answer about being optional, because a stage nobody has configured runs
// nothing and a person works the card there by hand.
func setupSteps(declared BoardSetup) []BoardSetupStep {
	// A board that names its steps names the whole plan: what is left to infer
	// is the same for every board, and laying it over a board that has said
	// something would be answering a question nobody asked.
	if len(declared.Steps) > 0 {
		return append([]BoardSetupStep{{Kind: SetupStepName}}, declared.Steps...)
	}

	said := make(map[string]BoardSetupStep, len(declared.Steps))
	for _, step := range declared.Steps {
		said[strings.ToLower(strings.TrimSpace(step.Kind))] = step
	}
	out := make([]BoardSetupStep, 0, len(SetupStepDefs))
	add := func(step BoardSetupStep) {
		if own, ok := said[strings.ToLower(step.Kind)]; ok {
			step.Hint, step.Required = own.Hint, own.Required
		}
		out = append(out, step)
	}
	for _, step := range impliedSetup() {
		if step.Kind == SetupStepDone {
			continue // last, after anything only the board can ask for
		}
		add(step)
	}
	for _, def := range SetupStepDefs {
		if _, wanted := said[def.Kind]; wanted && !inferredSetupKinds[def.Kind] {
			add(BoardSetupStep{Kind: def.Kind})
		}
	}
	return append(out, BoardSetupStep{Kind: SetupStepDone})
}

// inferredSetupKinds are the steps impliedSetup can work out from the board's
// stages. Anything else a board declares is kept because nothing else could
// have produced it.
var inferredSetupKinds = map[string]bool{
	SetupStepName:   true,
	SetupStepAgent:  true,
	SetupStepDeploy: true,
	SetupStepDone:   true,
}

// impliedSetup is what the board's own stages need. A board with no automation
// at all rules nothing out, so it is offered every step — it has said nothing
// about the machine either.
func impliedSetup() []BoardSetupStep {
	// Every board is asked its name, whatever it holds: it is the one question
	// that is not about the machine, and a board has to be told apart from the
	// others in the sidebar. The rest is what an agent needs to work a card of
	// it at all: somebody to do it. Where is the card's question, not the
	// board's — without an answer it works in the board's drafts folder.
	return []BoardSetupStep{
		{Kind: SetupStepName},
		{Kind: SetupStepAgent},
		{Kind: SetupStepDone},
	}
}

// setupStatus answers a step from the machine first and the record second: a
// registry that already has an entry is an answered question however it came to
// be answered, and only a step nobody can answer for you is read off the record.
func (m *Manager) setupStatus(def SetupStepDef, states map[string]string) string {
	if status, ok := states[def.Kind]; ok {
		return status
	}
	if def.Kind == SetupStepDone {
		return SetupDone
	}
	return SetupPending
}

// registryFilled says the machine can already answer this step — there is a
// agent, somewhere to deploy to. It is deliberately not
// the same as the step being *answered*: setting up one board must not mark the
// next one set up, or every board after the first is created in silence, which
// is what this used to do. The wizard shows it as "already registered" and lets
// the step be passed with one click; the status stays this board's own answer.
func (m *Manager) registryFilled(registry string) bool {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	switch registry {
	case "agents":
		return len(m.cfg.Agents) > 0
	case "deploys":
		return len(m.cfg.Deploys) > 0
	}
	return false
}

func (m *Manager) setupStates(boardID string) map[string]string {
	out := map[string]string{}
	if m.store == nil {
		return out
	}
	states, err := m.store.SetupSteps(boardID)
	if err != nil {
		m.log.Warn("acp: cannot read what was already set up", "board", boardID, "err", err)
		return out
	}
	for _, st := range states {
		out[st.Step] = st.Status
	}
	return out
}

// MarkSetupOffered remembers that the wizard opened itself for this board, so
// it does not do so again — on the next board, on the next launch, or after the
// page's storage is cleared.
func (m *Manager) MarkSetupOffered(boardID string) error {
	if m.store == nil {
		return nil
	}
	return m.store.SaveSetupStep(SetupStepState{BoardID: boardID, Step: setupWizardStep, Status: "offered"})
}

// RecordSetupStep remembers what was done with a step. Skipping is the one
// answer nothing else can be read off: a registry says whether it was filled,
// but only a person can say they meant to leave it empty.
func (m *Manager) RecordSetupStep(boardID, step, status string) error {
	if _, ok := SetupStepDefinition(step); !ok {
		return fmt.Errorf("unknown setup step %q", step)
	}
	switch status {
	case SetupPending, SetupDone, SetupSkipped:
	default:
		return fmt.Errorf("unknown step state %q", status)
	}
	if m.store == nil {
		return nil
	}
	return m.store.SaveSetupStep(SetupStepState{BoardID: boardID, Step: step, Status: status})
}

// isBoardTemplate reports whether the board is a template. A build with no
// board reader wired answers "no": that is what every board was before this was
// asked, and a false "yes" would silence the wizard everywhere.
func (m *Manager) isBoardTemplate(boardID string) bool {
	if boardID == "" || m.meta == nil {
		return false
	}
	parent := m.rootCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	isTemplate, err := m.forBoard(boardID).Meta.IsBoardTemplate(ctx, boardID)
	if err != nil {
		m.log.Warn("acp: cannot tell whether the board is a template", "board", boardID, "err", err)
		return false
	}
	return isTemplate
}

// boardProperties reads a board's own settings, or nothing if there is no board
// reader wired (a test, or a build without one).
func (m *Manager) boardProperties(boardID string) map[string]any {
	if boardID == "" || m.meta == nil {
		return nil
	}
	// A nil parent context is a panic rather than a timeout, and the UI may ask
	// before the trigger loop has started.
	parent := m.rootCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	props, err := m.forBoard(boardID).Meta.BoardProperties(ctx, boardID)
	if err != nil {
		m.log.Warn("acp: cannot read the board's own settings", "board", boardID, "err", err)
		return nil
	}
	return props
}
