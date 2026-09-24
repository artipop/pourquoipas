package acp

import (
	"testing"
	"time"
)

func acked(e *fakeEmitter, key string) bool {
	p := lastAttention(e, key)
	return p != nil && p["acked"] == true
}

// The whole point of the ✕ on a notification: a person who has dealt with a
// wait has dealt with it, and no other window may go on announcing it.
func TestAWaitSomebodyHasSeenIsNotAnnouncedAgain(t *testing.T) {
	emitter := &fakeEmitter{}
	m := &Manager{ui: emitter}
	q := Question{ID: "q1", CardID: "card-1", BoardID: "board-1", Text: "Выкатывать?", AskedAt: time.Now()}
	m.questions = map[string]*pendingQuestion{q.ID: {q: q, reply: make(chan Answer, 1)}}

	m.emitQuestion(q, true)
	if acked(emitter, "q:"+q.ID) {
		t.Fatal("a wait nobody has seen was announced as already seen")
	}

	m.AckAttention("q:" + q.ID)

	if !acked(emitter, "q:"+q.ID) {
		t.Error("acknowledging a question was not said back to the other windows")
	}
	for _, a := range m.Attention() {
		if a.Key == "q:"+q.ID && !a.Acked {
			t.Error("a page opening now would announce a question somebody has already seen")
		}
	}
}

// An answered question is gone, and the acknowledgement of it must not outlive
// it: the next question on the same card is a new thing to be told about.
func TestTheAcknowledgementDoesNotOutliveTheQuestion(t *testing.T) {
	emitter := &fakeEmitter{}
	m := &Manager{ui: emitter, rootCtx: t.Context()}
	q := Question{ID: "q1", CardID: "card-1", BoardID: "board-1", Text: "Выкатывать?", AskedAt: time.Now()}

	done := make(chan Answer, 1)
	go func() { done <- m.awaitAnswer(t.Context(), q) }()
	waitFor(t, 2*time.Second, "the question to be asked", func() bool {
		return len(m.Questions()) == 1
	})
	m.AckAttention("q:" + q.ID)
	if err := m.AnswerQuestion(q.ID, Answer{OptionID: "yes"}); err != nil {
		t.Fatal(err)
	}
	<-done

	if m.ackStanding("q:" + q.ID) {
		t.Error("the next question under this id would arrive already waved away")
	}
}
