package acp

import "strings"

// The routes an install starts with. They exist so a fresh board is not a blank
// registry: the the developer template board template ships the columns these
// routes name and a "Workflow" property whose options are their names, so the
// whole mechanism can be seen — and picked — before anything is configured.
//
// Nothing is imposed by seeding them. A card takes a route only when it names
// one (its "Workflow" option), so a board that ignores the property behaves
// exactly as it did before: the standalone trigger columns and nothing else.
// Several routes also mean the "single registered flow" fallback in resolveFlow
// stays out of the way.
//
// Columns come from the config wherever the config already names one, so
// renaming triggerColumn renames the stage with it; the two stages with no
// config key of their own are named here.
const (
	// The columns a template route stands on. They live here, with the thing
	// that creates boards, rather than in the machine's settings — where they
	// used to be five keys naming the columns of anybody's board and matching
	// them by name.
	TemplateWorkColumn   = "В работе"
	TemplateDeployColumn = "Деплой"
	TemplateTestColumn   = "QA"

	TemplateReviewColumn = "На ревью"
	TemplateDoneColumn   = "Готово 🙌"
)

// There were three more: «Проверено», «Не прошло» and «Заблокировано». Each was
// a column whose whole content was one fact about the stage before it — how
// that stage ended — and a fact is not a place where work happens. It reads as
// one on the board, where it is a column nobody works in, and on the canvas,
// where it is a stage that «ждёт» for ever.
//
// The engine writes the outcome onto the card instead (BoardPropOutcome), for
// every stage and without anybody declaring it, so a route no longer needs a
// column to say "this failed". A stage whose outcome has no edge parks the card
// where it is and says why (stallCardSoft) — which is what those columns were
// standing in for, and it does it without adding a place to the board.

// Flow names, which are also the options of the board template's "Workflow"
// property — the two lists are kept in step by TestTemplateFlowsMatchTheBoardTemplate.
const (
	TemplateFlowFeature = "Фича"
	TemplateFlowHotfix  = "Хотфикс"
	TemplateFlowReview  = "Только ревью"
)

// flowBuilder assembles one route, quietly dropping stages whose column the
// config leaves empty and any transition that would then dangle.
type flowBuilder struct {
	flow FlowEntry
}

func newFlow(name, property string) *flowBuilder {
	return &flowBuilder{flow: FlowEntry{Name: name, Property: property}}
}

// node adds a stage and returns its id, or "" when there is no column to put it
// on. Ids are stable strings rather than generated ones: they are what the
// engine records in flow_state, so a reseeded config must not invalidate them.
func (b *flowBuilder) node(id, column, action string) string {
	if strings.TrimSpace(column) == "" {
		return ""
	}
	for _, n := range b.flow.Nodes {
		if strings.EqualFold(n.Column, column) {
			return n.ID // two config keys naming one column: keep the first
		}
	}
	b.flow.Nodes = append(b.flow.Nodes, FlowNode{ID: id, Column: column, Action: action})

	// Transition 0: the way onto the route, which every route has and none has
	// to declare. It is where a card arrives from the backlog or from a source,
	// and therefore the one crossing that happens exactly once — which is what
	// makes it the place to ask how this card's work is to be arranged. Coming
	// back to the first stage later arrives from a real stage and asks nothing.
	if len(b.flow.Nodes) == 1 {
		b.flow.Edges = append(b.flow.Edges, FlowEdge{
			From: EntryNode, To: id,
			Ask: []EdgeAsk{{ID: "workmode", Kind: AskWorkMode}},
		})
	}
	return id
}

func (b *flowBuilder) edge(from, to, on string) {
	if from == "" || to == "" || from == to {
		return
	}
	b.flow.Edges = append(b.flow.Edges, FlowEdge{From: from, To: to, On: on})
}

// passed is the other half of the pair: the same field, the other value, and
// the way on. A wait stage's forward move usually has an automatic trigger too
// — «На ревью» goes on when the branch is merged — but the automatic one is the
// world telling us, and this is a person telling us. Both are needed: the
// branch is not always merged from here, and somebody watching a review does
// not want to wait for a poll to notice.
func (b *flowBuilder) passed(from, to string) {
	if from == "" || to == "" || from == to {
		return
	}
	b.flow.Edges = append(b.flow.Edges, FlowEdge{
		From: from, To: to, On: TriggerCardChanged,
		If: &EdgeCond{Property: OutcomePropertyTitle, Value: OutcomePassed},
	})
}

// rejected is the way back from a stage where nobody works and nothing runs.
// A wait stage produces no outcome — «На ревью» has no "failed" of its own to
// leave by — so the signal is a person saying so on the card: «Исход» set to
// «не прошло». The same field the engine writes for a stage that does run, and
// the same three values, because "how did this step end" is one question
// whether a machine or a reviewer answers it.
//
// The engine's own write is silent (SetCardOption disables notify), so it can
// never fire this edge itself.
func (b *flowBuilder) rejected(from, to string) {
	if from == "" || to == "" || from == to {
		return
	}
	b.flow.Edges = append(b.flow.Edges, FlowEdge{
		From: from, To: to, On: TriggerCardChanged,
		If: &EdgeCond{Property: OutcomePropertyTitle, Value: OutcomeFailed},
	})
}

// TemplateFlows builds the routes the editor offers a board that has none. The
// column names are the templates' own; the property is the machine's default,
// which is the last thing here that is still a name rather than an id.
func TemplateFlows(cfg Config) []FlowEntry {
	var out []FlowEntry
	for _, f := range []FlowEntry{featureFlow(cfg), hotfixFlow(cfg), reviewFlow(cfg)} {
		if len(f.Nodes) > 0 {
			out = append(out, f)
		}
	}
	return out
}

// featureFlow is the long way round: write the code, wait for the branch to be
// merged, publish it, check the preview. A failed check goes back to the agent
// rather than to a person — that loop is the point of the whole thing.
func featureFlow(cfg Config) FlowEntry {
	b := newFlow(TemplateFlowFeature, cfg.TriggerProperty)
	agent := b.node("agent", TemplateWorkColumn, FlowActionAgent)
	review := b.node("review", TemplateReviewColumn, FlowActionNone)
	deploy := b.node("deploy", TemplateDeployColumn, FlowActionDeploy)
	qa := b.node("qa", TemplateTestColumn, FlowActionTest)
	done := b.node("done", TemplateDoneColumn, FlowActionNone)

	b.edge(agent, review, TriggerSuccess)
	// Merged rather than opened: the local git watcher sees it without a token,
	// so the route works on any hosting and with no credentials at all.
	b.edge(review, deploy, TriggerBranchMerged)
	b.edge(deploy, qa, TriggerSuccess)
	b.edge(qa, done, TriggerSuccess)

	// What failed goes back to the agent — that loop is the point of the whole
	// route, and it is where the work can actually be fixed. It replaced a
	// «Не прошло» column, which was the same arrow with a parking space bolted
	// to the end of it.
	b.edge(deploy, agent, TriggerFailure)
	b.edge(qa, agent, TriggerFailure)

	// A review, answered by a person: nothing runs in «На ревью», so both ways
	// out of it are somebody saying how it went. Until the first of these
	// existed the card simply stood there, which is what the route looked like
	// it was missing.
	b.rejected(review, agent)
	b.passed(review, deploy)

	// Except the agent's own stage and «проверить не удалось»: sending a task
	// back to the agent that just failed it is a loop with nothing new in it,
	// and a check that could not run is a person's to fix — no browser, no
	// preview to reach. Both park the card where it stopped with the outcome on
	// it and the reason on the strip.
	return b.flow
}

// hotfixFlow is the short way: written and published, no review and no browser
// pass. What it buys is speed, and it says so by having nowhere to check.
func hotfixFlow(cfg Config) FlowEntry {
	b := newFlow(TemplateFlowHotfix, cfg.TriggerProperty)
	agent := b.node("agent", TemplateWorkColumn, FlowActionAgent)
	deploy := b.node("deploy", TemplateDeployColumn, FlowActionDeploy)
	done := b.node("done", TemplateDoneColumn, FlowActionNone)

	b.edge(agent, deploy, TriggerSuccess)
	b.edge(deploy, done, TriggerSuccess)
	b.edge(deploy, agent, TriggerFailure)
	return b.flow
}

// reviewFlow is for work that is never deployed from here — a refactor, a
// document, a spike. It ends when the branch lands in the main one.
func reviewFlow(cfg Config) FlowEntry {
	b := newFlow(TemplateFlowReview, cfg.TriggerProperty)
	agent := b.node("agent", TemplateWorkColumn, FlowActionAgent)
	review := b.node("review", TemplateReviewColumn, FlowActionNone)
	done := b.node("done", TemplateDoneColumn, FlowActionNone)

	b.edge(agent, review, TriggerSuccess)
	b.edge(review, done, TriggerBranchMerged)
	b.rejected(review, agent)
	b.passed(review, done)
	return b.flow
}

// FlowTemplates returns the seeded routes rebuilt from the current config, for
// an install whose registry predates them: the editor offers whichever of them
// is missing, so they can be added without being retyped.
func (m *Manager) FlowTemplates() []FlowEntry {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return TemplateFlows(m.cfg)
}
