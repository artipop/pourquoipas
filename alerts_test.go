package main

import (
	"strings"
	"testing"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/apperr"
)

func keys(waits []acp.Attention) []string {
	out := make([]string, 0, len(waits))
	for _, a := range waits {
		out = append(out, a.Key)
	}
	return out
}

// The point of the whole feature: an agent that stops to ask something reaches
// somebody who is not looking at the board.
func TestAWaitNobodyHasHeardOfIsAnnounced(t *testing.T) {
	fresh, stale := alertPlan(map[string]bool{}, []acp.Attention{{Key: "t1", Title: "Почини логин"}})
	if got := keys(fresh); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("fresh = %v, want the one wait", got)
	}
	if len(stale) != 0 {
		t.Fatalf("stale = %v, want nothing to take down", stale)
	}
}

// Said once. A notification that comes back every time the manager re-reports
// the same wait is the spam the acknowledgement was built to end
// (internal/acp/attentionack.go), and it would come back here instead.
func TestAWaitAlreadyAnnouncedIsNotAnnouncedAgain(t *testing.T) {
	waiting := []acp.Attention{{Key: "t1"}}
	fresh, stale := alertPlan(map[string]bool{"t1": true}, waiting)
	if len(fresh) != 0 || len(stale) != 0 {
		t.Fatalf("fresh = %v, stale = %v, want neither", keys(fresh), stale)
	}
}

// Being told is one act wherever it happens. A person who waved the
// notification away on the board, on a second window or on their phone has been
// told, and the copy standing in the notification centre has to go with it.
func TestAnAcknowledgedWaitTakesItsNotificationDown(t *testing.T) {
	fresh, stale := alertPlan(map[string]bool{"t1": true}, []acp.Attention{{Key: "t1", Acked: true}})
	if len(fresh) != 0 {
		t.Fatalf("fresh = %v, want nothing announced again", keys(fresh))
	}
	if len(stale) != 1 || stale[0] != "t1" {
		t.Fatalf("stale = %v, want the acknowledged wait", stale)
	}
}

// An agent that was answered is not still asking, and a notification saying it
// is outlives the truth.
func TestAWaitThatEndedTakesItsNotificationDown(t *testing.T) {
	_, stale := alertPlan(map[string]bool{"t1": true}, nil)
	if len(stale) != 1 || stale[0] != "t1" {
		t.Fatalf("stale = %v, want the wait that ended", stale)
	}
}

// The acknowledgement is dropped when the CLI does a turn and stops again,
// because that is a new question. It has to reach a person as one.
func TestAWaitRaisedAgainIsAnnouncedAgain(t *testing.T) {
	// Acknowledged: the notification comes down and this side forgets it.
	_, stale := alertPlan(map[string]bool{"t1": true}, []acp.Attention{{Key: "t1", Acked: true}})
	told := map[string]bool{"t1": true}
	for _, key := range stale {
		delete(told, key)
	}
	fresh, _ := alertPlan(told, []acp.Attention{{Key: "t1"}})
	if got := keys(fresh); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("fresh = %v, want the wait announced afresh", got)
	}
}

// What a notification has to carry is which agent and which card: the person
// reading it is somewhere else entirely and has neither in front of them.
func TestANotificationNamesTheAgentAndTheCard(t *testing.T) {
	a := acp.Attention{Key: "t1", Agent: "клаус", Title: "Почини логин", Text: "агент ждёт ответа в терминале"}
	if got := alertTitle(englishWords, a); got != "клаус is asking" {
		t.Errorf("title = %q", got)
	}
	if got := alertBody(englishWords, a); !strings.Contains(got, "Почини логин") || !strings.Contains(got, "ждёт ответа") {
		t.Errorf("body = %q, want the card and the question", got)
	}
}

// A conversation with no card behind it can still be waiting, and a notification
// with an empty body says nothing at all.
func TestAWaitWithNothingToQuoteStillSaysWhatToDo(t *testing.T) {
	if got := alertBody(englishWords, acp.Attention{Key: "t1"}); got == "" {
		t.Fatal("a wait with no card and no question should still say something")
	}
}

// A menu of the menu bar is read standing up: a card whose title is a paragraph
// must not make the menu as wide as the screen.
func TestALongCardTitleIsClippedInTheMenu(t *testing.T) {
	long := strings.Repeat("длинный заголовок ", 20)
	got := alertMenuLabel(englishWords, acp.Attention{Key: "t1", Agent: "клаус", Title: long})
	if n := len([]rune(got)); n > 64 {
		t.Fatalf("label is %d runes: %q", n, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("a clipped label should say it was clipped: %q", got)
	}
}

// The words are the page's: a person reading the app in Russian gets the menu
// bar in Russian, and the words come from the same catalogue as the rest.
func TestTheMenuBarSpeaksTheWordsThePageSent(t *testing.T) {
	w := mergeWords(englishWords, uiWords{AgentAsking: "{agent} спрашивает", Quit: "Выход"})
	if got := alertTitle(w, acp.Attention{Agent: "клаус"}); got != "клаус спрашивает" {
		t.Errorf("title = %q", got)
	}
	if w.Quit != "Выход" {
		t.Errorf("quit = %q", w.Quit)
	}
	// A word the page did not send keeps its English default rather than
	// coming out blank: a page older than the word is still a page.
	if w.Open != "Open" {
		t.Errorf("open = %q, want the English default", w.Open)
	}
}

// How many waits the menu stopped naming is counted in the page's own plural:
// Russian takes a different form for 2 and for 5, and 21 goes with 1.
func TestHiddenWaitsTakeThePluralFormThePageChose(t *testing.T) {
	counts := make([]string, 100)
	for n := range counts {
		switch {
		case n%10 == 1 && n%100 != 11:
			counts[n] = "one"
		case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
			counts[n] = "few"
		default:
			counts[n] = "many"
		}
	}
	w := uiWords{MoreWaits: pluralWords{Counts: counts, Forms: map[string]string{
		"one": "ещё {count} агент", "few": "ещё {count} агента", "many": "ещё {count} агентов", "other": "ещё {count}",
	}}}
	for n, want := range map[int]string{1: "ещё 1 агент", 3: "ещё 3 агента", 5: "ещё 5 агентов", 21: "ещё 21 агент", 150: "ещё 150"} {
		if got := hiddenWaitsLabel(w, n); got != want {
			t.Errorf("%d: %q, want %q", n, got, want)
		}
	}
	if got := hiddenWaitsLabel(englishWords, 4); got != "…and 4 more" {
		t.Errorf("default: %q", got)
	}
}

// A permission is worded by this side rather than by the agent, so it reaches
// the notification as a code, and the page's sentence for it is the one used.
func TestAPermissionIsAskedInThePagesWords(t *testing.T) {
	ask := apperr.Say("question.allow-tool", "Allow {tool}?", "tool", "Bash")
	a := acp.Attention{Key: "t1", Title: "Почини логин", Text: ask.Text, Say: &ask}
	w := mergeWords(englishWords, uiWords{Messages: map[string]string{"question.allow-tool": "Разрешить {tool}?"}})
	if got := alertBody(w, a); got != "Почини логин — Разрешить Bash?" {
		t.Errorf("body = %q", got)
	}
	if got := alertBody(englishWords, a); got != "Почини логин — Allow Bash?" {
		t.Errorf("without the page's words: %q", got)
	}
}
