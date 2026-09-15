//go:build !server

package main

import "github.com/google/uuid"

// localSessionToken makes the API synthesize a session for every request and
// refuse /login. Minted per launch, so one that leaked dies with the process.
func localSessionToken() string { return "su-" + uuid.New().String() }

// newSessionGuard: nothing authenticates a user on a desktop, so the loopback
// origin and the Host are the whole of the protection (frontdoor.go).
func newSessionGuard(sessionSource) func(string) bool { return nil }

// attachableBuild: a desktop is what attaches.
const attachableBuild = true
