package main

import (
	"context"

	"github.com/artipop/xciii/server/app"
	"github.com/artipop/xciii/server/mlog"
	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/services/notify"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardadapter"
	"github.com/artipop/xciii/internal/boardclient"
	"github.com/artipop/xciii/internal/boardmodel"
)

// Which board the agents work.
//
// internal/acp asks a board six questions and does not care who answers them.
// Two things can: internal/boardadapter, which has the board in this process,
// and internal/boardclient, which has one over HTTP. This is the one place that
// chooses, so nothing downstream has to know there was a choice.
//
// It is a swap rather than a set: attached, the board is the server's, and the
// page was pointed at the same one (proxy.go). Working both at once is a
// different thing — it needs the manager to route by board id — and it is what
// the sidebar switcher waits for.
type boardSeam struct {
	writer    acp.BoardWriter
	cardState acp.BoardCardState
	reader    acp.BoardReader
	meta      acp.BoardMeta
	users     acp.BoardUsers
	events    acp.BoardEvents

	// lists is the board as the phone at /m and the share dialog read it.
	lists boardLists

	// notifyBackend is how the in-process board announces a card move. Nil for
	// a board on a server, which says so over its own websocket instead.
	notifyBackend notify.Backend
}

// boardLists is the small, flat surface a phone reads. Both adapters have it;
// app.go holds one of them.
type boardLists interface {
	Boards(ctx context.Context) ([]boardmodel.BoardSummary, error)
	BoardCards(ctx context.Context, boardID string) ([]boardmodel.CardSummary, error)
	MoveCardToBoard(ctx context.Context, cardID, toBoardID, column string) error
}

// localSeam is the board this process serves.
func localSeam(events *boardadapter.EventsBackend, a *app.App) boardSeam {
	writer := boardadapter.NewWriter(a)
	return boardSeam{
		writer:        writer,
		cardState:     writer,
		reader:        events,
		meta:          events,
		users:         events,
		events:        events,
		lists:         writer,
		notifyBackend: events,
	}
}

// remoteSeam is a board on a server this desktop has logged in to. One object
// answers all six, because over HTTP there is nothing to split them by.
func remoteSeam(serverURL, token string, log mlog.LoggerIFace) (boardSeam, *boardclient.Client) {
	client := boardclient.New(serverURL, token, model.GlobalTeamID, log)
	return boardSeam{
		writer:    client,
		cardState: client,
		reader:    client,
		meta:      client,
		users:     client,
		events:    client,
		lists:     client,
	}, client
}
