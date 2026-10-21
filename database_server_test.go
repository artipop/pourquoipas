//go:build server

package main

import (
	"path"
	"testing"
)

// A server with nothing said keeps its boards where a desktop does: a SQLite
// file in its own directory. Starting a server should not require a database
// administrator.
func TestAServerWithNothingSaidKeepsItsBoardsInSQLite(t *testing.T) {
	t.Setenv("POURQUOIPAS_DB_TYPE", "")
	t.Setenv("POURQUOIPAS_DB_DSN", "")
	dbType, dsn, err := boardDatabase("/data/server")
	if err != nil {
		t.Fatal(err)
	}
	if dbType != "sqlite3" || dsn != path.Join("/data/server", "pourquoipas.db") {
		t.Fatalf("got %s %q", dbType, dsn)
	}
}

// Postgres and MySQL are named by the environment, with the driver's own
// connection string beside them.
func TestAServerIsToldItsDatabaseByTheEnvironment(t *testing.T) {
	t.Setenv("POURQUOIPAS_DB_TYPE", "postgres")
	t.Setenv("POURQUOIPAS_DB_DSN", "postgres://pourquoipas@db/pourquoipas?sslmode=disable")
	dbType, dsn, err := boardDatabase("/data/server")
	if err != nil {
		t.Fatal(err)
	}
	if dbType != "postgres" || dsn != "postgres://pourquoipas@db/pourquoipas?sslmode=disable" {
		t.Fatalf("got %s %q", dbType, dsn)
	}
}

// A misspelled type or a missing connection string stops the server at once:
// quietly falling back to a SQLite file would put a team's boards somewhere
// nobody backs up.
func TestAServerRefusesADatabaseItCannotOpen(t *testing.T) {
	for _, tc := range []struct{ dbType, dsn string }{
		{"postgres", ""},
		{"mysql", ""},
		{"postgresql", "postgres://db"},
	} {
		t.Setenv("POURQUOIPAS_DB_TYPE", tc.dbType)
		t.Setenv("POURQUOIPAS_DB_DSN", tc.dsn)
		if _, _, err := boardDatabase("/data/server"); err == nil {
			t.Errorf("POURQUOIPAS_DB_TYPE=%q POURQUOIPAS_DB_DSN=%q was accepted", tc.dbType, tc.dsn)
		}
	}
}
