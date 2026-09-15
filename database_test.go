package main

import "testing"

// On a server that keeps its boards in Postgres or MySQL the agents and the
// sources are switched off, and the log says why: their queries are written
// for SQLite, and a subsystem that fails at its first write — a card moved, an
// item fetched — is worse than one that says at startup that it is not there.
func TestTheAppsOwnTablesAreRefusedOnADatabaseTheyAreNotWrittenFor(t *testing.T) {
	for _, dbType := range []string{"postgres", "mysql"} {
		if _, err := openAgentStore(board{dbType: dbType}, ""); err == nil {
			t.Errorf("the agent store opened on %s", dbType)
		}
		if _, err := openSourceStore(board{dbType: dbType}, ""); err == nil {
			t.Errorf("the source store opened on %s", dbType)
		}
	}
	if _, err := openAgentStore(board{dbType: "sqlite3"}, ""); err != nil {
		t.Errorf("the agent store refused SQLite: %v", err)
	}
	if _, err := openSourceStore(board{dbType: "sqlite3"}, ""); err != nil {
		t.Errorf("the source store refused SQLite: %v", err)
	}
}
