package acp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artipop/pourquoipas/internal/modelapi"
)

// scriptedModel is an Anthropic endpoint that answers each request with the
// next reply of a script and keeps what it was sent.
type scriptedModel struct {
	mu       sync.Mutex
	replies  []string
	requests []map[string]any
}

func (s *scriptedModel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.requests = append(s.requests, body)
	if len(s.replies) == 0 {
		http.Error(w, "the script is over", http.StatusInternalServerError)
		return
	}
	next := s.replies[0]
	s.replies = s.replies[1:]
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = io.WriteString(w, next)
}

func (s *scriptedModel) sent() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.requests...)
}

type call struct{ name, input string }

// callIDs numbers tool calls across a whole script, so each result can be
// told apart.
var (
	callIDsMu sync.Mutex
	callIDs   int
)

func nextCallID() string {
	callIDsMu.Lock()
	defer callIDsMu.Unlock()
	callIDs++
	return fmt.Sprintf("tu%d", callIDs)
}

// modelReply is one streamed Messages API response: text, then tool calls.
func modelReply(text string, calls ...call) string {
	var b strings.Builder
	event := func(data string) {
		var probe struct{ Type string }
		_ = json.Unmarshal([]byte(data), &probe)
		fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", probe.Type, data)
	}
	event(`{"type":"message_start","message":{"id":"m","type":"message","role":"assistant","model":"claude-test","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":5,"output_tokens":1}}}`)
	index := 0
	if text != "" {
		event(fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"text","text":""}}`, index))
		quoted, _ := json.Marshal(text)
		event(fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"text_delta","text":%s}}`, index, quoted))
		event(fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, index))
		index++
	}
	for _, c := range calls {
		event(fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"tool_use","id":%q,"name":%q,"input":{}}}`, index, nextCallID(), c.name))
		quoted, _ := json.Marshal(c.input)
		event(fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"input_json_delta","partial_json":%s}}`, index, quoted))
		event(fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, index))
		index++
	}
	stop := "end_turn"
	if len(calls) > 0 {
		stop = "tool_use"
	}
	event(fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":%q,"stop_sequence":null},"usage":{"output_tokens":7}}`, stop))
	event(`{"type":"message_stop"}`)
	return b.String()
}

func modelAgent() AgentEntry {
	return AgentEntry{ID: newID(), Name: testAgentName, Kind: AgentKindModel, Provider: modelapi.ProviderAnthropic, Model: "claude-test"}
}

func useModel(m *Manager, url string) {
	m.SetModelConnections(func(owner, provider string) (modelapi.Connection, error) {
		return modelapi.Connection{Key: "sk-test", BaseURL: url}, nil
	})
}

// toolResults is every tool_result the model was handed back, by call id.
func toolResults(requests []map[string]any) map[string]string {
	out := map[string]string{}
	for _, req := range requests {
		msgs, _ := req["messages"].([]any)
		for _, msg := range msgs {
			content, _ := msg.(map[string]any)["content"].([]any)
			for _, block := range content {
				b, _ := block.(map[string]any)
				if b["type"] != "tool_result" {
					continue
				}
				text := ""
				switch c := b["content"].(type) {
				case string:
					text = c
				case []any:
					for _, part := range c {
						if p, ok := part.(map[string]any); ok {
							text += fmt.Sprint(p["text"])
						}
					}
				}
				out[fmt.Sprint(b["tool_use_id"])] = text
			}
		}
	}
	return out
}

// A model works a card the way a CLI does: in the card's own copy of the
// folder, asking the person what only they can decide, running a command only
// once it is allowed, and ending the stage through the board's own tools —
// the same tools, the same question on the card, the same comment.
func TestAModelWorksACardInItsFolderAndEndsItThroughTheBoardsTools(t *testing.T) {
	model := &scriptedModel{replies: []string{
		modelReply("Let me ask first.", call{"ask_person", `{"question":"Which greeting?","options":["hi","hello"]}`}),
		modelReply("", call{"write_file", `{"path":"greeting.txt","content":"hello\n"}`}),
		modelReply("", call{"run_command", `{"command":"printf done: && cat greeting.txt"}`}),
		modelReply("", call{"comment_card", `{"text":"Wrote the greeting."}`}, call{"finish_work", `{"done":true,"summary":"greeting written"}`}),
		modelReply("All done."),
	}}
	srv := httptest.NewServer(model)
	defer srv.Close()

	m, writer, events, _ := testManager(t, fakeClaudeHappy, func(c *Config) {
		a := modelAgent()
		// Narrower than the default, which lets every command through: the
		// model's commands answer to the agent's policy as a CLI's do.
		a.AutoAllowTools = []string{"Bash(git status*)"}
		c.Agents = []AgentEntry{a}
	})
	useModel(m, srv.URL)

	events.ch <- moveEvent("cardM", "opt-backlog", "opt-agent")

	// The model asks; the card shows it as any agent's question.
	waitFor(t, 15*time.Second, "the model to ask", func() bool { return len(m.Questions()) == 1 })
	q := m.Questions()[0]
	if q.Kind != QuestionForm || q.CardID != "cardM" || q.Text != "Which greeting?" || len(q.Options) != 2 {
		t.Fatalf("question %+v", q)
	}
	if err := m.AnswerQuestion(q.ID, Answer{OptionID: q.Options[1].ID}); err != nil {
		t.Fatal(err)
	}

	// The command is not in the policy, so it is a permission question.
	waitFor(t, 15*time.Second, "the command to be asked about", func() bool {
		qs := m.Questions()
		return len(qs) == 1 && qs[0].Kind == QuestionPermission
	})
	perm := m.Questions()[0]
	if !strings.Contains(perm.Text, "printf done") {
		t.Errorf("permission question %q does not name the command", perm.Text)
	}
	if err := m.AnswerQuestion(perm.ID, Answer{OptionID: "allow_once"}); err != nil {
		t.Fatal(err)
	}

	waitFor(t, 15*time.Second, "session done", func() bool {
		sessions, _, err := m.store.SessionsForCard("cardM")
		return err == nil && len(sessions) == 1 && sessions[0].Status == StatusDone
	})

	sessions, _, _ := m.store.SessionsForCard("cardM")
	// The work is in the card's own copy, on the card's branch, as a CLI's is.
	if sessions[0].Branch == "" {
		t.Error("the model's session has no branch")
	}
	results := toolResults(model.sent())
	all := ""
	for _, r := range results {
		all += r + "\n"
	}
	if !strings.Contains(all, "The person chose: hello") {
		t.Errorf("the answer did not reach the model: %v", results)
	}
	// The command ran in the card's copy and read the file written there.
	if !strings.Contains(all, "done:hello") {
		t.Errorf("the command's output did not come back: %v", results)
	}

	// The comment is the model's own, through comment_card, and only it.
	comments := writer.cardComments("cardM")
	if len(comments) != 1 || comments[0] != "Wrote the greeting." {
		t.Errorf("comments = %q", comments)
	}

	// Every request carried the same system text and tool list: the prefix
	// a provider caches.
	sent := model.sent()
	first, _ := json.Marshal(sent[0]["tools"])
	last, _ := json.Marshal(sent[len(sent)-1]["tools"])
	if string(first) != string(last) {
		t.Error("the tool list changed between requests of one turn")
	}
	names := string(first)
	for _, tool := range []string{"comment_card", "finish_work", "move_card", "read_file", "run_command", "ask_person"} {
		if !strings.Contains(names, `"`+tool+`"`) {
			t.Errorf("the model was not given %s", tool)
		}
	}
	if sent[0]["system"] == nil {
		t.Error("no system text")
	}
	task := fmt.Sprint(sent[0]["messages"])
	if !strings.Contains(task, "Test task") || !strings.Contains(task, "finish_work") {
		t.Errorf("the task lacks the card or the brief: %s", task)
	}
}

// A hosted server has no folders: the model gets the board and nothing of a
// machine, and what it produces is what it says on the card.
func TestOnAHostedServerAModelWorksACardWithNoFolder(t *testing.T) {
	model := &scriptedModel{replies: []string{
		modelReply("", call{"comment_card", `{"text":"Here is the plan."}`}, call{"finish_work", `{"done":true,"summary":"planned"}`}),
		modelReply("Done."),
	}}
	srv := httptest.NewServer(model)
	defer srv.Close()

	m, writer, events, project := testManager(t, fakeClaudeHappy, func(c *Config) {
		c.Agents = []AgentEntry{modelAgent()}
	})
	m.SetHosted(true)
	useModel(m, srv.URL)

	events.ch <- moveEvent("cardH", "opt-backlog", "opt-agent")
	waitFor(t, 15*time.Second, "session done", func() bool {
		sessions, _, err := m.store.SessionsForCard("cardH")
		return err == nil && len(sessions) == 1 && sessions[0].Status == StatusDone
	})
	sessions, _, _ := m.store.SessionsForCard("cardH")
	if sessions[0].Cwd != "" || sessions[0].Branch != "" {
		t.Errorf("a hosted session stood somewhere: cwd %q, branch %q", sessions[0].Cwd, sessions[0].Branch)
	}
	if got := writer.cardComments("cardH"); len(got) != 1 || got[0] != "Here is the plan." {
		t.Errorf("comments = %q", got)
	}
	tools, _ := json.Marshal(model.sent()[0]["tools"])
	for _, tool := range []string{"read_file", "write_file", "run_command"} {
		if strings.Contains(string(tools), `"`+tool+`"`) {
			t.Errorf("a hosted model was given %s", tool)
		}
	}
	if !strings.Contains(fmt.Sprint(model.sent()[0]["messages"]), "There are no files") {
		t.Error("the task does not say there are no files")
	}
	// Nothing was made in the folder the fixtures registered.
	entries, _ := os.ReadDir(filepath.Join(project, ".git", "worktrees"))
	if len(entries) != 0 {
		t.Errorf("a hosted session made a worktree: %v", entries)
	}
}

// A model with no key is said on the card, as what to do — not discovered as
// a failed request a minute later.
func TestAModelWithNoKeyStallsTheCardSayingSo(t *testing.T) {
	m, _, events, _ := testManager(t, fakeClaudeHappy, func(c *Config) {
		c.Agents = []AgentEntry{modelAgent()}
	})
	m.SetModelConnections(func(owner, provider string) (modelapi.Connection, error) {
		return modelapi.Connection{}, modelapi.ErrNoKey
	})
	events.ch <- moveEvent("cardK", "opt-backlog", "opt-agent")
	waitFor(t, 10*time.Second, "the stall", func() bool {
		stall, ok, err := m.store.Stall("cardK")
		return err == nil && ok && stall.Reason.Code == "ai.no-key"
	})
	if sessions, _, _ := m.store.SessionsForCard("cardK"); len(sessions) != 0 {
		t.Errorf("a session was started: %v", sessions)
	}
}

// A hosted server runs no program of anybody's: a CLI agent assigned there
// stalls the card with what to do instead.
func TestAHostedServerRefusesACLIAgent(t *testing.T) {
	m, _, events, _ := testManager(t, fakeClaudeHappy, nil)
	m.SetHosted(true)
	events.ch <- moveEvent("cardC", "opt-backlog", "opt-agent")
	waitFor(t, 10*time.Second, "the stall", func() bool {
		stall, ok, err := m.store.Stall("cardC")
		return err == nil && ok && stall.Reason.Code == "agents.cli-not-here"
	})
}

// A model agent's own remote server is accepted; a CLI agent's is not, since
// session/new carries stdio servers alone.
func TestOnlyAModelAgentMayCarryARemoteMCPServer(t *testing.T) {
	remote := MCPServerSet{"docs": {URL: "https://example.test/mcp"}}
	a := modelAgent()
	a.MCPServers = remote
	got, err := validateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	if got.MCPServers["docs"].Type != "http" {
		t.Errorf("type = %q", got.MCPServers["docs"].Type)
	}
	cli := testAgent([]string{"agent"})
	cli.MCPServers = remote
	if _, err := validateAgent(cli); err == nil {
		t.Error("a CLI agent was given a remote server")
	}
}
