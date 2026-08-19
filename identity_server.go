//go:build server

package main

// localSessionToken is empty on a hosted server: there is no single-user token,
// so /login and /register are open and every request carries a real session.
func localSessionToken() string { return "" }

// newSessionGuard closes the routes that are about this machine rather than
// about a board — the bindings and both sockets — to anybody who has not
// logged in.
func newSessionGuard(src sessionSource) func(string) bool { return boardSessionGuard(src) }
