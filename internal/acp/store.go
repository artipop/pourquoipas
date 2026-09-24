package acp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Store is what this application knows about a card: its conversations, its
// place on a route, why it is standing still, the workspace it holds.
//
// It lives in the board's own database so that a foreign key has somewhere to
// point: deleting a card is a real DELETE FROM blocks, and nothing tells this
// side about it, so the keys are what stop a deleted card leaving its
// conversations and its queue place behind. They are enforced — `foreign_keys`
// travels in the DSN the board opens the handle with (server.NewStore).
//
// It does not own the handle. On SQLite it is one connection for the whole
// application, which is what lets a card moving, its route being recorded and
// its stall being lifted become one transaction.
type Store struct {
	db *sql.DB
}

// NewStore wraps the board's database handle. It creates nothing: the tables
// are rungs on the board's own migration ladder.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(query, args...)
}

func (s *Store) query(q string, args ...any) (*sql.Rows, error) {
	return s.db.Query(q, args...)
}

func (s *Store) queryRow(q string, args ...any) *sql.Row {
	return s.db.QueryRow(q, args...)
}

// newID is what a row of ours is called: UUIDv7, which sorts by the time it was
// made, so `ORDER BY id` means "as it happened" in the journals and no dialect
// needs its own spelling of AUTO_INCREMENT.
func newID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// A worse order, never a refused write: the caller is recording
		// something that already happened.
		return uuid.NewString()
	}
	return id.String()
}

// SessionRecord mirrors one agent_session row.
type SessionRecord struct {
	ID           string        `json:"id"`
	CardID       string        `json:"cardId"`
	BoardID      string        `json:"boardId"`
	AgentKind    string        `json:"agentKind"`
	ACPSessionID string        `json:"acpSessionId"`
	Status       SessionStatus `json:"status"`
	// ColumnKey is which column's limit this run counts against. Recorded so
	// the count is a question for the database rather than for one process's
	// memory — see RunningInColumn.
	ColumnKey    string     `json:"columnKey,omitempty"`
	Cwd          string     `json:"cwd"`
	WorktreePath string     `json:"worktreePath"`
	Branch       string     `json:"branch"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	ErrorText    string     `json:"errorText,omitempty"`
}

// SessionEventRecord mirrors one session_event row. There is no `seq`: the id
// is a UUIDv7 and so already sorts by the moment the event was recorded, which
// is the only thing the sequence number ever meant.
type SessionEventRecord struct {
	ID        string          `json:"id"`
	SessionID string          `json:"sessionId"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}

// SaveSetupStep records what was done with one step of a board's setup. It
// lives here rather than in the browser because it is about this machine: the
// same install seen from the server build, or after the page's storage is
// cleared, has to remember that a question was deliberately passed over.
func (s *Store) SaveSetupStep(st SetupStepState) error {
	if st.At.IsZero() {
		st.At = time.Now()
	}
	if err := s.ensureBoardRef(st.BoardID, true); err != nil {
		return err
	}
	_, err := s.exec(`INSERT INTO board_setup (board_id, step, status, changed_at)
		VALUES (?,?,?,?)
		ON CONFLICT(board_id, step) DO UPDATE SET status=excluded.status, changed_at=excluded.changed_at`,
		st.BoardID, st.Step, st.Status, st.At.UnixMilli())
	return err
}

// SetupSteps returns what is recorded about a board's setup.
func (s *Store) SetupSteps(boardID string) ([]SetupStepState, error) {
	rows, err := s.query(`SELECT board_id, step, status, changed_at FROM board_setup WHERE board_id=?`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SetupStepState
	for rows.Next() {
		var st SetupStepState
		var at int64
		if err := rows.Scan(&st.BoardID, &st.Step, &st.Status, &at); err != nil {
			return nil, err
		}
		st.At = time.UnixMilli(at)
		out = append(out, st)
	}
	return out, rows.Err()
}

// Every write that names a card or a board makes the row its foreign key points
// at first — an upsert rather than a read, because a card this side has never
// heard of is the ordinary case on the first write about it.

// LocalSource is the board this process serves, written as NULL: "no source
// named" is what a machine that never attached to anything says.
const LocalSource = ""

// ensureBoardRef makes the row a board key can point at. local says the board
// is this process's own, which is what carries the cascade.
func (s *Store) ensureBoardRef(boardID string, local bool) error {
	if boardID == "" {
		return nil
	}
	var localID any
	if local {
		localID = boardID
	}
	_, err := s.exec(`INSERT INTO board_ref (board_id, source_id, local_board_id, created_at)
		VALUES (?,?,?,?)
		ON CONFLICT(board_id) DO NOTHING`,
		boardID, nullable(LocalSource), localID, time.Now().UnixMilli())
	return err
}

// ensureCardRef makes the row a card key can point at, and the board's with it.
func (s *Store) ensureCardRef(cardID, boardID string) error {
	if cardID == "" {
		return nil
	}
	if err := s.ensureBoardRef(boardID, true); err != nil {
		return err
	}
	_, err := s.exec(`INSERT INTO card_ref (card_id, board_id, block_id, created_at)
		VALUES (?,?,?,?)
		ON CONFLICT(card_id) DO UPDATE SET
			board_id=COALESCE(excluded.board_id, card_ref.board_id)`,
		cardID, nullable(boardID), cardID, time.Now().UnixMilli())
	return err
}

// ensureRefs is the pair, for the writers that know both.
func (s *Store) ensureRefs(cardID, boardID string) error {
	return s.ensureCardRef(cardID, boardID)
}

// StallRecord is why nothing is happening on a card that was asked to do
// something: a folder another card holds, a column with no free place, an
// agent that could not start. One per card — a newer reason replaces the old
// one, and any progress deletes it.
type StallRecord struct {
	CardID    string    `json:"cardId"`
	NodeID    string    `json:"nodeId,omitempty"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

// SetStall records the reason, replacing whatever was there.
func (s *Store) SetStall(r StallRecord) error {
	if err := s.ensureCardRef(r.CardID, ""); err != nil {
		return err
	}
	_, err := s.exec(`INSERT INTO card_stall (card_id, node_id, reason, created_at)
		VALUES (?,?,?,?)
		ON CONFLICT(card_id) DO UPDATE SET
			node_id=excluded.node_id, reason=excluded.reason, created_at=excluded.created_at`,
		r.CardID, r.NodeID, r.Reason, time.Now().UnixMilli())
	return err
}

// ClearStall forgets the reason and reports whether there was one — the caller
// only announces a change that happened.
func (s *Store) ClearStall(cardID string) (bool, error) {
	res, err := s.exec(`DELETE FROM card_stall WHERE card_id=?`, cardID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Stall returns the card's recorded reason, if it has one.
func (s *Store) Stall(cardID string) (StallRecord, bool, error) {
	row := s.queryRow(`SELECT card_id, COALESCE(node_id,''), reason, created_at FROM card_stall WHERE card_id=?`, cardID)
	var r StallRecord
	var created int64
	err := row.Scan(&r.CardID, &r.NodeID, &r.Reason, &created)
	if err == sql.ErrNoRows {
		return StallRecord{}, false, nil
	}
	if err != nil {
		return StallRecord{}, false, err
	}
	r.CreatedAt = time.UnixMilli(created)
	return r, true, nil
}

// CardChangesRecord is how much a card's work amounts to, in numbers. What it
// is for is the button that opens the diff: the diff itself is read from git
// every time and never from here.
//
// Recorded rather than counted when asked, for two reasons. A stage ending
// folds the card's working copy away (FoldWorktree) while the question — is
// there anything to read — is asked on the stage *after* that one. And a board
// asks it per card, which would be a git process per card.
type CardChangesRecord struct {
	CardID string
	// BoardID is carried so the card's ref row knows its board — which is what
	// CardChangesForBoard reads, and what an empty one would hide the card from.
	BoardID string
	Branch  string
	Base    string
	Files   int
	Added   int
	Removed int
	// Dirty is files changed and not committed. Counted apart because they
	// exist only while a working copy does, and the committed half outlives it.
	Dirty      int
	ComputedAt time.Time
}

// Any reports whether there is anything to read at all.
func (c CardChangesRecord) Any() bool { return c.Files > 0 || c.Dirty > 0 }

// SetCardChanges records the count, replacing whatever was there.
func (s *Store) SetCardChanges(r CardChangesRecord) error {
	if err := s.ensureCardRef(r.CardID, r.BoardID); err != nil {
		return err
	}
	_, err := s.exec(`INSERT INTO card_changes (card_id, branch, base, files, added, removed, dirty, computed_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(card_id) DO UPDATE SET
			branch=excluded.branch, base=excluded.base, files=excluded.files,
			added=excluded.added, removed=excluded.removed, dirty=excluded.dirty,
			computed_at=excluded.computed_at`,
		r.CardID, nullable(r.Branch), nullable(r.Base), r.Files, r.Added, r.Removed, r.Dirty,
		time.Now().UnixMilli())
	return err
}

// CardChanges is what was last counted for this card.
func (s *Store) CardChanges(cardID string) (CardChangesRecord, bool, error) {
	row := s.queryRow(`SELECT card_id, COALESCE(branch,''), COALESCE(base,''), files, added, removed, dirty, computed_at
		FROM card_changes WHERE card_id=?`, cardID)
	var r CardChangesRecord
	var computed int64
	switch err := row.Scan(&r.CardID, &r.Branch, &r.Base, &r.Files, &r.Added, &r.Removed, &r.Dirty, &computed); {
	case err == sql.ErrNoRows:
		return CardChangesRecord{}, false, nil
	case err != nil:
		return CardChangesRecord{}, false, err
	}
	r.ComputedAt = time.UnixMilli(computed)
	return r, true, nil
}

// CardChangesForBoard is every counted card of one board, as card id → record.
// One query rather than one per card, the bargain ListTerminals and
// StallsOfKind already take: a board draws this on every card it has.
func (s *Store) CardChangesForBoard(boardID string) (map[string]CardChangesRecord, error) {
	rows, err := s.query(`SELECT c.card_id, COALESCE(c.branch,''), COALESCE(c.base,''),
			c.files, c.added, c.removed, c.dirty, c.computed_at
		FROM card_changes c JOIN card_ref r ON r.card_id = c.card_id
		WHERE r.board_id = ?`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]CardChangesRecord{}
	for rows.Next() {
		var r CardChangesRecord
		var computed int64
		if err := rows.Scan(&r.CardID, &r.Branch, &r.Base, &r.Files, &r.Added, &r.Removed, &r.Dirty, &computed); err != nil {
			return nil, err
		}
		r.ComputedAt = time.UnixMilli(computed)
		out[r.CardID] = r
	}
	return out, rows.Err()
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface{ Scan(dest ...any) error }

// InsertSession stores a new session row.
func (s *Store) InsertSession(r SessionRecord) error {
	if err := s.ensureRefs(r.CardID, r.BoardID); err != nil {
		return err
	}
	_, err := s.exec(`INSERT INTO agent_session
		(id, card_id, board_id, agent_kind, acp_session_id, status, column_key, cwd, worktree_path, branch, started_at, error_text)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, nullable(r.CardID), nullable(r.BoardID), r.AgentKind, r.ACPSessionID, string(r.Status),
		nullable(r.ColumnKey), r.Cwd, r.WorktreePath, r.Branch, r.StartedAt.UnixMilli(), r.ErrorText)
	return err
}

// UpdateSession updates the mutable fields of a session row.
func (s *Store) UpdateSession(id string, status SessionStatus, acpSessionID, cwd, worktreePath, branch, errorText string, finishedAt *time.Time) error {
	var fin any
	if finishedAt != nil {
		fin = finishedAt.UnixMilli()
	}
	_, err := s.exec(`UPDATE agent_session
		SET status=?, acp_session_id=?, cwd=?, worktree_path=?, branch=?, error_text=?, finished_at=?
		WHERE id=?`,
		string(status), acpSessionID, cwd, worktreePath, branch, errorText, fin, id)
	return err
}

// SetSessionStatus updates only the status (and finished_at for terminal states).
func (s *Store) SetSessionStatus(id string, status SessionStatus, errorText string) error {
	var fin any
	if status.Terminal() {
		fin = time.Now().UnixMilli()
	}
	_, err := s.exec(`UPDATE agent_session SET status=?, error_text=?, finished_at=COALESCE(?, finished_at) WHERE id=?`,
		string(status), errorText, fin, id)
	return err
}

// AppendEvent stores one session event.
func (s *Store) AppendEvent(sessionID, kind string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		b = []byte(fmt.Sprintf("%q", fmt.Sprint(payload)))
	}
	_, err = s.exec(`INSERT INTO session_event (id, session_id, kind, payload_json, created_at) VALUES (?,?,?,?,?)`,
		newID(), sessionID, kind, string(b), time.Now().UnixMilli())
	return err
}

// SessionsForCard returns all sessions of a card, newest first, with events.
func (s *Store) SessionsForCard(cardID string) ([]SessionRecord, []SessionEventRecord, error) {
	rows, err := s.query(`SELECT id, COALESCE(card_id,''), COALESCE(board_id,''), agent_kind, COALESCE(acp_session_id,''), status,
		COALESCE(column_key,''), COALESCE(cwd,''), COALESCE(worktree_path,''), COALESCE(branch,''), started_at, finished_at, COALESCE(error_text,'')
		FROM agent_session WHERE card_id=? ORDER BY started_at DESC`, cardID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var sessions []SessionRecord
	for rows.Next() {
		r, err := scanSession(rows)
		if err != nil {
			return nil, nil, err
		}
		sessions = append(sessions, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var events []SessionEventRecord
	for _, sess := range sessions {
		evRows, err := s.query(`SELECT id, session_id, kind, payload_json, created_at
			FROM session_event WHERE session_id=? ORDER BY id`, sess.ID)
		if err != nil {
			return nil, nil, err
		}
		for evRows.Next() {
			var ev SessionEventRecord
			var payload string
			var created int64
			if err := evRows.Scan(&ev.ID, &ev.SessionID, &ev.Kind, &payload, &created); err != nil {
				evRows.Close()
				return nil, nil, err
			}
			ev.Payload = json.RawMessage(payload)
			ev.CreatedAt = time.UnixMilli(created)
			events = append(events, ev)
		}
		evRows.Close()
		if err := evRows.Err(); err != nil {
			return nil, nil, err
		}
	}
	return sessions, events, nil
}

// StaleSessions returns sessions left in a non-terminal state (e.g. after an
// app crash/restart).
func (s *Store) StaleSessions() ([]SessionRecord, error) {
	rows, err := s.query(`SELECT id, COALESCE(card_id,''), COALESCE(board_id,''), agent_kind, COALESCE(acp_session_id,''), status,
		COALESCE(column_key,''), COALESCE(cwd,''), COALESCE(worktree_path,''), COALESCE(branch,''), started_at, finished_at, COALESCE(error_text,'')
		FROM agent_session WHERE status IN (?,?,?)`,
		string(StatusQueued), string(StatusRunning), string(StatusWaitingPermission))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionRecord
	for rows.Next() {
		r, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClaimIdempotency records key and reports whether it was free (true = first
// claim within the window; false = duplicate). Expired keys are purged first.
func (s *Store) ClaimIdempotency(key, sessionID string, window time.Duration) (bool, error) {
	cutoff := time.Now().Add(-window).UnixMilli()
	if _, err := s.exec(`DELETE FROM idempotency WHERE created_at < ?`, cutoff); err != nil {
		return false, err
	}
	res, err := s.exec(`INSERT INTO idempotency (token, session_id, created_at) VALUES (?,?,?) ON CONFLICT(token) DO NOTHING`,
		key, sessionID, time.Now().UnixMilli())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func scanSession(rows *sql.Rows) (SessionRecord, error) {
	var r SessionRecord
	var status string
	var started int64
	var finished sql.NullInt64
	if err := rows.Scan(&r.ID, &r.CardID, &r.BoardID, &r.AgentKind, &r.ACPSessionID, &status, &r.ColumnKey,
		&r.Cwd, &r.WorktreePath, &r.Branch, &started, &finished, &r.ErrorText); err != nil {
		return r, err
	}
	r.Status = SessionStatus(status)
	r.StartedAt = time.UnixMilli(started)
	if finished.Valid {
		t := time.UnixMilli(finished.Int64)
		r.FinishedAt = &t
	}
	return r, nil
}

// LatestBranchForCard is the branch the card was last worked on: the worktree
// branch of its most recent session. With worktrees on — the default — that is
// the branch the agent commits to, and the card itself never learns its name,
// so this is where anything watching the folder has to ask.
func (s *Store) LatestBranchForCard(cardID string) (string, error) {
	var branch string
	err := s.queryRow(`SELECT branch FROM agent_session
		WHERE card_id=? AND branch<>'' ORDER BY started_at DESC LIMIT 1`, cardID).Scan(&branch)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return branch, nil
}

// Checkout is the git state of one owner's work in one workspace: which branch
// was made for it, which directory the work happens in, and what it was cut
// from. A row exists only where the workspace is a repository — an ordinary
// folder creates nothing and so has nothing to record, which is why this is
// called a checkout at all.
//
// The owner is a card, or a board for a conversation with no card, and they are
// two columns because a foreign key cannot point at two tables through one. It
// is what makes a checkout outlive the run that made it: the second stage of a
// route, and the terminal a person opens beside it, get the same branch and the
// same directory as the first, instead of each making its own.
type Checkout struct {
	WorkspaceID string
	CardID      string
	BoardID     string
	Mode        string
	Branch      string
	Path        string
	Base        string
	CreatedAt   time.Time
}

// Owner names whoever holds this checkout, for a message a person reads. It is
// not a key and nothing looks anything up by it.
func (c Checkout) Owner() string {
	if c.CardID != "" {
		return c.CardID
	}
	if c.BoardID != "" {
		return BoardOwner(c.BoardID)
	}
	return ""
}

// SaveCheckout records one. Called when it is created, and again when an owner
// that had released it takes it back.
func (s *Store) SaveCheckout(c Checkout) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	// Upserted on the owner rather than on the id: the id is ours and a second
	// claim by the same owner is the same checkout, not another one.
	column, value := "card_id", any(c.CardID)
	if c.CardID == "" {
		column, value = "board_id", any(nullable(c.BoardID))
	}
	var id string
	err := s.queryRow(`SELECT id FROM checkout WHERE workspace_id=? AND `+column+`=?`,
		c.WorkspaceID, value).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		id = newID()
	case err != nil:
		return err
	}
	// The owner is a card or a board, never both — see the checkout table.
	if err := s.ensureCardRef(c.CardID, ""); err != nil {
		return err
	}
	if err := s.ensureBoardRef(c.BoardID, true); err != nil {
		return err
	}
	_, err = s.exec(`INSERT INTO checkout
		(id, workspace_id, card_id, board_id, mode, branch, path, base, created_at, released_at)
		VALUES (?,?,?,?,?,?,?,?,?,NULL)
		ON CONFLICT(id) DO UPDATE SET
			mode=excluded.mode, branch=excluded.branch, path=excluded.path,
			base=excluded.base, created_at=excluded.created_at, released_at=NULL`,
		id, c.WorkspaceID, nullable(c.CardID), nullable(c.BoardID), c.Mode,
		nullable(c.Branch), nullable(c.Path), nullable(c.Base), c.CreatedAt.UnixMilli())
	return err
}

const checkoutColumns = `workspace_id, COALESCE(card_id,''), COALESCE(board_id,''), mode,
	COALESCE(branch,''), COALESCE(path,''), COALESCE(base,''), created_at`

// CheckoutOf is what this owner holds in this workspace, if it still holds
// anything.
func (s *Store) CheckoutOf(workspaceID, cardID, boardID string) (Checkout, bool, error) {
	column, value := "card_id", any(cardID)
	if cardID == "" {
		column, value = "board_id", any(nullable(boardID))
	}
	row := s.queryRow(`SELECT `+checkoutColumns+` FROM checkout
		WHERE workspace_id=? AND `+column+`=? AND released_at IS NULL`, workspaceID, value)
	return scanCheckout(row)
}

// WorkspaceHeldBy is whoever holds this workspace now — and only a branch in
// the folder itself holds one. A copy per card holds nothing, which is the
// whole point of it.
func (s *Store) WorkspaceHeldBy(workspaceID string) (Checkout, bool, error) {
	row := s.queryRow(`SELECT `+checkoutColumns+` FROM checkout
		WHERE workspace_id=? AND released_at IS NULL AND mode=?
		ORDER BY created_at LIMIT 1`, workspaceID, WorkModeBranch)
	return scanCheckout(row)
}

// ReleaseCheckout gives a workspace back. The row stays: which branch a card
// worked on is worth remembering after the folder is free again.
func (s *Store) ReleaseCheckout(workspaceID, cardID, boardID string) error {
	column, value := "card_id", any(cardID)
	if cardID == "" {
		column, value = "board_id", any(nullable(boardID))
	}
	_, err := s.exec(`UPDATE checkout SET released_at=?
		WHERE workspace_id=? AND `+column+`=? AND released_at IS NULL`,
		time.Now().UnixMilli(), workspaceID, value)
	return err
}

// ReleaseBranch gives back whatever checkout was on this branch, whoever holds
// it — how a merge frees the folder for the next card. It answers with the
// owner, for the line that says so in the log.
func (s *Store) ReleaseBranch(workspaceID, branch string) (string, error) {
	row := s.queryRow(`SELECT COALESCE(card_id,''), COALESCE(board_id,'') FROM checkout
		WHERE workspace_id=? AND branch=? AND released_at IS NULL`, workspaceID, branch)
	var cardID, boardID string
	switch err := row.Scan(&cardID, &boardID); {
	case err == sql.ErrNoRows:
		return "", nil
	case err != nil:
		return "", err
	}
	owner := Checkout{CardID: cardID, BoardID: boardID}.Owner()
	return owner, s.ReleaseCheckout(workspaceID, cardID, boardID)
}

func scanCheckout(row scanner) (Checkout, bool, error) {
	var (
		c       Checkout
		created int64
	)
	switch err := row.Scan(&c.WorkspaceID, &c.CardID, &c.BoardID, &c.Mode,
		&c.Branch, &c.Path, &c.Base, &created); {
	case err == sql.ErrNoRows:
		return Checkout{}, false, nil
	case err != nil:
		return Checkout{}, false, err
	}
	c.CreatedAt = time.UnixMilli(created)
	return c, true, nil
}

// BranchForCard is the branch this card works on, in whichever workspace it
// holds one. What a deploy publishes when the card names no branch itself.
func (s *Store) BranchForCard(cardID string) (string, error) {
	row := s.queryRow(`SELECT branch FROM checkout
		WHERE card_id=? AND released_at IS NULL AND branch IS NOT NULL AND branch<>''
		ORDER BY created_at DESC LIMIT 1`, cardID)
	var branch string
	switch err := row.Scan(&branch); {
	case err == sql.ErrNoRows:
		return "", nil
	case err != nil:
		return "", err
	}
	return branch, nil
}

// CheckoutForCard is the whole live checkout of a card — the directory
// included, which is what CheckoutModeForCard deliberately leaves out and what
// anything reading the work itself needs (carddiff.go).
func (s *Store) CheckoutForCard(cardID string) (Checkout, bool, error) {
	row := s.queryRow(`SELECT `+checkoutColumns+` FROM checkout
		WHERE card_id=? AND released_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, cardID)
	return scanCheckout(row)
}

// CheckoutModeForCard is how the card's live checkout is arranged — "worktree"
// or "branch" — or "" when it holds none. The card's stamp names its line after
// it: a person who set a copy of its own should read worktree off the card, not
// a bare branch label that spells the *other* setting.
func (s *Store) CheckoutModeForCard(cardID string) (mode, branch, base string, err error) {
	row := s.queryRow(`SELECT mode, COALESCE(branch,''), COALESCE(base,'') FROM checkout
		WHERE card_id=? AND released_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, cardID)
	switch err := row.Scan(&mode, &branch, &base); {
	case err == sql.ErrNoRows:
		return "", "", "", nil
	case err != nil:
		return "", "", "", err
	}
	return mode, branch, base, nil
}

// nullable turns an empty string into absence. A key column says "there is
// none" with NULL and never with "" — a planning conversation has no card, an
// ordinary folder has no branch (CLAUDE.md, "Absence is NULL").
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// RunningInColumn counts the sessions of one column that have not finished.
//
// In the database rather than in this process's memory: a column's limit is a
// fact about the board, and two windows of this app each counted their own, so a
// limit of one ran two. A row left by a crash is swept at launch.
func (s *Store) RunningInColumn(columnKey string) (int, error) {
	if columnKey == "" {
		return 0, nil
	}
	row := s.queryRow(`SELECT COUNT(*) FROM agent_session WHERE column_key=? AND status IN (?,?,?)`,
		columnKey, string(StatusQueued), string(StatusRunning), string(StatusWaitingPermission))
	var n int
	err := row.Scan(&n)
	return n, err
}
