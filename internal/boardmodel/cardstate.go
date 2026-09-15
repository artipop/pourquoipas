package boardmodel

import (
	"encoding/json"
	"time"

	"github.com/artipop/xciii/internal/acp"
)

// Where a card stands on its route lives on the card, under one key of its own.
//
// A field and not a table: a card block is the only thing that survives the trip
// a board makes between machines, since an import renumbers every id and a table
// keyed by card id arrives pointing at cards that no longer exist. Not a block
// of our own either — the page reads every block that is not a board, view or
// comment as the card's content.
//
// Writing it is a field patch, which merges, so it cannot disturb the properties
// a person filled in.
const CardFieldFlow = "flow"

// FlowStateFromField reads the key back into the real type.
func FlowStateFromField(raw any) (acp.FlowState, error) {
	var st acp.FlowState
	return st, ReinterpretField(raw, &st)
}

// ReinterpretField reads one of our own keys back into the real type. It arrives
// as whatever the block store decoded it into — and on some paths as the JSON
// text of itself — so it goes through JSON rather than being picked apart.
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
// The block history's key is (id, insert_at) in whole milliseconds, so two
// writes to one card inside one millisecond collide — and two writes that fast
// are always ours: a stage moves the card and then records where it stands.
//
// Only for a write that repeats with the same result, which a field patch is,
// and deliberately not a loop: a second collision means the cause is not the
// clock. Shared, because over HTTP the same clash comes back as a 500 and needs
// the same answer.
func RetryOnceInTheSameMillisecond(write func() error) error {
	if err := write(); err == nil {
		return nil
	}
	time.Sleep(2 * time.Millisecond)
	return write()
}
