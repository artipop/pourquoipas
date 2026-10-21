package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/artipop/pourquoipas/server/model"
)

// The card's history already exists in `blocks_history` — every write leaves a
// copy with the id of whoever made it. This is what turns those copies into the
// difference between them, which is what a person came to read.
func TestACardSaysWhoChangedWhatOnIt(t *testing.T) {
	th, tearDown := SetupTestHelper(t)
	defer tearDown()

	const cardID = "card-1"
	board := &model.Board{ID: testBoardID, CardProperties: []map[string]any{{
		"id":   "prop-status",
		"name": "Статус",
		"type": "select",
		"options": []any{
			map[string]any{"id": "opt-work", "value": "В работе"},
			map[string]any{"id": "opt-done", "value": "Готово"},
		},
	}}}
	card := &model.Block{ID: cardID, BoardID: testBoardID, Type: model.TypeCard}

	version := func(at int64, by, title, option string) *model.Block {
		props := map[string]any{}
		if option != "" {
			props["prop-status"] = option
		}
		return &model.Block{
			ID: cardID, BoardID: testBoardID, Type: model.TypeCard, Title: title,
			CreatedBy: "person-1", CreateAt: 100, ModifiedBy: by, UpdateAt: at,
			Fields: map[string]any{"properties": props},
		}
	}

	th.Store.EXPECT().GetBoardAndCardByID(cardID).Return(board, card, nil)
	th.Store.EXPECT().GetBlockHistory(cardID, model.QueryBlockHistoryOptions{}).Return([]*model.Block{
		version(100, "person-1", "Починить логин", ""),
		version(200, "person-1", "Починить логин", "opt-work"),
		version(300, "agent-1", "Починить вход по SSO", "opt-done"),
	}, nil)
	th.Store.EXPECT().GetBlockHistoryNewestChildren(cardID, model.QueryBlockHistoryChildOptions{PerPage: -1}).
		Return([]*model.Block{{
			ID: "comment-1", ParentID: cardID, Type: model.TypeComment,
			Title: "Сделал, проверьте", CreatedBy: "agent-1", CreateAt: 290,
			Fields: map[string]any{"agent": true},
		}}, false, nil)

	entries, err := th.App.GetCardHistory(cardID)
	require.NoError(t, err)

	// Newest first, the way a history is read.
	kinds := make([]string, 0, len(entries))
	for _, e := range entries {
		kinds = append(kinds, e.Kind)
	}
	require.Equal(t, []string{
		CardHistoryTitle, CardHistoryProperty, CardHistoryComment, CardHistoryProperty, CardHistoryCreated,
	}, kinds)

	// The move says the column by its name, not by the id it is stored under —
	// naming two option ids is not an answer to "what happened".
	moved := entries[1]
	require.Equal(t, "Статус", moved.Property)
	require.Equal(t, "В работе", moved.From)
	require.Equal(t, "Готово", moved.To)

	// And who: the agent has a board account of its own, so the line about the
	// move names the agent rather than the owner of the machine.
	require.Equal(t, "agent-1", moved.UserID)
	require.Equal(t, "person-1", entries[len(entries)-1].UserID)

	// A comment is one entry with what was said, marked as an agent's so the
	// card can draw it the way it already draws one.
	comment := entries[2]
	require.Equal(t, "Сделал, проверьте", comment.Text)
	require.True(t, comment.Agent)
}

// A property the board has forgotten still changed, and saying so by id is
// better than saying nothing: a board whose property was deleted keeps a
// readable history of what happened while it existed.
func TestAHistoryOfAPropertyTheBoardNoLongerHasStillReads(t *testing.T) {
	th, tearDown := SetupTestHelper(t)
	defer tearDown()

	const cardID = "card-2"
	board := &model.Board{ID: testBoardID}
	card := &model.Block{ID: cardID, BoardID: testBoardID, Type: model.TypeCard}

	th.Store.EXPECT().GetBoardAndCardByID(cardID).Return(board, card, nil)
	th.Store.EXPECT().GetBlockHistory(cardID, model.QueryBlockHistoryOptions{}).Return([]*model.Block{
		{ID: cardID, BoardID: testBoardID, CreatedBy: "person-1", CreateAt: 100, UpdateAt: 100},
		{
			ID: cardID, BoardID: testBoardID, CreatedBy: "person-1", CreateAt: 100,
			ModifiedBy: "person-1", UpdateAt: 200,
			Fields: map[string]any{"properties": map[string]any{"gone": "что-то"}},
		},
	}, nil)
	th.Store.EXPECT().GetBlockHistoryNewestChildren(cardID, model.QueryBlockHistoryChildOptions{PerPage: -1}).
		Return(nil, false, nil)

	entries, err := th.App.GetCardHistory(cardID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, CardHistoryProperty, entries[0].Kind)
	require.Equal(t, "gone", entries[0].Property)
	require.Equal(t, "что-то", entries[0].To)
}
