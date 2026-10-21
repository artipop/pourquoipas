package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/artipop/pourquoipas/internal/boardmcp"
)

// The board tools are how an agent says something back to this application
// instead of only to the person watching it: an MCP server the app serves
// itself, over HTTP, on the front door (internal/boardmcp).
//
// What an agent may reach through it is decided here, by a grant. The token
// names the board, is minted for one agent run and dies with it, so an agent
// that read every byte of its own configuration still cannot leave cards on
// another board, and a token found afterwards opens nothing. This is the same
// bargain the dokku server takes: the model chooses steps, never targets.

// BoardGrant is one agent run's permission to work on one board.
type BoardGrant struct {
	BoardID string
	// Property is the board's column property, so a card asked for by column
	// name lands where the automation is watching. It comes from the config
	// rather than from the agent, which knows column names and nothing else.
	Property string
	// CardID is the card the run stands on, when it stands on one — a stage
	// has it, a planning conversation has not. It is what a tool call that
	// names no card means: an agent working on a card is the caller these tools
	// mostly have, and making it find its own card by title first would be an
	// invitation to act on somebody else's.
	CardID string
	// SessionID is the session this run *is*, which is how finish_work reaches
	// the stage it ends without the agent naming it. A grant is minted per run,
	// so there is exactly one.
	SessionID string
}

// GrantBoardTools opens a grant for one agent run and returns the token that
// carries it. The caller revokes it when the run ends; a grant that outlives
// its run is a door left open.
func (m *Manager) GrantBoardTools(boardID, cardID, sessionID string) string {
	if boardID == "" {
		return ""
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// Without a token there is no door, which is the safe way to fail.
		m.log.Warn("acp: cannot mint a board tools token", "err", err)
		return ""
	}
	token := hex.EncodeToString(buf)

	// Which property the columns live on is the board's answer, not the
	// machine's: cfg.TriggerProperty is one name for every board this install
	// ever sees, and its default is the Russian the status property, so a board in another
	// language — or one where somebody renamed the field — handed the agent a
	// property that does not exist there. The board records the id itself
	// (BoardPropColumnProperty), and the name in the settings is the fallback
	// for a board that predates that record.
	property := m.boardProperty(boardID, BoardPropColumnProperty)
	if property == "" {
		m.cfgMu.RLock()
		property = m.cfg.TriggerProperty
		m.cfgMu.RUnlock()
	}

	m.grantsMu.Lock()
	defer m.grantsMu.Unlock()
	if m.grants == nil {
		m.grants = map[string]BoardGrant{}
	}
	m.grants[token] = BoardGrant{BoardID: boardID, Property: property, CardID: cardID, SessionID: sessionID}
	return token
}

// RevokeBoardTools closes a grant.
func (m *Manager) RevokeBoardTools(token string) {
	if token == "" {
		return
	}
	m.grantsMu.Lock()
	defer m.grantsMu.Unlock()
	delete(m.grants, token)
}

// boardGrant resolves a token, which is the only way into everything below.
func (m *Manager) boardGrant(token string) (BoardGrant, bool) {
	m.grantsMu.RLock()
	defer m.grantsMu.RUnlock()
	g, ok := m.grants[token]
	return g, ok
}

// SetOrigin records the address the front door answers on, which is what an
// MCP server of ours is pointed at. Set once the front door is listening.
func (m *Manager) SetOrigin(url string) {
	m.grantsMu.Lock()
	defer m.grantsMu.Unlock()
	m.origin = url
}

func (m *Manager) originURL() string {
	m.grantsMu.RLock()
	defer m.grantsMu.RUnlock()
	return m.origin
}

// BoardToolsURL is where the tools are served — the front door, on the /acp/
// subtree that is already ours.
func (m *Manager) BoardToolsURL() string {
	origin := m.originURL()
	if origin == "" {
		return ""
	}
	return strings.TrimSuffix(origin, "/") + boardmcp.Path
}

// sessionBoardServer grants a session the board's tools and describes the
// server for session/new, or returns nil when the session gets none.
//
// The server is the app's own, over HTTP on the front door, so it goes by the
// protocol's HTTP transport — which an agent may not speak: that is a
// capability it declares in initialize, and one that does not gets no board
// server rather than one it would refuse. A deploy gets none on purpose: it
// publishes a branch, and an agent moving its own card would be a loop.
func (m *Manager) sessionBoardServer(s *Session, caps acpsdk.AgentCapabilities) *acpsdk.McpServer {
	if s.BoardID == "" || s.Deploy != nil || !caps.McpCapabilities.Http {
		return nil
	}
	url := m.BoardToolsURL()
	if url == "" {
		return nil
	}
	token := m.GrantBoardTools(s.BoardID, s.CardID, s.ID)
	if token == "" {
		return nil
	}
	s.mu.Lock()
	s.boardTools = token
	s.mu.Unlock()
	// The board's tools are the app's own and consented to by being handed
	// over; nobody is asked before each call.
	s.allowToolPrefix("mcp__" + boardmcp.ServerName + "__")
	return &acpsdk.McpServer{Http: &acpsdk.McpServerHttpInline{
		Name:    boardmcp.ServerName,
		Type:    "http",
		Url:     url,
		Headers: []acpsdk.HttpHeader{{Name: "Authorization", Value: "Bearer " + token}},
	}}
}

// CheckBoardTools reports whether a token still opens a door, which is what
// the HTTP end answers a call with before it reads a body.
func (m *Manager) CheckBoardTools(token string) error {
	if _, ok := m.boardGrant(token); !ok {
		return fmt.Errorf("no access to the board")
	}
	return nil
}

// BoardToolColumn is one column as an agent is told about it: the name it must
// use and what putting a card there sets off.
type BoardToolColumn struct {
	Name   string   `json:"name"`
	Action string   `json:"action,omitempty"`
	Agents []string `json:"agents,omitempty"`
}

// BoardToolColumns answers "where can this card go, and what happens then".
// The columns come from the board's own configuration rather than from its
// schema: a column with no action is a place to park a card, and the agent
// needs to know which is which to put work where work starts.
func (m *Manager) BoardToolColumns(token string) ([]BoardToolColumn, error) {
	g, ok := m.boardGrant(token)
	if !ok {
		return nil, fmt.Errorf("no access to the board")
	}
	names := m.boardColumnNames(g.BoardID)
	out := make([]BoardToolColumn, 0, len(names))
	for _, name := range names {
		out = append(out, BoardToolColumn{Name: name})
	}
	return out, nil
}

// BoardToolCard is one card as an agent is told about it: named values only,
// because names are the whole vocabulary these tools have.
type BoardToolCard struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Column  string   `json:"column,omitempty"`
	Options []string `json:"options,omitempty"`
	Body    string   `json:"body,omitempty"` // only when one card was asked about
	// Mine says this is the card the agent's own run stands on — the one a call
	// that names no card acts on.
	Mine bool `json:"mine,omitempty"`

	// Whether an agent is working this card right now.
	Running bool `json:"running,omitempty"`
}

// BoardToolField is one property of the granted board's cards, and whether a
// value may be put in it.
type BoardToolField struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Values   []string `json:"values,omitempty"`
	Writable bool     `json:"writable"`
}

// BoardToolFields is the board's card properties as an agent needs them: what a
// value may be, and which of them this stage has to report. The stage half
// comes from the grant — the conversation names the stage waiting on it — so an
// agent that asks gets an answer about its own run and not about the board in
// general.
//
// It exists because nothing else describes a property. The brief names the
// declared writes (stageOutputs), and an agent that wants to check what a
// select accepts before reporting had no way to but to be refused.
func (m *Manager) BoardToolFields(ctx context.Context, token string) ([]BoardToolField, error) {
	g, ok := m.boardGrant(token)
	if !ok {
		return nil, fmt.Errorf("no access to the board")
	}
	if m.meta == nil {
		return nil, fmt.Errorf("the board is not available")
	}
	props, err := m.meta.CardProperties(ctx, g.BoardID)
	if err != nil {
		return nil, err
	}
	out := make([]BoardToolField, 0, len(props))
	for _, p := range props {
		// The column property is left out of the writable set on purpose: a
		// card is moved with move_card, and finish_work refuses it
		// (FinishWorkFromTools).
		out = append(out, BoardToolField{
			Name: p.Name, Kind: p.Type, Values: p.Values,
			Writable: writableProperty(p.Type) && !strings.EqualFold(p.Name, g.Property),
		})
	}
	return out, nil
}

// writableProperty says a stage can put a value in this kind of field at all.
// SetCardFields resolves a select by one of its option names and stores
// everything else as text, so a date or a person would take a string the board
// cannot read back, and createdBy and its kin are the board's own bookkeeping.
// The same closed set the editor offers (WRITABLE_TYPES in automation.ts).
func writableProperty(kind string) bool {
	switch kind {
	case "text", "url", "email", "phone", "number", "select":
		return true
	default:
		return false
	}
}

// BoardToolCards lists the granted board's cards, optionally only those in one
// column. Nothing else an agent can do to a card is possible without this: every
// other call takes an id, and an id is not something a conversation carries.
func (m *Manager) BoardToolCards(ctx context.Context, token, column string) ([]BoardToolCard, error) {
	g, ok := m.boardGrant(token)
	if !ok {
		return nil, fmt.Errorf("no access to the board")
	}
	if m.reader == nil {
		return nil, fmt.Errorf("the board is not available")
	}
	events, err := m.forBoard(g.BoardID).Reader.CardsForBoard(ctx, g.BoardID)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	running := make(map[string]bool, len(m.byCard))
	for cardID := range m.byCard {
		running[cardID] = true
	}
	m.mu.Unlock()

	column = strings.TrimSpace(column)
	out := make([]BoardToolCard, 0, len(events))
	for _, ev := range events {
		card := m.toolCard(g, ev)
		if column != "" && !strings.EqualFold(card.Column, column) {
			continue
		}
		card.Running = running[ev.CardID]
		out = append(out, card)
	}
	return out, nil
}

// BoardToolCardByID is one card in full, its description included. A cardID of
// "" is the run's own card, which is what an agent working on one means when it
// asks about "the card".
func (m *Manager) BoardToolCardByID(ctx context.Context, token, cardID string) (BoardToolCard, error) {
	g, ev, err := m.grantedCard(ctx, token, cardID)
	if err != nil {
		return BoardToolCard{}, err
	}
	card := m.toolCard(g, ev)
	card.Body = ev.Body
	m.mu.Lock()
	card.Running = m.byCard[ev.CardID] != nil
	m.mu.Unlock()
	return card, nil
}

// UpdateCardFromTools changes a card: its title, its column, its other named
// values. A cardID of "" is the run's own card.
//
// Moving a card is this same call with a column in it, and that is deliberate:
// a column is a property like the others, and what makes moving special is not
// how it is written but what the board does about it afterwards.
func (m *Manager) UpdateCardFromTools(ctx context.Context, token, cardID string, edit CardEdit) error {
	g, ev, err := m.grantedCard(ctx, token, cardID)
	if err != nil {
		return err
	}
	if m.writer == nil {
		return fmt.Errorf("the board is not available")
	}
	if strings.TrimSpace(edit.Title) == "" && strings.TrimSpace(edit.Column) == "" && len(edit.Options) == 0 {
		return fmt.Errorf("nothing to change was given")
	}
	edit.Property = g.Property
	if err := m.forCard(ev.CardID).Writer.UpdateCard(m.asGrantedAgent(ctx, g), ev.CardID, edit); err != nil {
		return err
	}
	// A move the agent made is the result of its run, not a new one: the board
	// will say the card moved, and without this the card would come straight
	// back to the agent that has just finished with it (cardplace.go).
	if strings.TrimSpace(edit.Column) != "" {
		m.noteAgentMove(ev.CardID)
	}
	m.log.Info("acp: card changed by an agent", "card", ev.CardID, "board", g.BoardID, "column", edit.Column)
	return nil
}

// SetFieldsFromTools writes a card's properties by name. A name the board does
// not have yet becomes a new text property first — that is how an agent adds a
// field. The column is refused here as it is in finish_work: the card moves by
// its outcome or by move_card, which is the one write that notifies.
func (m *Manager) SetFieldsFromTools(ctx context.Context, token, cardID string, fields map[string]string) error {
	g, ev, err := m.grantedCard(ctx, token, cardID)
	if err != nil {
		return err
	}
	if len(fields) == 0 {
		return fmt.Errorf("nothing to write was given")
	}
	if g.Property != "" {
		if _, taken := fieldsHave(fields, g.Property); taken {
			return fmt.Errorf("the column (%s) is not changed here: move_card moves the card", g.Property)
		}
	}
	if m.writer == nil {
		return fmt.Errorf("the board is not available")
	}
	writer := m.forCard(ev.CardID).Writer
	ctx = m.asGrantedAgent(ctx, g)
	known := map[string]bool{}
	for _, p := range m.boardCardProperties(g.BoardID) {
		known[strings.ToLower(p.Name)] = true
	}
	for name := range fields {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("a field needs a name")
		}
		if known[strings.ToLower(name)] {
			continue
		}
		if err := writer.AddCardProperty(ctx, ev.CardID, name); err != nil {
			return err
		}
	}
	if err := writer.SetCardFields(ctx, ev.CardID, fields); err != nil {
		return err
	}
	m.log.Info("acp: card fields written by an agent", "card", ev.CardID, "board", g.BoardID, "fields", len(fields))
	return nil
}

// CommentFromTools writes on a card in the agent's own voice. It is how an agent
// says something that belongs to the card rather than to the conversation it is
// having — the same place a session's report goes, so a person reading the card
// later finds one history and not two.
func (m *Manager) CommentFromTools(ctx context.Context, token, cardID, text string) error {
	g, ev, err := m.grantedCard(ctx, token, cardID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("the comment is empty")
	}
	if m.writer == nil {
		return fmt.Errorf("the board is not available")
	}
	if err := m.forCard(ev.CardID).Writer.AddComment(m.asGrantedAgent(ctx, g), ev.CardID, text); err != nil {
		return err
	}
	// On its own card, this is the stage's report, and the one comment the
	// stage leaves (commentFirstTurn).
	if g, ok := m.boardGrant(token); ok && g.CardID == ev.CardID {
		if s := m.session(g.SessionID); s != nil {
			s.markCommented()
		}
	}
	return nil
}

// CreateCardFromTools is the write itself. Everything an agent may decide is a
// name a person would have typed; the board is the grant's.
func (m *Manager) CreateCardFromTools(ctx context.Context, token string, card NewCard) (string, error) {
	g, ok := m.boardGrant(token)
	if !ok {
		return "", fmt.Errorf("no access to the board")
	}
	if strings.TrimSpace(card.Title) == "" {
		return "", fmt.Errorf("a card needs a title")
	}
	if m.writer == nil {
		return "", fmt.Errorf("the board is not available")
	}
	card.BoardID = g.BoardID
	card.Property = g.Property

	id, err := m.forBoard(card.BoardID).Writer.CreateCard(m.asGrantedAgent(ctx, g), card)
	if err != nil {
		return "", err
	}
	m.log.Info("acp: card created by an agent", "card", id, "board", g.BoardID, "column", card.Column)
	return id, nil
}

// grantedCard resolves the card a call names — or the run's own card, when it
// names none — and refuses one that is not on the granted board. That check is
// what keeps the grant a board and not a doorway: card ids are guessable in the
// sense that an agent may read one anywhere, and without this a token for one
// board would edit cards on every other.
func (m *Manager) grantedCard(ctx context.Context, token, cardID string) (BoardGrant, CardMoved, error) {
	g, ok := m.boardGrant(token)
	if !ok {
		return BoardGrant{}, CardMoved{}, fmt.Errorf("no access to the board")
	}
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		cardID = g.CardID
	}
	if cardID == "" {
		return g, CardMoved{}, fmt.Errorf("no card was given")
	}
	if m.reader == nil {
		return g, CardMoved{}, fmt.Errorf("the board is not available")
	}
	ev, err := m.forCard(cardID).Reader.CardByID(ctx, cardID)
	if err != nil {
		return g, CardMoved{}, fmt.Errorf("card %s was not found", cardID)
	}
	if ev.BoardID != g.BoardID {
		return g, CardMoved{}, fmt.Errorf("card %s is not on this board", cardID)
	}
	return g, ev, nil
}

// toolCard is a card read back the way it was written: the column by name, and
// everything else selected on it by name too, minus the column so it is not said
// twice.
func (m *Manager) toolCard(g BoardGrant, ev CardMoved) BoardToolCard {
	card := BoardToolCard{
		ID:    ev.CardID,
		Title: ev.Title,
		Mine:  g.CardID != "" && ev.CardID == g.CardID,
	}
	if g.Property != "" {
		// The grant names the column property the way the board records it — by
		// id — so Values is where to look; Props is keyed by lowercased *name*
		// and answers for a grant that fell back to the config's name. Both are
		// consulted because a board that predates BoardPropColumnProperty has
		// only the second.
		//
		// Either way the board upper-cases a select value on the way out, so the
		// agent is told the option's own name — that is the name it has to send
		// back.
		card.Column = ev.Values[g.Property]
		if card.Column == "" {
			card.Column = ev.Props[strings.ToLower(g.Property)]
		}
		for _, name := range ev.OptionNames {
			if strings.EqualFold(name, card.Column) {
				card.Column = name
				break
			}
		}
	}
	for _, name := range ev.OptionNames {
		if card.Column != "" && strings.EqualFold(name, card.Column) {
			continue
		}
		card.Options = append(card.Options, name)
	}
	return card
}
