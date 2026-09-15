package localpermissions

import (
	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/services/permissions"

	"github.com/artipop/xciii/server/mlog"
)

type Service struct {
	store  permissions.Store
	logger mlog.LoggerIFace
}

func New(store permissions.Store, logger mlog.LoggerIFace) *Service {
	return &Service{
		store:  store,
		logger: logger,
	}
}

// isAdmin: who administers this install. A desktop asks nobody who they are and
// has no admin at all — the id its session carries has no row — which is right:
// there is nobody to administer *against*.
func (s *Service) isAdmin(userID string) bool {
	if userID == "" {
		return false
	}
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		if !model.IsErrNotFound(err) {
			s.logger.Error("error reading a user for a permission check",
				mlog.String("userID", userID), mlog.Err(err))
		}
		return false
	}
	return user != nil && user.Role == model.RoleAdmin
}

// HasPermissionTo is only ever asked PermissionManageSystem — five call sites,
// all of them "may this person see or do something about the install rather
// than about a board".
func (s *Service) HasPermissionTo(userID string, permission *model.Permission) bool {
	if permission == nil {
		return false
	}
	return s.isAdmin(userID)
}

func (s *Service) HasPermissionToTeam(userID, teamID string, permission *model.Permission) bool {
	if userID == "" || teamID == "" || permission == nil {
		return false
	}
	// Managing the team is the invite: the signup token is the door into this
	// install, so it is the admin's and not everybody's.
	if permission.Id == model.PermissionManageTeam.Id {
		return s.isAdmin(userID)
	}
	return true
}

func (s *Service) HasPermissionToChannel(userID, channelID string, permission *model.Permission) bool {
	if userID == "" || channelID == "" || permission == nil {
		return false
	}
	return true
}

func (s *Service) HasPermissionToBoard(userID, boardID string, permission *model.Permission) bool {
	if userID == "" || boardID == "" || permission == nil {
		return false
	}

	member, err := s.store.GetMemberForBoard(boardID, userID)
	if model.IsErrNotFound(err) {
		return false
	}
	if err != nil {
		s.logger.Error("error getting member for board",
			mlog.String("boardID", boardID),
			mlog.String("userID", userID),
			mlog.Err(err),
		)
		return false
	}

	switch member.MinimumRole {
	case "admin":
		member.SchemeAdmin = true
	case "editor":
		member.SchemeEditor = true
	case "commenter":
		member.SchemeCommenter = true
	case "viewer":
		member.SchemeViewer = true
	}

	switch permission {
	case model.PermissionManageBoardType, model.PermissionDeleteBoard, model.PermissionManageBoardRoles, model.PermissionShareBoard, model.PermissionDeleteOthersComments:
		return member.SchemeAdmin
	case model.PermissionManageBoardCards, model.PermissionManageBoardProperties:
		return member.SchemeAdmin || member.SchemeEditor
	case model.PermissionCommentBoardCards:
		return member.SchemeAdmin || member.SchemeEditor || member.SchemeCommenter
	case model.PermissionViewBoard:
		return member.SchemeAdmin || member.SchemeEditor || member.SchemeCommenter || member.SchemeViewer
	default:
		return false
	}
}
