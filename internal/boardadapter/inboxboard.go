package boardadapter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/artipop/xciii/internal/boardmodel"
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
// by this and never by title: a person may rename the board, and its name is
// not a key in any language.
const InboxBoardProperty = "xciiiInbox"

// The inbox's words, in the source language like the template files, and
// translated through the same catalogue (templatetext.go): the board and its
// one view are called inboxTitle, the two card properties are named by the
// other two. Nothing finds the board or its fields by these — the board by its
// mark, the fields by their type — so a translation is only ever a label.
const (
	inboxTitle   = "Inbox"
	inboxAuthor  = "Author"
	inboxCreated = "Created"
)

// inboxWords is what the inbox says to a person, for the catalogue guard.
var inboxWords = []string{inboxTitle, inboxAuthor, inboxCreated, boardmodel.LinkPropertyTitle}

// boardLanguage is the language the app gives names in, as main last told
// EnsureInboxBoard or RelabelInboxBoard. A field made later — the link a source
// brings — is named in it too.
var boardLanguage atomic.Value

func namingWords() func(string) string {
	lang, _ := boardLanguage.Load().(string)
	return TemplateWords(lang)
}

// inboxViewType is what the inbox is drawn as: a kanban grouped by who brought
// the card, so each source is a column — the question a person has of an inbox
// is "what did the mail bring, what did the tracker". The person's own cards
// are a column too, headed as the person's own tasks by the page.
const inboxViewType = "board"

// inboxBoardMu keeps two first arrivals from making two inboxes: the board is
// found and made in one step, and both halves are reads and writes apart.
var inboxBoardMu sync.Mutex

// EnsureInboxBoard returns the inbox board, making it when the board server has
// none. main calls it at startup with the UI language, so the property names
// are the person's; a later caller that finds none — the startup call failed —
// makes it in the source language rather than losing what arrived.
func EnsureInboxBoard(a *app.App, lang string) (string, error) {
	if lang != "" {
		boardLanguage.Store(lang)
	}
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
		Title: words(inboxTitle),
		Icon:  "📥",
		Properties: map[string]any{
			InboxBoardProperty: true,
		},
		CardProperties: []map[string]any{
			{"id": authorID, "name": words(inboxAuthor), "type": "createdBy", "options": []any{}},
			{"id": utils.NewID(utils.IDTypeBlock), "name": words(inboxCreated), "type": "createdTime", "options": []any{}},
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
		Title:    words(inboxTitle),
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

// inboxPropertyWords is which of the inbox's words names a card property of
// that type.
var inboxPropertyWords = map[string]string{
	"createdBy":   inboxAuthor,
	"createdTime": inboxCreated,
}

// RelabelInboxBoard puts the inbox's own names into a language. The inbox is
// made once, in whatever language the app was read in that day, and unlike a
// board made from a template it is the app's rather than the person's — so it
// follows a switch in the settings. What follows is only what still says one of
// the app's names, in any language a catalogue has: a board, view or field the
// person renamed keeps their name.
func RelabelInboxBoard(a *app.App, lang string) error {
	boardLanguage.Store(lang)
	inboxBoardMu.Lock()
	defer inboxBoardMu.Unlock()
	id, ok, err := findInboxBoard(a)
	if err != nil || !ok {
		return err
	}
	board, err := a.GetBoard(id)
	if err != nil {
		return fmt.Errorf("read the inbox board: %w", err)
	}
	words := TemplateWords(lang)

	patch := &model.BoardPatch{}
	changed := false
	if title, ok := relabel(board.Title, inboxTitle, words); ok {
		patch.Title = &title
		changed = true
	}
	for _, prop := range board.CardProperties {
		kind, _ := prop["type"].(string)
		source, ok := inboxPropertyWords[kind]
		if !ok {
			continue
		}
		name, _ := prop["name"].(string)
		if next, ok := relabel(name, source, words); ok {
			updated := make(map[string]any, len(prop))
			for k, v := range prop {
				updated[k] = v
			}
			updated["name"] = next
			patch.UpdatedCardProperties = append(patch.UpdatedCardProperties, updated)
			changed = true
		}
	}
	if changed {
		if _, err := a.PatchBoard(patch, id, model.SingleUser); err != nil {
			return fmt.Errorf("rename the inbox board: %w", err)
		}
	}

	views, err := a.GetBlocks(id, "", model.TypeView)
	if err != nil {
		return fmt.Errorf("read the inbox views: %w", err)
	}
	for _, view := range views {
		if title, ok := relabel(view.Title, inboxTitle, words); ok {
			if _, err := a.PatchBlock(view.ID, &model.BlockPatch{Title: &title}, model.SingleUser); err != nil {
				return fmt.Errorf("rename the inbox view: %w", err)
			}
		}
	}
	return nil
}

// relabel is the name source should now be said as, when current is still one
// of the ways the app has said it.
func relabel(current, source string, words func(string) string) (string, bool) {
	next := words(source)
	if current == next || !saidAs(source)[current] {
		return "", false
	}
	return next, true
}
