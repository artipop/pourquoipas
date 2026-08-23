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
	if d.Reason != "" {
		t.Errorf("a card with work to read said there was none: %q", d.Reason)
	}
}
