package boardclient

import (
	"context"
	"fmt"

	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/acp"
)

var _ acp.BoardUsers = (*Client)(nil)

// EnsureAgentAccounts makes the account each registered agent is named by.
//
// The one thing in the seam the board's REST API could not already do:
// /register wants the signup token and cannot say what kind of account it is
// making, so the route is ours.
//
// Idempotent by username, with a consequence worth stating: three desktops with
// an agent called an agent named in Russian share one account on the board. That is right, and it
// is a trap — behind it are three registry entries with three binaries.
func (c *Client) EnsureAgentAccounts(_ context.Context, agents []acp.AgentUser) ([]acp.AgentUser, error) {
	out := make([]acp.AgentUser, 0, len(agents))
	for _, agent := range agents {
		filled, err := c.ensureAccount(agent)
		if err != nil {
			return out, err
		}
		if filled.UserID != "" {
			out = append(out, filled)
		}
	}
	return out, nil
}

// ensureAccount fills in one agent's user id. A name that folds to nothing is
// skipped with an empty UserID rather than an error: one unusable registry
// entry must not stop the rest.
func (c *Client) ensureAccount(agent acp.AgentUser) (acp.AgentUser, error) {
	if agent.Username == "" {
		return agent, nil
	}
	userID, resp := c.api.EnsureMachineUser(c.teamID, agent.Username, model.UserKindAgent)
	if err := failed(resp, "account for agent %q", agent.Username); err != nil {
		return agent, err
	}
	agent.UserID = userID
	return agent, nil
}

// EnsureAgentUsers puts the agents into one board's members, which is what
// makes the name resolve in its person picker without searching for it.
func (c *Client) EnsureAgentUsers(_ context.Context, boardID string, agents []acp.AgentUser) ([]acp.AgentUser, error) {
	if boardID == "" {
		return nil, fmt.Errorf("не указана доска")
	}
	out := make([]acp.AgentUser, 0, len(agents))
	for _, agent := range agents {
		filled, err := c.ensureAccount(agent)
		if err != nil {
			return out, err
		}
		if filled.UserID == "" {
			continue
		}
		member := &model.BoardMember{
			BoardID:         boardID,
			UserID:          filled.UserID,
			SchemeEditor:    true,
			SchemeCommenter: true,
			SchemeViewer:    true,
		}
		if _, resp := c.api.AddMemberToBoard(member); resp != nil && resp.Error != nil {
			return out, fmt.Errorf("добавление %q в участники доски: %w", filled.Username, resp.Error)
		}
		out = append(out, filled)
	}
	return out, nil
}

// RetireAgentUser drops the agent's board memberships and reports how many. The
// account stays: cards may still name it. A request per board, because the
// board has no route for "the memberships of one user" and adding one for a
// thing that happens when an agent is deleted is a route for nothing.
func (c *Client) RetireAgentUser(_ context.Context, agent acp.AgentUser) (int, error) {
	if agent.Username == "" {
		return 0, nil
	}
	userID, resp := c.api.EnsureMachineUser(c.teamID, agent.Username, model.UserKindAgent)
	if err := failed(resp, "account for agent %q", agent.Username); err != nil {
		return 0, err
	}
	if userID == "" {
		return 0, nil
	}

	boards, resp := c.api.GetBoardsForTeam(c.teamID)
	if err := failed(resp, "list boards"); err != nil {
		return 0, err
	}
	removed := 0
	for _, board := range boards {
		if board == nil {
			continue
		}
		members, resp := c.api.GetMembersForBoard(board.ID)
		if resp != nil && resp.Error != nil {
			continue
		}
		for _, member := range members {
			if member == nil || member.UserID != userID {
				continue
			}
			if _, resp := c.api.DeleteBoardMember(member); resp != nil && resp.Error != nil {
				return removed, fmt.Errorf("удаление %q из участников доски %s: %w", agent.Username, board.ID, resp.Error)
			}
			removed++
		}
	}
	return removed, nil
}

// AssignCardAgent writes the agent into the card's person property, which is
// how a card names the agent working it and the only way it does.
func (c *Client) AssignCardAgent(_ context.Context, cardID string, agent acp.AgentUser) error {
	filled, err := c.ensureAccount(agent)
	if err != nil {
		return err
	}
	if filled.UserID == "" {
		return fmt.Errorf("у агента %q нет учётной записи на доске", agent.Name)
	}

	_, schema, err := c.cardSchema(cardID)
	if err != nil {
		return err
	}
	var person *model.PropDef
	for _, def := range schema {
		def := def
		if def.Type == "person" || def.Type == "multiPerson" {
			// The first by a board's own order: a board with two person
			// properties gets its primary one, which is the one its views show
			// first.
			if person == nil || def.Index < person.Index {
				person = &def
			}
		}
	}
	if person == nil {
		return nil
	}

	var value any = filled.UserID
	if person.Type == "multiPerson" {
		value = []string{filled.UserID}
	}
	return c.patchCard(cardID, &model.CardPatch{UpdatedProperties: map[string]any{person.ID: value}}, true)
}
