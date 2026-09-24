package author

import "context"

// Who a write is by.
//
// Every write the board takes is stamped with a user id (`modified_by`, and
// `created_by` on a new block), and that stamp is the whole of the card's
// history: who moved it, who wrote the comment, who filled the field in. On a
// desktop the board server runs single-user, so every write arrives as the one
// account there is — and an agent's write then reads as the owner's, which is
// exactly the question the history is asked to answer.
//
// The answer travels on the context because that is what every write already
// carries. An agent's run sets it once (internal/acp), the board adapter reads
// it at the moment it calls the board, and anything that does not set it keeps
// the behaviour it had.
//
// A package of its own, and a leaf: internal/acp sets it and knows nothing
// about the board, internal/boardadapter reads it and is the only package that
// knows about both.

type authorKey struct{}

// With says whose the writes made under this context are. An empty id
// changes nothing, so a caller with nobody to name may pass what it has.
func With(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, authorKey{}, userID)
}

// From is who the writes under this context are by, or "" when nobody said.
func From(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(authorKey{}).(string)
	return id
}
