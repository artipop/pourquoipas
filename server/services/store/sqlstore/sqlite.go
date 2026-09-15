//go:build sqlite3 && cgo

package sqlstore

import (
	"database/sql"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/mattn/go-sqlite3" // sqlite driver
)

// sqliteMigrationDriver is golang-migrate's SQLite half for the driver this
// build opens. Its two SQLite packages are the same code and run on the
// connection they are handed; they differ only in the driver each imports on
// the side, so taking the other one would link a second SQLite into the binary
// (modernc was 18 s of CPU in a cold cgo build, for nothing).
func sqliteMigrationDriver(db *sql.DB, table string) (database.Driver, error) {
	return sqlite3.WithInstance(db, &sqlite3.Config{MigrationsTable: table})
}
