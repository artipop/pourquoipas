package acp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// The viewer draws a file list and one patch per file, so the answer has to be
// cut up per file — and the counts beside each name have to be that file's own.
func TestTheDiffIsCutUpPerFile(t *testing.T) {
	dir := initTestWorkdir(t)
	git(t, dir, "checkout", "-b", "work")
	write(t, filepath.Join(dir, "one.go"), "package one\n")
	write(t, filepath.Join(dir, "two.go"), "package two\nvar x = 1\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "two files")

	budget := cardDiffTotalLimit
	files := readDiff(context.Background(), dir, &budget, "main...HEAD")
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %+v", len(files), files)
	}
	byPath := map[string]DiffFile{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	if got := byPath["two.go"]; got.Added != 2 || got.Removed != 0 {
		t.Errorf("two.go counted +%d -%d, want +2 -0", got.Added, got.Removed)
	}
	if !strings.Contains(byPath["one.go"].Patch, "+package one") {
		t.Errorf("one.go carries no patch to read:\n%s", byPath["one.go"].Patch)
	}
}

// A file an agent has just written is not tracked yet, and a viewer that showed
// only what git diffs would show a stage's whole output as nothing at all.
func TestUntrackedFilesAreNamedEvenWithoutAPatch(t *testing.T) {
	dir := initTestWorkdir(t)
	write(t, filepath.Join(dir, "brief.md"), "# Brief\n")

	files := untrackedFiles(context.Background(), dir)
	if len(files) != 1 || files[0].Path != "brief.md" || !files[0].Untracked {
		t.Fatalf("the new file is missing from the list: %+v", files)
	}
}

// A rename is the one change where the file has two names, and the list has to
// be able to say so — matching a patch to a numstat line by path is exactly
// what a rename breaks, which is why the counts are read off the patch.
func TestARenameKeepsBothNames(t *testing.T) {
	dir := initTestWorkdir(t)
	git(t, dir, "checkout", "-b", "work")
	git(t, dir, "mv", "README.md", "GUIDE.md")
	git(t, dir, "commit", "-m", "rename")

	budget := cardDiffTotalLimit
	files := readDiff(context.Background(), dir, &budget, "main...HEAD")
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1: %+v", len(files), files)
	}
	if files[0].Path != "GUIDE.md" || files[0].OldPath != "README.md" {
		t.Errorf("the rename reads as %q ← %q", files[0].Path, files[0].OldPath)
	}
}

// A patch of a patch: this repository is full of them, and splitting the answer
// on a blank line or on the mere text "diff --git" would cut one in half.
func TestAPatchInsideAPatchIsOneFile(t *testing.T) {
	dir := initTestWorkdir(t)
	git(t, dir, "checkout", "-b", "work")
	write(t, filepath.Join(dir, "sample.patch"), "diff --git a/x b/x\n--- a/x\n+++ b/x\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "a patch as content")

	budget := cardDiffTotalLimit
	files := readDiff(context.Background(), dir, &budget, "main...HEAD")
	if len(files) != 1 || files[0].Path != "sample.patch" {
		t.Fatalf("the patch's own content was read as a second file: %+v", files)
	}
}

// The case the viewer exists for: a card whose work is a branch in a folder on
// this machine, read through the claim rather than through anything the card
// says. The committed half is what the branch adds to its base — the question a
// reviewer is asking.
func TestTheDiffOfACardWithAWorkspaceIsWhatItsBranchAdds(t *testing.T) {
	m, _, _, project := testManager(t, fakeClaudeHappy, nil)
	git(t, project, "checkout", "-b", "fix-login")
	write(t, filepath.Join(project, "auth.go"), "package auth\n")
	git(t, project, "add", ".")
	git(t, project, "commit", "-m", "add auth")

	if err := m.store.SaveCheckout(Checkout{
		WorkspaceID: m.workspaceID(project),
		CardID:      "card1",
		Mode:        WorkModeBranch,
		Branch:      "fix-login",
		Path:        project,
		Base:        "main",
	}); err != nil {
		t.Fatal(err)
	}

	d, err := m.CardDiff("card1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Branch != "fix-login" || d.Base != "main" || d.Dir != project {
		t.Fatalf("the diff is about the wrong place: %+v", d)
	}
	if len(d.Committed) != 1 || d.Committed[0].Path != "auth.go" {
		t.Fatalf("what the branch adds is missing: %+v", d.Committed)
	}
	if d.Reason != nil {
		t.Errorf("a card with work to read said there was none: %q", d.Reason)
	}
}

// The case this exists for. A stage ending folds the card's working copy away
// (FoldWorktree) and leaves the branch behind — and «На ревью», the stage that
// wants to read the work, is the one after that. Reading the copy would answer
// "the folder is gone" exactly where the answer matters.
func TestTheWorkIsStillReadableAfterTheCopyIsFoldedAway(t *testing.T) {
	m, _, _, project := testManager(t, fakeClaudeHappy, nil)
	git(t, project, "checkout", "-b", "fix-login")
	write(t, filepath.Join(project, "auth.go"), "package auth\n")
	git(t, project, "add", ".")
	git(t, project, "commit", "-m", "add auth")
	git(t, project, "checkout", "main")

	// A claim whose copy is not on disk any more, which is what a folded
	// worktree leaves behind.
	if err := m.store.SaveCheckout(Checkout{
		WorkspaceID: m.workspaceID(project),
		CardID:      "card1",
		Mode:        WorkModeWorktree,
		Branch:      "fix-login",
		Path:        filepath.Join(t.TempDir(), "gone"),
		Base:        "main",
	}); err != nil {
		t.Fatal(err)
	}

	d, err := m.CardDiff("card1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Reason != nil {
		t.Fatalf("a folded copy read as nothing to show: %q", d.Reason)
	}
	if len(d.Committed) != 1 || d.Committed[0].Path != "auth.go" {
		t.Fatalf("what the branch holds is missing: %+v", d.Committed)
	}
	// Unsaved work exists only in a copy, and this card has none — what is in
	// the folder belongs to whoever is sitting in it.
	if len(d.Uncommitted) != 0 {
		t.Errorf("the folder's own state was read as this card's: %+v", d.Uncommitted)
	}
}

// The button that opens the diff is drawn on a count, not on a look: it is
// asked on a stage where the copy is already folded, and a board would ask it
// once per card. So the count is written when the work moves.
func TestWhatTheWorkAmountsToIsCountedWhenItMoves(t *testing.T) {
	m, _, _, project := testManager(t, fakeClaudeHappy, nil)
	git(t, project, "checkout", "-b", "fix-login")
	write(t, filepath.Join(project, "auth.go"), "package auth\nvar x = 1\n")
	git(t, project, "add", ".")
	git(t, project, "commit", "-m", "add auth")

	if _, ok := m.CardChanges("card1"); ok {
		t.Fatal("a card nobody has worked on claims to have changes")
	}
	if err := m.store.SaveCheckout(Checkout{
		WorkspaceID: m.workspaceID(project),
		CardID:      "card1",
		Mode:        WorkModeBranch,
		Branch:      "fix-login",
		Path:        project,
		Base:        "main",
	}); err != nil {
		t.Fatal(err)
	}

	m.refreshCardChanges("card1", "board1")
	rec, ok := m.CardChanges("card1")
	if !ok {
		t.Fatal("the work was not counted")
	}
	if rec.Files != 1 || rec.Added != 2 || rec.Branch != "fix-login" {
		t.Errorf("counted %+v, want one file and two added lines on fix-login", rec)
	}
}

// The count is worked out when it is asked for, not read off the row: a stale
// answer is wrong in both directions — a button that opens «ничего не
// изменено», and no button on a card an agent has just committed to. The row is
// what a board would read, and this look keeps it current.
func TestACardWorkedBeforeAnythingCountedIsCountedOnTheSpot(t *testing.T) {
	m, _, _, project := testManager(t, fakeClaudeHappy, nil)
	if err := m.store.SaveCheckout(Checkout{
		WorkspaceID: m.workspaceID(project),
		CardID:      "card1",
		Mode:        WorkModeBranch,
		Branch:      "main",
		Path:        project,
		Base:        "main",
	}); err != nil {
		t.Fatal(err)
	}

	if _, ok := m.CardChanges("card1"); ok {
		t.Error("an untouched branch claims to have changes")
	}
	if _, found, err := m.store.CardChanges("card1"); err != nil || !found {
		t.Errorf("the look left no record for a board to read (found=%v, err=%v)", found, err)
	}

	// And a commit made since is seen without anything having recorded it.
	git(t, project, "checkout", "-b", "later")
	write(t, filepath.Join(project, "late.go"), "package late\n")
	git(t, project, "add", ".")
	git(t, project, "commit", "-m", "later")
	git(t, project, "checkout", "main")
	if err := m.store.SaveCheckout(Checkout{
		WorkspaceID: m.workspaceID(project),
		CardID:      "card1",
		Mode:        WorkModeBranch,
		Branch:      "later",
		Path:        project,
		Base:        "main",
	}); err != nil {
		t.Fatal(err)
	}
	if rec, ok := m.CardChanges("card1"); !ok || rec.Files != 1 {
		t.Errorf("the look answered from a stale row: %+v (ok=%v)", rec, ok)
	}
}
