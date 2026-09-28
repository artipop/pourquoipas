package acp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// How a stage says what became of it.
//
// A stage is an ACP session, and its turn ending is what ends it. What the turn
// cannot say by ending is the verdict and the values a route branches on, so
// those come through the board tools: finish_work with done or not done and the
// stage's declared outputs, and the card's own tools for the rest. The result a
// person reads is a comment the agent writes itself (comment_card) — one, with
// what matters — rather than the transcript of its turn pasted onto the card.

// stageReport is what an agent hands back through finish_work: whether the work
// is done, and one line about it.
type stageReport struct {
	ok      bool
	summary string
}

// resultBrief is how a stage with the board's tools is told to report, said in
// the task itself: whether the tools arrive loaded is the agent's business, and
// the task text is the one thing that always arrives.
const resultBrief = `When the work is finished, report it on the card through the board's tools:

1. Post the result as ONE comment with ` + "`comment_card`" + `: only what matters — what was done or decided, and what a person has to know or do next. Not a log of your steps.
2. If the task calls for it, put values into the card's fields with ` + "`update_card`" + ` (` + "`list_card_fields`" + ` lists them).
3. Move the card on with ` + "`move_card`" + `, into the column whose name says what has happened to it. Leave it where it is if none of them does.
4. Call ` + "`finish_work`" + ` — done or not done, and one line — to end the stage. When you cannot finish, say why in the comment and call it with done set to false.`

// boardToolsBrief is what a session handed the board's tools is told about
// reporting through them: a stage its result; a conversation with no card
// behind it nothing, since it has no stage to end and its own instructions say
// what the tools are for.
func (m *Manager) boardToolsBrief(s *Session) string {
	if s.CardID == "" {
		return ""
	}
	return resultBrief
}

// setReport records the verdict finish_work delivered. The first one stands:
// a stage is reported once.
func (s *Session) setReport(r stageReport) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.report != nil {
		return false
	}
	s.report = &r
	return true
}

func (s *Session) stageReport() (stageReport, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.report == nil {
		return stageReport{}, false
	}
	return *s.report, true
}

// markCommented records that the agent wrote on its own card, which is what
// keeps this side from writing a second comment after it (commentFirstTurn).
func (s *Session) markCommented() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commented = true
}

func (s *Session) hasCommented() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commented
}

// FinishWorkFromTools is the agent saying the stage is over, and how.
//
// fields are whatever the agent puts into the card's own properties on its way
// out, written **before the report is accepted**: a write the board refuses
// then comes back as this call's own error, to the one party that can fix it.
func (m *Manager) FinishWorkFromTools(token string, ok bool, summary string, fields map[string]string) error {
	g, found := m.boardGrant(token)
	if !found {
		return fmt.Errorf("no access to the board")
	}
	s := m.session(g.SessionID)
	if s == nil || s.CardID == "" {
		return fmt.Errorf("there is no card behind this conversation, so there is nobody to tell that the work is finished")
	}
	if len(fields) > 0 {
		if g.Property != "" {
			if _, taken := fieldsHave(fields, g.Property); taken {
				return fmt.Errorf("the column (%s) is not changed here: move_card moves the card", g.Property)
			}
		}
		if m.writer == nil {
			return fmt.Errorf("the board is not available")
		}
		ctx, cancel := context.WithTimeout(m.rootCtx, 10*time.Second)
		defer cancel()
		if err := m.forCard(s.CardID).Writer.SetCardFields(m.asAgent(ctx, s), s.CardID, fields); err != nil {
			return err
		}
	}
	if !s.setReport(stageReport{ok: ok, summary: strings.TrimSpace(summary)}) {
		return fmt.Errorf("the end of the work was already reported")
	}
	return nil
}

// fieldsHave reads a field the way a person named the property: ignoring case.
func fieldsHave(fields map[string]string, property string) (string, bool) {
	for k, v := range fields {
		if strings.EqualFold(k, property) {
			return v, true
		}
	}
	return "", false
}

// joinPrompts is texts separated by blank lines, empties dropped.
func joinPrompts(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n\n")
}

// boardFolder is the board's drafts folder: the board's own directory under the
// app's data, where a conversation with no folder of its own runs — and where
// what an agent leaves for one card is on hand for the next. Named by the
// board's id and nothing else, so it needs no state at all.
func (m *Manager) boardFolder(boardID string) string {
	if boardID == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(m.cfg.WorktreeDir), "boards", boardID)
}

// cardDrafts is where a card that names no folder is worked: a directory of
// its own inside the board's drafts. Its own, because an ordinary folder is
// held by one session at a time, and one shared directory would let a board
// run a single folderless card at once. Inside the drafts, because what the
// agent writes has to outlive the run — a scratch directory removed at the end
// took the result with it — and be where the board's other drafts are.
func (m *Manager) cardDrafts(boardID, cardID string) (string, error) {
	if boardID == "" || cardID == "" {
		return "", fmt.Errorf("a card's drafts need a board and a card")
	}
	dir := filepath.Join(m.boardFolder(boardID), "cards", cardID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create the card's drafts: %w", err)
	}
	return dir, nil
}

// gitLines runs git in dir and returns its non-empty output lines, nil on any
// failure: every caller treats "git could not say" as "nothing to say".
func gitLines(ctx context.Context, dir string, args ...string) []string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
