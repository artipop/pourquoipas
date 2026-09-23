package acp

import (
	"sync"
	"time"
)

// Having been told once is enough.
//
// The ack is kept here rather than in the page, so it holds for every window and
// for the phone at once — the page that drew the notification is not the only
// one that drew it — and survives a reload.
//
// A wait that ends takes its ack with it (emitAttentionRecord clears on Awaiting
// false), so nothing is left behind to suppress the next one.

// ackedWaits is the acknowledgements, keyed by Attention.Key and holding when
// the person made them.
type ackedWaits struct {
	mu sync.Mutex
	at map[string]time.Time
}

// AckAttention records that a person has seen this wait. Unknown keys are
// accepted rather than refused: the page acks what it is drawing, and a wait
// that ended between the click and the call is exactly the case where nothing
// needs to happen.
func (m *Manager) AckAttention(key string) {
	if m == nil || key == "" {
		return
	}
	m.acked.mu.Lock()
	if m.acked.at == nil {
		m.acked.at = map[string]time.Time{}
	}
	m.acked.at[key] = time.Now()
	m.acked.mu.Unlock()

	// Said back, so every other window takes the notification down too — and so
	// the page that clicked does not have to guess what the record now says.
	if a, ok := m.waitByKey(key); ok {
		m.emitAttentionRecord(a)
	}
}

// ackStanding reports whether this wait has already been seen.
func (m *Manager) ackStanding(key string) bool {
	m.acked.mu.Lock()
	defer m.acked.mu.Unlock()
	_, ok := m.acked.at[key]
	return ok
}

// clearAck forgets an acknowledgement, so the next wait under this key is
// announced.
func (m *Manager) clearAck(key string) {
	m.acked.mu.Lock()
	delete(m.acked.at, key)
	m.acked.mu.Unlock()
}

// waitByKey finds the live question a key names.
func (m *Manager) waitByKey(key string) (Attention, bool) {
	for _, q := range m.Questions() {
		if a := q.attention(); a.withKey().Key == key {
			return a, true
		}
	}
	return Attention{}, false
}
