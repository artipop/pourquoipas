package boardmodel

import (
	"encoding/json"
	"time"

	"github.com/artipop/xciii/internal/acp"
)

// Where a card stands on its route lives on the card, under one key of its own
// in the block's fields.
//
// A field and not a table, and not a block of our own either. A card block is
// the only place that survives the trip a board makes between machines: an
// export carries the board record and its blocks and nothing else, and an
// import gives every board and block a new id — so a table keyed by card id
// arrives pointing at cards that no longer exist. A block of our own would
// survive that, but the page reads every block that is not a board, view or
// comment as the card's content (webapp/src/store/contents.ts), and the board
// server has no id prefix for a type it does not know.
//
// The key is ours and the page never looks at it. Writing it is a field patch,
// which merges (model.BlockPatch.Patch), so it cannot disturb the properties a
// person filled in or the card's contentOrder.
const CardFieldFlow = "flow"

// FlowStateFromField reads the key back into the real type.
func FlowStateFromField(raw any) (acp.FlowState, error) {
	var st acp.FlowState
	return st, ReinterpretField(raw, &st)
}

// ReinterpretField reads one of our own keys back into the real type. It
// arrives as whatever the block store decoded it into — and, on some paths, as
// the JSON text of itself — so it goes through JSON rather than being picked
// apart by hand. The same reasoning, and the same two cases, as acp.reinterpret
// for a board's properties.
func ReinterpretField(raw any, into any) error {
	if text, ok := raw.(string); ok {
		if text == "" {
			return nil
		}
		return json.Unmarshal([]byte(text), into)
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, into)
}

// FlowStateField encodes the position for a field patch.
func FlowStateField(st acp.FlowState) (map[string]any, error) {
	encoded, err := json.Marshal(st)
	if err != nil {
		return nil, err
	}
	var field map[string]any
	if err := json.Unmarshal(encoded, &field); err != nil {
		return nil, err
	}
	return field, nil
}

// RetryOnceInTheSameMillisecond runs a card write again a millisecond later.
//
// The block history's primary key is (id, insert_at) in whole milliseconds, so
// two writes to one card inside the same millisecond collide on it. Two writes
// that fast are always ours rather than a person's: a stage moves the card and
// then records where it now stands. Without the second attempt a route stops
// advancing for no reason anybody could see.
//
// Only for a write that can be repeated with the same result, which a field
// patch is. Carrying a card to another board is not one of them. It is
// deliberately not a loop: a second collision means the cause is not the clock.
//
// It travels with the two adapters because the collision is the board's, not
// one adapter's — over HTTP the same clash comes back as a 500 rather than as a
// driver error, and needs the same answer.
func RetryOnceInTheSameMillisecond(write func() error) error {
	if err := write(); err == nil {
		return nil
	}
	time.Sleep(2 * time.Millisecond)
	return write()
}
