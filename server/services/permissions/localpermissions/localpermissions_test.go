package localpermissions

import (
	"database/sql"
	"testing"

	"github.com/artipop/pourquoipas/server/model"

	"github.com/stretchr/testify/assert"
)

func TestHasPermissionToTeam(t *testing.T) {
	th := SetupTestHelper(t)

	t.Run("empty input should always unauthorize", func(t *testing.T) {
		assert.False(t, th.permissions.HasPermissionToTeam("", "team-id", model.PermissionManageBoardCards))
		assert.False(t, th.permissions.HasPermissionToTeam("user-id", "", model.PermissionManageBoardCards))
		assert.False(t, th.permissions.HasPermissionToTeam("user-id", "team-id", nil))
	})

	t.Run("all users have all permissions on teams", func(t *testing.T) {
		hasPermission := th.permissions.HasPermissionToTeam("user-id", "team-id", model.PermissionManageBoardCards)
		assert.True(t, hasPermission)
	})

	// Managing the team is the invite link, which is the door into this
	// install: the admin's, and nobody else's.
	t.Run("only an admin has PermissionManageTeam on teams", func(t *testing.T) {
		th.store.EXPECT().
			GetUserByID("member-id").
			Return(&model.User{ID: "member-id", Role: model.RoleMember}, nil).
			Times(1)
		assert.False(t, th.permissions.HasPermissionToTeam("member-id", "team-id", model.PermissionManageTeam))

		th.store.EXPECT().
			GetUserByID("admin-id").
			Return(&model.User{ID: "admin-id", Role: model.RoleAdmin}, nil).
			Times(1)
		assert.True(t, th.permissions.HasPermissionToTeam("admin-id", "team-id", model.PermissionManageTeam))
	})

	// A row written before the column existed is not an admin, which is what
	// lets the column be nullable and the CHECK admit NULL.
	t.Run("an account with no role at all is not an admin", func(t *testing.T) {
		th.store.EXPECT().
			GetUserByID("old-id").
			Return(&model.User{ID: "old-id"}, nil).
			Times(1)
		assert.False(t, th.permissions.HasPermissionToTeam("old-id", "team-id", model.PermissionManageTeam))
	})
}

// PermissionManageSystem is the only permission HasPermissionTo is ever asked
// for, and it means "about the install rather than about a board".
func TestHasPermissionTo(t *testing.T) {
	th := SetupTestHelper(t)

	t.Run("empty input should always unauthorize", func(t *testing.T) {
		assert.False(t, th.permissions.HasPermissionTo("", model.PermissionManageSystem))
		assert.False(t, th.permissions.HasPermissionTo("user-id", nil))
	})

	t.Run("an admin administers the install and a member does not", func(t *testing.T) {
		th.store.EXPECT().
			GetUserByID("admin-id").
			Return(&model.User{ID: "admin-id", Role: model.RoleAdmin}, nil).
			Times(1)
		assert.True(t, th.permissions.HasPermissionTo("admin-id", model.PermissionManageSystem))

		th.store.EXPECT().
			GetUserByID("member-id").
			Return(&model.User{ID: "member-id", Role: model.RoleMember}, nil).
			Times(1)
		assert.False(t, th.permissions.HasPermissionTo("member-id", model.PermissionManageSystem))
	})

	// A desktop's session carries an id with no row behind it. Not an error,
	// and not an admin either.
	t.Run("an id with no account behind it is not an admin", func(t *testing.T) {
		th.store.EXPECT().
			GetUserByID(model.SingleUser).
			Return(nil, model.NewErrNotFound("user")).
			Times(1)
		assert.False(t, th.permissions.HasPermissionTo(model.SingleUser, model.PermissionManageSystem))
	})
}

func TestHasPermissionToBoard(t *testing.T) {
	th := SetupTestHelper(t)

	t.Run("empty input should always unauthorize", func(t *testing.T) {
		assert.False(t, th.permissions.HasPermissionToBoard("", "board-id", model.PermissionManageBoardCards))
		assert.False(t, th.permissions.HasPermissionToBoard("user-id", "", model.PermissionManageBoardCards))
		assert.False(t, th.permissions.HasPermissionToBoard("user-id", "board-id", nil))
	})

	t.Run("nonexistent user", func(t *testing.T) {
		userID := "user-id"
		boardID := "board-id"

		th.store.EXPECT().
			GetMemberForBoard(boardID, userID).
			Return(nil, sql.ErrNoRows).
			Times(1)

		hasPermission := th.permissions.HasPermissionToBoard(userID, boardID, model.PermissionManageBoardCards)
		assert.False(t, hasPermission)
	})

	// The install's own templates are replaced on an upgrade and translated on
	// display, so a write to one would be lost and shown to everybody in the
	// language it was typed in. Even the board's admin only reads it.
	t.Run("install template", func(t *testing.T) {
		member := &model.BoardMember{UserID: "user-id", BoardID: "template-id", SchemeAdmin: true}
		th.store.EXPECT().GetMemberForBoard(member.BoardID, member.UserID).Return(member, nil).AnyTimes()
		th.store.EXPECT().GetBoard(member.BoardID).
			Return(&model.Board{ID: member.BoardID, IsTemplate: true, CreatedBy: model.SystemUserID}, nil).AnyTimes()

		assert.True(t, th.permissions.HasPermissionToBoard(member.UserID, member.BoardID, model.PermissionViewBoard))
		for _, p := range []*model.Permission{model.PermissionManageBoardCards, model.PermissionManageBoardProperties, model.PermissionDeleteBoard} {
			assert.False(t, th.permissions.HasPermissionToBoard(member.UserID, member.BoardID, p), p.Id)
		}
	})

	t.Run("board admin", func(t *testing.T) {
		member := &model.BoardMember{
			UserID:      "user-id",
			BoardID:     "board-id",
			SchemeAdmin: true,
		}

		hasPermissionTo := []*model.Permission{
			model.PermissionManageBoardType,
			model.PermissionDeleteBoard,
			model.PermissionManageBoardRoles,
			model.PermissionShareBoard,
			model.PermissionManageBoardCards,
			model.PermissionViewBoard,
			model.PermissionManageBoardProperties,
		}

		hasNotPermissionTo := []*model.Permission{}

		th.checkBoardPermissions("admin", member, hasPermissionTo, hasNotPermissionTo)
	})

	t.Run("board editor", func(t *testing.T) {
		member := &model.BoardMember{
			UserID:       "user-id",
			BoardID:      "board-id",
			SchemeEditor: true,
		}

		hasPermissionTo := []*model.Permission{
			model.PermissionManageBoardCards,
			model.PermissionViewBoard,
			model.PermissionManageBoardProperties,
		}

		hasNotPermissionTo := []*model.Permission{
			model.PermissionManageBoardType,
			model.PermissionDeleteBoard,
			model.PermissionManageBoardRoles,
			model.PermissionShareBoard,
		}

		th.checkBoardPermissions("editor", member, hasPermissionTo, hasNotPermissionTo)
	})

	t.Run("board commenter", func(t *testing.T) {
		member := &model.BoardMember{
			UserID:          "user-id",
			BoardID:         "board-id",
			SchemeCommenter: true,
		}

		hasPermissionTo := []*model.Permission{
			model.PermissionViewBoard,
		}

		hasNotPermissionTo := []*model.Permission{
			model.PermissionManageBoardType,
			model.PermissionDeleteBoard,
			model.PermissionManageBoardRoles,
			model.PermissionShareBoard,
			model.PermissionManageBoardCards,
			model.PermissionManageBoardProperties,
		}

		th.checkBoardPermissions("commenter", member, hasPermissionTo, hasNotPermissionTo)
	})

	t.Run("board viewer", func(t *testing.T) {
		member := &model.BoardMember{
			UserID:       "user-id",
			BoardID:      "board-id",
			SchemeViewer: true,
		}

		hasPermissionTo := []*model.Permission{
			model.PermissionViewBoard,
		}

		hasNotPermissionTo := []*model.Permission{
			model.PermissionManageBoardType,
			model.PermissionDeleteBoard,
			model.PermissionManageBoardRoles,
			model.PermissionShareBoard,
			model.PermissionManageBoardCards,
			model.PermissionManageBoardProperties,
		}

		th.checkBoardPermissions("viewer", member, hasPermissionTo, hasNotPermissionTo)
	})

	t.Run("Manage Team Permission ", func(t *testing.T) {
		member := &model.BoardMember{
			UserID:       "user-id",
			BoardID:      "board-id",
			SchemeViewer: true,
		}

		hasPermissionTo := []*model.Permission{
			model.PermissionViewBoard,
		}

		hasNotPermissionTo := []*model.Permission{
			model.PermissionManageBoardType,
			model.PermissionDeleteBoard,
			model.PermissionManageBoardRoles,
			model.PermissionShareBoard,
			model.PermissionManageBoardCards,
			model.PermissionManageBoardProperties,
		}

		th.checkBoardPermissions("viewer", member, hasPermissionTo, hasNotPermissionTo)
	})
}
