// Package boardmodel reads a board's own types — a block, a board, a property
// schema — as the types internal/acp works in. It imports the board's model and
// nothing else of the board: no app layer, no store, no HTTP client.
//
// It exists because there are two ways to reach a board and they must not grow
// two answers to "what does this card say". internal/boardadapter has the board
// in this process; internal/boardclient has one over HTTP. Everything below is
// the half neither of them gets to have an opinion about.
package boardmodel

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/acp"
)

// UserLookup resolves a user id to a username. Person property values are ids;
// an agent is matched by the name behind them.
type UserLookup func(userID string) string

// Resolver adapts a UserLookup to model.PropValueResolver, memoizing lookups:
// ParseProperties asks once per person value, and the caller may be on a hot
// path — the notify worker in one adapter, a websocket frame in the other.
type Resolver struct {
	lookup UserLookup
	seen   map[string]*model.User
}

func NewResolver(lookup UserLookup) *Resolver {
	return &Resolver{lookup: lookup, seen: make(map[string]*model.User)}
}

// GetUserByID never fails: an unknown or deleted user leaves the raw id as the
// property value instead of discarding the whole property map.
func (r *Resolver) GetUserByID(userID string) (*model.User, error) {
	if r == nil || r.lookup == nil || userID == "" {
		return nil, nil
	}
	if user, ok := r.seen[userID]; ok {
		return user, nil
	}
	var user *model.User
	if username := r.lookup(userID); username != "" {
		user = &model.User{ID: userID, Username: username}
	}
	r.seen[userID] = user
	return user, nil
}

// RawProperties is the card's property map as the board stores it: option ids
// and user ids, before anything is resolved into a name.
func RawProperties(block *model.Block) map[string]any {
	if block == nil {
		return nil
	}
	props, _ := block.Fields["properties"].(map[string]any)
	return props
}

func StringValue(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// Column names one select value: the property it belongs to and the option
// chosen in it.
func Column(def model.PropDef, optionID string) acp.Column {
	name := ""
	if opt, ok := def.Options[optionID]; ok {
		name = opt.Value
	}
	return acp.Column{
		PropertyID:   def.ID,
		PropertyName: def.Name,
		OptionID:     optionID,
		Name:         name,
	}
}

// SelectedOptions is SelectedOptionNames with the ids kept: every single-select
// value on the card as a full Column. It is what tells a caller which node a
// card read on demand stands on, where the names alone cannot say which
// property they belong to.
func SelectedOptions(props map[string]any, schema model.PropSchema) []acp.Column {
	var out []acp.Column
	for propID, def := range schema {
		if def.Type != "select" {
			continue
		}
		v, ok := props[propID]
		if !ok || v == nil {
			continue
		}
		id, ok := v.(string)
		if !ok {
			continue
		}
		if opt, ok := def.Options[id]; ok && opt.Value != "" {
			out = append(out, acp.Column{
				PropertyID:   propID,
				PropertyName: def.Name,
				OptionID:     id,
				Name:         opt.Value,
			})
		}
	}
	return out
}

// SelectedOptionNames collects the display names of every selected
// select/multiSelect option on the card.
func SelectedOptionNames(props map[string]any, schema model.PropSchema) []string {
	var out []string
	appendOption := func(def model.PropDef, v any) {
		id, ok := v.(string)
		if !ok {
			return
		}
		if opt, ok := def.Options[id]; ok && opt.Value != "" {
			out = append(out, opt.Value)
		}
	}
	for propID, def := range schema {
		v, ok := props[propID]
		if !ok || v == nil {
			continue
		}
		switch def.Type {
		case "select":
			appendOption(def, v)
		case "multiSelect":
			if vals, ok := v.([]any); ok {
				for _, item := range vals {
					appendOption(def, item)
				}
			}
		}
	}
	return out
}

// PersonNames collects the usernames of every selected person/multiPerson
// value — how an assignee routes a card to an agent.
func PersonNames(props map[string]any, schema model.PropSchema, resolver *Resolver) (names, ids []string) {
	appendUser := func(v any) {
		id, ok := v.(string)
		if !ok || id == "" {
			return
		}
		// The id goes out whether or not the account can be read: it is what
		// the card stores and what an agent is matched by, and a user the
		// resolver cannot reach right now must not read as an empty assignee.
		ids = append(ids, id)
		user, _ := resolver.GetUserByID(id)
		if user != nil && user.Username != "" {
			names = append(names, user.Username)
		}
	}
	for propID, def := range schema {
		v, ok := props[propID]
		if !ok || v == nil {
			continue
		}
		switch def.Type {
		case "person":
			appendUser(v)
		case "multiPerson":
			if vals, ok := v.([]any); ok {
				for _, item := range vals {
					appendUser(item)
				}
			}
		}
	}
	return names, ids
}

// NamedProperties resolves the card's properties into a lowercased
// name → display-value map (repo_path, branch, …). Person values come out as
// usernames when a resolver is supplied.
func NamedProperties(block *model.Block, schema model.PropSchema, resolver *Resolver) map[string]string {
	out := make(map[string]string)
	parsed, err := model.ParseProperties(block, schema, resolver)
	if err != nil {
		return out
	}
	for _, prop := range parsed {
		if prop.Name == "" {
			continue
		}
		out[strings.ToLower(prop.Name)] = prop.Value
	}
	return out
}

// ValuesByID is the same card, keyed the way this app's own fields are found:
// by the id the board recorded for each of them (BoardPropBranch and the rest).
// The name-keyed map above is for the fields nobody records — a person's own
// «repo_path» — and cannot serve these, because what they are called is the
// owner's business.
func ValuesByID(block *model.Block, schema model.PropSchema, resolver *Resolver) map[string]string {
	out := make(map[string]string)
	parsed, err := model.ParseProperties(block, schema, resolver)
	if err != nil {
		return out
	}
	for id, prop := range parsed {
		if v := strings.TrimSpace(prop.Value); v != "" {
			out[id] = v
		}
	}
	return out
}

// CardEvent reads one card as the event every caller of internal/acp works in.
// It carries no columns — nothing moved — and the caller supplies the body,
// because fetching one is a query of its own and a listing deliberately skips
// it (see acp.BoardReader).
func CardEvent(boardID string, block *model.Block, schema model.PropSchema, body string, resolver *Resolver) acp.CardMoved {
	props := RawProperties(block)
	names, ids := PersonNames(props, schema, resolver)
	return acp.CardMoved{
		EventID:         uuid.NewString(),
		CardID:          block.ID,
		BoardID:         boardID,
		Title:           block.Title,
		Body:            body,
		Props:           NamedProperties(block, schema, resolver),
		Values:          ValuesByID(block, schema, resolver),
		OptionNames:     SelectedOptionNames(props, schema),
		SelectedOptions: SelectedOptions(props, schema),
		PersonNames:     names,
		PersonIDs:       ids,
		At:              time.Now(),
	}
}

// MovedEvents is the card's move read as events: one per select property whose
// value changed, carrying where it came from and where it went.
//
// The two adapters get `old` from different places and that is the whole of
// their difference here — one is handed the previous block by the notify
// service, the other keeps the last value it saw, because the board's websocket
// carries the new block alone. Everything after that must be the same, so it is
// this function.
func MovedEvents(boardID string, block *model.Block, schema model.PropSchema, oldProps map[string]any, body string, resolver *Resolver) []acp.CardMoved {
	newProps := RawProperties(block)
	base := CardEvent(boardID, block, schema, body, resolver)

	var out []acp.CardMoved
	for propID, def := range schema {
		if def.Type != "select" {
			continue
		}
		oldVal := StringValue(oldProps[propID])
		newVal := StringValue(newProps[propID])
		if oldVal == newVal {
			continue
		}
		ev := base
		ev.EventID = uuid.NewString()
		ev.FromColumn = Column(def, oldVal)
		ev.ToColumn = Column(def, newVal)
		out = append(out, ev)
	}
	return out
}
