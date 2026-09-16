package sqlstore

import "strings"

// The connection settings SQLite takes in its DSN rather than in SQL.
//
// They are per *connection*, which is why they are here and not in a migration:
// `PRAGMA foreign_keys` executed once would hold for one connection and be
// silently off on the next. The pool is capped at one connection on SQLite
// (server.NewStore), so a pragma would in fact stick — but relying on that
// would make a pool setting load-bearing for referential integrity, which is a
// trap for whoever raises the cap.
//
// modernc takes pragmas through a repeated `_pragma=`, and the busy timeout is
// one of them rather than a parameter of its own. A parameter it does not know
// is ignored silently, so a DSN spelled for mattn (`_foreign_keys=on`) is not an
// error but keys left off.
var (
	foreignKeysOn = [2]string{"_pragma", "foreign_keys(1)"}
	busyTimeout   = [2]string{"_pragma", "busy_timeout(5000)"}
)

// SQLiteDSN adds the connection settings the application runs with: foreign
// keys enforced and a busy timeout.
func SQLiteDSN(dsn string) string {
	return addParams(dsn, [][2]string{foreignKeysOn, busyTimeout})
}

// SQLiteDSNWithoutForeignKeys is SQLiteDSN with the keys left off — for a test
// database whose fixtures name cards no board ever had (appschema.Open).
func SQLiteDSNWithoutForeignKeys(dsn string) string {
	return addParams(dsn, [][2]string{busyTimeout})
}

// addParams appends the settings the DSN has not already got. Matched on the
// pragma rather than the key, because every pragma shares one key — and on
// `name(` rather than the bare name, because `_busy_timeout=5000` contains the
// bare name and would otherwise be read as having asked for it already.
//
// A DSN that already has a query is appended to rather than given a second `?`;
// that was found by a test whose DSN already carried a busy timeout.
func addParams(dsn string, params [][2]string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	var b strings.Builder
	b.WriteString(dsn)
	for _, p := range params {
		name := p[1]
		if i := strings.IndexByte(name, '('); i > 0 {
			name = name[:i+1]
		}
		if strings.Contains(dsn, name) {
			continue
		}
		b.WriteString(sep)
		b.WriteString(p[0])
		b.WriteString("=")
		b.WriteString(p[1])
		sep = "&"
	}
	return b.String()
}
