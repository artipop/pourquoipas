package main

import "github.com/artipop/xciii/server/model"

// Who this install asks about, and how it asks — a property of the build
// rather than of a settings file.
//
// A desktop is one person: the board server is handed a single-user token, the
// API synthesizes a session for every request, and /login and /register are
// closed. A hosted server is always a team: no token, everybody logs in, and
// the routes that are about the machine rather than about a board require a
// live session.
//
// It used to be a mode of the install — <dataDir>/team/settings.json, read once
// at startup because the board server is constructed with the answer, so
// throwing the switch owed a restart. Two products were wearing one coat: a
// server that *can* be single-user is a server with a wrong setting, and a
// desktop that can be a team is a second answer to "who runs the agents" that
// nothing else in the product gives. The team is the server now, and a desktop
// joins one by logging in to it (docs/teamwork.md).

// sessionSource is the half of the board server the front door's guard needs.
// An interface so the guard is testable without a database.
type sessionSource interface {
	GetSession(token string) (*model.Session, error)
}

// boardSessionGuard answers the front door: is this a live board session.
// Whose it is stays the board's business — docs/deferred.md, «Терминал видит
// любой из команды».
func boardSessionGuard(src sessionSource) func(string) bool {
	return func(token string) bool {
		if src == nil || token == "" {
			return false
		}
		session, err := src.GetSession(token)
		return err == nil && session != nil
	}
}
