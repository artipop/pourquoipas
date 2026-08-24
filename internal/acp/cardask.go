package acp

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// A transition that asks before it lets the card through.
//
// Everything else on a route is decided without a person: the stage runs, the
// outcome is written, the edge is chosen. This is the one place where the route
// stops and asks, and it exists because some decisions are neither the agent's
// nor the board's — how the work is going to be arranged, which of two ways
// this particular card should take. They used to be settled once for a folder
// or once for a board, which is the wrong grain for a decision made per card.
//
// It hangs off the arrow rather than off the stage, because one stage is
// entered in more than one way and the answers differ: the way into work from
// the backlog is where somebody decides how the work will be arranged, and the
// same card coming back from a review has that decided already.
//
// The card waits where it is. Nothing is started, the outcome is already
// written, and the card shows the question on its own route strip — the same
// place the two outcome buttons are drawn, and for the same reason: the answer
// belongs on the card's face rather than three clicks inside a field.

const (
	// AskWorkMode is the machine's own question: how this card's work is
	// arranged in the folder — a copy of its own, or a branch in the folder
	// itself. The answer is not a card property, because it is not a fact
	// about the work but what the workspace is made of: it goes onto the
	// card's flow state, from there into the checkout ClaimWorkspace makes,
	// and from the checkout into everything downstream — the review's diff and
	// the deploy both read the card's checkout and nothing else.
	AskWorkMode = "workmode"

	// AskProperty asks for a value of one of the board's own select
	// properties, and the answer is written onto the card. That puts it in the
	// route's dataflow like any other write: a later stage reads it as an
	// input, and an edge can branch on it.
	AskProperty = "property"
)

// FlowAsk is a transition held up by a question: everything needed to finish
// the move once the last answer arrives.
type FlowAsk struct {
	// ToNode is the stage the card is going to, and On/Detail/Said are the
	// transition it is going on — kept whole so answering resumes exactly the
	// move that was interrupted rather than working it out again from a route
	// somebody may have edited in the meantime.
	ToNode string `json:"toNode"`
	On     string `json:"on,omitempty"`
	Detail string `json:"detail,omitempty"`
	Said   string `json:"said,omitempty"`

	// Move says the card still has to be carried into the target column. False
	// when a person dragged it there themselves — the board already shows it
	// where it is going, and only the stage is being held.
	Move bool `json:"move"`

	Questions []AskQuestion `json:"questions"`

	AskedAt time.Time `json:"askedAt"`
}

// AskQuestion is one question with its answers resolved. The options are worked
// out when the question is raised rather than when it is drawn, for the reason
// a stage's brief names a select's values: the board is read once, by the side
// that has it, and whoever answers — the card, a phone — needs no second
// source.
type AskQuestion struct {
	ID       string      `json:"id"`
	Kind     string      `json:"kind"`
	Property string      `json:"property,omitempty"`
	Text     string      `json:"text"`
	Options  []AskOption `json:"options"`

	// Answer is what was picked. Empty means nobody has.
	Answer string `json:"answer,omitempty"`
}

// AskOption is one answer offered. Value is what is stored — a work mode, an
// option name; Label is what the person reads.
type AskOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Pending reports whether anything is still unanswered.
func (a *FlowAsk) Pending() bool {
	return a != nil && a.Next() != nil
}

// Next is the question being asked now. One at a time, which is the shape every
// other pick in this app has: the folder, the agent, the two outcome buttons.
func (a *FlowAsk) Next() *AskQuestion {
	if a == nil {
		return nil
	}
	for i := range a.Questions {
		if a.Questions[i].Answer == "" {
			return &a.Questions[i]
		}
	}
	return nil
}

// askQuestions turns what an arrow declares into what a person can answer.
//
// A question this card cannot be asked is dropped rather than parked on: a
// select that has left the board, a work mode asked about a folder that is not
// a repository. The route carried cards across this arrow before anybody wrote
// a question on it, and an unanswerable question must not be the thing that
// stops it — it is said in the log, where whoever wrote it can find it.
func (m *Manager) askQuestions(ev CardMoved, edge *FlowEdge) []AskQuestion {
	if edge == nil || len(edge.Ask) == 0 {
		return nil
	}
	var props []CardProperty
	out := make([]AskQuestion, 0, len(edge.Ask))
	for _, a := range edge.Ask {
		q := AskQuestion{ID: a.ID, Kind: a.Kind, Property: a.Property, Text: strings.TrimSpace(a.Text)}
		switch a.Kind {
		case AskWorkMode:
			workdir, _ := m.resolveWorkdir(ev)
			if m.WorkModeFor(ev.BoardID, workdir) == WorkModePlain {
				m.log.Info("acp: the work-mode question does not apply to this card's folder",
					"card", ev.CardID, "workdir", workdir)
				continue
			}
			if q.Text == "" {
				q.Text = "Как агенту работать с папкой для этой карточки?"
			}
			q.Options = workModeOptions()
		case AskProperty:
			if props == nil {
				props = m.boardCardProperties(ev.BoardID)
			}
			values := selectValues(props, a.Property)
			if len(values) == 0 {
				m.log.Warn("acp: a transition asks about a property the board has no closed set for",
					"card", ev.CardID, "property", a.Property)
				continue
			}
			if q.Text == "" {
				q.Text = fmt.Sprintf("Что выбрать в «%s» для этой карточки?", a.Property)
			}
			for _, v := range values {
				q.Options = append(q.Options, AskOption{Value: v, Label: v})
			}
		default:
			continue
		}
		out = append(out, q)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// workModeOptions is the machine's own question, in the words the folder list
// asks it in. Two answers and no third: "work in it as it stands" is what an
// ordinary folder does, and offering it for a repository would make which of
// the two decides a rule instead of a fact.
func workModeOptions() []AskOption {
	return []AskOption{
		{Value: WorkModeWorktree, Label: "своя копия папки"},
		{Value: WorkModeBranch, Label: "ветка в самой папке"},
	}
}

// selectValues is the closed set of a board property, by name.
func selectValues(props []CardProperty, name string) []string {
	name = strings.TrimSpace(name)
	for _, p := range props {
		if strings.EqualFold(strings.TrimSpace(p.Name), name) {
			return p.Values
		}
	}
	return nil
}

// holdForAsk parks the card at a transition whose arrow asks something. It
// reports whether the card was held; false means nothing was asked and the
// caller carries on exactly as it did before this existed.
func (m *Manager) holdForAsk(ev CardMoved, edge *FlowEdge, flowID string, to FlowNode, move bool, on, detail, said string) bool {
	questions := m.askQuestions(ev, edge)
	if len(questions) == 0 {
		return false
	}
	ask := &FlowAsk{
		ToNode: to.ID, On: on, Detail: detail, Said: said, Move: move,
		Questions: questions, AskedAt: time.Now(),
	}

	// The card stays on the stage it is leaving: it has not entered anything,
	// and the flow state is what says where it stands.
	st, ok, err := m.flowState(ev.CardID)
	if err != nil {
		m.log.Warn("acp: cannot read where the card stands on its route",
			"card", ev.CardID, "node", to.ID, "err", err)
		return false
	}
	if !ok {
		// The way onto the route is the one transition crossed before the card
		// has a place on the route at all, so there is no record to write the
		// question onto and one is made here. It stands on no stage, which is
		// the truth: the card is being held before the first one.
		st = FlowState{CardID: ev.CardID, BoardID: ev.BoardID, FlowID: flowID}
	}
	if st.Ask != nil && st.Ask.ToNode == to.ID && st.Ask.Pending() {
		// Already held at this very transition. A second board event about the
		// same card — a property touched while it waits — must not reset the
		// answers or announce the question again.
		return true
	}
	st.Ask = ask
	m.saveFlowState(st)

	m.log.Info("acp: card held at a transition until somebody answers",
		"card", ev.CardID, "to", to.ID, "questions", len(questions))
	m.handOverQuestion(ev, askAttentionText(questions[0], to.Column), &questions[0])
	return true
}

// askAttentionText is what the notification and «Ждут» say. It names the stage
// the answer leads to, because the question on its own does not say what is
// being held up by it.
func askAttentionText(q AskQuestion, column string) string {
	return fmt.Sprintf("перед стадией «%s» нужен ответ: %s", column, q.Text)
}

// AnswerCardAsk is a person answering one of those questions. The last answer
// finishes the transition that was interrupted.
func (m *Manager) AnswerCardAsk(ctx context.Context, cardID, questionID, value string) error {
	st, ok, err := m.flowState(cardID)
	if err != nil {
		return err
	}
	if !ok || st.Ask == nil {
		return fmt.Errorf("карточку никто ни о чём не спрашивает")
	}
	q := st.Ask.Next()
	if q == nil {
		return fmt.Errorf("на все вопросы уже ответили")
	}
	if questionID != "" && questionID != q.ID {
		return fmt.Errorf("сейчас спрашивают о другом")
	}
	value = strings.TrimSpace(value)
	if !hasOption(q.Options, value) {
		return fmt.Errorf("такого ответа нет среди предложенных")
	}
	q.Answer = value

	// The answer is put where it belongs before the card moves, so the stage
	// that starts on the other side of the arrow already sees it: a property
	// on the card, a work mode on the card's flow state, which is what the
	// first claim of a workspace reads.
	if err := m.applyAnswer(ctx, st.CardID, *q); err != nil {
		return err
	}
	if q.Kind == AskWorkMode {
		st.WorkMode = value
	}

	if st.Ask.Pending() {
		m.saveFlowState(st)
		m.emitAskRaised(st)
		return nil
	}

	// Two people answering the last question at once would move the card
	// twice: the read above and the write below are not one step, and the
	// window between them is a board round trip wide. One answer releases the
	// card, the same way one event moves it once.
	if !m.claimStage(cardID, fmt.Sprintf("ask|%s|%s", cardID, q.ID)) {
		return nil
	}

	ask := *st.Ask
	st.Ask = nil
	m.saveFlowState(st)
	m.resumeAfterAsk(st, ask)
	return nil
}

// applyAnswer writes an answer that belongs to the board onto the card. The
// write is silent, like every other write of ours: the route is about to move
// the card itself, and a notify would set the column's automation off a second
// time for the same decision.
func (m *Manager) applyAnswer(ctx context.Context, cardID string, q AskQuestion) error {
	if q.Kind != AskProperty || m.writer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := m.forCard(cardID).Writer.SetCardFields(ctx, cardID, map[string]string{q.Property: q.Answer}); err != nil {
		return fmt.Errorf("не удалось записать ответ в «%s»: %w", q.Property, err)
	}
	return nil
}

// resumeAfterAsk finishes the move the question interrupted. The card is read
// again rather than remembered: an answer written onto it a moment ago is part
// of what the next stage reads, and a conditional edge further on asks about
// the card as it is now.
func (m *Manager) resumeAfterAsk(st FlowState, ask FlowAsk) {
	flow, ok := m.FlowByID(st.FlowID)
	if !ok {
		m.stallCardSoft(st.CardID, st.NodeID, "маршрут карточки исчез, пока она ждала ответа")
		return
	}
	node, ok := flow.Node(ask.ToNode)
	if !ok {
		m.stallCardSoft(st.CardID, st.NodeID,
			"стадия, куда ехала карточка, исчезла из маршрута — карточка осталась на месте")
		return
	}
	if m.reader == nil {
		return
	}
	ctx, cancel := context.WithTimeout(m.rootOr(), 10*time.Second)
	ev, err := m.forCard(st.CardID).Reader.CardByID(ctx, st.CardID)
	cancel()
	if err != nil {
		m.log.Warn("acp: cannot read the card whose answer just arrived", "card", st.CardID, "err", err)
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.enterNode(ev, flow, node, ask.Move, ask.On, ask.Detail, ask.Said)
	}()
}

// emitAskRaised tells the windows that the question on a card has changed —
// answered, with another one behind it. It rides the session event for the
// reason emitStall does: the strip and the card's panels already refetch on it,
// and a card held at a transition is exactly the state they are drawing.
func (m *Manager) emitAskRaised(st FlowState) {
	if m.ui == nil {
		return
	}
	m.ui.Emit(EventSession, map[string]any{"cardId": st.CardID, "ask": true})
}

func hasOption(options []AskOption, value string) bool {
	for _, o := range options {
		if o.Value == value {
			return true
		}
	}
	return false
}

// cardWorkModeChoice is what a person answered about this card's workspace, if
// anything. It is read by ClaimWorkspace, and it is deliberately the card's own
// record rather than the folder's: the folder's setting is what a board does by
// default, and this is one card being an exception to it.
func (m *Manager) cardWorkModeChoice(owner string) string {
	cardID, _ := ownerColumns(owner)
	if cardID == "" {
		return ""
	}
	st, ok, err := m.flowState(cardID)
	if err != nil || !ok {
		return ""
	}
	switch st.WorkMode {
	case WorkModeWorktree, WorkModeBranch:
		return st.WorkMode
	}
	return ""
}
