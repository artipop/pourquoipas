package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/artipop/pourquoipas/internal/dokku"
	"github.com/artipop/pourquoipas/internal/feedmcp"
	"github.com/artipop/pourquoipas/internal/sources"
	"github.com/artipop/pourquoipas/internal/sources/inbox"
	"github.com/artipop/pourquoipas/internal/sources/kaiten"
)

// maybeRunMCP handles `<binary> mcp <server>`: the same executable doubles as
// the MCP servers an agent session spawns, which keeps the desktop app a single
// binary with nothing extra to install. It must run before the board
// server and Wails are touched, and it never returns — stdout belongs to the
// JSON-RPC stream from here on.
func maybeRunMCP(args []string) {
	if len(args) == 0 || args[0] != "mcp" {
		return
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: pourquoipas mcp %s|%s|%s|%s\n", dokku.ServerName, inbox.ServerName, kaiten.ServerName, feedmcp.ServerName)
		os.Exit(2)
	}
	var err error
	switch args[1] {
	case dokku.ServerName:
		err = runDokkuMCP()
	case inbox.ServerName:
		err = runInboxMCP()
	case kaiten.ServerName:
		err = runKaitenMCP()
	case feedmcp.ServerName:
		err = runFeedMCP()
	default:
		fmt.Fprintf(os.Stderr, "unknown MCP server %q (there are %q, %q and %q)\n",
			args[1], dokku.ServerName, inbox.ServerName, kaiten.ServerName)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp %s: %v\n", args[1], err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runInboxMCP is the server an agent source files through. Everything it may
// touch — which app, which source, with what token — arrives in the
// environment, so the model chooses what to file and never where.
func runInboxMCP() error {
	return inbox.ServeStdio(context.Background(), inbox.Config{
		BaseURL: os.Getenv(sources.EnvInboxURL),
		Source:  os.Getenv(sources.EnvInboxSource),
		Token:   os.Getenv(sources.EnvInboxToken),
	})
}

// runKaitenMCP is the source side of Kaiten: one tool, the cards assigned to
// whoever the token belongs to. The site and the token arrive in the
// environment, from the fields the source dialog asked for.
func runKaitenMCP() error {
	return kaiten.ServeStdio(context.Background(), kaiten.Config{
		Site:  os.Getenv(kaiten.EnvSite),
		Token: os.Getenv(kaiten.EnvToken),
	})
}

// runFeedMCP bridges one stdio session to the column feed of the running app,
// or of the server POURQUOIPAS_URL names. It is what another application starts to
// read a board (Gandvík's xciii.json manifest).
func runFeedMCP() error {
	dir, err := appDataDir("", 0o755)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := feedmcp.ServeStdio(ctx, dir, os.Stdin, os.Stdout); err != nil &&
		!errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "server is closing") {
		return err
	}
	return nil
}

func runDokkuMCP() error {
	raw := os.Getenv(dokku.EnvTarget)
	if raw == "" {
		return fmt.Errorf("%s is not set", dokku.EnvTarget)
	}
	var target dokku.Target
	if err := json.Unmarshal([]byte(raw), &target); err != nil {
		return fmt.Errorf("could not read %s: %w", dokku.EnvTarget, err)
	}
	cl, err := dokku.New(target, os.Getenv(dokku.EnvRepo), os.Getenv(dokku.EnvBranch))
	if err != nil {
		return err
	}

	// The agent kills the server by closing stdio; signals are the fallback for
	// a session torn down from the outside.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The agent closing our stdio is how a session ends, so EOF is success.
	if err := dokku.ServeStdio(ctx, cl, os.Getenv(dokku.EnvArtifacts)); err != nil &&
		!errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// runWebtestMCP serves the browser tools. The browser is launched up front
// rather than on the first tool call: a session whose browser cannot start is
// better off failing at once, with the reason on the card, than half-way
// through a scenario.
