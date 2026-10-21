package app

import (
	"github.com/artipop/pourquoipas/server/model"

	"github.com/artipop/pourquoipas/server/mlog"
)

// registrationOpen: nobody has claimed this install yet, so the next person to
// register administers it. A desktop never says so — /register is refused there
// whatever this answers — and a broken read says no, because a database that
// cannot be counted must not advertise an open door.
func (a *App) registrationOpen() bool {
	if !a.config.TeamMode {
		return false
	}
	admins, err := a.store.GetAdminCount()
	if err != nil {
		a.logger.Error("unable to tell whether this install has an admin", mlog.Err(err))
		return false
	}
	return admins == 0
}

func (a *App) GetClientConfig() *model.ClientConfig {
	return &model.ClientConfig{
		RegistrationOpen:         a.registrationOpen(),
		EnablePublicSharedBoards: a.config.EnablePublicSharedBoards,
		TeammateNameDisplay:      a.config.TeammateNameDisplay,
		FeatureFlags:             a.config.FeatureFlags,
		MaxFileSize:              a.config.MaxFileSize,
		TeamMode:                 a.config.TeamMode,
	}
}
