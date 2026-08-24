package acp

import (
	"fmt"
	"time"
)

// The route handing a card to a person.
//
// Everything else that says "somebody is needed here" is an agent that stopped
// mid-work — a question on the protocol, a CLI drawing nothing. This is the
// opposite case and the commonest one: the machine did its part, moved the card
// on, and the next stage is a person's. «На ревью» is exactly that, and until
// this existed the card simply appeared in a column and waited for somebody to
// happen to look at the board.
//
// Only when *we* moved the card (enterNode's `move`). A person who drags a card
// into a review column is already looking at it, and telling them what they
// just did is the noise this is meant to be the opposite of.
//
// It differs from an agent's wait in one way, and the difference is the whole
// design: being told is the whole of it. An agent that has asked something goes
// on waiting after somebody has looked, so its ack silences the notification
// and leaves the wait standing; a card that has arrived is on the board, where
// the person now knows to find it, so the ack ends the wait outright
// (AckAttention). Otherwise every card parked at a review would hold the tray
// icon amber for as long as it stood there, and the icon would stop meaning
// anything.

// AttentionCard is the reason code for it. Kept apart from AttentionQuestion
// and AttentionTerminal because what it offers a person is different: there is
// no terminal to open — nothing is running — and the card's own amber button,
// which is about a CLI, deliberately ignores it.
const AttentionCard = "card"

// handOverToPerson records and announces that the route has parked this card on
// somebody. text is what is being asked of them, in the words of the stage.
func (m *Manager) handOverToPerson(ev CardMoved, text string) {
	m.handOverQuestion(ev, text, nil)
}

// handOverQuestion is the same handover carrying the question that caused it,
// for the one kind that has answers to offer: a transition holding the card
// until somebody says how the work is arranged. The wait then describes itself
// — id and options — so a surface can put the question in front of whoever is
// looking instead of pointing them at the card, which is what a person who has
// just dragged that card needs.
//
// The rest of the handovers pass nil, and mean what they always did: the card
// is on a stage nobody works, and being told is the whole of it.
func (m *Manager) handOverQuestion(ev CardMoved, text string, q *AskQuestion) {
	if ev.CardID == "" {
		return
	}
	a := Attention{
		CardID:   ev.CardID,
		BoardID:  ev.BoardID,
		Title:    ev.Title,
		Reason:   AttentionCard,
		Text:     text,
		Awaiting: true,
		Since:    time.Now().Format(time.RFC3339),
	}
	if q != nil {
		// The options and not the question's id: withKey would key the wait by
		// that id, and an ask id is the arrow's own word for the question
		// («workmode») rather than something unique — two cards held on one
		// route would then be one wait, and answering either would take the
		// other off every screen. Keyed by the card, which is what a wait about
		// a card is. Whoever answers names no question, and AnswerCardAsk reads
		// that as the one the card is being asked now.
		for _, o := range q.Options {
			a.Options = append(a.Options, QuestionOption{ID: o.Value, Label: o.Label})
		}
	}
	a = a.withKey()

	m.stageMu.Lock()
	if m.cardWaits == nil {
		m.cardWaits = map[string]Attention{}
	}
	m.cardWaits[ev.CardID] = a
	m.stageMu.Unlock()

	// A fresh arrival is a fresh thing to be told, even about a card somebody
	// waved away last time it came round — a route loops on purpose, and the
	// second time through is not the first time's notification.
	m.clearAck(a.Key)
	m.emitAttentionRecord(a)
}

// handoverText says what the person is being asked to do, naming the stage. A
// stage whose action is nobody's is the ordinary case; a card that an agent
// would have worked but for the person holding it is the other.
func handoverText(column string, taken bool) string {
	if taken {
		return fmt.Sprintf("карточка на стадии «%s» — она за вами, агент её не берёт", column)
	}
	return fmt.Sprintf("карточка приехала на стадию «%s» — дальше нужен человек", column)
}

// cardAttention is every card standing on somebody, for the list the UI asks
// for when its socket reconnects.
func (m *Manager) cardAttention() []Attention {
	m.stageMu.Lock()
	defer m.stageMu.Unlock()
	out := make([]Attention, 0, len(m.cardWaits))
	for _, a := range m.cardWaits {
		out = append(out, a)
	}
	return out
}

// forgetCardWait ends the wait on a card, whatever ended it — the card moved
// on, an agent took it, somebody said they had seen it. Answers with the record
// so the caller can say so.
func (m *Manager) forgetCardWait(cardID string) (Attention, bool) {
	if m == nil || cardID == "" {
		return Attention{}, false
	}
	m.stageMu.Lock()
	a, ok := m.cardWaits[cardID]
	delete(m.cardWaits, cardID)
	m.stageMu.Unlock()
	return a, ok
}

// cardWaitKey is the card a wait key names, or "" when the key is not one of
// these. The key is what the page hands back, and it is the card's own
// (Attention.withKey).
func (m *Manager) cardWaitKey(key string) string {
	m.stageMu.Lock()
	defer m.stageMu.Unlock()
	for cardID, a := range m.cardWaits {
		if a.Key == key {
			return cardID
		}
	}
	return ""
}
