package boardmodel

import (
	"strings"

	"github.com/artipop/pourquoipas/server/model"
)

// The board as a list shows it — what the phone at /m and the share dialog
// read. Shared for the same reason reading a card is: there are two ways to
// reach a board and a list must not look different for having come the other
// way.

// BoardSummary is a board as a list shows it, with the columns a card can be
// moved into. The columns come with it because the two questions are always
// asked together — which board, then which column of it — and a phone on a
// slow connection should ask once.
type BoardSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	// Inbox marks the board everything a source brings lands on: the phone
	// lists it as the inbox and the share dialog offers it first.
	Inbox bool `json:"inbox,omitempty"`

	// Property is what this board calls the property its columns live in, and
	// Columns are the options of it, in the board's own order.
	Property string         `json:"property,omitempty"`
	Columns  []ColumnOption `json:"columns,omitempty"`
}

// ColumnOption is one column of a board: the name a person reads and the colour
// the board draws it in. Named apart from Column above, which is a select value
// on a card; this is a column offered by the board. The colour travels so that a phone can draw a column the
// way the board does rather than as plain text — the same chip, in the same
// colour, is what makes a list of columns recognisable as columns.
type ColumnOption struct {
	Value string `json:"value"`
	Color string `json:"color,omitempty"`
}

// CardSummary is a card as a list shows it: enough to recognise it and to
// decide where it belongs.
type CardSummary struct {
	ID      string `json:"id"`
	BoardID string `json:"boardId"`
	Title   string `json:"title"`
	Icon    string `json:"icon,omitempty"`
	Column  string `json:"column,omitempty"`

	// Author is who made the card — for what arrived, the source that brought
	// it. It is the board's own answer, so a phone says where a thing came from
	// without a property of ours beside it.
	Author string `json:"author,omitempty"`

	// Properties are the card's own, by the names a person reads — which is what
	// they are for: showing. Nothing keys off them.
	Properties map[string]string `json:"properties,omitempty"`

	// Link is the way back to whatever brought the card, resolved here from the
	// board's url property rather than guessed from a property name on the
	// screen that reads it. A phone that looked for the link property showed a link on a
	// Russian board and nothing on any other.
	Link string `json:"link,omitempty"`

	UpdateAt int64 `json:"updateAt,omitempty"`
}

// displayProperties turns a card's stored properties into the names and values a
// person reads. A select becomes the option's own text; anything else is taken
// as it is, and what the board has no definition for is left out — it is a
// leftover of a property somebody deleted.
func DisplayProperties(schema model.PropSchema, block *model.Block) map[string]string {
	stored, ok := block.Fields["properties"].(map[string]any)
	if !ok || len(stored) == 0 {
		return nil
	}
	out := make(map[string]string, len(stored))
	for id, value := range stored {
		def, ok := schema[id]
		if !ok {
			continue
		}
		if text := PropertyText(def, value); text != "" {
			out[def.Name] = text
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func PropertyText(def model.PropDef, value any) string {
	switch typed := value.(type) {
	case string:
		if option, ok := def.Options[typed]; ok {
			return option.Value
		}
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			id, ok := item.(string)
			if !ok {
				continue
			}
			if option, ok := def.Options[id]; ok {
				parts = append(parts, option.Value)
			} else {
				parts = append(parts, id)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

// propertyOfType finds the board's first property of a given type, walking
// CardProperties rather than the parsed schema: the schema is a map, and a
// board with two properties of one type would otherwise answer differently on
// different runs.
func PropertyOfType(board *model.Board, schema model.PropSchema, propType string) (string, bool) {
	for _, prop := range board.CardProperties {
		id, ok := prop["id"].(string)
		if !ok {
			continue
		}
		if def, ok := schema[id]; ok && def.Type == propType {
			return id, true
		}
	}
	return "", false
}

// findCardProperty returns the board's raw definition of a property, which is
// what a patch replaces. The parsed schema cannot be used for this: it drops
// everything the board wrote that we do not read, and patching with it would
// silently delete those fields.
func FindCardProperty(props []map[string]any, name string) (map[string]any, bool) {
	for _, prop := range props {
		if propType, _ := prop["type"].(string); propType != "select" {
			continue
		}
		if propName, _ := prop["name"].(string); strings.EqualFold(propName, name) {
			return prop, true
		}
	}
	return nil, false
}
