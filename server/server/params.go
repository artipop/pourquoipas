package server

import (
	"fmt"

	"github.com/artipop/pourquoipas/server/services/config"
	"github.com/artipop/pourquoipas/server/services/notify"
	"github.com/artipop/pourquoipas/server/services/permissions"
	"github.com/artipop/pourquoipas/server/services/store"
	"github.com/artipop/pourquoipas/server/ws"

	"github.com/artipop/pourquoipas/server/mlog"
)

type Params struct {
	Cfg                *config.Configuration
	SingleUserToken    string
	DBStore            store.Store
	Logger             mlog.LoggerIFace
	ServerID           string
	WSAdapter          ws.Adapter
	NotifyBackends     []notify.Backend
	PermissionsService permissions.PermissionsService
}

func (p Params) CheckValid() error {
	if p.Cfg == nil {
		return ErrServerParam{name: "Cfg", issue: "cannot be nil"}
	}

	if p.DBStore == nil {
		return ErrServerParam{name: "DbStore", issue: "cannot be nil"}
	}

	if p.Logger == nil {
		return ErrServerParam{name: "Logger", issue: "cannot be nil"}
	}

	if p.PermissionsService == nil {
		return ErrServerParam{name: "Permissions", issue: "cannot be nil"}
	}
	return nil
}

type ErrServerParam struct {
	name  string
	issue string
}

func (e ErrServerParam) Error() string {
	return fmt.Sprintf("invalid server params: %s %s", e.name, e.issue)
}
