package app

import (
	"runtime"

	"github.com/artipop/xciii/server/model"
)

// ServerMetadata is what /ping answers: enough to tell which application is
// running and on what.
//
// Four fields went with the plugin mode: build_number, build_date and commit
// were -ldflags upstream injected and this tree never sets, and edition named
// whether the board was a Mattermost plugin, which it cannot be any more. sku
// went with them — it existed to say "suite" for that same edition.
type ServerMetadata struct {
	Version string `json:"version"`
	// Capabilities is what this server answers beyond Focalboard's API, by name
	// (model.Capabilities); a client checks for a name, never for a version.
	Capabilities []string `json:"capabilities"`
	DBType       string   `json:"db_type"`
	DBVersion    string   `json:"db_version"`
	OSType       string   `json:"os_type"`
	OSArch       string   `json:"os_arch"`
}

func (a *App) GetServerMetadata() *ServerMetadata {
	var dbType string
	var dbVersion string
	if a != nil && a.store != nil {
		dbType = a.store.DBType()
		dbVersion = a.store.DBVersion()
	}

	return &ServerMetadata{
		Version:      model.CurrentVersion,
		Capabilities: model.Capabilities,
		DBType:       dbType,
		DBVersion:    dbVersion,
		OSType:       runtime.GOOS,
		OSArch:       runtime.GOARCH,
	}
}
