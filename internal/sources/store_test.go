package sources

import (
	"path/filepath"
	"testing"
)

// The log is read per source, so it is kept per source: a limit on the whole
// table let one chatty source wash out the log of every quiet one, which is
// exactly the log somebody opens to ask why that quiet source did nothing.
func TestAChattySourceDoesNotWashOutAQuietOnesLog(t *testing.T) {
	store, err := newTestStore(t, filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := store.AppendEvent(EventRecord{Source: "chatty", Outcome: OutcomeDropped}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := store.AppendEvent(EventRecord{Source: "quiet", Outcome: OutcomeInbox}); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.PruneEvents("chatty", 2); err != nil {
		t.Fatal(err)
	}
	if err := store.PruneEvents("quiet", 2); err != nil {
		t.Fatal(err)
	}

	chatty, _ := store.Events("chatty", 100)
	quiet, _ := store.Events("quiet", 100)
	if len(chatty) != 2 || len(quiet) != 2 {
		t.Fatalf("kept %d chatty and %d quiet lines, want 2 of each", len(chatty), len(quiet))
	}

	// A log shorter than the limit is left alone.
	if err := store.PruneEvents("quiet", 10); err != nil {
		t.Fatal(err)
	}
	if quiet, _ := store.Events("quiet", 100); len(quiet) != 2 {
		t.Fatalf("a short log lost lines: %d left", len(quiet))
	}
}

// A source created again under an old name must start clean, log included.
func TestForgettingASourceTakesItsLogWithIt(t *testing.T) {
	store, err := newTestStore(t, filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RememberItem("mail", "m1", "v1", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(EventRecord{Source: "mail", Outcome: OutcomeInbox}); err != nil {
		t.Fatal(err)
	}

	if err := store.ForgetSource("mail"); err != nil {
		t.Fatal(err)
	}

	if state, _, _ := store.StateOf("mail", "m1", "v1"); state != ItemNew {
		t.Errorf("item state %q after forgetting, want %q", state, ItemNew)
	}
	if events, _ := store.Events("mail", 100); len(events) != 0 {
		t.Errorf("%d log lines survived forgetting the source", len(events))
	}
}
