package acp

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestZZDebugOffCard(t *testing.T) {
	m, _, events, _ := testManager(t, fakeClaudeHappy, nil)
	var off atomic.Bool
	off.Store(true)
	m.SetSwitchedOff(off.Load)

	events.ch <- moveEvent("card-off", "opt-backlog", "opt-agent")
	time.Sleep(2 * time.Second)
	sessions, _, err := m.store.SessionsForCard("card-off")
	t.Logf("err=%v sessions=%d", err, len(sessions))
	for _, s := range sessions {
		t.Logf("row: id=%s card=%s status=%s kind=%s err=%q", s.ID, s.CardID, s.Status, s.AgentKind, s.ErrorText)
	}
}
