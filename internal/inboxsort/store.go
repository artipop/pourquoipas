package inboxsort

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Store keeps what the sorter has read about each board and what it decided
// about each card, in the board's own database (tools/schemagen:
// board_summary, inbox_sort). The handle belongs to the board, which opened it
// and closes it.
type Store struct {
	db *sql.DB
}

// NewStore wraps the board's database handle. It creates nothing: the tables
// are rungs on the board's own migration ladder.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Summary is what one board's cards are about, as the agent last read them.
type Summary struct {
	BoardID     string    `json:"boardId"`
	Text        string    `json:"text"`
	CardCount   int       `json:"cardCount"`
	Fingerprint string    `json:"-"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Summaries returns every summary this machine has, by board.
func (s *Store) Summaries() (map[string]Summary, error) {
	rows, err := s.db.Query(`SELECT board_id, summary, card_count, fingerprint, updated_at FROM board_summary`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Summary{}
	for rows.Next() {
		var sum Summary
		var at int64
		if err := rows.Scan(&sum.BoardID, &sum.Text, &sum.CardCount, &sum.Fingerprint, &at); err != nil {
			return nil, err
		}
		sum.UpdatedAt = time.UnixMilli(at)
		out[sum.BoardID] = sum
	}
	return out, rows.Err()
}

// SaveSummary writes one board's summary over whatever it had.
func (s *Store) SaveSummary(sum Summary) error {
	if err := s.ensureBoardRef(sum.BoardID); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO board_summary (board_id, summary, card_count, fingerprint, updated_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT(board_id) DO UPDATE SET
			summary=excluded.summary, card_count=excluded.card_count,
			fingerprint=excluded.fingerprint, updated_at=excluded.updated_at`,
		sum.BoardID, sum.Text, sum.CardCount, sum.Fingerprint, sum.UpdatedAt.UnixMilli())
	return err
}

// Outcomes a decision can record — the CHECK on inbox_sort.outcome.
const (
	OutcomeMoved  = "moved"
	OutcomeKept   = "kept"
	OutcomeFailed = "failed"
)

// Decision is one line of the sorter's log.
type Decision struct {
	ID        string    `json:"id"`
	CardID    string    `json:"cardId,omitempty"`
	CardTitle string    `json:"cardTitle,omitempty"`
	BoardID   string    `json:"boardId,omitempty"`
	Outcome   string    `json:"outcome"`
	Reason    string    `json:"reason,omitempty"`
	At        time.Time `json:"at"`
}

// Record appends a decision and trims the log to decisionsKept.
func (s *Store) Record(d Decision) error {
	if err := s.ensureCardRef(d.CardID); err != nil {
		return err
	}
	if err := s.ensureBoardRef(d.BoardID); err != nil {
		return err
	}
	if d.ID == "" {
		d.ID = newID()
	}
	if d.At.IsZero() {
		d.At = time.Now()
	}
	if _, err := s.db.Exec(`INSERT INTO inbox_sort (id, card_id, card_title, board_id, outcome, reason, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		d.ID, nullable(d.CardID), nullable(d.CardTitle), nullable(d.BoardID), d.Outcome, nullable(d.Reason), d.At.UnixMilli()); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM inbox_sort WHERE id NOT IN (SELECT id FROM inbox_sort ORDER BY id DESC LIMIT ?)`, decisionsKept)
	return err
}

// decisionsKept is how much of the log survives. It answers "why is this card
// here" about recent arrivals; nobody reads it as an archive.
const decisionsKept = 300

// Decisions returns the log, newest first.
func (s *Store) Decisions(limit int) ([]Decision, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, COALESCE(card_id,''), COALESCE(card_title,''), COALESCE(board_id,''),
		outcome, COALESCE(reason,''), created_at FROM inbox_sort ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Decision
	for rows.Next() {
		var d Decision
		var at int64
		if err := rows.Scan(&d.ID, &d.CardID, &d.CardTitle, &d.BoardID, &d.Outcome, &d.Reason, &at); err != nil {
			return nil, err
		}
		d.At = time.UnixMilli(at)
		out = append(out, d)
	}
	return out, rows.Err()
}

// ensureBoardRef and ensureCardRef make the rows the keys point at, the way
// internal/acp and internal/sources do (tools/schemagen, card_ref). Every board
// and card here is this process's own: the inbox is this machine's, and so is
// every board the sorter carries a card to.
func (s *Store) ensureBoardRef(boardID string) error {
	if boardID == "" {
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO board_ref (board_id, source_id, local_board_id, created_at)
		VALUES (?,NULL,?,?) ON CONFLICT(board_id) DO NOTHING`,
		boardID, boardID, time.Now().UnixMilli())
	return err
}

func (s *Store) ensureCardRef(cardID string) error {
	if cardID == "" {
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO card_ref (card_id, board_id, block_id, created_at)
		VALUES (?,NULL,?,?) ON CONFLICT(card_id) DO NOTHING`,
		cardID, cardID, time.Now().UnixMilli())
	return err
}

// newID names a row of the log: UUIDv7, so ORDER BY id is the order things
// happened in.
func newID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
