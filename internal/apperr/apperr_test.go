package apperr

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// A code wrapped in context on its way up still reaches the page: the page
// words the code, and the context is only for logs.
func TestAWrappedErrorStillHandsThePageItsCode(t *testing.T) {
	err := fmt.Errorf("saving: %w", New("folders.dirty", "The folder {dir} has unsaved changes", "dir", "/w"))
	var got wire
	if raw := MarshalError(err); raw == nil || json.Unmarshal(raw, &got) != nil {
		t.Fatalf("no cause for the page: %s", raw)
	}
	if got.Code != "folders.dirty" || got.Args["dir"] != "/w" {
		t.Errorf("cause = %+v", got)
	}
	if MarshalError(errors.New("plain")) != nil {
		t.Error("an error with no code should be left to Wails' default")
	}
}

// Kind keeps errors.Is working for the code that branches on the sentinel,
// without saying the sentinel's own words twice.
func TestAKindIsItsSentinelAndSaysItOnce(t *testing.T) {
	busy := errors.New("the folder is held by another card")
	err := Kind(busy, "folders.busy-branch", "Held by {card}", "card", "c1")
	if !errors.Is(err, busy) {
		t.Error("errors.Is lost the sentinel")
	}
	if err.Error() != "Held by c1" {
		t.Errorf("text = %q", err.Error())
	}
	if MessageOf(fmt.Errorf("start: %w", err)).Code != "folders.busy-branch" {
		t.Error("MessageOf did not find the code under the wrapping")
	}
}

// What is kept in a table comes back as it went in, and absence is NULL.
func TestAMessageSurvivesItsColumns(t *testing.T) {
	m := Say("carddiff.not-git", "The folder {dir} is not a git repository", "dir", "/w")
	text, code, args := m.Columns()
	back, err := FromColumns(text, code.(string), args.(string))
	if err != nil || back.Code != m.Code || back.Args["dir"] != "/w" || back.Text != m.Text {
		t.Fatalf("back = %+v, %v", back, err)
	}
	_, code, args = Plain("said by a plugin").Columns()
	if code != nil || args != nil {
		t.Errorf("a plain message stored code %v and args %v, want NULLs", code, args)
	}
}
