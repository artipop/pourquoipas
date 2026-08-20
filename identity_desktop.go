//go:build !server

package main

import "github.com/google/uuid"

// localSessionToken is what makes the board API synthesize a session for every
// request and refuse /login. Minted per launch, so a token that leaked with a
// screenshot dies with the process.
func localSessionToken() string { return "su-" + uuid.New().String() }

// newSessionGuard: nothing authenticates a user on a desktop, which is what
// makes the loopback origin and the Host the whole of the protection
// (frontdoor.go). A nil guard is the guard standing aside.
func newSessionGuard(sessionSource) func(string) bool { return nil }

// attachableBuild: a desktop is what attaches. Its front door authenticates
// nobody, so the page carrying the attached board's session costs it nothing.
const attachableBuild = true
