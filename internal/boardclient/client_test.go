package boardclient_test

import (
	"context"
	"testing"
	"time"

	"github.com/artipop/xciii/server/client"
	"github.com/artipop/xciii/server/integrationtests"
	"github.com/artipop/xciii/server/mlog"
	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/utils"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardclient"

	"github.com/stretchr/testify/require"
)

// The seam against a real board server, because that is the only thing that can
// answer whether it works: every method here is an HTTP call whose shape is the
// board's business, and a fake would only prove that this file agrees with
// itself.

const password = "sample-password-1"

type fixture struct {
	th     *integrationtests.TestHelper
	client *boardclient.Client
	board  *model.Board
	// column is the select property the board's cards move between.
	column  model.PropDef
	options map[string]string // option name -> id
}

func raise(t *testing.T) *fixture {
	t.Helper()
	th := integrationtests.SetupTestHelperLocalMode(t)
	t.Cleanup(th.TearDown)

	th.Client = client.NewClient(th.Server.Config().ServerRoot, "")
	th.RegisterAndLogin(th.Client, "somebody", "somebody@sample.com", password, "")

	board := &model.Board{
		TeamID: model.GlobalTeamID,
		Type:   model.BoardTypeOpen,
		Title:  "Работа",
		CardProperties: []map[string]any{{
			"id":   "prop-status",
			"name": "Статус",
			"type": "select",
			"options": []any{
				map[string]any{"id": "opt-todo", "value": "Взять в работу", "color": "propDefault"},
				map[string]any{"id": "opt-doing", "value": "Разработка", "color": "propDefault"},
			},
		}},
	}
	made, resp := th.Client.CreateBoard(board)
	th.CheckOK(resp)
	require.NotNil(t, made)

	logger, _ := mlog.NewLogger()
	t.Cleanup(func() { _ = logger.Shutdown() })

	return &fixture{
		th:      th,
		client:  boardclient.New(th.Server.Config().ServerRoot, th.Client.Token, model.GlobalTeamID, logger),
		board:   made,
		column:  model.PropDef{ID: "prop-status", Name: "Статус", Type: "select"},
		options: map[string]string{"Взять в работу": "opt-todo", "Разработка": "opt-doing"},
	}
}

// card puts a card on the board in the named column and returns its id.
func (f *fixture) card(t *testing.T, title, column string) string {
	t.Helper()
	now := utils.GetMillis()
	block := &model.Block{
		ID:       utils.NewID(utils.IDTypeCard),
		BoardID:  f.board.ID,
		ParentID: f.board.ID,
		Type:     model.TypeCard,
		Title:    title,
		Fields: map[string]any{
			"properties":   map[string]any{f.column.ID: f.options[column]},
			"contentOrder": []any{},
		},
		CreateAt: now,
		UpdateAt: now,
	}
	made, resp := f.th.Client.InsertBlocks(f.board.ID, []*model.Block{block}, false)
	f.th.CheckOK(resp)
	require.Len(t, made, 1)
	return made[0].ID
}

func TestACardIsReadTheSameWayOverHTTP(t *testing.T) {
	f := raise(t)
	cardID := f.card(t, "Почини логин", "Разработка")

	got, err := f.client.CardByID(context.Background(), cardID)
	require.NoError(t, err)

	require.Equal(t, cardID, got.CardID)
	require.Equal(t, f.board.ID, got.BoardID)
	require.Equal(t, "Почини логин", got.Title)
	// The option names and the ids beside them: reading a card by name alone is
	// what made the same card say different things depending on who asked.
	require.Contains(t, got.OptionNames, "Разработка")
	require.Len(t, got.SelectedOptions, 1)
	require.Equal(t, "opt-doing", got.SelectedOptions[0].OptionID)
	require.Equal(t, "Статус", got.SelectedOptions[0].PropertyName)
}

func TestABoardListsItsCards(t *testing.T) {
	f := raise(t)
	f.card(t, "Первая", "Взять в работу")
	f.card(t, "Вторая", "Разработка")

	cards, err := f.client.CardsForBoard(context.Background(), f.board.ID)
	require.NoError(t, err)
	require.Len(t, cards, 2)
}

// Where a card stands on its route lives on the card, so it travels with the
// board — and it must survive the trip through JSON and back.
func TestTheFlowPositionSurvivesARoundTrip(t *testing.T) {
	f := raise(t)
	cardID := f.card(t, "Почини логин", "Разработка")
	ctx := context.Background()

	_, found, err := f.client.CardFlow(ctx, cardID)
	require.NoError(t, err)
	require.False(t, found, "a card that has never been on a route stands nowhere")

	want := acp.FlowState{CardID: cardID, BoardID: f.board.ID, FlowID: "flow-1", NodeID: "opt-doing"}
	require.NoError(t, f.client.SetCardFlow(ctx, cardID, want))

	got, found, err := f.client.CardFlow(ctx, cardID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "flow-1", got.FlowID)
	require.Equal(t, "opt-doing", got.NodeID)

	parked, err := f.client.BoardCardFlows(ctx, f.board.ID)
	require.NoError(t, err)
	require.Len(t, parked, 1)

	require.NoError(t, f.client.ClearCardFlow(ctx, cardID))
	_, found, err = f.client.CardFlow(ctx, cardID)
	require.NoError(t, err)
	require.False(t, found)
}

// A one-key patch used to erase every other property, because a block patch
// replaces the field it names whole.
func TestWritingOnePropertyKeepsTheRest(t *testing.T) {
	f := raise(t)
	cardID := f.card(t, "Почини логин", "Разработка")
	ctx := context.Background()

	require.NoError(t, f.client.SetCardFlow(ctx, cardID, acp.FlowState{FlowID: "flow-1"}))
	require.NoError(t, f.client.MoveCardByOptionName(ctx, cardID, "Статус", "Взять в работу"))

	got, err := f.client.CardByID(ctx, cardID)
	require.NoError(t, err)
	require.Contains(t, got.OptionNames, "Взять в работу")

	_, found, err := f.client.CardFlow(ctx, cardID)
	require.NoError(t, err)
	require.True(t, found, "moving the card must not take its route position with it")
}

func TestACommentLandsOnTheCard(t *testing.T) {
	f := raise(t)
	cardID := f.card(t, "Почини логин", "Разработка")

	require.NoError(t, f.client.AddComment(context.Background(), cardID, "ветка готова"))

	blocks, resp := f.th.Client.GetBlocksForBoard(f.board.ID)
	f.th.CheckOK(resp)
	var comments int
	for _, b := range blocks {
		if b.Type == model.TypeComment && b.ParentID == cardID {
			comments++
			require.Equal(t, "ветка готова", b.Title)
		}
	}
	require.Equal(t, 1, comments)
}

// The one thing REST could not already do. An agent is named by a card being
// assigned to it, so the name needs an account, and an attached desktop is not
// the process holding the users table.
func TestAnAgentGetsAnAccountAndACardNamesIt(t *testing.T) {
	f := raise(t)
	cardID := f.card(t, "Почини логин", "Разработка")
	ctx := context.Background()

	// The board needs somewhere to put a person before it can name one.
	patch := &model.BoardPatch{UpdatedCardProperties: []map[string]any{{
		"id": "prop-who", "name": "Кто занимается", "type": "person", "options": []any{},
	}}}
	_, resp := f.th.Client.PatchBoard(f.board.ID, patch)
	f.th.CheckOK(resp)

	made, err := f.client.EnsureAgentAccounts(ctx, []acp.AgentUser{{Name: "клаус", Username: "клаус"}})
	require.NoError(t, err)
	require.Len(t, made, 1)
	require.NotEmpty(t, made[0].UserID)

	// Idempotent: a second desktop registering the same agent shares the
	// account rather than making a rival one.
	again, err := f.client.EnsureAgentAccounts(ctx, []acp.AgentUser{{Name: "клаус", Username: "клаус"}})
	require.NoError(t, err)
	require.Equal(t, made[0].UserID, again[0].UserID)

	require.NoError(t, f.client.AssignCardAgent(ctx, cardID, made[0]))

	got, err := f.client.CardByID(ctx, cardID)
	require.NoError(t, err)
	require.Contains(t, got.PersonIDs, made[0].UserID)
	require.Contains(t, got.PersonNames, "клаус")
}

func TestTheBoardsOwnAutomationIsReadAndWritten(t *testing.T) {
	f := raise(t)
	ctx := context.Background()

	require.NoError(t, f.client.SetBoardProperties(ctx, f.board.ID,
		map[string]any{"columns": `[{"board":"x"}]`, "leaving": "soon"}, nil))

	props, err := f.client.BoardProperties(ctx, f.board.ID)
	require.NoError(t, err)
	require.Equal(t, `[{"board":"x"}]`, props["columns"])

	require.NoError(t, f.client.SetBoardProperties(ctx, f.board.ID, nil, []string{"leaving"}))
	props, err = f.client.BoardProperties(ctx, f.board.ID)
	require.NoError(t, err)
	require.NotContains(t, props, "leaving")

	columns, err := f.client.BoardColumnOptions(ctx, f.board.ID)
	require.NoError(t, err)
	require.Len(t, columns, 2)

	isTemplate, err := f.client.IsBoardTemplate(ctx, f.board.ID)
	require.NoError(t, err)
	require.False(t, isTemplate)
}

// The socket is the whole reason this package can drive a route at all.
func TestACardMovedElsewhereArrivesAsAMove(t *testing.T) {
	f := raise(t)
	cardID := f.card(t, "Почини логин", "Взять в работу")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The shadow is filled by reading the board, which is what the manager does
	// when it first sees one. Without it the first sighting of a card is
	// recorded and announced to nobody, on purpose.
	_, err := f.client.CardsForBoard(ctx, f.board.ID)
	require.NoError(t, err)

	moves, err := f.client.Subscribe(ctx)
	require.NoError(t, err)
	// Give the socket a moment to connect and subscribe before making a move it
	// has to hear.
	time.Sleep(500 * time.Millisecond)

	// Somebody else moves the card — a person dragging it, in the board's own
	// vocabulary rather than through this client.
	_, resp := f.th.Client.PatchBlock(f.board.ID, cardID, &model.BlockPatch{
		UpdatedFields: map[string]any{"properties": map[string]any{"prop-status": "opt-doing"}},
	}, false)
	f.th.CheckOK(resp)

	select {
	case ev := <-moves:
		require.Equal(t, cardID, ev.CardID)
		require.Equal(t, "Взять в работу", ev.FromColumn.Name, "the shadow is what supplies where the card came from")
		require.Equal(t, "Разработка", ev.ToColumn.Name)
	case <-time.After(10 * time.Second):
		t.Fatal("the move never arrived")
	}
}

// The app's own bookkeeping is broadcast too, because the page has to redraw.
// Acting on it would mean starting the stage this client had just finished.
func TestOurOwnBookkeepingIsNotAMove(t *testing.T) {
	f := raise(t)
	cardID := f.card(t, "Почини логин", "Взять в работу")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := f.client.CardsForBoard(ctx, f.board.ID)
	require.NoError(t, err)
	moves, err := f.client.Subscribe(ctx)
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// A silent write of ours that changes the very property a route watches.
	require.NoError(t, f.client.MoveCardByOptionName(ctx, cardID, "Статус", "Разработка"))

	select {
	case ev := <-moves:
		t.Fatalf("our own write came back as a move: %s → %s", ev.FromColumn.Name, ev.ToColumn.Name)
	case <-time.After(3 * time.Second):
	}

	// And the shadow has kept up, so the *next* move from outside is still read
	// against the truth rather than against what this client last announced.
	_, resp := f.th.Client.PatchBlock(f.board.ID, cardID, &model.BlockPatch{
		UpdatedFields: map[string]any{"properties": map[string]any{"prop-status": "opt-todo"}},
	}, false)
	f.th.CheckOK(resp)

	select {
	case ev := <-moves:
		require.Equal(t, "Разработка", ev.FromColumn.Name)
		require.Equal(t, "Взять в работу", ev.ToColumn.Name)
	case <-time.After(10 * time.Second):
		t.Fatal("the move after our own write never arrived")
	}
}
