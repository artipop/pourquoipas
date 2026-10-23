package acp

import (
	"strings"
	"testing"
)

// A conversation about a board's work opens with what the board is: its name
// and what the person wrote it is for.
func TestThePlanningConversationOpensWithTheBoardsNameAndDescription(t *testing.T) {
	about := BoardAbout{Title: "Ремонт", Description: "Всё про ремонт кухни"}
	got := planningPrompt("", "", AgentEntry{}, WorkdirEntry{}, about)
	if !strings.Contains(got, "The board: Ремонт") || !strings.Contains(got, "What the board is for: Всё про ремонт кухни") {
		t.Fatalf("the opening says nothing of the board:\n%s", got)
	}
	if bare := planningPrompt("", "", AgentEntry{}, WorkdirEntry{}, BoardAbout{}); strings.Contains(bare, "The board:") {
		t.Errorf("a board with no name is still named:\n%s", bare)
	}
}
