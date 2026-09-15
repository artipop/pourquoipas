package sqlstore

// SQLiteDSN adds the connection settings the application runs with: foreign
// keys enforced and a busy timeout, spelled for whichever driver the build
// chose (sqlite_dsn_cgo.go, sqlite_dsn_pure.go).
func SQLiteDSN(dsn string) string {
	return addParams(dsn, [][2]string{foreignKeysOn, busyTimeout})
}

// SQLiteDSNWithoutForeignKeys is SQLiteDSN with the keys left off — for a test
// database whose fixtures name cards no board ever had (appschema.Open).
func SQLiteDSNWithoutForeignKeys(dsn string) string {
	return addParams(dsn, [][2]string{busyTimeout})
}
