package acp

import (
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/artipop/xciii/internal/boardmcp"
)

// The grant is the whole of the door: the token names the board, so an agent
// that read its own environment still cannot write anywhere else.
func TestBoardToolsWriteOnlyToTheGrantedBoard(t *testing.T) {
	m, writer, _, _ := testManager(t, "idle", func(cfg *Config) {
		cfg.TriggerProperty = "Статус"
	})

	token := m.GrantBoardTools("board-1", "", "")
	if token == "" {
		t.Fatal("no token was minted")
	}

	id, err := m.CreateCardFromTools(t.Context(), token, NewCard{
		Title:   "Починить окно планирования",
		Body:    "Оно открывается пополам.",
		Column:  "К агенту",
		Options: []string{"xciii"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("the card was created without an id to point at")
	}

	if len(writer.created) != 1 {
		t.Fatalf("board writes: %d, want 1", len(writer.created))
	}
	got := writer.created[0]
	if got.BoardID != "board-1" {
		t.Errorf("card landed on board %q, want the granted one", got.BoardID)
	}
	// The column property is the config's, not the agent's: it names columns
	// and nothing else.
	if got.Property != "Статус" {
		t.Errorf("column property %q, want the configured one", got.Property)
	}
	if got.Column != "К агенту" || len(got.Options) != 1 {
		t.Errorf("card lost what was asked for: %+v", got)
	}
}

// A property is described to the agent so it can put a value in one without
// guessing: what a value may be, and whether it may be written at all. Without
// it an agent filling «Вердикт» learns what it accepts by being refused.
func TestBoardToolFieldsSayWhatAValueMayBe(t *testing.T) {
	m, _, _, _ := testManager(t, "idle", func(cfg *Config) {
		cfg.TriggerProperty = "Статус"
	})
	m.SetBoardMeta(&fakeBoardMeta{cardProps: []CardProperty{
		{Name: "Автор", Type: "createdBy"},
		{Name: "Вердикт", Type: "select", Values: []string{"blocked", "fail", "pass"}},
		{Name: "Превью", Type: "url"},
		{Name: "Статус", Type: "select", Values: []string{"QA", "Готово"}},
	}})

	stage := &Session{ID: "sess-1", CardID: "card-1"}
	m.mu.Lock()
	m.active = map[string]*Session{stage.ID: stage}
	m.mu.Unlock()
	token := m.GrantBoardTools("board-1", "card-1", stage.ID)

	fields, err := m.BoardToolFields(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]BoardToolField{}
	for _, f := range fields {
		by[f.Name] = f
	}

	verdict := by["Вердикт"]
	if !verdict.Writable || len(verdict.Values) != 3 {
		t.Errorf("the property's closed set is not on it: %+v", verdict)
	}
	if !by["Превью"].Writable {
		t.Errorf("an ordinary property should be writable: %+v", by["Превью"])
	}
	// Two the agent must not be told it can write: the board's own bookkeeping,
	// and the column — a card is moved with move_card, and finish_work refuses
	// the column property outright.
	if by["Автор"].Writable {
		t.Error("createdBy was offered as writable")
	}
	if by["Статус"].Writable {
		t.Error("the column property was offered as writable")
	}
}

// A conversation with no card behind it still gets the board's properties: it
// may be about to create one.
func TestBoardToolFieldsOutsideACardStillDescribeTheBoard(t *testing.T) {
	m, _, _, _ := testManager(t, "idle", nil)
	m.SetBoardMeta(&fakeBoardMeta{cardProps: []CardProperty{{Name: "Превью", Type: "url"}}})

	fields, err := m.BoardToolFields(t.Context(), m.GrantBoardTools("board-1", "", ""))
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].Name != "Превью" {
		t.Errorf("the board's properties did not arrive: %+v", fields)
	}
}

// A grant that outlived its agent run would be a door left open, so revoking is
// what closing a run means — and a card still needs a title.
func TestBoardToolsRefuseWhatTheyShould(t *testing.T) {
	m, _, _, _ := testManager(t, "idle", nil)

	if _, err := m.CreateCardFromTools(t.Context(), "not-a-token", NewCard{Title: "Задача"}); err == nil {
		t.Error("a made-up token opened the board")
	}

	token := m.GrantBoardTools("board-1", "", "")
	if _, err := m.CreateCardFromTools(t.Context(), token, NewCard{Title: "   "}); err == nil {
		t.Error("a card with no title was accepted")
	}

	m.RevokeBoardTools(token)
	if _, err := m.CreateCardFromTools(t.Context(), token, NewCard{Title: "Задача"}); err == nil {
		t.Error("a revoked grant still opened the board")
	}
	if err := m.CheckBoardTools(token); err == nil {
		t.Error("a revoked grant still checks out")
	}
}

// boardWithCards is a manager whose board can be read: two cards on the granted
// board and one on another, which is the case every card call has to refuse.
func boardWithCards(t *testing.T) (*Manager, *fakeWriter) {
	t.Helper()
	m, writer, _, _ := testManager(t, "idle", func(cfg *Config) {
		cfg.TriggerProperty = "Статус"
	})
	m.SetBoardMeta(&fakeBoardMeta{options: []Column{
		{PropertyID: "p", PropertyName: "Статус", OptionID: "opt-toagent", Name: "К агенту"},
		{PropertyID: "p", PropertyName: "Статус", OptionID: "opt-review", Name: "Ревью"},
	}})
	m.SetBoardReader(&fakeReader{cards: []CardMoved{
		{
			CardID:  "card-1",
			BoardID: "board-1",
			Title:   "Починить окно",
			Body:    "Оно открывается пополам.",
			// The board renders a select value upper-cased, which is what Props
			// carries everywhere — see toolCard.
			Props:       map[string]string{"статус": "К АГЕНТУ"},
			OptionNames: []string{"К агенту", "xciii"},
		},
		{
			CardID: "card-2", BoardID: "board-1", Title: "Вторая",
			Props:       map[string]string{"статус": "ИДЕИ"},
			OptionNames: []string{"Идеи"},
		},
		{CardID: "elsewhere", BoardID: "board-2", Title: "Чужая"},
	}})
	return m, writer
}

// An agent that finished its work moves the card on, and moving it is what sets
// the next column off — so the change has to reach the board named the way the
// board names things: a column in the configured property, values by name.
func TestBoardToolsMoveACardOnByName(t *testing.T) {
	m, writer := boardWithCards(t)
	token := m.GrantBoardTools("board-1", "card-1", "")

	// The card a call names nothing for is the run's own, which is the case an
	// agent working on one card always has.
	if err := m.UpdateCardFromTools(t.Context(), token, "", CardEdit{Column: "Ревью"}); err != nil {
		t.Fatal(err)
	}
	edit, ok := writer.cardEdit("card-1")
	if !ok {
		t.Fatalf("the run's own card was not the one changed: %+v", writer.edits)
	}
	if edit.Column != "Ревью" || edit.Property != "Статус" {
		t.Errorf("the move landed as %+v, want the column property from the config", edit)
	}

	// And another card of the same board is fair game when it is named.
	if err := m.UpdateCardFromTools(t.Context(), token, "card-2", CardEdit{Options: []string{"Одобрено"}}); err != nil {
		t.Fatal(err)
	}
	if edit, _ := writer.cardEdit("card-2"); strings.Join(edit.Options, ",") != "Одобрено" {
		t.Errorf("the value the route waits on landed as %+v", edit)
	}

	// A change that says nothing is a mistake worth naming rather than a write.
	if err := m.UpdateCardFromTools(t.Context(), token, "", CardEdit{}); err == nil {
		t.Error("an empty change was accepted")
	}
}

// The grant is a board, not a doorway: a card id read anywhere else opens
// nothing, or one board's tools would reach every other board's cards.
func TestBoardToolsRefuseACardOfAnotherBoard(t *testing.T) {
	m, writer := boardWithCards(t)
	token := m.GrantBoardTools("board-1", "card-1", "")

	if err := m.UpdateCardFromTools(t.Context(), token, "elsewhere", CardEdit{Column: "Ревью"}); err == nil {
		t.Error("a card of another board was moved")
	}
	if err := m.CommentFromTools(t.Context(), token, "elsewhere", "Привет"); err == nil {
		t.Error("a card of another board was written on")
	}
	if _, err := m.BoardToolCardByID(t.Context(), token, "elsewhere"); err == nil {
		t.Error("a card of another board was read")
	}
	if _, ok := writer.cardEdit("elsewhere"); ok {
		t.Error("another board's card was written to")
	}

	// A grant that stands on no card has no default either: a planning terminal
	// must name what it means.
	planning := m.GrantBoardTools("board-1", "", "")
	if err := m.CommentFromTools(t.Context(), planning, "", "Привет"); err == nil {
		t.Error("a comment with no card behind it was accepted")
	}
}

// Listing is how an agent finds a card at all: everything else takes an id, and
// an id is not something a conversation carries. It shows the granted board and
// says which card is the agent's own.
func TestBoardToolsListTheGrantedBoardsCards(t *testing.T) {
	m, _ := boardWithCards(t)
	token := m.GrantBoardTools("board-1", "card-1", "")

	cards, err := m.BoardToolCards(t.Context(), token, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("cards listed: %d, want the granted board's two — %+v", len(cards), cards)
	}
	first := cards[0]
	// The column comes back under the option's own name rather than the shouted
	// one the board renders, because that name is what the agent has to send back.
	if first.ID != "card-1" || first.Column != "К агенту" || !first.Mine {
		t.Errorf("the run's own card reads as %+v", first)
	}
	// The column is said once: it is its own field, not one of the values.
	if strings.Join(first.Options, ",") != "xciii" {
		t.Errorf("values read back as %v", first.Options)
	}

	// A column narrows the listing, which is what "what is waiting for review"
	// is asked with.
	inColumn, err := m.BoardToolCards(t.Context(), token, "идеи")
	if err != nil {
		t.Fatal(err)
	}
	if len(inColumn) != 1 || inColumn[0].ID != "card-2" {
		t.Errorf("the column filter gave %+v", inColumn)
	}

	// One card asked about by itself carries its description; a listing does not.
	if first.Body != "" {
		t.Errorf("a listing carried a body, which costs a query per card: %q", first.Body)
	}
	card, err := m.BoardToolCardByID(t.Context(), token, "card-1")
	if err != nil {
		t.Fatal(err)
	}
	if card.Body != "Оно открывается пополам." {
		t.Errorf("the card read back without its description: %+v", card)
	}
}

// A terminal is the vendor CLI, so the tools reach it as a config file the CLI
// is pointed at — and the file carries the grant, so it goes when the grant does.
// The board's tools reach an agent as an HTTP server declared in session/new,
// so an agent that does not speak that transport is handed none — rather than
// one it would refuse, on a stage that could then never report.
func TestTheBoardServerIsOfferedOnlyToAnAgentThatSpeaksHTTP(t *testing.T) {
	m, _, _, _ := testManager(t, "idle", func(*Config) {})
	m.SetOrigin("http://127.0.0.1:8088/")

	http := acpsdk.AgentCapabilities{McpCapabilities: acpsdk.McpCapabilities{Http: true}}
	stdio := acpsdk.AgentCapabilities{}

	s := &Session{ID: "sess-http", BoardID: "board-1", CardID: "card-1"}
	server := m.sessionBoardServer(s, http)
	if server == nil || server.Http == nil {
		t.Fatal("an agent that speaks HTTP was handed no board server")
	}
	if server.Http.Name != boardmcp.ServerName {
		t.Errorf("the board server arrived under %q", server.Http.Name)
	}
	if len(server.Http.Headers) != 1 || !strings.HasPrefix(server.Http.Headers[0].Value, "Bearer ") {
		t.Errorf("the grant did not travel with the server: %v", server.Http.Headers)
	}
	if m.CheckBoardTools(strings.TrimPrefix(server.Http.Headers[0].Value, "Bearer ")) != nil {
		t.Error("the token handed to the agent opens nothing")
	}

	if got := m.sessionBoardServer(&Session{ID: "sess-stdio", BoardID: "board-1"}, stdio); got != nil {
		t.Error("an agent with no HTTP transport was handed a server it cannot reach")
	}
	// A deploy reads the board for nobody and moves its own card nowhere: it
	// gets no tools, whatever it speaks.
	deploy := &Session{ID: "sess-deploy", BoardID: "board-1", CardID: "card-1", Deploy: &DeployEntry{Name: "prod"}}
	if got := m.sessionBoardServer(deploy, http); got != nil {
		t.Error("a deploy session was handed the board's tools")
	}
}

func TestTheGrantTakesTheColumnPropertyFromTheBoard(t *testing.T) {
	m, writer, _, _ := testManager(t, "idle", func(cfg *Config) {
		cfg.TriggerProperty = "Статус"
	})
	m.SetBoardMeta(&fakeBoardMeta{props: map[string]any{
		BoardPropColumnProperty: "prop-workflow",
	}})

	if _, err := m.CreateCardFromTools(t.Context(), m.GrantBoardTools("board-1", "", ""),
		NewCard{Title: "Задача", Column: "К агенту"}); err != nil {
		t.Fatal(err)
	}

	if len(writer.created) != 1 {
		t.Fatalf("board writes: %d, want 1", len(writer.created))
	}
	if got := writer.created[0].Property; got != "prop-workflow" {
		t.Errorf("column property %q, want the one the board records", got)
	}
}

// A board made before that record exists says nothing, and the machine's
// configured name is the only answer there is.
func TestTheGrantFallsBackToTheConfiguredColumnProperty(t *testing.T) {
	m, writer, _, _ := testManager(t, "idle", func(cfg *Config) {
		cfg.TriggerProperty = "Статус"
	})
	m.SetBoardMeta(&fakeBoardMeta{props: map[string]any{}})

	if _, err := m.CreateCardFromTools(t.Context(), m.GrantBoardTools("board-1", "", ""),
		NewCard{Title: "Задача", Column: "К агенту"}); err != nil {
		t.Fatal(err)
	}

	if got := writer.created[0].Property; got != "Статус" {
		t.Errorf("column property %q, want the configured fallback", got)
	}
}
