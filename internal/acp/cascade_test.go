package acp

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/appschema"
)

// enforcingStore is a store on a database with the foreign keys enforced, the
// way the application enforces them.
func enforcingStore(t *testing.T) (*Store, func(cardID string)) {
	t.Helper()
	db, err := appschema.OpenEnforcing(filepath.Join(t.TempDir(), "pourquoipas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appschema.AddCard(db, "board-1", "card-1"); err != nil {
		t.Fatal(err)
	}
	remove := func(cardID string) {
		t.Helper()
		if _, err := db.Exec(`DELETE FROM blocks WHERE id=?`, cardID); err != nil {
			t.Fatal(err)
		}
	}
	return NewStore(db), remove
}

// Deleting a card takes everything this application knew about it. This is the
// whole reason the tables moved into the board's database:
// deleting a card is a real DELETE FROM blocks, this side never heard about it —
// BlockChanged handles only notify.Update — so every deleted card left its
// sessions, its stall and its recorded changes behind for ever, in a file where
// no foreign key could reach them.
func TestADeletedCardTakesWhatWeKnewAboutIt(t *testing.T) {
	st, deleteCard := enforcingStore(t)

	if err := st.InsertSession(SessionRecord{
		ID: "s1", CardID: "card-1", BoardID: "board-1", AgentKind: "claude",
		Status: StatusRunning, StartedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetStall(StallRecord{CardID: "card-1", Reason: apperr.Plain("stands")}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetCardChanges(CardChangesRecord{CardID: "card-1", BoardID: "board-1", Files: 2}); err != nil {
		t.Fatal(err)
	}

	deleteCard("card-1")

	if rows, _, err := st.SessionsForCard("card-1"); err != nil || len(rows) != 0 {
		t.Errorf("the card's sessions outlived it: %+v %v", rows, err)
	}
	if _, ok, err := st.Stall("card-1"); err != nil || ok {
		t.Errorf("the card's stall outlived it: %v %v", ok, err)
	}
	if _, ok, err := st.CardChanges("card-1"); err != nil || ok {
		t.Errorf("the card's recorded changes outlived it: %v %v", ok, err)
	}
}

// A source's log is the exception, and deliberately: what a source decided is a
// fact about the source, so the line stays and only the card reference goes.
func TestASourcesLogSurvivesTheCardItMade(t *testing.T) {
	db, err := appschema.OpenEnforcing(filepath.Join(t.TempDir(), "pourquoipas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appschema.AddCard(db, "board-1", "card-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO source_event (id, source, outcome, card_id, created_at)
		VALUES (?,?,?,?,?)`, newID(), "почта", "created", "card-1", time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`DELETE FROM blocks WHERE id='card-1'`); err != nil {
		t.Fatal(err)
	}

	var count int
	var cardID any
	if err := db.QueryRow(`SELECT COUNT(*), MAX(card_id) FROM source_event`).Scan(&count, &cardID); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("the log line went with the card: %d rows left", count)
	}
	if cardID != nil {
		t.Errorf("the log still points at a card that is gone: %v", cardID)
	}
}

// A workspace a card is still working in cannot be deleted out from under it:
// there is a copy on disk, and it has to be folded away first.
func TestAWorkspaceInUseCannotBeDeleted(t *testing.T) {
	db, err := appschema.OpenEnforcing(filepath.Join(t.TempDir(), "pourquoipas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appschema.AddCard(db, "board-1", "card-1"); err != nil {
		t.Fatal(err)
	}
	st := NewStore(db)
	if err := st.SaveWorkspace(WorkdirEntry{ID: "ws-1", Name: "код", Path: "/repo", BoardID: "board-1"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCheckout(Checkout{
		WorkspaceID: "ws-1", CardID: "card-1", Mode: WorkModeWorktree, Branch: "feat/x",
	}); err != nil {
		t.Fatal(err)
	}

	if err := st.DeleteWorkspace("ws-1"); err == nil {
		t.Error("a folder with work in it was deleted anyway")
	}

	// The card going is what releases it — and then the folder can go.
	if _, err := db.Exec(`DELETE FROM blocks WHERE id='card-1'`); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteWorkspace("ws-1"); err != nil {
		t.Errorf("nothing holds the folder now, and it still would not go: %v", err)
	}
}

// A typo in a column whose set is closed has to be refused by the database, not
// stored and read back later as a state nothing handles. Which columns get a
// check is a judgement stated in tools/schemagen/types.go — the sets closed by
// the model, never the ones that grow with a new vendor CLI or a new event.
func TestAValueOutsideAClosedSetIsRefused(t *testing.T) {
	db, err := appschema.OpenEnforcing(filepath.Join(t.TempDir(), "pourquoipas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appschema.AddCard(db, "board-1", "card-1"); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		what  string
		query string
		args  []any
	}{
		{
			"a session status nothing can mean",
			`INSERT INTO agent_session (id, card_id, agent_kind, status, started_at)
			 VALUES ('s1', 'card-1', 'claude', ?, 1)`,
			[]any{"runnnig"},
		},
		{
			"a way of working a folder that does not exist",
			`INSERT INTO checkout (id, workspace_id, card_id, mode, created_at)
			 VALUES ('c1', 'w1', 'card-1', ?, 1)`,
			[]any{"worktee"},
		},
		{
			"a setup step in a state the wizard never writes",
			`INSERT INTO board_setup (board_id, step, status, changed_at)
			 VALUES ('board-1', 'workdir', ?, 1)`,
			[]any{"answered"},
		},
		{
			"an outcome no rule produces",
			`INSERT INTO source_event (id, source, outcome, created_at)
			 VALUES ('e1', 'mail', ?, 1)`,
			[]any{"maybe"},
		},
	} {
		t.Run(c.what, func(t *testing.T) {
			if _, err := db.Exec(c.query, c.args...); err == nil {
				t.Errorf("the database accepted %v", c.args[0])
			}
		})
	}

	// And the values the code actually writes go in. A check that refuses
	// everything would pass the loop above and break the product.
	if _, err := db.Exec(`INSERT INTO agent_session (id, card_id, agent_kind, status, started_at)
		VALUES ('s2', 'card-1', 'claude', 'running', 1)`); err != nil {
		t.Errorf("a status the code writes was refused: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO board_setup (board_id, step, status, changed_at)
		VALUES ('board-1', 'workdir', 'skipped', 1)`); err != nil {
		t.Errorf("a setup status the code writes was refused: %v", err)
	}
}

// A card on somebody else's board has no row in this machine's blocks, and a
// foreign key reaches only inside its own database — so before card_ref stood
// between, writing anything about such a card was not ambiguous, it was
// refused. This is the case the whole ref table exists for.
func TestACardOnAnotherServerCanBeWrittenAbout(t *testing.T) {
	db, err := appschema.OpenEnforcing(filepath.Join(t.TempDir(), "pourquoipas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st := NewStore(db)

	// No AddCard: this card is on a server, and nothing about it exists here.
	if _, err := db.Exec(`INSERT INTO board_ref (board_id, source_id, local_board_id, created_at)
		VALUES ('remote-board', 'https://board.example.com', NULL, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO card_ref (card_id, board_id, block_id, created_at)
		VALUES ('remote-card', 'remote-board', NULL, 0)`); err != nil {
		t.Fatal(err)
	}

	if err := st.SetStall(StallRecord{CardID: "remote-card", Reason: apperr.Plain("no folder")}); err != nil {
		t.Fatalf("a stall on a card of another board was refused: %v", err)
	}
	if err := st.SetCardChanges(CardChangesRecord{CardID: "remote-card", BoardID: "remote-board", Files: 1}); err != nil {
		t.Fatalf("a change count on a card of another board was refused: %v", err)
	}

	// And it is still owned: the client that hears the board say the card is
	// gone deletes the ref, and everything behind it goes.
	if _, err := db.Exec(`DELETE FROM card_ref WHERE card_id='remote-card'`); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.Stall("remote-card"); err != nil || ok {
		t.Errorf("the stall outlived the card: %v %v", ok, err)
	}
	if _, ok, err := st.CardChanges("remote-card"); err != nil || ok {
		t.Errorf("the change count outlived the card: %v %v", ok, err)
	}
}

// The ref is made by the write itself, not by somebody remembering to. Every
// path that records something about a card goes through the store, and one that
// forgot would be a foreign-key error at the worst moment — a card moving.
func TestWritingAboutACardMakesItsRefOnTheWay(t *testing.T) {
	db, err := appschema.OpenEnforcing(filepath.Join(t.TempDir(), "pourquoipas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appschema.AddBoard(db, "board-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO blocks (id, board_id, type) VALUES ('card-9','board-1','card')`); err != nil {
		t.Fatal(err)
	}
	st := NewStore(db)

	if err := st.SetStall(StallRecord{CardID: "card-9", Reason: apperr.Plain("no folder")}); err != nil {
		t.Fatalf("the first write about a card was refused: %v", err)
	}

	// And it carries the block, so the cascade a local card always had is the
	// same cascade.
	if _, err := db.Exec(`DELETE FROM blocks WHERE id='card-9'`); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.Stall("card-9"); err != nil || ok {
		t.Errorf("what we knew about the card outlived the card: %v %v", ok, err)
	}
}

// The reason a card stands still is kept as a code and its values, so the page
// words it in the language of whoever reads it, and the English beside them
// for a page that does not know the code.
func TestAStallKeepsItsCodeAndValues(t *testing.T) {
	st, _ := enforcingStore(t)
	why := apperr.Say("folders.dirty", "The folder {dir} has unsaved changes", "dir", "/work/web")
	if err := st.SetStall(StallRecord{CardID: "card-1", Reason: why}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.Stall("card-1")
	if err != nil || !ok {
		t.Fatalf("stall: %v %v", ok, err)
	}
	if got.Reason.Code != why.Code || got.Reason.Args["dir"] != "/work/web" || got.Reason.Text != why.Text {
		t.Errorf("got %+v, want %+v", got.Reason, why)
	}
}
