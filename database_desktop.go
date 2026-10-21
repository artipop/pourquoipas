//go:build !server

package main

import "path"

// boardDatabase is where a desktop keeps its boards: one SQLite file in the
// install's own directory. A desktop is one person, and there is nothing here
// to configure — the server build is the one that asks (database_server.go).
func boardDatabase(data string) (dbType, dsn string, err error) {
	return "sqlite3", path.Join(data, "pourquoipas.db"), nil
}
