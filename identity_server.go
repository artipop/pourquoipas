//go:build server

package main

import (
	"context"

	"github.com/artipop/pourquoipas/server/model"
)

// localSessionToken is empty here: no single-user token, so /login and
// /register are open and every request carries a real session.
func localSessionToken() string { return "" }

// newSessionGuard closes the routes about this machine rather than about a
// board — the bindings and both sockets — to anybody who has not logged in.
func newSessionGuard(src sessionSource) func(string) bool { return boardSessionGuard(src) }

// attachableBuild: a hosted server does not attach to another one — its front
// door guards on its own sessions and the page would carry somebody else's.
const attachableBuild = false

// ownsTheInstall: on a server, whoever administers it.
func ownsTheInstall(ctx context.Context, owners installOwners) bool {
	token := sessionTokenOf(ctx)
	if owners == nil || token == "" {
		return false
	}
	session, err := owners.GetSession(token)
	if err != nil || session == nil {
		return false
	}
	return owners.HasPermissionTo(session.UserID, model.PermissionManageSystem)
}

// personOf is whose a call is: the user of the session that made it.
func personOf(ctx context.Context, owners installOwners) (string, error) {
	token := sessionTokenOf(ctx)
	if owners == nil || token == "" {
		return "", errNobodySignedIn
	}
	session, err := owners.GetSession(token)
	if err != nil || session == nil || session.UserID == "" {
		return "", errNobodySignedIn
	}
	return session.UserID, nil
}

// hostedBuild: a server has no machine of anybody's to work on (acp.SetHosted).
const hostedBuild = true
