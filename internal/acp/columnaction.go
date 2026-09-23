package acp

// What a column does with a card that lands in it, and what it owes the card
// when it is done.
//
// This is all that is left of the route engine. A board used to carry routes —
// a graph of stages with edges, conditions and questions on the way through —
// and walking it was the biggest thing this package did. It is gone: a card
// moves because a person dragged it or because an agent asked the board to
// (move_card), and what a column does is the one rule the app still applies
// on its own.

// Column actions: what runs when a card enters the column.
const (
	ColumnActionNone   = "none"   // nothing runs; the card waits for a person
	ColumnActionAgent  = "agent"  // an agent session on the card's task
	ColumnActionDeploy = "deploy" // a deploy session (Dokku)
	ColumnActionTest   = "test"   // a browser test session against the preview
)

// The two outcomes a session has, which is what the card's «Исход» field is
// written from.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

// PropertyWrite is one card property a column's work is expected to fill in:
// the verdict of a review, the address a deploy published. Required says the
// stage cannot end without it (finish_work refuses the report).
type PropertyWrite struct {
	Property string `json:"property"`
	Required bool   `json:"required,omitempty"`
}

// Where a column's work happens.
const (
	// RunInOwner is the card's own workspace — its branch, its copy.
	RunInOwner = "owner"
	// RunInWorkdir is the folder itself, whatever is checked out there.
	RunInWorkdir = "workdir"
)

// runsIn is where the work of this column happens, with the default for its
// action filled in: an agent writes code, so it works in the card's own
// workspace; a deploy publishes a branch that already exists and a test drives
// a browser against something already published, so neither needs a checkout
// of the card's.
func runsIn(declared, action string) string {
	switch declared {
	case RunInOwner, RunInWorkdir:
		return declared
	}
	if action == ColumnActionAgent {
		return RunInOwner
	}
	return RunInWorkdir
}
