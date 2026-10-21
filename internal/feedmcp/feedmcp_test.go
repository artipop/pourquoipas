package feedmcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/artipop/pourquoipas/internal/boardmodel"
)

type fakeLists struct{}

func (fakeLists) Boards(context.Context) ([]boardmodel.BoardSummary, error) {
	return []boardmodel.BoardSummary{
		{ID: "b1", Title: "Работа", Columns: []boardmodel.ColumnOption{{Value: "Входящие"}, {Value: "Сегодня"}, {Value: "Готово"}}},
		{ID: "b2", Title: "Дом"},
	}, nil
}

func (fakeLists) BoardCards(_ context.Context, boardID string) ([]boardmodel.CardSummary, error) {
	if boardID != "b1" {
		return nil, nil
	}
	return []boardmodel.CardSummary{
		{ID: "c1", Title: "Починить логин", Column: "Сегодня", UpdateAt: 1700, Properties: map[string]string{"Статус": "Сегодня", "Проект": "xxvi"}},
		{ID: "c2", Title: "Позвонить", Column: "Входящие", UpdateAt: 1600},
		{ID: "c3", Title: "Обновить зависимости", Column: "сегодня", UpdateAt: 1500},
	}, nil
}

// A column is found whatever the case it was typed in, and only its cards come
// back, each versioned by its last change.
func TestAColumnListsItsCards(t *testing.T) {
	cards, err := ColumnCards(context.Background(), fakeLists{}, "работа", "Сегодня", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || cards[0].ID != "c1" || cards[1].ID != "c3" {
		t.Fatalf("cards: %+v", cards)
	}
	if cards[0].Updated != "1700" || cards[0].Board != "Работа" || cards[0].Properties["Проект"] != "xxvi" {
		t.Fatalf("card: %+v", cards[0])
	}
	// No stable address, no link: a desktop's port is gone by the next launch.
	if cards[0].URL != "" {
		t.Fatalf("a link to a random port: %q", cards[0].URL)
	}
}

// A name typed into another application's form is answered with the names
// there are.
func TestAMissingBoardOrColumnSaysWhatThereIs(t *testing.T) {
	_, err := ColumnCards(context.Background(), fakeLists{}, "Работа", "Завтра", "")
	if err == nil || !strings.Contains(err.Error(), "Входящие, Сегодня, Готово") {
		t.Fatalf("err: %v", err)
	}
	_, err = ColumnCards(context.Background(), fakeLists{}, "Хобби", "", "")
	if err == nil || !strings.Contains(err.Error(), "Работа, Дом") {
		t.Fatalf("err: %v", err)
	}
}

func serve(t *testing.T, token string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(NewHandler(fakeLists{}, func(got string) bool { return token != "" && got == token }))
	t.Cleanup(srv.Close)
	return srv
}

// Without the token a person issued, the feed says nothing at all.
func TestTheFeedWantsItsToken(t *testing.T) {
	srv := serve(t, "t0k")
	resp, err := http.Post(srv.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

// The bridge end to end: stdio in, the feed over HTTP behind it, and the links
// made against the address given — because it was given, it is stable.
func TestTheBridgeReadsTheFeed(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(Path, NewHandler(fakeLists{}, func(got string) bool { return got == "t0k" }))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv(EnvToken, "t0k")
	t.Setenv(EnvURL, srv.URL+"/")
	endpoint, base, _, err := Endpoint(t.TempDir())
	if err != nil || base != srv.URL || endpoint != srv.URL+Path {
		t.Fatalf("endpoint %q, base %q, err %v", endpoint, base, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- ServeStdio(ctx, t.TempDir(), inR, outW) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: outR, Writer: inW}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "list_column_cards", Arguments: map[string]any{"board": "Работа", "column": "Входящие"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("refused: %+v", res.Content[0])
	}
	var out struct {
		Cards []Card `json:"cards"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Cards) != 1 || out.Cards[0].URL != srv.URL+"/team/0/b1/0/c2" {
		t.Fatalf("cards: %+v", out.Cards)
	}
}

// A desktop app not running is a sentence, not a connection error.
func TestWithoutTheAppTheBridgeSaysSo(t *testing.T) {
	t.Setenv(EnvURL, "")
	if _, _, _, err := Endpoint(t.TempDir()); err == nil || !strings.Contains(err.Error(), "не запущен") {
		t.Fatalf("err: %v", err)
	}
	dir := t.TempDir()
	if err := WriteHandoff(dir, "http://127.0.0.1:5555", "s3cret"); err != nil {
		t.Fatal(err)
	}
	endpoint, base, secret, err := Endpoint(dir)
	if err != nil || endpoint != "http://127.0.0.1:5555"+Path || base != "" || secret != "s3cret" {
		t.Fatalf("endpoint %q, base %q, secret %q, err %v", endpoint, base, secret, err)
	}
}

// A token that does not open the feed is said as that, with where to get one.
func TestTheBridgeSaysTheTokenIsWrong(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(Path, NewHandler(fakeLists{}, func(got string) bool { return got == "t0k" }))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv(EnvToken, "wrong")
	t.Setenv(EnvURL, srv.URL)

	inR, _ := io.Pipe()
	_, outW := io.Pipe()
	err := ServeStdio(context.Background(), t.TempDir(), inR, outW)
	if err == nil || !strings.Contains(err.Error(), "токен фида не подходит") {
		t.Fatalf("err: %v", err)
	}
}

// A form in another application offers boards and, for the one chosen, its
// columns, by the names the board shows.
func TestBoardsAndColumnsAreListed(t *testing.T) {
	boards, err := Boards(context.Background(), fakeLists{})
	if err != nil || len(boards) != 2 || boards[0].Title != "Работа" || boards[1].ID != "b2" {
		t.Fatalf("boards: %+v, %v", boards, err)
	}
	columns, err := Columns(context.Background(), fakeLists{}, "работа")
	if err != nil || len(columns) != 3 || columns[1].Name != "Сегодня" {
		t.Fatalf("columns: %+v, %v", columns, err)
	}
	// A board with no columns of its own is a list, not an error.
	if columns, err = Columns(context.Background(), fakeLists{}, "Дом"); err != nil || len(columns) != 0 {
		t.Fatalf("columns of a board without any: %+v, %v", columns, err)
	}
	if _, err = Columns(context.Background(), fakeLists{}, "Хобби"); err == nil || !strings.Contains(err.Error(), "Работа, Дом") {
		t.Fatalf("err: %v", err)
	}
}

// The same lists reach the caller over MCP, as JSON with the key the manifest
// reads them at.
func TestTheListsAreTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	serverT, clientT := mcp.NewInMemoryTransports()
	srv := NewServer(fakeLists{}, "")
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	text := func(name string, args map[string]any) string {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %+v", name, err, res)
		}
		return res.Content[0].(*mcp.TextContent).Text
	}
	if got := text("list_boards", nil); !strings.Contains(got, `"boards":[{"id":"b1","title":"Работа"}`) {
		t.Fatalf("boards: %s", got)
	}
	if got := text("list_columns", map[string]any{"board": "Работа"}); got != `{"columns":[{"name":"Входящие"},{"name":"Сегодня"},{"name":"Готово"}]}` {
		t.Fatalf("columns: %s", got)
	}
}

// On the same machine nobody carries a token: the bridge takes the one the app
// left beside its address, and a token that was issued still wins over it.
func TestTheBridgeTakesTheSecretTheAppLeft(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(Path, NewHandler(fakeLists{}, func(got string) bool { return got == "run-secret" }))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	dir := t.TempDir()
	if err := WriteHandoff(dir, srv.URL, "run-secret"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvToken, "")
	t.Setenv(EnvURL, "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	go func() { _ = ServeStdio(ctx, dir, inR, outW) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: outR, Writer: inW}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_boards"})
	if err != nil || res.IsError {
		t.Fatalf("list_boards: %v %+v", err, res)
	}

	// A secret left by a run that is over opens nothing, and says what to do.
	if err := WriteHandoff(dir, srv.URL, "last-weeks"); err != nil {
		t.Fatal(err)
	}
	inR2, _ := io.Pipe()
	_, outW2 := io.Pipe()
	err = ServeStdio(context.Background(), dir, inR2, outW2)
	if err == nil || !strings.Contains(err.Error(), "перезапустите Pourquoi Pas?") {
		t.Fatalf("err: %v", err)
	}
}
