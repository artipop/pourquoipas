package boardadapter

import (
	"testing"
	"time"

	"github.com/artipop/xciii/server/app"
	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/server/utils"

	"github.com/artipop/xciii/internal/sources"
)

// The inbox is a board the app makes, found by a mark on it, and a view on it —
// none of which can be checked without a board server. So this runs the real
// one and looks at what a person would see.

// boardWithAKanban is a board of work, the kind the share dialog may pick: a
// column property and one kanban grouped by it.
func boardWithAKanban(t *testing.T, a *app.App) *model.Board {
	t.Helper()
	board, err := a.CreateBoard(&model.Board{
		TeamID: model.GlobalTeamID,
		Type:   model.BoardTypeOpen,
		Title:  "Доска",
		CardProperties: []map[string]any{
			{
				"id": "prop-status", "name": "Статус", "type": "select",
				"options": []any{
					map[string]any{"id": "opt-todo", "value": "Не начата", "color": ""},
					map[string]any{"id": "opt-done", "value": "Готово", "color": ""},
				},
			},
		},
	}, model.SingleUser, true)
	if err != nil {
		t.Fatal(err)
	}
	now := utils.GetMillis()
	view := &model.Block{
		ID:       utils.NewID(utils.IDTypeView),
		BoardID:  board.ID,
		ParentID: board.ID,
		Type:     model.TypeView,
		Title:    "Дела",
		Fields: map[string]any{
			"viewType":         "board",
			"groupById":        "prop-status",
			"visibleOptionIds": []any{"opt-todo", "opt-done"},
			"hiddenOptionIds":  []any{},
		},
		CreatedBy: model.SingleUser,
		CreateAt:  now,
		UpdateAt:  now,
	}
	if _, err := a.InsertBlocksAndNotify([]*model.Block{view}, model.SingleUser, false); err != nil {
		t.Fatal(err)
	}
	// The block history is keyed by (id, insert_at) in milliseconds, so a view
	// written and patched inside the same millisecond collides with its own
	// history row. Only a test builds a board that fast.
	time.Sleep(2 * time.Millisecond)
	return board
}

func viewsByTitle(t *testing.T, w *Writer, boardID string) map[string]*model.Block {
	t.Helper()
	views, err := w.boardViews(boardID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*model.Block{}
	for _, view := range views {
		out[view.Title] = view
	}
	return out
}

func boardByID(t *testing.T, a *app.App, id string) *model.Board {
	t.Helper()
	board, err := a.GetBoard(id)
	if err != nil {
		t.Fatal(err)
	}
	return board
}

// There is one inbox. Asked for twice it is the same board, and the second
// asking makes nothing — two inboxes would be two places to look, which is the
// thing an inbox is for not having.
func TestTheInboxIsOneBoardMadeOnce(t *testing.T) {
	a := newTestApp(t)

	first, err := EnsureInboxBoard(a, "ru")
	if err != nil {
		t.Fatal(err)
	}
	again, err := NewWriter(a).InboxBoard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatalf("a second inbox was made: %s and %s", first, again)
	}
	if !IsInboxBoard(boardByID(t, a, first)) {
		t.Fatal("the inbox does not carry the mark it is found by")
	}
}

// The inbox is read as a kanban with a column per source: what a person asks of
// it is what the mail brought and what the tracker did. So its one view groups
// by the card's author, and the board has no column property of its own —
// nothing on it is a stage.
func TestTheInboxGroupsByWhoBroughtTheCard(t *testing.T) {
	a := newTestApp(t)
	id, err := EnsureInboxBoard(a, "ru")
	if err != nil {
		t.Fatal(err)
	}
	board := boardByID(t, a, id)
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		t.Fatal(err)
	}
	for _, def := range schema {
		if def.Type == "select" {
			t.Fatalf("the inbox should have no columns, it has %q", def.Name)
		}
	}

	views := viewsByTitle(t, NewWriter(a), id)
	if len(views) != 1 {
		t.Fatalf("the inbox should have one view, it has %d", len(views))
	}
	view, ok := views[InboxViewTitle]
	if !ok {
		t.Fatalf("no view called %q", InboxViewTitle)
	}
	groupBy, _ := view.Fields["groupById"].(string)
	if def, ok := schema[groupBy]; !ok || def.Type != "createdBy" {
		t.Fatalf("the view groups by %q, not by the author", groupBy)
	}
	// In the person's language: a field called "Author" on a Russian board is
	// a field somebody forgot to translate.
	if schema[groupBy].Name != "Автор" {
		t.Fatalf("the author field is called %q", schema[groupBy].Name)
	}
}

// The mark is the answer, not the title: a person may call the inbox anything,
// and the next arrival must not make a second one beside the renamed board.
func TestARenamedInboxIsStillTheInbox(t *testing.T) {
	a := newTestApp(t)
	id, err := EnsureInboxBoard(a, "ru")
	if err != nil {
		t.Fatal(err)
	}
	title := "Всё подряд"
	if _, err := a.PatchBoard(&model.BoardPatch{Title: &title}, id, model.SingleUser); err != nil {
		t.Fatal(err)
	}
	again, err := EnsureInboxBoard(a, "ru")
	if err != nil || again != id {
		t.Fatalf("the renamed inbox was not found: %q, %v", again, err)
	}
}

// The phone and the share dialog tell the inbox from the other boards by the
// list itself, since neither may look at a board's properties.
func TestTheBoardListSaysWhichBoardIsTheInbox(t *testing.T) {
	a := newTestApp(t)
	work := boardWithAKanban(t, a)
	inbox, err := EnsureInboxBoard(a, "ru")
	if err != nil {
		t.Fatal(err)
	}
	boards, err := NewWriter(a).Boards(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	marked := map[string]bool{}
	for _, b := range boards {
		marked[b.ID] = b.Inbox
	}
	if !marked[inbox] || marked[work.ID] {
		t.Fatalf("inbox marks: %v", marked)
	}
}

// A card a source brings to a board with columns stands in the first of them —
// outside every column it would be on the board and on no kanban. On the inbox
// there are no columns and the card stands in none.
func TestASourcesCardStandsInTheFirstColumn(t *testing.T) {
	a := newTestApp(t)
	work := boardWithAKanban(t, a)
	w := NewSourceWriter(a)

	cardID, err := w.CreateCard(t.Context(), work.ID, sources.CardSpec{Title: "Статья", Source: "Поделиться"})
	if err != nil {
		t.Fatal(err)
	}
	card, err := w.cardBlock(cardID)
	if err != nil {
		t.Fatal(err)
	}
	props, _ := card.Fields["properties"].(map[string]any)
	if props["prop-status"] != "opt-todo" {
		t.Fatalf("the card stands in %v, not in the first column", props["prop-status"])
	}

	inbox, err := w.InboxBoard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.CreateCard(t.Context(), inbox, sources.CardSpec{Title: "Письмо", Source: "почта"}); err != nil {
		t.Fatalf("a card on the inbox: %v", err)
	}
}
