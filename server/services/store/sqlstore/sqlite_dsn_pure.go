//go:build !(sqlite3 && cgo)

package sqlstore

import "strings"

// The same settings for the pure-Go driver (modernc.org/sqlite), which spells
// them differently: pragmas go through a repeated `_pragma=` and the busy
// timeout is one of them rather than a parameter of its own. See the cgo half
// for why this is a DSN and not a migration.
var (
	foreignKeysOn = [2]string{"_pragma", "foreign_keys(1)"}
	busyTimeout   = [2]string{"_pragma", "busy_timeout(5000)"}
)

// addParams appends the settings the DSN has not already got. Matched on the
// pragma rather than the key, because every pragma shares one key — and on
// `name(` rather than the bare name, because a DSN spelled for mattn
// (`_busy_timeout=5000`, which modernc ignores) contains the bare name and
// would otherwise be read as having asked for it already.
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
