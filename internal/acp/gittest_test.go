package acp

import (
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
