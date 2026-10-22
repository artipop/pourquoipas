package modelapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeAPI answers each request with the next canned SSE body and keeps what it
// was sent.
type fakeAPI struct {
	mu       sync.Mutex
	streams  []string
	requests []map[string]any
	headers  []http.Header
	models   string
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.headers = append(f.headers, r.Header.Clone())
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models") {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, f.models)
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.requests = append(f.requests, body)
	if len(f.streams) == 0 {
		http.Error(w, "no more replies", http.StatusInternalServerError)
		return
	}
	next := f.streams[0]
	f.streams = f.streams[1:]
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = io.WriteString(w, next)
}

func sse(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		var probe struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(e), &probe)
		if probe.Type != "" {
			fmt.Fprintf(&b, "event: %s\n", probe.Type)
		}
		fmt.Fprintf(&b, "data: %s\n\n", e)
	}
	return b.String()
}

var anthropicToolTurn = sse(
	`{"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"claude-test","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":12,"output_tokens":1,"cache_read_input_tokens":7}}}`,
	`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
	`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"look first"}}`,
	`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-1"}}`,
	`{"type":"content_block_stop","index":0}`,
	`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
	`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Reading "}}`,
	`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"the board."}}`,
	`{"type":"content_block_stop","index":1}`,
	`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"tu1","name":"list_columns","input":{}}}`,
	`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"column\":"}}`,
	`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"Done\"}"}}`,
	`{"type":"content_block_stop","index":2}`,
	`{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":20}}`,
	`{"type":"message_stop"}`,
)

var anthropicLastTurn = sse(
	`{"type":"message_start","message":{"id":"m2","type":"message","role":"assistant","model":"claude-test","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":30,"output_tokens":1}}}`,
	`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
	`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Moved."}}`,
	`{"type":"content_block_stop","index":0}`,
	`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}`,
	`{"type":"message_stop"}`,
)

// The loop a session runs — ask, call a tool, hand the result back — has to
// reach the model with the whole history intact, the thinking block included:
// a reasoning model refuses a history with its own blocks edited out.
func TestAnAnthropicConversationCallsAToolAndCarriesOnWithItsResult(t *testing.T) {
	api := &fakeAPI{streams: []string{anthropicToolTurn, anthropicLastTurn}}
	srv := httptest.NewServer(api)
	defer srv.Close()

	chat, err := NewChat(ChatConfig{
		Connection: Connection{Provider: ProviderAnthropic, Key: "sk-test", BaseURL: srv.URL},
		Model:      "claude-test",
		System:     "You work a card.",
		Effort:     "high",
		Tools: []Tool{{Name: "list_columns", Description: "The board's columns",
			Schema: map[string]any{"type": "object", "properties": map[string]any{"column": map[string]any{"type": "string"}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var streamed strings.Builder
	reply, err := chat.Say(context.Background(), "Do the card.", func(s string) { streamed.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if reply.Stop != StopTools || len(reply.Calls) != 1 || reply.Calls[0].Name != "list_columns" {
		t.Fatalf("reply = %+v", reply)
	}
	if got := string(reply.Calls[0].Input); got != `{"column":"Done"}` {
		t.Errorf("input = %s", got)
	}
	if streamed.String() != "Reading the board." || reply.Text != "Reading the board." {
		t.Errorf("streamed %q, text %q", streamed.String(), reply.Text)
	}
	if reply.Usage.Input != 12 || reply.Usage.Output != 20 || reply.Usage.CacheRead != 7 {
		t.Errorf("usage = %+v", reply.Usage)
	}

	reply, err = chat.Answer(context.Background(), []ToolResult{{CallID: "tu1", Content: "Done, Todo"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Stop != StopEnd || reply.Text != "Moved." {
		t.Fatalf("reply = %+v", reply)
	}

	if got := api.headers[0].Get("X-Api-Key"); got != "sk-test" {
		t.Errorf("key header = %q", got)
	}
	first := api.requests[0]
	if first["model"] != "claude-test" || first["cache_control"] == nil {
		t.Errorf("first request = %v", first)
	}
	if oc, _ := first["output_config"].(map[string]any); oc["effort"] != "high" {
		t.Errorf("output_config = %v", first["output_config"])
	}
	second := api.requests[1]["messages"].([]any)
	if len(second) != 3 {
		t.Fatalf("second request carries %d messages, want 3", len(second))
	}
	assistant := second[1].(map[string]any)["content"].([]any)
	if kind := assistant[0].(map[string]any)["type"]; kind != "thinking" {
		t.Errorf("the thinking block did not go back first: %v", assistant[0])
	}
	if sig := assistant[0].(map[string]any)["signature"]; sig != "sig-1" {
		t.Errorf("signature = %v", sig)
	}
	result := second[2].(map[string]any)["content"].([]any)[0].(map[string]any)
	if result["type"] != "tool_result" || result["tool_use_id"] != "tu1" {
		t.Errorf("tool result = %v", result)
	}
}

func chunk(delta string, finish string) string {
	fin := "null"
	if finish != "" {
		fin = `"` + finish + `"`
	}
	return `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-test","choices":[{"index":0,"delta":` + delta + `,"finish_reason":` + fin + `}]}`
}

func openAISSE(chunks ...string) string {
	var b strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&b, "data: %s\n\n", c)
	}
	b.WriteString("data: [DONE]\n\n")
	return b.String()
}

var openAIToolTurn = openAISSE(
	chunk(`{"role":"assistant","content":"Look"}`, ""),
	chunk(`{"content":"ing."}`, ""),
	chunk(`{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"list_columns","arguments":""}}]}`, ""),
	chunk(`{"tool_calls":[{"index":0,"function":{"arguments":"{\"column\":"}}]}`, ""),
	chunk(`{"tool_calls":[{"index":0,"function":{"arguments":"\"Done\"}"}}]}`, ""),
	chunk(`{}`, "tool_calls"),
	`{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-test","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13,"prompt_tokens_details":{"cached_tokens":2}}}`,
)

var openAILastTurn = openAISSE(
	chunk(`{"role":"assistant","content":"Moved."}`, ""),
	chunk(`{}`, "stop"),
)

// The same loop over chat completions, which is what every OpenAI-compatible
// endpoint speaks: the call arrives in pieces, and the result goes back as a
// tool message under the call's id.
func TestAnOpenAICompatibleConversationCallsAToolAndCarriesOnWithItsResult(t *testing.T) {
	api := &fakeAPI{streams: []string{openAIToolTurn, openAILastTurn}}
	srv := httptest.NewServer(api)
	defer srv.Close()

	chat, err := NewChat(ChatConfig{
		Connection: Connection{Provider: ProviderOpenAI, Key: "sk-o", BaseURL: srv.URL + "/v1"},
		Model:      "gpt-test",
		System:     "You work a card.",
		Tools:      []Tool{{Name: "list_columns", Schema: map[string]any{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var streamed strings.Builder
	reply, err := chat.Say(context.Background(), "Do the card.", func(s string) { streamed.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if reply.Stop != StopTools || len(reply.Calls) != 1 || string(reply.Calls[0].Input) != `{"column":"Done"}` || reply.Calls[0].ID != "call_1" {
		t.Fatalf("reply = %+v", reply)
	}
	if streamed.String() != "Looking." {
		t.Errorf("streamed %q", streamed.String())
	}
	if reply.Usage.Input != 9 || reply.Usage.CacheRead != 2 {
		t.Errorf("usage = %+v", reply.Usage)
	}
	reply, err = chat.Answer(context.Background(), []ToolResult{{CallID: "call_1", Content: "no such column", IsError: true}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Stop != StopEnd || reply.Text != "Moved." {
		t.Fatalf("reply = %+v", reply)
	}

	if got := api.headers[0].Get("Authorization"); got != "Bearer sk-o" {
		t.Errorf("auth header = %q", got)
	}
	tools := api.requests[0]["tools"].([]any)
	params := tools[0].(map[string]any)["function"].(map[string]any)["parameters"].(map[string]any)
	if params["type"] != "object" {
		t.Errorf("a schema with no type was sent as %v", params)
	}
	msgs := api.requests[1]["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("second request carries %d messages, want system, user, assistant, tool", len(msgs))
	}
	tool := msgs[3].(map[string]any)
	if tool["role"] != "tool" || tool["tool_call_id"] != "call_1" || !strings.HasPrefix(fmt.Sprint(tool["content"]), "Error: ") {
		t.Errorf("tool message = %v", tool)
	}
}

// A key is checked by listing the models it opens: that is the cheapest call
// both dialects have, and it is the list the agent form offers anyway.
func TestModelsAreListedWithTheKeyTheyWereAskedWith(t *testing.T) {
	api := &fakeAPI{models: `{"data":[{"id":"claude-b","display_name":"B","type":"model","created_at":"2026-01-01T00:00:00Z"},{"id":"claude-a","display_name":"A","type":"model","created_at":"2025-01-01T00:00:00Z"}],"has_more":false,"first_id":"claude-b","last_id":"claude-a"}`}
	srv := httptest.NewServer(api)
	defer srv.Close()
	models, err := Models(context.Background(), Connection{Provider: ProviderAnthropic, Key: "k", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ID != "claude-b" || models[0].Name != "B" {
		t.Errorf("models = %+v", models)
	}

	if _, err := Models(context.Background(), Connection{Provider: ProviderAnthropic}); err != ErrNoKey {
		t.Errorf("no key: err = %v", err)
	}
}

// The agent a first key makes has to start on a model that exists for that
// key: the default where it is listed, a known name next, the first listed
// otherwise.
func TestANewAgentStartsOnAModelTheProviderListed(t *testing.T) {
	listed := []Model{{ID: "o3"}, {ID: "gpt-4.1"}, {ID: "gpt-5"}}
	if got := SuggestModel(ProviderOpenAI, listed); got != "gpt-5" {
		t.Errorf("openai: %q", got)
	}
	if got := SuggestModel(ProviderOpenAI, []Model{{ID: "qwen3:14b"}}); got != "qwen3:14b" {
		t.Errorf("a local server's only model: %q", got)
	}
	if got := SuggestModel(ProviderAnthropic, []Model{{ID: "claude-x"}, {ID: "claude-opus-5-5"}}); got != "claude-opus-5-5" {
		t.Errorf("anthropic: %q", got)
	}
}
