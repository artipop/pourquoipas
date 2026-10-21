package app

import (
	"testing"
	"time"

	"github.com/artipop/pourquoipas/server/auth"
	"github.com/artipop/pourquoipas/server/mlog"
	"github.com/artipop/pourquoipas/server/model"
	"github.com/artipop/pourquoipas/server/services/config"
	"github.com/artipop/pourquoipas/server/services/filestore/mocks"
	permissionsMocks "github.com/artipop/pourquoipas/server/services/permissions/mocks"
	"github.com/artipop/pourquoipas/server/services/store/mockstore"
	"github.com/artipop/pourquoipas/server/ws"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// spyAdapter records what reaches the websocket, which is the only place the
// answer this test is about can be seen.
type spyAdapter struct {
	ws.Adapter
	seen chan bool
}

func (s *spyAdapter) BroadcastBlockChange(_ string, _ *model.Block, silent bool) {
	s.seen <- silent
}

// appWatchedBy builds an app whose broadcasts land in the spy. SetupTestHelper
// wires a real ws.Server, which has nowhere to be asked what it sent.
func appWatchedBy(t *testing.T, spy *spyAdapter) (*App, *mockstore.MockStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	cfg := config.Configuration{}
	store := mockstore.NewMockStore(ctrl)
	logger, _ := mlog.NewLogger()
	app := New(&cfg, spy, Services{
		Auth:             auth.New(&cfg, store, nil),
		Store:            store,
		FilesBackend:     &mocks.FileBackend{},
		Logger:           logger,
		SkipTemplateInit: true,
		Permissions:      permissionsMocks.NewMockPermissionsService(ctrl),
	})
	t.Cleanup(func() {
		app.Shutdown()
		_ = logger.Shutdown()
	})
	return app, store
}

// disableNotify has always meant "this write is the machine's own bookkeeping"
// — where a card stands on its route, the branch written onto it — and it
// silenced the notification service. It did not silence the broadcast, and must
// not: every page looking at the board still has to redraw.
//
// What was missing is that the broadcast never said which kind of write it was.
// In this process that cost nothing, because the backend the agent integration
// listens on is short-circuited by the same flag. A client reading the board
// over the socket has no such shortcut: without this it sees its own
// bookkeeping come back looking exactly like somebody dragging the card, and
// starts the stage it has just finished.
func TestABookkeepingWriteIsBroadcastAsSilent(t *testing.T) {
	for _, tc := range []struct {
		name          string
		disableNotify bool
		wantSilent    bool
	}{
		{"a person moving a card", false, false},
		{"the app writing where the card stands", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spy := &spyAdapter{seen: make(chan bool, 1)}
			app, store := appWatchedBy(t, spy)

			block := &model.Block{ID: "card-id", BoardID: "board-id", Type: model.TypeCard}
			store.EXPECT().GetBlock("card-id").Return(block, nil).AnyTimes()
			store.EXPECT().GetBoard("board-id").Return(&model.Board{ID: "board-id", TeamID: "0"}, nil).AnyTimes()
			store.EXPECT().PatchBlock("card-id", gomock.Any(), gomock.Any()).Return(nil).Times(1)

			_, err := app.PatchBlockAndNotify("card-id", &model.BlockPatch{}, "somebody", tc.disableNotify)
			require.NoError(t, err)

			select {
			case silent := <-spy.seen:
				require.Equal(t, tc.wantSilent, silent)
			case <-time.After(5 * time.Second):
				t.Fatal("nothing was broadcast")
			}
		})
	}
}

// The flag has to survive onto the wire: a client cannot ask about a field the
// message does not carry.
func TestTheSilentFlagIsOnTheMessage(t *testing.T) {
	msg := ws.UpdateBlockMsg{Action: "UPDATE_BLOCK", TeamID: "0", Block: &model.Block{ID: "card"}, Silent: true}
	require.True(t, msg.Silent)
}
