package sources

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeBoard is the board these tests write to: it records what happened instead
// of doing it, which is the whole of what the pipeline needs from a board.
//
// It is guarded, because the runner writes to it from the goroutine of the
// source it is running while the test reads it.
type fakeBoard struct {
	mu       sync.Mutex
	created  []CardSpec
	boards   []string
	comments []string
	nextID   int
	failNext error

	// asked counts how often the board was asked which board is the inbox:
	// once per batch is the whole point of the delivery scope.
	asked int
}

func (f *fakeBoard) CreateCard(_ context.Context, boardID string, spec CardSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return "", err
	}
	f.nextID++
	f.created = append(f.created, spec)
	f.boards = append(f.boards, boardID)
	return "card" + string(rune('0'+f.nextID)), nil
}

func (f *fakeBoard) AddComment(_ context.Context, cardID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.comments = append(f.comments, cardID+":"+text)
	return nil
}

func (f *fakeBoard) InboxBoard(_ context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked++
	return "inbox", nil
}

// The readers a test uses. Snapshots rather than the slices themselves, so a
// test cannot read one while the runner appends to it.
func (f *fakeBoard) cards() []CardSpec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]CardSpec(nil), f.created...)
}

func (f *fakeBoard) boardIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.boards...)
}

func (f *fakeBoard) commentLines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.comments...)
}

func (f *fakeBoard) inboxAsks() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked
}

func (f *fakeBoard) refuseOnce(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext = err
}

func testManager(t *testing.T, entry SourceEntry) (*Manager, *fakeBoard, *Store) {
	t.Helper()
	store, err := newTestStore(t, filepath.Join(t.TempDir(), "sources.db"))
	if err != nil {
		t.Fatal(err)
	}
	board := &fakeBoard{}
	// No config path: the registry is in memory, which is what a test wants.
	return NewManager(Config{Sources: []SourceEntry{entry}}, "", store, board, nil), board, store
}

func phoneSource() SourceEntry {
	return SourceEntry{
		Name: "телефон", Enabled: true, Noisy: true,
		Rules: []Rule{{
			Name:  "доставка",
			When:  Match{Props: map[string]string{"app": "delivery"}},
			Then:  ActionCard,
			Props: map[string]string{"Ссылка": "{{.URL}}"},
		}},
	}
}

func deliveryItem() Item {
	return Item{ExternalID: "n1", Version: "v1", Title: "Доставка завтра",
		Body: "Заказ №123", URL: "https://example.com/1",
		Props: map[string]string{"app": "delivery"}}
}

func TestAnItemBecomesACardOnTheInbox(t *testing.T) {
	m, board, _ := testManager(t, phoneSource())

	res, err := m.Deliver(context.Background(), "телефон", []Item{deliveryItem()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 {
		t.Fatalf("result: %+v", res)
	}
	if len(board.cards()) != 1 || board.cards()[0].Title != "Доставка завтра" {
		t.Fatalf("created: %+v", board.cards())
	}
	if board.boardIDs()[0] != "inbox" {
		t.Fatalf("every source writes to the inbox, the card went to %q", board.boardIDs()[0])
	}
	// The way back to the original is a property; where it came from is not —
	// the card is authored by the source, which is the board's own answer and
	// the one the inbox groups by.
	props := board.cards()[0].Properties
	if props["Ссылка"] != "https://example.com/1" {
		t.Fatalf("properties: %+v", props)
	}
	if board.cards()[0].Source != "телефон" {
		t.Fatalf("the card should be authored by its source: %+v", board.cards()[0])
	}
}

func TestTheSameItemTwiceMakesOneCard(t *testing.T) {
	m, board, _ := testManager(t, phoneSource())
	ctx := context.Background()

	if _, err := m.Deliver(ctx, "телефон", []Item{deliveryItem()}); err != nil {
		t.Fatal(err)
	}
	res, err := m.Deliver(ctx, "телефон", []Item{deliveryItem()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 || res.Created != 0 {
		t.Fatalf("result: %+v", res)
	}
	if len(board.cards()) != 1 {
		t.Fatalf("a source reports its whole state every time: %+v", board.cards())
	}
}

func TestAChangedItemComesBackAsACommentOnItsOwnCard(t *testing.T) {
	m, board, _ := testManager(t, phoneSource())
	ctx := context.Background()

	if _, err := m.Deliver(ctx, "телефон", []Item{deliveryItem()}); err != nil {
		t.Fatal(err)
	}
	changed := deliveryItem()
	changed.Version = "v2"
	changed.Body = "Курьер задерживается"
	res, err := m.Deliver(ctx, "телефон", []Item{changed})
	if err != nil {
		t.Fatal(err)
	}

	if res.Commented != 1 || len(board.cards()) != 1 {
		t.Fatalf("an update must not create a second card: %+v %+v", res, board.cards())
	}
	if len(board.commentLines()) != 1 || !strings.Contains(board.commentLines()[0], "Курьер задерживается") {
		t.Fatalf("comments: %+v", board.commentLines())
	}
	if !strings.HasPrefix(board.commentLines()[0], "card1:") {
		t.Fatalf("the comment went to another card: %q", board.commentLines()[0])
	}
}

// A stream of notifications is mostly noise: there a rule is a subscription,
// and what no rule claimed is deliberately dropped.
func TestOnANoisySourceWhatMatchedNothingIsDropped(t *testing.T) {
	m, board, store := testManager(t, phoneSource())

	res, err := m.Deliver(context.Background(), "телефон",
		[]Item{{ExternalID: "n2", Title: "2 новых сообщения", Props: map[string]string{"app": "chat"}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dropped != 1 || len(board.cards()) != 0 {
		t.Fatalf("result: %+v, created: %+v", res, board.cards())
	}
	// Dropped is still recorded: "why did nothing happen" is the only question
	// anybody asks of a source.
	events, err := store.Events("телефон", 10)
	if err != nil || len(events) != 1 || events[0].Outcome != OutcomeDropped {
		t.Fatalf("events: %+v, %v", events, err)
	}
}

// On an API source the opposite holds: losing an item silently is what makes an
// integration impossible to debug.
func TestOnAQuietSourceWhatMatchedNothingGoesToTheInbox(t *testing.T) {
	entry := phoneSource()
	entry.Noisy = false
	m, board, _ := testManager(t, entry)

	res, err := m.Deliver(context.Background(), "телефон",
		[]Item{{ExternalID: "n3", Title: "Счёт за свет"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || len(board.boardIDs()) != 1 || board.boardIDs()[0] != "inbox" {
		t.Fatalf("it should have gone to the inbox: %+v, boards: %+v", res, board.boardIDs())
	}
	// The log tells a card no rule claimed from one a rule made.
	events, err := m.Events("телефон", 10)
	if err != nil || len(events) != 1 || events[0].Outcome != OutcomeInbox {
		t.Fatalf("events: %+v, %v", events, err)
	}
}

// An item lost to a failed write has to come back, so nothing is recorded until
// the card exists.
func TestAnItemIsNotForgottenWhenTheBoardRefusesIt(t *testing.T) {
	m, board, _ := testManager(t, phoneSource())
	board.refuseOnce(errors.New("доска недоступна"))
	ctx := context.Background()

	res, err := m.Deliver(ctx, "телефон", []Item{deliveryItem()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 {
		t.Fatalf("result: %+v", res)
	}

	res, err = m.Deliver(ctx, "телефон", []Item{deliveryItem()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || len(board.cards()) != 1 {
		t.Fatalf("the item should have come back: %+v, %+v", res, board.cards())
	}
}

func TestASourceThatIsOffAcceptsNothing(t *testing.T) {
	entry := phoneSource()
	entry.Enabled = false
	m, _, _ := testManager(t, entry)

	if _, err := m.Deliver(context.Background(), "телефон", []Item{deliveryItem()}); err == nil {
		t.Fatal("a disabled source must refuse, not silently accept")
	}
	if _, err := m.Deliver(context.Background(), "нет такого", nil); err == nil {
		t.Fatal("an unknown source must be refused")
	}
}

func TestRemovingASourceForgetsWhatItBrought(t *testing.T) {
	m, board, store := testManager(t, phoneSource())
	ctx := context.Background()
	if _, err := m.Deliver(ctx, "телефон", []Item{deliveryItem()}); err != nil {
		t.Fatal(err)
	}

	if err := m.RemoveSource("телефон"); err != nil {
		t.Fatal(err)
	}
	if events, err := store.Events("телефон", 10); err != nil || len(events) != 0 {
		t.Fatalf("events survived removal: %+v, %v", events, err)
	}

	// A source created again under the same name must start clean, not ignore
	// everything it brings as already seen.
	if _, err := m.AddSource(phoneSource()); err != nil {
		t.Fatal(err)
	}
	res, err := m.Deliver(ctx, "телефон", []Item{deliveryItem()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || len(board.cards()) != 2 {
		t.Fatalf("result: %+v, created: %+v", res, board.cards())
	}
}

// A batch is one poll, and the inbox does not move halfway through it. Asking
// once is a board read per card that a source bringing fifty cards no longer
// does.
func TestABatchAsksTheBoardOnce(t *testing.T) {
	entry := phoneSource()
	entry.Noisy = false
	entry.Rules = nil
	m, board, _ := testManager(t, entry)

	res, err := m.Deliver(context.Background(), "телефон", []Item{
		{ExternalID: "n1", Title: "Первое"},
		{ExternalID: "n2", Title: "Второе"},
		{ExternalID: "n3", Title: "Третье"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 3 {
		t.Fatalf("result: %+v", res)
	}
	if got := board.inboxAsks(); got != 1 {
		t.Fatalf("the inbox should be asked for once per batch, asked %d times", got)
	}
}

// The link travels as the card's own field rather than as a property named in
// this package: which property holds it is the board's answer, and a name here
// would have obliged every board to speak one language.
func TestTheLinkTravelsBesideTheCardRatherThanAsANamedProperty(t *testing.T) {
	m, board, _ := testManager(t, phoneSource())

	if _, err := m.Deliver(context.Background(), "телефон", []Item{deliveryItem()}); err != nil {
		t.Fatal(err)
	}
	if board.cards()[0].URL != "https://example.com/1" {
		t.Fatalf("card: %+v", board.cards()[0])
	}
}

// fakeBoardItems is the board asked what a source has already brought it.
type fakeBoardItems struct {
	cards map[string]string // source + "\x00" + externalID → card id
	// version is what the board says the item was at when it was brought.
	version string
	fail    error
}

func (f *fakeBoardItems) CardBySourceItem(_ context.Context, _, source, externalID string) (string, string, bool, error) {
	if f.fail != nil {
		return "", "", false, f.fail
	}
	cardID, ok := f.cards[source+"\x00"+externalID]
	return cardID, f.version, ok, nil
}

// A board carried here from another machine already holds the cards a source
// made there, and this machine has never heard of the items. Asking the board
// is what stops the next poll turning every one of them into a second card.
func TestAnItemTheBoardAlreadyHoldsIsNotBroughtAgain(t *testing.T) {
	m, board, store := testManager(t, phoneSource())
	m.SetBoardItems(&fakeBoardItems{
		cards:   map[string]string{"телефон\x00n1": "cardFromElsewhere"},
		version: "v1",
	})

	res, err := m.Deliver(context.Background(), "телефон", []Item{deliveryItem()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 0 || res.Skipped != 1 {
		t.Fatalf("result: %+v", res)
	}
	if len(board.cards()) != 0 {
		t.Fatalf("a second card was made for an item the board already holds: %+v", board.cards())
	}

	// The board is asked once: the answer is written into this machine's own
	// table, so the next poll is the fast path again.
	if _, cardID, err := store.StateOf("телефон", "n1", "v1"); err != nil || cardID != "cardFromElsewhere" {
		t.Errorf("the board's answer was not remembered: %q, %v", cardID, err)
	}
}

// The same item at a newer version is not a new card either — it is a comment
// on the card the board already has.
func TestAChangedItemTheBoardHoldsIsCommentedOn(t *testing.T) {
	m, board, _ := testManager(t, phoneSource())
	m.SetBoardItems(&fakeBoardItems{
		cards:   map[string]string{"телефон\x00n1": "cardFromElsewhere"},
		version: "v0",
	})

	res, err := m.Deliver(context.Background(), "телефон", []Item{deliveryItem()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 0 || res.Commented != 1 {
		t.Fatalf("result: %+v", res)
	}
	if len(board.comments) != 1 {
		t.Fatalf("comments: %+v", board.comments)
	}
}

// A board that cannot be asked is not a board with nothing on it: the item is
// left for the next poll rather than turned into a card that may be a
// duplicate.
func TestAnItemIsLeftAloneWhileTheBoardCannotBeAsked(t *testing.T) {
	m, board, _ := testManager(t, phoneSource())
	m.SetBoardItems(&fakeBoardItems{fail: errors.New("доска недоступна")})

	res, err := m.Deliver(context.Background(), "телефон", []Item{deliveryItem()})
	if err != nil {
		t.Fatal(err)
	}
	// Counted as a failure, which is what leaves it to be tried again — one
	// item that could not be decided must not stop the rest of the batch.
	if res.Failed != 1 || res.Created != 0 {
		t.Fatalf("result: %+v", res)
	}
	if len(board.cards()) != 0 {
		t.Fatalf("a card was made anyway: %+v", board.cards())
	}
}

// A card that lands in the inbox is announced — that is how the inbox sorter
// hears of it — and a card the share dialog put on a board somebody picked is
// not: it has already arrived where it belongs.
func TestACardInTheInboxIsAnnouncedAndAPickedBoardIsNot(t *testing.T) {
	m, _, _ := testManager(t, phoneSource())
	var heard []string
	m.SetArrivals(func(cardID string) { heard = append(heard, cardID) })

	if _, err := m.Deliver(context.Background(), "телефон", []Item{deliveryItem()}); err != nil {
		t.Fatal(err)
	}
	if len(heard) != 1 {
		t.Fatalf("the inbox card was announced %d times", len(heard))
	}

	share, err := m.EnsureSource(ShareSource())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Deliver(context.Background(), share.Name,
		[]Item{ShareItem("work", "Статья", "https://example.com/a", "")}); err != nil {
		t.Fatal(err)
	}
	if len(heard) != 1 {
		t.Fatalf("a card on a picked board was announced: %v", heard)
	}
}
