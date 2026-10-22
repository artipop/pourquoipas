package acp

import (
	"context"

	"github.com/artipop/pourquoipas/internal/boardmcp"
)

// BoardForGrant is the board's tools as one grant sees them: what the front
// door's MCP route serves a CLI, and what a model reaches in memory
// (modelconv.go). One adapter, so the two cannot drift apart.
func (m *Manager) BoardForGrant(token string) boardmcp.Board {
	return &grantedBoard{m: m, token: token}
}

// grantedBoard is the manager seen through one grant (BoardForGrant): every call is already
// bound to the board the token names, so nothing below takes a board id.
type grantedBoard struct {
	m     *Manager
	token string
}

var _ boardmcp.Board = (*grantedBoard)(nil)

func (g *grantedBoard) Columns(_ context.Context) ([]boardmcp.Column, error) {
	columns, err := g.m.BoardToolColumns(g.token)
	if err != nil {
		return nil, err
	}
	out := make([]boardmcp.Column, 0, len(columns))
	for _, c := range columns {
		out = append(out, boardmcp.Column{Name: c.Name, Action: c.Action, Agents: c.Agents})
	}
	return out, nil
}

func (g *grantedBoard) Cards(ctx context.Context, column string) ([]boardmcp.CardInfo, error) {
	cards, err := g.m.BoardToolCards(ctx, g.token, column)
	if err != nil {
		return nil, err
	}
	out := make([]boardmcp.CardInfo, 0, len(cards))
	for _, c := range cards {
		out = append(out, cardInfo(c))
	}
	return out, nil
}

func (g *grantedBoard) Card(ctx context.Context, cardID string) (boardmcp.CardInfo, error) {
	card, err := g.m.BoardToolCardByID(ctx, g.token, cardID)
	if err != nil {
		return boardmcp.CardInfo{}, err
	}
	return cardInfo(card), nil
}

// Fields is the board's card properties, with the stage's own contract marked
// on them — the grant is what knows which stage this conversation is.
func (g *grantedBoard) Fields(ctx context.Context) ([]boardmcp.FieldInfo, error) {
	fields, err := g.m.BoardToolFields(ctx, g.token)
	if err != nil {
		return nil, err
	}
	out := make([]boardmcp.FieldInfo, 0, len(fields))
	for _, f := range fields {
		out = append(out, boardmcp.FieldInfo{
			Name: f.Name, Kind: f.Kind, Values: f.Values,
			Writable: f.Writable,
		})
	}
	return out, nil
}

func (g *grantedBoard) UpdateCard(ctx context.Context, change boardmcp.CardChange) error {
	return g.m.UpdateCardFromTools(ctx, g.token, change.CardID, CardEdit{
		Title:   change.Title,
		Column:  change.Column,
		Options: change.Options,
	})
}

func (g *grantedBoard) SetFields(ctx context.Context, cardID string, fields map[string]string) error {
	return g.m.SetFieldsFromTools(ctx, g.token, cardID, fields)
}

func (g *grantedBoard) Comment(ctx context.Context, cardID, text string) error {
	return g.m.CommentFromTools(ctx, g.token, cardID, text)
}

// Finish ends the stage of a route this conversation is, which is what the
// grant knows. The fields land on the card first: they are what the route's own
// edges may read.
func (g *grantedBoard) Finish(_ context.Context, ok bool, summary string, fields map[string]string) error {
	return g.m.FinishWorkFromTools(g.token, ok, summary, fields)
}

func cardInfo(c BoardToolCard) boardmcp.CardInfo {
	return boardmcp.CardInfo{
		ID:      c.ID,
		Title:   c.Title,
		Column:  c.Column,
		Options: c.Options,
		Body:    c.Body,
		Mine:    c.Mine,
		Running: c.Running,
	}
}

func (g *grantedBoard) CreateCards(ctx context.Context, cards []boardmcp.Card) ([]boardmcp.CardResult, error) {
	results := make([]boardmcp.CardResult, 0, len(cards))
	for _, card := range cards {
		id, err := g.m.CreateCardFromTools(ctx, g.token, NewCard{
			Title:   card.Title,
			Body:    card.Description,
			Column:  card.Column,
			Options: card.Options,
		})
		if err != nil {
			results = append(results, boardmcp.CardResult{Title: card.Title, Error: err.Error()})
			continue
		}
		results = append(results, boardmcp.CardResult{ID: id, Title: card.Title})
	}
	return results, nil
}
