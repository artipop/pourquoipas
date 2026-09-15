package acp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A stage that follows another one is told what that one left behind. Without
// this the reviewer's brief is the worker's brief, and the commits it is meant
// to read are not mentioned anywhere in it.
func TestAStageIsToldWhatIsOnTheBranch(t *testing.T) {
	dir := initTestWorkdir(t)
	git(t, dir, "checkout", "-b", "fix-login")
	if err := os.WriteFile(filepath.Join(dir, "auth.go"), []byte("package auth\n\nfunc Login() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "Fix the token refresh")

	got := branchState(context.Background(), WorktreeInfo{Path: dir, Branch: "fix-login", BaseRef: "main"})
	for _, want := range []string{
		"fix-login",
		"cut from main",
		"Fix the token refresh",
		"auth.go (+3 -0)",
		"git diff main...HEAD",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the brief does not say %q:\n%s", want, got)
		}
	}
}

// A folder worked in place is told to leave its changes uncommitted for review
// (composePrompt), so a stage that only counted commits would find the work
// invisible.
func TestUncommittedWorkCountsAsSomethingToRead(t *testing.T) {
	dir := initTestWorkdir(t)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\nthere\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := branchState(context.Background(), WorktreeInfo{Path: dir, BaseRef: "main"})
	if !strings.Contains(got, "Uncommitted changes (1 files)") {
		t.Errorf("the brief says nothing about the unsaved work:\n%s", got)
	}
}

// A workspace with nothing in it yet — the first stage of a route — gets no
// block at all: naming an empty branch would be one more thing to read that
// says nothing.
func TestTheFirstStageIsToldNothingAboutABranchWithNothingOnIt(t *testing.T) {
	dir := initTestWorkdir(t)
	if got := branchState(context.Background(), WorktreeInfo{Path: dir, Branch: "main", BaseRef: "main"}); got != "" {
		t.Errorf("an untouched workspace was described anyway:\n%s", got)
	}
}
