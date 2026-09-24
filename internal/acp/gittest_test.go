package acp

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

// What a test needs to build a repository to work in. Shared by every suite
// that puts an agent in a folder: the workspace, the branch state a stage is
// told about, the diff a card shows.

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// git runs one command in the folder, with an identity of its own so the test
// does not depend on whoever is running it.
func git(t *testing.T, project string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", project}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// The columns a board made from the shipped template has, by the names the
// English template gives them: what the fixtures move cards between.
const (
	TemplateWorkColumn   = "In progress"
	TemplateDeployColumn = "Deploy"
	TemplateTestColumn   = "QA"
)

// fakeBoardMeta is the board seen as the few properties this application keeps
// on it: what it says, what its column property offers, and what it was told to
// keep.
type fakeBoardMeta struct {
	props    map[string]any
	template bool
	// options is what the board's column property offers, for binding a stage
	// that knows only a column's name to the option it means.
	options []Column

	// cardProps is what the board's cards have, for the stage contract a brief
	// names and list_card_fields answers with.
	cardProps []CardProperty

	// written is what each board was told to keep, by board id — the board
	// database stands in for itself here.
	written map[string]map[string]any
	fail    error
}

func (f *fakeBoardMeta) BoardProperties(context.Context, string) (map[string]any, error) {
	return f.props, nil
}

func (f *fakeBoardMeta) BoardColumnOptions(context.Context, string) ([]Column, error) {
	return f.options, nil
}

func (f *fakeBoardMeta) SetBoardProperties(_ context.Context, boardID string, props map[string]any, remove []string) error {
	if f.fail != nil {
		return f.fail
	}
	if f.written == nil {
		f.written = map[string]map[string]any{}
	}
	board := f.written[boardID]
	if board == nil {
		board = map[string]any{}
		f.written[boardID] = board
	}
	for k, v := range props {
		board[k] = v
	}
	// The real board store deletes these; a fake that kept them would let a
	// reader go on finding the old name for ever.
	for _, k := range remove {
		delete(board, k)
	}
	return nil
}

func (f *fakeBoardMeta) CardProperties(context.Context, string) ([]CardProperty, error) {
	return f.cardProps, nil
}

func (f *fakeBoardMeta) IsBoardTemplate(context.Context, string) (bool, error) {
	return f.template, nil
}
