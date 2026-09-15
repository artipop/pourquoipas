//go:build server

package main

// localSessionToken is empty here: no single-user token, so /login and
// /register are open and every request carries a real session.
func localSessionToken() string { return "" }

// newSessionGuard closes the routes about this machine rather than about a
// board — the bindings and both sockets — to anybody who has not logged in.
func newSessionGuard(src sessionSource) func(string) bool { return boardSessionGuard(src) }

// attachableBuild: a hosted server does not attach to another one — its front
// door guards on its own sessions and the page would carry somebody else's.
const attachableBuild = false
