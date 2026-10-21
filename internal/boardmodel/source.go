package boardmodel

import (
	"strings"

	"github.com/artipop/pourquoipas/server/model"
)

// A card a source brought: how its named properties become the ids a card
// stores, and how the card remembers which item of which source it is.
//
// Shared for the reason everything here is: cards arrive on a board in this
// process and on one over HTTP, and a card that says where it came from must
// say it the same way either time — the board itself is what answers "have we
// brought this one already" once it has left this machine.

func CardProperties(schema model.PropSchema, props map[string]string) map[string]any {
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]any, len(props))
	for name, value := range props {
		for id, def := range schema {
			if !strings.EqualFold(def.Name, name) {
				continue
			}
			if def.Type != "select" {
				out[id] = value
				break
			}
			// An option the board does not have is not invented here: adding
			// options to a board is a deliberate, add-only act elsewhere.
			for oid, opt := range def.Options {
				if strings.EqualFold(opt.Value, value) {
					out[id] = oid
					break
				}
			}
			break
		}
	}
	return out
}

// CardFieldSource is the key on the card. Where a card came from lives on the
// card, under one key of its own — the same bargain as the flow key: a card
// block is what an export carries and an import renumbers, and a table keyed by
// card id is neither.
const CardFieldSource = "source"

// CardSource is the field's shape. sources.ItemRef is the half the pipeline
// knows; the source's name is added here because the field has to stand on its
// own on the card.
type CardSource struct {
	Source     string `json:"source,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
	Version    string `json:"version,omitempty"`
}

// CardSourceOf reads the origin off a card. A card with nothing under the key —
// one a person typed — is not an origin nobody recorded, it is a card nobody
// brought.
func CardSourceOf(block *model.Block) (CardSource, bool) {
	raw, ok := block.Fields[CardFieldSource]
	if !ok || raw == nil {
		return CardSource{}, false
	}
	var origin CardSource
	if err := ReinterpretField(raw, &origin); err != nil {
		return CardSource{}, false
	}
	return origin, origin.ExternalID != ""
}

// LinkPropertyTitle is what the board calls the way back to what a source
// brought. Like AuthorPropertyTitle it is a name given at creation and never a
// key: the property is found by its *type*, so a board that calls it "Link", or
// whose owner renamed it, keeps working and does not grow a second one.
const LinkPropertyTitle = "Link"
