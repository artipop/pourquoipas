package boardmodel

import (
	"fmt"
	"strings"

	"github.com/artipop/xciii/server/model"

	"github.com/artipop/xciii/internal/acp"
)

// Naming a property or an option the way a person does, and turning that into
// the ids a card stores. Shared because it is the board's vocabulary, not one
// adapter's: an agent says "Tested", a config says the review column, and both adapters
// have to find the same option.

// findPropertyByName resolves a property the way a person names it.
func FindPropertyByName(schema model.PropSchema, name string) (string, model.PropDef, bool) {
	for id, def := range schema {
		if strings.EqualFold(def.Name, name) {
			return id, def, true
		}
	}
	return "", model.PropDef{}, false
}

// cardPatchFor turns named values into the ids a card stores, or says which name
// the board does not have.
func CardPatchFor(schema model.PropSchema, edit acp.CardEdit) (*model.CardPatch, error) {
	patch := &model.CardPatch{}
	if title := strings.TrimSpace(edit.Title); title != "" {
		patch.Title = &title
	}
	properties := map[string]any{}
	if edit.Column != "" {
		propID, optionID, ok := FindSelectOption(schema, edit.Property, edit.Column)
		if !ok {
			return nil, fmt.Errorf("на доске нет колонки %q в свойстве %q", edit.Column, edit.Property)
		}
		properties[propID] = optionID
	}
	for _, option := range edit.Options {
		propID, optionID, ok := FindOptionByName(schema, option)
		if !ok {
			return nil, fmt.Errorf("на доске нет значения %q", option)
		}
		// The column is the edit's own field, so an option name that happens to
		// match a column must not move the card somewhere nobody asked for.
		if properties[propID] == nil {
			properties[propID] = optionID
		}
	}
	if patch.Title == nil && len(properties) == 0 {
		return nil, fmt.Errorf("не сказано, что менять")
	}
	if len(properties) > 0 {
		patch.UpdatedProperties = properties
	}
	return patch, nil
}

// findSelectOption resolves a (property name, option name) pair to the ids the
// card actually stores. Matching is case-insensitive, like the trigger columns.
// The property may be named or identified. A name is what the config and an
// agent's own words carry; an id is what a board records for itself
// (BoardPropColumnProperty), and it is the answer that survives a rename and a
// board in another language.
func FindSelectOption(schema model.PropSchema, property, optionName string) (propID, optionID string, ok bool) {
	for id, def := range schema {
		if def.Type != "select" || (id != property && !strings.EqualFold(def.Name, property)) {
			continue
		}
		for oid, opt := range def.Options {
			if strings.EqualFold(opt.Value, optionName) {
				return id, oid, true
			}
		}
	}
	return "", "", false
}

// findOptionByName resolves a bare option name — "xciii", "claude", "Быстрый
// маршрут" — to the property that owns it. Which property that is, is the
// board's business: a card is read back the same way, by the names of the
// options selected on it and not by where they sit.
func FindOptionByName(schema model.PropSchema, optionName string) (propID, optionID string, ok bool) {
	name := strings.TrimSpace(optionName)
	if name == "" {
		return "", "", false
	}
	for id, def := range schema {
		if def.Type != "select" {
			continue
		}
		for oid, opt := range def.Options {
			if strings.EqualFold(opt.Value, name) {
				return id, oid, true
			}
		}
	}
	return "", "", false
}

// MergeCardProperties folds the card's current properties into the patch.
//
// CardPatch2BlockPatch puts UpdatedProperties into the block's `properties`
// field whole, and a block patch replaces a field it names — so a patch that
// set one property silently erased every other one. The board's own webapp
// never hits this because its mutator always sends the full map; every write
// from this side — a route moving the card, an agent's update_card, the
// assignee kept truthful by a stage — sends one key and lost the rest.
//
// It takes the properties rather than a way to read them, because the two
// adapters read a card differently and this is the part that must not differ.
func MergeCardProperties(existing map[string]any, patch *model.CardPatch) {
	if len(patch.UpdatedProperties) == 0 {
		return
	}
	merged := map[string]any{}
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range patch.UpdatedProperties {
		merged[k] = v
	}
	patch.UpdatedProperties = merged
}
