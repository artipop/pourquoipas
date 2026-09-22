package boardadapter

import (
	"context"
	"fmt"
	"sync"

	"github.com/artipop/xciii/server/app"
	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/utils"
)

// The inbox is one board, not a view of every board.
//
// Everything a source brings lands there, and a person carries each card on to
// the board it belongs to (MoveCardToBoard) — collect first, decide later. A
// board of its own rather than an inbox view on each board, because a source
// that had to name a board had to be told where a thing belongs before anybody
// had looked at it, which is the decision the inbox exists to postpone.

// InboxBoardProperty marks the board the app made to be the inbox. It is found
// by this and never by title: a person may rename the board, and "Входящие" is
// not a key (Russian never is).
const InboxBoardProperty = "xciiiInbox"

// InboxViewTitle is what the inbox board and its one view are called. A name
// the app gives, Russian in every language like the other given names, and
// matched on by the page (frontend inboxView.ts) to know the view it is on.
const InboxViewTitle = "Входящие"

// inboxViewType is what the inbox is drawn as: a kanban grouped by who brought
// the card, so each source is a column — the question a person has of an inbox
// is "what did the mail bring, what did the tracker". The person's own cards
// are a column too, headed "Мои задачи" by the page.
const inboxViewType = "board"

// inboxBoardMu keeps two first arrivals from making two inboxes: the board is
// found and made in one step, and both halves are reads and writes apart.
var inboxBoardMu sync.Mutex

// EnsureInboxBoard returns the inbox board, making it when the board server has
// none. main calls it at startup with the UI language, so the property names
// are the person's; a later caller that finds none — the startup call failed —
// makes it in the source language rather than losing what arrived.
func EnsureInboxBoard(a *app.App, lang string) (string, error) {
	inboxBoardMu.Lock()
	defer inboxBoardMu.Unlock()
	if id, ok, err := findInboxBoard(a); err != nil || ok {
		return id, err
	}
	return createInboxBoard(a, TemplateWords(lang))
}

// InboxBoard is the inbox as the sources pipeline asks for it.
func (w *Writer) InboxBoard(_ context.Context) (string, error) {
	return EnsureInboxBoard(w.app, "")
}

func findInboxBoard(a *app.App) (string, bool, error) {
	boards, err := a.GetBoardsForUserAndTeam(model.SingleUser, model.GlobalTeamID, false)
	if err != nil {
		return "", false, fmt.Errorf("list boards: %w", err)
	}
	for _, board := range boards {
		if IsInboxBoard(board) {
			return board.ID, true, nil
		}
	}
	return "", false, nil
}

// IsInboxBoard reports whether this is the board the app made to be the inbox.
func IsInboxBoard(board *model.Board) bool {
	if board == nil || board.IsTemplate || board.DeleteAt != 0 {
		return false
	}
	marked, _ := board.Properties[InboxBoardProperty].(bool)
	return marked
}

// createInboxBoard makes the board and its view. No column property: nothing
// here is a stage and nothing moves between columns — a card is read, then
// carried to another board or deleted. What groups it is its author, which for
// what arrived is the source's account (EnsureSourceUser).
func createInboxBoard(a *app.App, words func(string) string) (string, error) {
	authorID := utils.NewID(utils.IDTypeBlock)
	board := &model.Board{
		TeamID: model.GlobalTeamID,
		// Open, so that on a hosted server every member of the team sees the
		// one inbox rather than whoever happened to be the first to need it.
		Type:  model.BoardTypeOpen,
		Title: InboxViewTitle,
		Icon:  "📥",
		Properties: map[string]any{
			InboxBoardProperty: true,
		},
		CardProperties: []map[string]any{
			{"id": authorID, "name": words("Author"), "type": "createdBy", "options": []any{}},
			{"id": utils.NewID(utils.IDTypeBlock), "name": words("Created"), "type": "createdTime", "options": []any{}},
		},
	}
	created, err := a.CreateBoard(board, model.SingleUser, true)
	if err != nil {
		return "", fmt.Errorf("create the inbox board: %w", err)
	}

	now := utils.GetMillis()
	view := &model.Block{
		ID:       utils.NewID(utils.IDTypeView),
		BoardID:  created.ID,
		ParentID: created.ID,
		Type:     model.TypeView,
		Title:    InboxViewTitle,
		Fields: map[string]any{
			"viewType":  inboxViewType,
			"groupById": authorID,
			// Nothing else on the face of a card: what a person reads in an
			// inbox is the title and who brought it, and the second is the
			// column it stands in.
			"visiblePropertyIds": []any{},
			"columnWidths":       map[string]any{},
			"sortOptions":        []any{},
			"visibleOptionIds":   []any{},
			"hiddenOptionIds":    []any{},
			"collapsedOptionIds": []any{},
			"cardOrder":          []any{},
			"columnCalculations": map[string]any{},
			"kanbanCalculations": map[string]any{},
			"defaultTemplateId":  "",
			"filter":             map[string]any{"operation": "and", "filters": []any{}},
		},
		CreatedBy: model.SingleUser,
		CreateAt:  now,
		UpdateAt:  now,
	}
	if _, err := a.InsertBlocksAndNotify([]*model.Block{view}, model.SingleUser, true); err != nil {
		// The board is there and takes cards; a board without its view opens
		// on the page's own default, which is worse to look at and loses
		// nothing.
		return created.ID, fmt.Errorf("the view of the inbox board: %w", err)
	}
	return created.ID, nil
}
