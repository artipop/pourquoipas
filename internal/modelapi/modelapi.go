// Package modelapi talks to a language model over its vendor's HTTP API: one
// conversation, its tools, a reply streamed as it is written. It knows nothing
// about the board or about ACP — internal/acp runs the loop and decides what
// the tools are.
//
// Two dialects cover every endpoint worth naming: Anthropic's Messages API,
// and the chat completions API OpenAI published and everybody else copied
// (OpenRouter, a local server, most of the rest). Each is driven through its
// vendor's official Go SDK.
//
// The history is kept in the vendor's own types rather than in a shape of ours:
// a reasoning model hands back blocks that must be returned unchanged on the
// next request, and a neutral transcript would drop them.
package modelapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// The providers. A provider is a dialect plus where it is spoken; the key is
// whoever's is using it.
const (
	ProviderAnthropic = "anthropic"
	// ProviderOpenAI is any endpoint that speaks OpenAI's chat completions:
	// OpenAI itself by default, anything else by its base URL.
	ProviderOpenAI = "openai"
)

// Providers lists what this package can talk to, in the order a person is
// offered them.
var Providers = []string{ProviderAnthropic, ProviderOpenAI}

// DefaultModel is what the agent form suggests for a provider before the
// provider has been asked what it has.
func DefaultModel(provider string) string {
	switch provider {
	case ProviderAnthropic:
		return "claude-opus-5-5"
	default:
		return ""
	}
}

// Connection is where requests go and what opens the door.
type Connection struct {
	Provider string
	Key      string
	// BaseURL overrides the provider's own address; empty means the vendor's.
	BaseURL string
}

func (c Connection) validate() error {
	switch c.Provider {
	case ProviderAnthropic, ProviderOpenAI:
	default:
		return fmt.Errorf("unknown model provider %q", c.Provider)
	}
	if strings.TrimSpace(c.Key) == "" {
		return ErrNoKey
	}
	return nil
}

// ErrNoKey is a connection with nothing to present.
var ErrNoKey = errors.New("no API key")

// Tool is one function the model may call. Schema is a JSON Schema object, as
// an MCP server declares its tools' input.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
}

// ToolCall is the model asking for one.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// ToolResult is what a call came back with.
type ToolResult struct {
	CallID  string
	Content string
	IsError bool
}

// Stop says why a reply ended.
type Stop string

const (
	StopEnd       Stop = "end"
	StopTools     Stop = "tools"
	StopMaxTokens Stop = "max_tokens"
	// StopRefusal is the model declining; the reply carries no work.
	StopRefusal Stop = "refusal"
)

// Usage is what one reply cost, in tokens.
type Usage struct {
	Input     int64 `json:"input"`
	Output    int64 `json:"output"`
	CacheRead int64 `json:"cacheRead,omitempty"`
}

// Reply is one response of the model.
type Reply struct {
	Text  string
	Calls []ToolCall
	Stop  Stop
	Usage Usage
}

// ChatConfig is a conversation's fixed part: who, which model, what it is
// told first and what it may call. Fixed for the conversation, which is also
// what lets the vendor cache it.
type ChatConfig struct {
	Connection
	Model  string
	System string
	Tools  []Tool
	// Effort is low, medium, high, xhigh or max; empty leaves the model's own.
	Effort    string
	MaxTokens int64
}

// Chat is one conversation. It is not safe for concurrent use: a turn is one
// request after another.
type Chat interface {
	// Say adds what the person says and runs one reply. onText receives the
	// reply's text as it streams; it may be nil.
	Say(ctx context.Context, text string, onText func(string)) (Reply, error)
	// Answer hands back the results of the previous reply's calls, all of them
	// at once, and runs the next reply.
	Answer(ctx context.Context, results []ToolResult, onText func(string)) (Reply, error)
}

// NewChat opens a conversation.
func NewChat(cfg ChatConfig) (Chat, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("no model was chosen")
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = defaultMaxTokens
	}
	switch cfg.Provider {
	case ProviderAnthropic:
		return newAnthropicChat(cfg), nil
	default:
		return newOpenAIChat(cfg), nil
	}
}

// defaultMaxTokens is generous on purpose: every request streams, so a long
// reply costs no timeout, and a reply cut off mid-thought costs a retry.
const defaultMaxTokens = 64000

// Model is one model a provider offers.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Models asks the provider what it has. It is also how a key is checked: a
// key the provider refuses fails here.
func Models(ctx context.Context, conn Connection) ([]Model, error) {
	if err := conn.validate(); err != nil {
		return nil, err
	}
	// In the provider's own order: Anthropic lists the newest first, which is
	// the order a person picks in.
	switch conn.Provider {
	case ProviderAnthropic:
		return anthropicModels(ctx, conn)
	default:
		return openAIModels(ctx, conn)
	}
}
