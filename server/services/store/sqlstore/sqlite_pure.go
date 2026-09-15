//go:build !(sqlite3 && cgo)

package sqlstore

import (
	"database/sql"

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
// modernc is already in every build through golang-migrate's sqlite driver
// (migrate.go), so this adds a name, not a dependency. The instance is the one
// modernc registered, fetched back through database/sql, so functions and
// hooks registered on it apply under either name.
func init() {
	db, err := sql.Open("sqlite", "")
	if err != nil {
		panic(err)
	}
	sql.Register("sqlite3", db.Driver())
	_ = db.Close()
}
