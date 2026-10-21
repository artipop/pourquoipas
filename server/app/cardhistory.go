package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/artipop/pourquoipas/server/model"
)

// What happened to a card, and who did it.
//
// Every write the board takes leaves a full copy of the block in
// `blocks_history` with the id of whoever made it, so the card's history is
// already stored — it has simply never been read. This reads it and turns
// consecutive copies into the difference between them: a title rewritten, a
// property filled in, a card moved from one column to another.
//
// Written here rather than on the page for two reasons. The rows are the
// board's and reach the page through no other door; and the diff needs the
// board's property schema to say «Статус: В работе → Готово» instead of naming
// two option ids.

// What a history entry is about.
const (
	CardHistoryCreated  = "created"  // the card was made
	CardHistoryTitle    = "title"    // its title was rewritten
	CardHistoryProperty = "property" // one property's value changed
	CardHistoryComment  = "comment"  // a comment was written
	CardHistoryDeleted  = "deleted"  // the card, or a comment, was deleted
)

// CardHistoryEntry is one thing somebody did to a card.
type CardHistoryEntry struct {
	// At is when, in unix milliseconds.
	At int64 `json:"at"`
	// UserID is who. It is a board account id — an agent has one of its own,
	// so «кто именно» is answerable — and empty for a write made before
	// anybody was recorded.
	UserID string `json:"userId"`
	Kind   string `json:"kind"`
	// Property is the name of the property that changed, for a property entry.
	Property string `json:"property,omitempty"`
	// PropertyType is what kind it is, so the page knows whether From and To
	// are text to show or user ids to draw as people.
	PropertyType string `json:"propertyType,omitempty"`
	// From and To are the value before and after, rendered the way a person
	// reads them: a select by its option's name, not by its id. Empty means
	// the value was not set.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Text is the comment, for a comment entry.
	Text string `json:"text,omitempty"`
	// Agent says the comment came from an agent's run rather than from
	// somebody typing, which is what the card already draws differently.
	Agent bool `json:"agent,omitempty"`
}

// GetCardHistory is everything that happened to a card, newest first.
func (a *App) GetCardHistory(cardID string) ([]CardHistoryEntry, error) {
	board, card, err := a.store.GetBoardAndCardByID(cardID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("card %s not found", cardID)
	}
	schema, err := model.ParsePropertySchema(board)
	if err != nil {
		// A board whose schema will not parse still has a readable history;
		// only the property names are lost, and an id is better than nothing.
		schema = model.PropSchema{}
	}

	versions, err := a.store.GetBlockHistory(cardID, model.QueryBlockHistoryOptions{})
	if err != nil {
		return nil, err
	}
	entries := cardVersionEntries(versions, schema)

	// The comments are children of the card and have a history of their own;
	// what matters about one is that it was written, and by whom.
	children, _, err := a.store.GetBlockHistoryNewestChildren(cardID, model.QueryBlockHistoryChildOptions{PerPage: -1})
	if err != nil {
		return nil, err
	}
	entries = append(entries, commentEntries(children)...)

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].At > entries[j].At })
	return entries, nil
}

// cardVersionEntries is the difference between each copy of the card and the
// one before it.
func cardVersionEntries(versions []*model.Block, schema model.PropSchema) []CardHistoryEntry {
	var out []CardHistoryEntry
	var previous *model.Block
	for _, v := range versions {
		if previous == nil {
			out = append(out, CardHistoryEntry{
				At: v.CreateAt, UserID: v.CreatedBy, Kind: CardHistoryCreated,
			})
			previous = v
			continue
		}
		if v.DeleteAt != 0 && previous.DeleteAt == 0 {
			out = append(out, CardHistoryEntry{
				At: v.UpdateAt, UserID: v.ModifiedBy, Kind: CardHistoryDeleted,
			})
			previous = v
			continue
		}
		if v.Title != previous.Title {
			out = append(out, CardHistoryEntry{
				At: v.UpdateAt, UserID: v.ModifiedBy, Kind: CardHistoryTitle,
				From: previous.Title, To: v.Title,
			})
		}
		out = append(out, propertyEntries(previous, v, schema)...)
		previous = v
	}
	return out
}

// propertyEntries is every property whose value differs between two copies.
func propertyEntries(before, after *model.Block, schema model.PropSchema) []CardHistoryEntry {
	was, is := blockProperties(before), blockProperties(after)
	ids := make([]string, 0, len(is))
	for id := range is {
		ids = append(ids, id)
	}
	for id := range was {
		if _, still := is[id]; !still {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids) // a stable order for two changes in the same millisecond

	var out []CardHistoryEntry
	for _, id := range ids {
		from, to := propertyText(schema, id, was[id]), propertyText(schema, id, is[id])
		if from == to {
			continue
		}
		def, known := schema[id]
		name, kind := id, ""
		if known {
			name, kind = def.Name, def.Type
		}
		out = append(out, CardHistoryEntry{
			At: after.UpdateAt, UserID: after.ModifiedBy, Kind: CardHistoryProperty,
			Property: name, PropertyType: kind, From: from, To: to,
		})
	}
	return out
}

// blockProperties is a card's `fields.properties`, or nothing.
func blockProperties(block *model.Block) map[string]any {
	if block == nil {
		return nil
	}
	props, _ := block.Fields["properties"].(map[string]any)
	return props
}

// propertyText renders one value the way a person reads it: a select by the
// name of its option, a multi-select by the names joined, a person by the id
// the page will draw as a name, anything else as it is stored.
func propertyText(schema model.PropSchema, id string, value any) string {
	if value == nil {
		return ""
	}
	def, known := schema[id]
	if !known {
		return fmt.Sprintf("%v", value)
	}
	switch def.Type {
	case "select":
		if optID, ok := value.(string); ok {
			if opt, found := def.Options[optID]; found {
				return opt.Value
			}
			return optID
		}
	case "multiSelect":
		return strings.Join(mapList(value, func(v string) string {
			if opt, found := def.Options[v]; found {
				return opt.Value
			}
			return v
		}), ", ")
	case "multiPerson":
		return strings.Join(mapList(value, func(v string) string { return v }), ", ")
	case "date":
		if raw, ok := value.(string); ok {
			if text, err := def.ParseDate(raw); err == nil {
				return text
			}
			return raw
		}
	}
	if text, ok := value.(string); ok {
		return text
	}
	if encoded, err := json.Marshal(value); err == nil {
		return string(encoded)
	}
	return fmt.Sprintf("%v", value)
}

// mapList reads a stored list of strings, whatever the decoder made of it.
func mapList(value any, render func(string) string) []string {
	items, ok := value.([]any)
	if !ok {
		if one, isString := value.(string); isString {
			return []string{render(one)}
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, isString := item.(string); isString {
			out = append(out, render(text))
		}
	}
	return out
}

// commentEntries is one entry per comment ever written on the card, and one
// more for each that was deleted.
func commentEntries(children []*model.Block) []CardHistoryEntry {
	var out []CardHistoryEntry
	for _, child := range children {
		if child.Type != model.TypeComment {
			continue
		}
		agent, _ := child.Fields["agent"].(bool)
		out = append(out, CardHistoryEntry{
			At: child.CreateAt, UserID: child.CreatedBy, Kind: CardHistoryComment,
			Text: child.Title, Agent: agent,
		})
		if child.DeleteAt != 0 {
			out = append(out, CardHistoryEntry{
				At: child.DeleteAt, UserID: child.ModifiedBy, Kind: CardHistoryDeleted,
				Text: child.Title, Agent: agent,
			})
		}
	}
	return out
}
