package acp

import "sort"

// Attention is one thing waiting for a person, from either of the two places a
// card can want one — see the reasons below. The card is what the UI shows it
// on; a planning conversation has no card at all.
type Attention struct {
	// Key identifies this wait for the UI, which keeps them in a map.
	Key string `json:"key"`
	// SessionID is who is asking, which is how a planning conversation — no
	// card to be found by — finds its own questions.
	SessionID string `json:"sessionId,omitempty"`
	CardID    string `json:"cardId,omitempty"`
	BoardID   string `json:"boardId,omitempty"`
	Title     string `json:"title,omitempty"`
	Agent     string `json:"agent,omitempty"`
	Reason    string `json:"reason"`
	// Tool is set for a permission question: what the agent wants to use.
	Tool string `json:"tool,omitempty"`
	// A question carries itself, so it can be answered where it is read.
	QuestionID string           `json:"questionId,omitempty"`
	Text       string           `json:"text,omitempty"`
	Options    []QuestionOption `json:"options,omitempty"`
	FreeText   bool             `json:"freeText,omitempty"`
	// Awaiting is false in an event that says a wait ended; the list only ever
	// carries true.
	Awaiting bool   `json:"awaiting"`
	Since    string `json:"since,omitempty"`
	// Acked is a wait a person has already seen — waved away, or opened. It is
	// still a wait: the card keeps its mark, because that is part of the card.
	// What it stops is the notification, which interrupts (attentionack.go).
	Acked bool `json:"acked,omitempty"`
}

// AttentionQuestion is the one reason a card wants a person: the protocol
// asking. A session sent session/request_permission or an elicitation, and the
// agent is waiting on the answer with its turn still open (question.go). It is
// answered where it is read — the notification, the phone, the card.
const AttentionQuestion = "question"

// withKey fills in what identifies this wait. A question is keyed by its own id
// rather than by its card: an agent making two tool calls at once asks twice,
// and answering one must not take the other off the screen.
func (a Attention) withKey() Attention {
	if a.QuestionID != "" {
		a.Key = "q:" + a.QuestionID
	} else {
		a.Key = "card:" + a.CardID
	}
	return a
}

// Attention lists every question waiting for a person, oldest first: the one
// that has been ignored longest is the one worth showing.
func (m *Manager) Attention() []Attention {
	questions := m.Questions()
	out := make([]Attention, 0, len(questions))
	for _, q := range questions {
		out = append(out, q.attention().withKey())
	}
	// Stamped here rather than stored on the record, so an ack withdrawn after
	// the wait was raised reads as withdrawn (attentionack.go).
	for i := range out {
		out[i].Acked = m.ackStanding(out[i].Key)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Since < out[j].Since })
	return out
}

func (m *Manager) emitAttentionRecord(a Attention) {
	if m == nil {
		return
	}
	a = a.withKey()
	a.Acked = a.Awaiting && m.ackStanding(a.Key)
	if m.ui == nil {
		return
	}
	m.ui.Emit(EventAttention, map[string]any{
		"key":        a.Key,
		"sessionId":  a.SessionID,
		"cardId":     a.CardID,
		"boardId":    a.BoardID,
		"title":      a.Title,
		"agent":      a.Agent,
		"reason":     a.Reason,
		"tool":       a.Tool,
		"questionId": a.QuestionID,
		"text":       a.Text,
		"options":    a.Options,
		"freeText":   a.FreeText,
		"awaiting":   a.Awaiting,
		"since":      a.Since,
		"acked":      a.Acked,
	})
}
