package acp

import (
	"testing"
)

// A card names its agent through the assignee field, which stores the board
// account's id. Matching on the username derived from the agent's name meant a
// rename unassigned every card the agent was working on.
func TestAnAssignedCardFindsItsAgentByAccount(t *testing.T) {
	m := agentManager(t, "", AgentEntry{Name: "клаус", Kind: AgentKindClaude})
	m.recordAgentAccounts([]AgentUser{{Name: "клаус", UserID: "user-1"}})

	agent := m.Agents()[0]
	if agent.UserID != "user-1" {
		t.Fatalf("the account was not recorded on the entry: %+v", agent)
	}

	renamed := agent
	renamed.Name = "не клаус"
	if _, err := m.UpdateAgent(renamed); err != nil {
		t.Fatal(err)
	}

	// The card says nothing but the account id — which is all it ever stored.
	got, busy, err := m.resolveSessionAgent(CardMoved{PersonIDs: []string{"user-1"}}, nil)
	if err != nil || busy {
		t.Fatalf("the assigned card did not resolve: %v, busy=%v", err, busy)
	}
	if got.Name != "не клаус" {
		t.Errorf("resolved %q, want the renamed agent the card is assigned to", got.Name)
	}
}
