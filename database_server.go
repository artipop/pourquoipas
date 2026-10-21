//go:build server

package main

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// boardDatabase is where a hosted server keeps its boards: SQLite in the
// install's own directory unless the environment names another database.
//
//	POURQUOIPAS_DB_TYPE  sqlite3 (the default), postgres or mysql
//	POURQUOIPAS_DB_DSN   the connection string in that driver's own syntax; for
//	               sqlite3 a file path, defaulting to <dataDir>/app.db
//
// The environment rather than a settings file, because that is how the rest of
// this build is configured — WAILS_SERVER_HOST/PORT — and how a container is
// (build/docker/Dockerfile.server). The board runs on all three; the agents and
// the sources run on SQLite only for now (appTablesSupported).
func boardDatabase(data string) (dbType, dsn string, err error) {
	dbType = strings.TrimSpace(os.Getenv("POURQUOIPAS_DB_TYPE"))
	dsn = strings.TrimSpace(os.Getenv("POURQUOIPAS_DB_DSN"))
	switch dbType {
	case "", "sqlite3":
		if dsn == "" {
			dsn = path.Join(data, "app.db")
		}
		return "sqlite3", dsn, nil
	case "postgres", "mysql":
		if dsn == "" {
			return "", "", fmt.Errorf("POURQUOIPAS_DB_TYPE=%s needs POURQUOIPAS_DB_DSN, the connection string", dbType)
		}
		return dbType, dsn, nil
	}
	return "", "", fmt.Errorf("POURQUOIPAS_DB_TYPE=%q: want sqlite3, postgres or mysql", dbType)
}
