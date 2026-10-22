//go:build !server

package main

import (
	"context"

	"github.com/google/uuid"
)

// localSessionToken makes the API synthesize a session for every request and
// refuse /login. Minted per launch, so one that leaked dies with the process.
func localSessionToken() string { return "su-" + uuid.New().String() }

// newSessionGuard: nothing authenticates a user on a desktop, so the loopback
// origin and the Host are the whole of the protection (frontdoor.go).
func newSessionGuard(sessionSource) func(string) bool { return nil }

// attachableBuild: a desktop is what attaches.
const attachableBuild = true

// ownsTheInstall: the one person at a desktop runs it.
func ownsTheInstall(context.Context, installOwners) bool { return true }

// personOf is whose a call is. On a desktop it is the one person there, who
// owns everything the install has; "" says so.
func personOf(context.Context, installOwners) (string, error) { return "", nil }

// hostedBuild: a desktop has a machine to work on — folders, CLIs, processes.
const hostedBuild = false
