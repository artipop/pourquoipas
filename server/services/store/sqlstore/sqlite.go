package sqlstore

import (
	"database/sql"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "modernc.org/sqlite" // registers itself as "sqlite"
)

// SQLite is modernc's pure-Go driver on every platform, registered again under
// the name the rest of the tree opens: "sqlite3" is DBType, and DBType is both
// the dialect and the driver name.
//
// One driver rather than mattn's cgo SQLite wherever a C compiler happened to
// be: macOS and Linux keep cgo for Wails alone, and no build tag decides which
// SQLite an install runs on (docs/build-and-platforms.md).
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

// sqliteMigrationDriver is golang-migrate's SQLite half for this driver. Its
// `sqlite3` package is the same code importing mattn on the side, which would
// link a second SQLite into the binary.
func sqliteMigrationDriver(db *sql.DB, table string) (database.Driver, error) {
	return sqlite.WithInstance(db, &sqlite.Config{MigrationsTable: table})
}
