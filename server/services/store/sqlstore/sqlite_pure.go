//go:build !(sqlite3 && cgo)

package sqlstore

import (
	"database/sql"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "modernc.org/sqlite" // registers itself as "sqlite"
)

// The pure-Go SQLite, registered under the name the rest of the tree opens:
// "sqlite3" is DBType, and DBType is both the dialect and the driver name.
//
// Chosen by `cgo` as well as by the tag, because a tag cannot tell whether a C
// compiler was there. `-tags sqlite3` with CGO_ENABLED=0 used to compile clean
// and link mattn's stub, which registers "sqlite3" and refuses every Open —
// the headless build died at its first query, and a Windows build needed MinGW
// for nothing else. Now that combination gets this driver instead.
//
// The instance is the one modernc registered, fetched back through
// database/sql, so functions and hooks registered on it apply under either
// name.
func init() {
	db, err := sql.Open("sqlite", "")
	if err != nil {
		panic(err)
	}
	sql.Register("sqlite3", db.Driver())
	_ = db.Close()
}

// sqliteMigrationDriver: see sqlite.go. This one imports modernc, which this
// build links anyway.
func sqliteMigrationDriver(db *sql.DB, table string) (database.Driver, error) {
	return sqlite.WithInstance(db, &sqlite.Config{MigrationsTable: table})
}
