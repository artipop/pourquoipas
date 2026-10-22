package modelapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const anthropicBaseURL = "https://api.anthropic.com/"

func anthropicClient(conn Connection) anthropic.Client {
	// The address is always given: the SDK would otherwise take
	// ANTHROPIC_BASE_URL from the environment of whoever started the app, and
	// a key meant for one place would be sent to another.
	base := strings.TrimSpace(conn.BaseURL)
	if base == "" {
		base = anthropicBaseURL
	}
	return anthropic.NewClient(
		option.WithAPIKey(conn.Key),
		option.WithBaseURL(base),
	)
}

type anthropicChat struct {
	client   anthropic.Client
	cfg      ChatConfig
	tools    []anthropic.ToolUnionParam
	messages []anthropic.MessageParam
}

func newAnthropicChat(cfg ChatConfig) *anthropicChat {
	c := &anthropicChat{client: anthropicClient(cfg.Connection), cfg: cfg}
	for _, t := range cfg.Tools {
		tool := anthropic.ToolParam{
			Name:        t.Name,
			InputSchema: anthropicSchema(t.Schema),
		}
		if t.Description != "" {
			tool.Description = anthropic.String(t.Description)
		}
		c.tools = append(c.tools, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return c
}

// anthropicSchema splits a JSON Schema object into the fields the SDK names
// and the rest, which it carries as they are.
func anthropicSchema(schema map[string]any) anthropic.ToolInputSchemaParam {
	out := anthropic.ToolInputSchemaParam{Properties: map[string]any{}}
	for key, value := range schema {
		switch key {
		case "type":
		case "properties":
			out.Properties = value
		case "required":
			out.Required = stringList(value)
		default:
			if out.ExtraFields == nil {
				out.ExtraFields = map[string]any{}
			}
			out.ExtraFields[key] = value
		}
	}
	return out
}

func stringList(v any) []string {
	switch list := v.(type) {
	case []string:
		return list
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (c *anthropicChat) Say(ctx context.Context, text string, onText func(string)) (Reply, error) {
	c.messages = append(c.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
	return c.reply(ctx, onText)
}

func (c *anthropicChat) Answer(ctx context.Context, results []ToolResult, onText func(string)) (Reply, error) {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(results))
	for _, r := range results {
		blocks = append(blocks, anthropic.NewToolResultBlock(r.CallID, r.Content, r.IsError))
	}
	c.messages = append(c.messages, anthropic.NewUserMessage(blocks...))
	return c.reply(ctx, onText)
}

func (c *anthropicChat) reply(ctx context.Context, onText func(string)) (Reply, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.cfg.Model),
		MaxTokens: c.cfg.MaxTokens,
		Messages:  c.messages,
		Tools:     c.tools,
		// The system text and the tools are the same on every request of a
		// conversation, so the prefix is worth caching; the top-level marker
		// puts the breakpoint on the last cacheable block itself.
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}
	if c.cfg.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: c.cfg.System}}
	}
	if c.cfg.Effort != "" {
		params.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(c.cfg.Effort)}
	}

	stream := c.client.Messages.NewStreaming(ctx, params)
	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return Reply{}, err
		}
		if onText == nil {
			continue
		}
		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok {
				onText(text.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return Reply{}, err
	}
	// The whole response goes back into the history, thinking blocks and all:
	// the next request must return them as they came.
	c.messages = append(c.messages, message.ToParam())

	reply := Reply{Usage: Usage{
		Input:     message.Usage.InputTokens,
		Output:    message.Usage.OutputTokens,
		CacheRead: message.Usage.CacheReadInputTokens,
	}}
	var text strings.Builder
	for _, block := range message.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			text.WriteString(b.Text)
		case anthropic.ToolUseBlock:
			reply.Calls = append(reply.Calls, ToolCall{ID: b.ID, Name: b.Name, Input: []byte(b.JSON.Input.Raw())})
		}
	}
	reply.Text = text.String()
	switch message.StopReason {
	case anthropic.StopReasonToolUse:
		reply.Stop = StopTools
	case anthropic.StopReasonMaxTokens, anthropic.StopReasonModelContextWindowExceeded:
		reply.Stop = StopMaxTokens
	case anthropic.StopReasonRefusal:
		reply.Stop = StopRefusal
	default:
		reply.Stop = StopEnd
	}
	if reply.Stop == StopTools && len(reply.Calls) == 0 {
		return reply, fmt.Errorf("the model stopped for a tool and named none")
	}
	return reply, nil
}

func anthropicModels(ctx context.Context, conn Connection) ([]Model, error) {
	client := anthropicClient(conn)
	pager := client.Models.ListAutoPaging(ctx, anthropic.ModelListParams{})
	var out []Model
	for pager.Next() {
		m := pager.Current()
		out = append(out, Model{ID: m.ID, Name: m.DisplayName})
	}
	if err := pager.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
