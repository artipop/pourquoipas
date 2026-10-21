// Command example is a source plugin with nothing in it but the shape: it
// invents one item and remembers, in its cursor, that it has already done so.
//
// It is here to be run, not read as a feature:
//
//	go run ./cmd/sourcecheck -- go run ./sources/sdk/example
//
// That is the whole of what writing a plugin looks like — a Poll that returns
// items, and no idea that boards, cards or rules exist.
package main

import (
	"context"
	"time"

	"github.com/artipop/pourquoipas/sources/protocol"
	"github.com/artipop/pourquoipas/sources/sdk"
)

func main() {
	sdk.Serve(sdk.Source{
		Capabilities: protocol.Capabilities{Poll: true, Cursor: true},
		Poll: func(_ context.Context, req sdk.PollRequest) (sdk.PollResult, error) {
			// The cursor is this plugin's own bookmark: the app stores it and
			// hands it back, and never looks inside. Here it means "already
			// told them".
			if req.Cursor != "" {
				return sdk.PollResult{Cursor: req.Cursor}, nil
			}
			return sdk.PollResult{
				Items: []sdk.Item{{
					ExternalID: "example-1",
					Version:    "1",
					Title:      "An example item from a plugin",
					Body:       "All a plugin does is return items like this one.",
					At:         time.Now(),
					Props:      map[string]string{"app": "example"},
				}},
				Cursor: "done",
			}, nil
		},
	})
}
