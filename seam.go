package main

import (
	"context"
	"sync"

	"github.com/artipop/xciii/server/app"
	"github.com/artipop/xciii/server/mlog"
	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/services/notify"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/boardadapter"
	"github.com/artipop/xciii/internal/boardclient"
	"github.com/artipop/xciii/internal/boardmodel"
	"github.com/artipop/xciii/internal/sources"
)

// Which board the agents work. Two things can answer internal/acp's questions —
// the board in this process and one over HTTP — and this is the one place that
// chooses. Attached, both are worked and the manager routes by which one has
// the card; what the *page* shows is the switcher's separate question, because
// a page holds one board's cards, members and sockets and cannot hold two.
type boardSeam struct {
	writer acp.BoardWriter
	reader acp.BoardReader
	meta   acp.BoardMeta
	users  acp.BoardUsers
	events acp.BoardEvents
	// lists is the board as the phone at /m and the share dialog read it.
	lists boardLists
	// sourceWriter is the board as internal/sources writes to it.
	sourceWriter sourceBoard

	// notifyBackend is how the in-process board announces a card move. Nil for
	// a board on a server, which says so over its own websocket instead.
	notifyBackend notify.Backend
}

// boardLists is the small, flat surface a phone reads. Which one answers
// follows what the page is showing, not what the agents are working.
type boardLists interface {
	Boards(ctx context.Context) ([]boardmodel.BoardSummary, error)
	BoardCards(ctx context.Context, boardID string) ([]boardmodel.CardSummary, error)
	MoveCardToBoard(ctx context.Context, cardID, toBoardID, column string) error
}

// localSeam is the board this process serves.
func localSeam(events *boardadapter.EventsBackend, a *app.App) boardSeam {
	writer := boardadapter.NewWriter(a)
	return boardSeam{
		sourceWriter:  boardadapter.NewSourceWriter(a),
		writer:        writer,
		reader:        events,
		meta:          events,
		users:         events,
		events:        events,
		lists:         writer,
		notifyBackend: events,
	}
}

// remoteSeam is a board on a server this desktop has logged in to. One object
// answers every part, because over HTTP there is nothing to split them by.
func remoteSeam(serverURL, token string, log mlog.LoggerIFace) (boardSeam, *boardclient.Client) {
	client := boardclient.New(serverURL, token, model.GlobalTeamID, log)
	return boardSeam{
		sourceWriter: boardclient.NewSourceWriter(client),
		writer:       client,
		reader:       client,
		meta:         client,
		users:        client,
		events:       client,
		lists:        client,
	}, client
}

// selectedLists picks the reader for the board on screen. A function because
// the switcher moves it while the app runs.
type selectedLists struct {
	local  boardLists
	remote boardLists
	// on is the selected source's id: empty for this machine's own board.
	on func() string
}

func (s selectedLists) pick() boardLists {
	if s.remote != nil && s.on != nil && s.on() != "" {
		return s.remote
	}
	return s.local
}

func (s selectedLists) Boards(ctx context.Context) ([]boardmodel.BoardSummary, error) {
	return s.pick().Boards(ctx)
}

func (s selectedLists) BoardCards(ctx context.Context, boardID string) ([]boardmodel.CardSummary, error) {
	return s.pick().BoardCards(ctx, boardID)
}

func (s selectedLists) MoveCardToBoard(ctx context.Context, cardID, toBoardID, column string) error {
	return s.pick().MoveCardToBoard(ctx, cardID, toBoardID, column)
}

// sourceBoard is everything internal/sources does to a board.
type sourceBoard interface {
	sources.BoardWriter
	sources.BoardItems
}

// sourcesByBoard sends a source's writes to the board they are for. Almost
// every card goes to the inbox, which is this machine's; the share dialog may
// pick any board the page lists, and a board belongs to this machine or to a
// server, so nobody has to decide where it writes.
type sourcesByBoard struct {
	local  sourceBoard
	remote sourceBoard
	// onServer says a board is the attached server's.
	onServer func(boardID string) bool

	mu sync.Mutex
	// seen is which board a card is on. The two writes below are keyed by card
	// alone, and a card id does not name a board — but the pipeline only ever
	// learns a card id two ways, by creating the card or by finding it, and
	// both name the board. So it is remembered there rather than asked for.
	seen map[string]string
}

func newSourcesByBoard(local, remote sourceBoard, onServer func(string) bool) *sourcesByBoard {
	return &sourcesByBoard{local: local, remote: remote, onServer: onServer, seen: map[string]string{}}
}

func (s *sourcesByBoard) pick(boardID string) sourceBoard {
	if s.remote != nil && s.onServer != nil && s.onServer(boardID) {
		return s.remote
	}
	return s.local
}

func (s *sourcesByBoard) remember(cardID, boardID string) {
	if cardID == "" || boardID == "" {
		return
	}
	s.mu.Lock()
	s.seen[cardID] = boardID
	s.mu.Unlock()
}

// forCard is the board a card was last seen on. A card this process has not
// created or found in this run goes to the local board, which is where it would
// have gone before there was a second one.
func (s *sourcesByBoard) forCard(cardID string) sourceBoard {
	s.mu.Lock()
	boardID := s.seen[cardID]
	s.mu.Unlock()
	return s.pick(boardID)
}

func (s *sourcesByBoard) CreateCard(ctx context.Context, boardID string, spec sources.CardSpec) (string, error) {
	cardID, err := s.pick(boardID).CreateCard(ctx, boardID, spec)
	s.remember(cardID, boardID)
	return cardID, err
}

// InboxBoard is always this machine's: the sources are this machine's registry,
// and what they bring is read here before it is carried anywhere.
func (s *sourcesByBoard) InboxBoard(ctx context.Context) (string, error) {
	return s.local.InboxBoard(ctx)
}

func (s *sourcesByBoard) CardBySourceItem(ctx context.Context, boardID, source, externalID string) (string, string, bool, error) {
	cardID, version, ok, err := s.pick(boardID).CardBySourceItem(ctx, boardID, source, externalID)
	if ok {
		s.remember(cardID, boardID)
	}
	return cardID, version, ok, err
}

func (s *sourcesByBoard) AddComment(ctx context.Context, cardID, text string) error {
	return s.forCard(cardID).AddComment(ctx, cardID, text)
}

// onServerBoard answers whether a board is the attached server's, by asking it.
// A yes is remembered — a board does not move between servers — and a no is
// not, or a board made on the server later could never become a yes.
func onServerBoard(meta acp.BoardMeta) func(string) bool {
	if meta == nil {
		return func(string) bool { return false }
	}
	var (
		mu    sync.Mutex
		known = map[string]bool{}
	)
	return func(boardID string) bool {
		if boardID == "" {
			return false
		}
		mu.Lock()
		yes := known[boardID]
		mu.Unlock()
		if yes {
			return true
		}
		if _, err := meta.BoardProperties(context.Background(), boardID); err != nil {
			return false
		}
		mu.Lock()
		known[boardID] = true
		mu.Unlock()
		return true
	}
}
