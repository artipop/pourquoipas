//go:generate mockgen -destination=mocks/mockstore.go -package mocks . Store

package permissions

import (
	"github.com/artipop/pourquoipas/server/model"
)

type PermissionsService interface {
	HasPermissionTo(userID string, permission *model.Permission) bool
	HasPermissionToTeam(userID, teamID string, permission *model.Permission) bool
	HasPermissionToChannel(userID, channelID string, permission *model.Permission) bool
	HasPermissionToBoard(userID, boardID string, permission *model.Permission) bool
}

type Store interface {
	// GetUserByID is what makes an install-wide admin possible: every other
	// question here is answered by board membership, and "who administers this
	// install" is not a board question.
	GetUserByID(userID string) (*model.User, error)
	GetBoard(boardID string) (*model.Board, error)
	GetMemberForBoard(boardID, userID string) (*model.BoardMember, error)
	GetBoardHistory(boardID string, opts model.QueryBoardHistoryOptions) ([]*model.Board, error)
}
