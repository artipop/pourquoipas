package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/boardmcp"
	"github.com/artipop/pourquoipas/internal/modelapi"
)

// An agent that is a model rather than a program: the loop an ACP adapter runs
// inside a CLI runs here instead — ask, call the tools it asked for, hand the
// results back, until it stops asking.
//
// What it is given is what a CLI is given, by the same means where there is
// one: the board's tools are the same MCP server (boardmcp), reached in memory
// instead of over the front door; the servers the agent carries are reached as
// an MCP client; a question to a person is Manager.ask. What a CLI brings of
// its own — files and a shell — is ours here (modeltools.go), and only where
// there is a machine to work on.

// ModelConnections answers where a model agent's requests go and with whose
// key: the agent's owner's, for the agent's provider. main supplies it, from
// the credential store.
type ModelConnections func(ownerID, provider string) (modelapi.Connection, error)

// SetModelConnections supplies the keys. Without it no model agent can start.
func (m *Manager) SetModelConnections(f ModelConnections) { m.modelConns = f }

// SetHosted says this build has no machine of the person's to work on: a
// server. No folders, no CLI agents, no process of a person's choosing.
func (m *Manager) SetHosted(hosted bool) { m.hosted = hosted }

// Hosted reports what SetHosted said.
func (m *Manager) Hosted() bool { return m.hosted }

func knownProvider(p string) bool {
	for _, known := range modelapi.Providers {
		if p == known {
			return true
		}
	}
	return false
}

// errNoKey is a model agent whose owner has not given its provider a key.
func errNoKey(provider string) error {
	return apperr.New("ai.no-key", "There is no API key for {provider}: add one in Settings → AI", "provider", provider)
}

// modelConnection resolves an agent's key, or says what is missing.
func (m *Manager) modelConnection(a AgentEntry) (modelapi.Connection, error) {
	if m.modelConns == nil {
		return modelapi.Connection{}, errNoKey(a.Provider)
	}
	conn, err := m.modelConns(a.OwnerID, a.Provider)
	if errors.Is(err, modelapi.ErrNoKey) || (err == nil && strings.TrimSpace(conn.Key) == "") {
		return modelapi.Connection{}, errNoKey(a.Provider)
	}
	if err != nil {
		return modelapi.Connection{}, err
	}
	conn.Provider = a.Provider
	return conn, nil
}

// maxToolRounds bounds one turn: a model that keeps calling tools without ever
// finishing is going round in circles, and every round is paid for.
const maxToolRounds = 60

// maxToolOutput is how much of one tool's answer goes back to the model.
const maxToolOutput = 60000

type modelTool struct {
	spec modelapi.Tool
	run  func(ctx context.Context, input json.RawMessage) (string, error)
}

type modelConversation struct {
	m       *Manager
	s       *Session
	chat    modelapi.Chat
	tools   map[string]modelTool
	printer *sessionClient

	closeOnce sync.Once
	closers   []func()
	gone      chan struct{}

	// spoke is set once this turn has written text, so the next reply of the
	// same turn starts on a line of its own.
	spoke bool
}

func (m *Manager) openModelConversation(ctx context.Context, s *Session) (conversation, error) {
	conn, err := m.modelConnection(s.Agent)
	if err != nil {
		return nil, err
	}
	c := &modelConversation{
		m: m, s: s,
		tools:   map[string]modelTool{},
		printer: &sessionClient{m: m, s: s},
		gone:    make(chan struct{}),
	}
	instructions, err := c.attachTools(ctx)
	if err != nil {
		c.close()
		return nil, err
	}
	names := make([]string, 0, len(c.tools))
	for name := range c.tools {
		names = append(names, name)
	}
	// Sorted, so the tool list — the head of every request — is the same
	// bytes each time and the provider can cache it.
	sort.Strings(names)
	specs := make([]modelapi.Tool, 0, len(names))
	for _, name := range names {
		specs = append(specs, c.tools[name].spec)
	}
	chat, err := modelapi.NewChat(modelapi.ChatConfig{
		Connection: conn,
		Model:      s.Agent.Model,
		System:     modelSystemPrompt(s, m.hosted, instructions),
		Tools:      specs,
		Effort:     s.Agent.Options["effort"],
	})
	if err != nil {
		c.close()
		return nil, err
	}
	c.chat = chat
	return c, nil
}

func (c *modelConversation) done() <-chan struct{} { return c.gone }

func (c *modelConversation) close() {
	c.closeOnce.Do(func() {
		for i := len(c.closers) - 1; i >= 0; i-- {
			c.closers[i]()
		}
		close(c.gone)
	})
}

// attachTools gathers what the model may call and returns the instructions
// the servers sent with their tools.
func (c *modelConversation) attachTools(ctx context.Context) (string, error) {
	var instructions []string
	s, m := c.s, c.m

	// The board's tools, under the grant a CLI would have been handed.
	if s.BoardID != "" {
		if token := m.GrantBoardTools(s.BoardID, s.CardID, s.ID); token != "" {
			s.mu.Lock()
			s.boardTools = token
			s.mu.Unlock()
			c.closers = append(c.closers, func() { m.RevokeBoardTools(token) })
			server := boardmcp.NewServer(m.BoardForGrant(token))
			clientEnd, serverEnd := mcp.NewInMemoryTransports()
			serverSession, err := server.Connect(ctx, serverEnd, nil)
			if err != nil {
				return "", fmt.Errorf("board tools: %w", err)
			}
			c.closers = append(c.closers, func() { _ = serverSession.Close() })
			text, err := c.attachMCP(ctx, "", clientEnd)
			if err != nil {
				return "", fmt.Errorf("board tools: %w", err)
			}
			instructions = append(instructions, text)
			s.PromptText = joinPrompts(s.PromptText, m.boardToolsBrief(s))
		}
	}

	// The servers this run was handed and the ones the agent carries.
	specs, err := sessionMCPServers(s, m.cfg)
	if err != nil {
		return "", err
	}
	for _, spec := range specs {
		transport, err := c.transportFor(spec)
		if err != nil {
			return "", err
		}
		text, err := c.attachMCP(ctx, spec.Name, transport)
		if err != nil {
			return "", fmt.Errorf("MCP server %s: %w", spec.Name, err)
		}
		instructions = append(instructions, text)
	}

	// Files and commands, where there is a machine and a folder to work in.
	if !m.hosted && s.Worktree.Path != "" {
		for _, t := range folderTools(c, !s.Planning) {
			c.tools[t.spec.Name] = t
		}
	}
	// Somebody to ask: a card, or a planning conversation on a board.
	if s.CardID != "" || (s.Planning && s.BoardID != "") {
		t := askPersonTool(c)
		c.tools[t.spec.Name] = t
	}
	return joinPrompts(instructions...), nil
}

// transportFor reaches one server. On a hosted server a process is started
// only when it is this program itself — one of ours; anything else a person
// named is reached over HTTP or not at all.
func (c *modelConversation) transportFor(spec mcpServerSpec) (mcp.Transport, error) {
	if spec.URL != "" {
		if spec.Type == "sse" {
			return &mcp.SSEClientTransport{Endpoint: spec.URL}, nil
		}
		return &mcp.StreamableClientTransport{Endpoint: spec.URL}, nil
	}
	if c.m.hosted && !isThisProgram(spec.Command) {
		return nil, apperr.New("mcp.no-processes-here", "The MCP server {name} is a program to start, and this server starts none: give its address (url) instead", "name", spec.Name)
	}
	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Env = os.Environ()
	for _, name := range sortedEnvNames(spec.Env) {
		cmd.Env = append(cmd.Env, name+"="+spec.Env[name])
	}
	if c.s.Worktree.Path != "" {
		cmd.Dir = c.s.Worktree.Path
	}
	return &mcp.CommandTransport{Command: cmd}, nil
}

func isThisProgram(command string) bool {
	self, err := os.Executable()
	if err != nil || command == "" {
		return false
	}
	return resolvedPath(filepath.Clean(command)) == resolvedPath(self)
}

// attachMCP connects one server and files its tools under prefix__name, or
// under their own names for the board's — the names the brief uses.
func (c *modelConversation) attachMCP(ctx context.Context, prefix string, transport mcp.Transport) (string, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "pourquoipas", Version: "1"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return "", err
	}
	c.closers = append(c.closers, func() { _ = session.Close() })
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		return "", err
	}
	for _, tool := range listed.Tools {
		name := tool.Name
		if prefix != "" {
			name = toolNameFor(prefix + "__" + tool.Name)
		}
		remote := tool.Name
		c.tools[name] = modelTool{
			spec: modelapi.Tool{Name: name, Description: tool.Description, Schema: schemaMap(tool.InputSchema)},
			run: func(ctx context.Context, input json.RawMessage) (string, error) {
				var args map[string]any
				if len(input) > 0 {
					if err := json.Unmarshal(input, &args); err != nil {
						return "", fmt.Errorf("the arguments are not a JSON object: %w", err)
					}
				}
				res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: remote, Arguments: args})
				if err != nil {
					return "", err
				}
				text := toolText(res)
				if res.IsError {
					return "", errors.New(text)
				}
				return text, nil
			},
		}
	}
	if init := session.InitializeResult(); init != nil {
		return init.Instructions, nil
	}
	return "", nil
}

var unsafeToolChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// toolNameFor fits a name to what both dialects accept: letters, digits,
// underscores and hyphens, at most 64 of them.
func toolNameFor(name string) string {
	name = unsafeToolChars.ReplaceAllString(name, "_")
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

func schemaMap(schema any) map[string]any {
	if m, ok := schema.(map[string]any); ok {
		return m
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{"type": "object"}
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil || out == nil {
		return map[string]any{"type": "object"}
	}
	return out
}

func toolText(res *mcp.CallToolResult) string {
	var parts []string
	for _, content := range res.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	if len(parts) == 0 && res.StructuredContent != nil {
		if raw, err := json.Marshal(res.StructuredContent); err == nil {
			parts = append(parts, string(raw))
		}
	}
	return strings.Join(parts, "\n")
}

// modelSystemPrompt is what a model is told before anything else. It stands
// in for what a CLI knows about itself; what the board and the agent say is
// in the task, as for every agent (promptLead).
func modelSystemPrompt(s *Session, hosted bool, instructions string) string {
	var b strings.Builder
	b.WriteString("You are an agent of Pourquoi Pas?, a board where cards are tasks. You act only through the tools you are given; text you write is read by a person only where the task says it will be.\n")
	switch {
	case hosted || s.Worktree.Path == "":
		b.WriteString("You have no files and no shell. Your work is what you put on the board.\n")
	case s.Planning:
		b.WriteString("You can read the files of the folder you stand in, with paths relative to it. You change nothing.\n")
	default:
		b.WriteString("You work in a folder of the person's machine: read and change its files and run commands with the tools, with paths relative to it. A command the person has not allowed is put to them as a question and waits for their answer.\n")
	}
	b.WriteString("When something only the person can decide stops you, ask them with ask_person rather than guessing.\n")
	if strings.TrimSpace(instructions) != "" {
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(instructions))
	}
	return b.String()
}

func (c *modelConversation) turn(ctx context.Context, prompt string) error {
	c.spoke = false
	reply, err := c.chat.Say(ctx, prompt, c.onText)
	for round := 0; ; round++ {
		if err != nil {
			return c.stopped(ctx, err)
		}
		c.s.appendEvent(c.m, "usage", reply.Usage)
		c.printer.flush()
		switch reply.Stop {
		case modelapi.StopRefusal:
			return apperr.New("ai.refused", "The model declined the task")
		case modelapi.StopMaxTokens:
			if len(reply.Calls) == 0 {
				return apperr.New("ai.cut-off", "The model's reply was cut off at its length limit")
			}
		}
		if len(reply.Calls) == 0 {
			return nil
		}
		if round >= maxToolRounds {
			return apperr.New("ai.too-many-rounds", "The model called tools {n} times without finishing", "n", fmt.Sprint(maxToolRounds))
		}
		results := c.call(ctx, reply.Calls)
		if ctx.Err() != nil {
			return c.stopped(ctx, ctx.Err())
		}
		c.spoke = false
		reply, err = c.chat.Answer(ctx, results, c.onText)
	}
}

// stopped tells a turn stopped from outside from one that failed: a cancel is
// the agent obeying, as an ACP agent answering "cancelled" is; a deadline is
// the session's own timeout.
func (c *modelConversation) stopped(ctx context.Context, err error) error {
	switch {
	case ctx.Err() == context.DeadlineExceeded && !c.s.wasCancelled():
		return fmt.Errorf("the turn timed out (%s)", c.m.cfg.SessionTimeout())
	case ctx.Err() != nil:
		c.s.markCancelled()
		return nil
	}
	return err
}

func (c *modelConversation) onText(text string) {
	if text == "" {
		return
	}
	c.s.finalMu.Lock()
	if !c.spoke && c.s.finalBuf.Len() > 0 {
		text = "\n\n" + text
	}
	c.spoke = true
	c.s.finalBuf.WriteString(text)
	c.s.finalMu.Unlock()
	c.s.appendEvent(c.m, "chunk", map[string]any{"text": text})
	c.printer.emitChunk(text, false)
}

// call runs a reply's calls one after another — a question to a person may be
// among them, and the answer can change what the next call should be — and
// answers every one, an unknown tool included, since a call left unanswered
// is a request the provider refuses.
func (c *modelConversation) call(ctx context.Context, calls []modelapi.ToolCall) []modelapi.ToolResult {
	results := make([]modelapi.ToolResult, 0, len(calls))
	for _, call := range calls {
		c.s.appendEvent(c.m, "tool_call", map[string]any{"toolCallId": call.ID, "title": call.Name})
		var (
			out string
			err error
		)
		if ctx.Err() != nil {
			err = errors.New("the turn was stopped")
		} else if tool, ok := c.tools[call.Name]; ok {
			out, err = tool.run(ctx, call.Input)
		} else {
			err = fmt.Errorf("there is no tool %q", call.Name)
		}
		result := modelapi.ToolResult{CallID: call.ID, Content: out}
		if err != nil {
			result.Content, result.IsError = err.Error(), true
		}
		if result.Content == "" {
			result.Content = "(no output)"
		}
		result.Content = truncateRunes(result.Content, maxToolOutput)
		c.s.appendEvent(c.m, "tool_update", map[string]any{"toolCallId": call.ID, "status": map[bool]string{true: "failed", false: "completed"}[result.IsError]})
		results = append(results, result)
	}
	return results
}
