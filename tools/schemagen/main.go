package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:generate go run . -write

// migrationsDir is where the board's migration ladder lives, relative to the
// module root. Ours is a rung on it rather than a ladder of its own: one
// database, one engine, one version.
const migrationsDir = "server/services/store/sqlstore/migrations"

// The schema is two files, and was eighty-one. The ladder is gone because every
// step of it led to this same schema and none of them means anything to a
// database being created now.
//
// What made that safe to do is that the result is checkable: `go run
// ./tools/schemagen -collapsed-sqlite` prints the whole schema as plain SQLite
// DDL, and it compares equal — table, column, key and index — to a database the
// eighty-one migrations built. See the test.
//
// The schema is two migrations, split by whose tables they are.
//
// 000001 is the board's own — the fork's tables as this product has them,
// corrections included. 000002 is what this application adds beside them:
// the registries, the conversations, where a card stands on its route. They
// are two files because they are two subjects, and one file of forty tables
// is one a reviewer scrolls rather than reads.
//
// The order is not a preference: MySQL wants a foreign key's target to exist
// by the time the key is written, and every key in the second file points at
// a table in the first.
//
// A third file, for what working as a team added on top, was written and thrown
// away. It worked — atlas plans an ALTER per dialect as happily as a CREATE —
// and it cost the generator a new idea: a column knowing which migration it
// arrived in. That is the layering the collapse of eighty-one migrations
// removed, coming back for two columns. The schema is what the tables say, and
// when it changes the file it is in changes with it.
const (
	boardMigration = "000001_board"
	appMigration   = "000002_app"
)

func main() {
	write := flag.Bool("write", false, "write the migration files instead of printing them")
	collapsed := flag.Bool("collapsed-sqlite", false,
		"print the whole schema — the fork's tables and ours — as plain SQLite DDL, "+
			"for diffing against a database the migrations built")
	erd := flag.Bool("erd", false, "print the schema as a mermaid ER diagram")
	flag.Parse()

	if *erd {
		fmt.Print(renderERDDoc())
		return
	}

	if *collapsed {
		sql, err := renderUp(dialects()[0], append(boardTables(), appTables()...))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(sql)
		return
	}

	boardUp, boardDown, err := render(boardTables(), boardHeader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	appUp, appDown, err := render(appTables(), appHeader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !*write {
		fmt.Print(boardUp)
		fmt.Print(appUp)
		return
	}
	root, err := moduleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dir := filepath.Join(root, migrationsDir)
	for name, body := range map[string]string{
		boardMigration + ".up.sql":   boardUp,
		boardMigration + ".down.sql": boardDown,
		appMigration + ".up.sql":     appUp,
		appMigration + ".down.sql":   appDown,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	// The diagram, its page and the HCL are generated beside the SQL rather
	// than by a second command, because a picture nobody remembers to
	// regenerate is a wrong picture.
	if err := os.WriteFile(filepath.Join(root, erdPath), []byte(renderERDDoc()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	page, err := renderERDPage()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(root, erdPagePath), []byte(page), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	hcl, err := renderHCL()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(root, hclPath), []byte(hcl), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

const (
	// erdPath is the schema drawn, and hclPath the same schema as Atlas HCL,
	// both relative to the module root.
	erdPath = "docs/schema/erd.md"
	hclPath = "docs/schema/app.hcl"
)

// moduleRoot walks up from the working directory to the go.mod, so the
// generator writes to the same place whether it was run by `go generate` from
// its own directory or by `go run ./tools/schemagen` from the root.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// header is what anybody who opens the SQL reads first, because the SQL is the
// file a reviewer sees in the diff and the one a migration failure names.
const header = `-- Generated by tools/schemagen. Do not edit by hand: change the schema there
-- and run ` + "`go generate ./tools/schemagen`" + `.
--
-- The three dialects below say the same thing. Writing them by hand is what
-- eighty migrations of this fork do, and every one of them is a place where
-- somebody has to remember three answers to one question.
`

const boardHeader = header + `--
-- The board's own tables: what a board, a card and a person are. The fork's,
-- as this product has them.
`

const appHeader = header + `--
-- What this application adds beside the board: the machine's registries, the
-- conversations and sessions, where a card stands on its route. Every foreign
-- key here points at a table the migration before it made.
`

// render builds both halves of one migration: every dialect's DDL, each behind
// the condition golang-migrate's template already understands.
func render(tables []Table, head string) (up, down string, err error) {
	var ub, db strings.Builder
	ub.WriteString(head)
	db.WriteString(head)
	for _, d := range dialects() {
		body, err := renderUp(d, tables)
		if err != nil {
			return "", "", err
		}
		ub.WriteString(section(d.name, body))

		body, err = renderDown(d, tables)
		if err != nil {
			return "", "", err
		}
		db.WriteString(section(d.name, body))
	}
	return ub.String(), db.String(), nil
}

func section(name, body string) string {
	return "\n{{if ." + name + "}}\n" + strings.TrimRight(body, "\n") + "\n{{end}}\n"
}
