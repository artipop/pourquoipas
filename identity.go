package main

import (
	"context"

	"github.com/artipop/pourquoipas/server/model"
)

// Who this install asks about is a property of the build, not of a settings
// file: a desktop is one person, a hosted server is a team. It was a mode of
// the install and a switch that owed a restart — docs/teamwork.md says why that
// was the wrong level.

// sessionSource is the half of the board server the guard needs, as an
// interface so it is testable without a database.
type sessionSource interface {
	GetSession(token string) (*model.Session, error)
}

// boardSessionGuard answers the front door. Whose session it is stays the
// board's business — docs/deferred.md, "any teammate can open any terminal".
func boardSessionGuard(src sessionSource) func(string) bool {
	return func(token string) bool {
		if src == nil || token == "" {
			return false
		}
		session, err := src.GetSession(token)
		return err == nil && session != nil
	}
}

// installOwners is the half of the board server that says who runs a hosted
// install: an administrator.
type installOwners interface {
	GetSession(token string) (*model.Session, error)
	HasPermissionTo(userID string, permission *model.Permission) bool
}

// sessionTokenKey carries the guard's session to a binding: Wails hands a bound
// method the request's context.
type sessionTokenKey struct{}

func withSessionToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, sessionTokenKey{}, token)
}

func sessionTokenOf(ctx context.Context) string {
	token, _ := ctx.Value(sessionTokenKey{}).(string)
	return token
}
