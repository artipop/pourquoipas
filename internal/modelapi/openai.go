package modelapi

import (
	"context"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

const openAIBaseURL = "https://api.openai.com/v1/"

func openAIClient(conn Connection) openai.Client {
	// Given always, for the reason the Anthropic client is: OPENAI_BASE_URL in
	// the environment must not redirect somebody's key.
	base := strings.TrimSpace(conn.BaseURL)
	if base == "" {
		base = openAIBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return openai.NewClient(
		option.WithAPIKey(conn.Key),
		option.WithBaseURL(base),
	)
}

type openAIChat struct {
	client   openai.Client
	cfg      ChatConfig
	tools    []openai.ChatCompletionToolUnionParam
	messages []openai.ChatCompletionMessageParamUnion
}

func newOpenAIChat(cfg ChatConfig) *openAIChat {
	c := &openAIChat{client: openAIClient(cfg.Connection), cfg: cfg}
	for _, t := range cfg.Tools {
		fn := shared.FunctionDefinitionParam{
			Name:       t.Name,
			Parameters: shared.FunctionParameters(withObjectType(t.Schema)),
		}
		if t.Description != "" {
			fn.Description = openai.String(t.Description)
		}
		c.tools = append(c.tools, openai.ChatCompletionFunctionTool(fn))
	}
	if cfg.System != "" {
		c.messages = append(c.messages, openai.SystemMessage(cfg.System))
	}
	return c
}

// withObjectType makes sure the schema says it is an object: chat completions
// refuses one that does not, and an MCP tool with no arguments may say nothing.
func withObjectType(schema map[string]any) map[string]any {
	out := make(map[string]any, len(schema)+2)
	for k, v := range schema {
		out[k] = v
	}
	if _, ok := out["type"]; !ok {
		out["type"] = "object"
	}
	if _, ok := out["properties"]; !ok {
		out["properties"] = map[string]any{}
	}
	return out
}

func (c *openAIChat) Say(ctx context.Context, text string, onText func(string)) (Reply, error) {
	c.messages = append(c.messages, openai.UserMessage(text))
	return c.reply(ctx, onText)
}

func (c *openAIChat) Answer(ctx context.Context, results []ToolResult, onText func(string)) (Reply, error) {
	for _, r := range results {
		content := r.Content
		if r.IsError && !strings.HasPrefix(content, "Error") {
			// Chat completions has no error flag on a tool message; the model
			// reads the word.
			content = "Error: " + content
		}
		c.messages = append(c.messages, openai.ToolMessage(content, r.CallID))
	}
	return c.reply(ctx, onText)
}

func (c *openAIChat) reply(ctx context.Context, onText func(string)) (Reply, error) {
	// No token cap: the endpoints that copy this API disagree about its name
	// (max_tokens or max_completion_tokens) and about how high it may go, and
	// each model's own default is a cap that is always accepted.
	params := openai.ChatCompletionNewParams{
		Model:         shared.ChatModel(c.cfg.Model),
		Messages:      c.messages,
		Tools:         c.tools,
		StreamOptions: openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)},
	}
	if c.cfg.Effort != "" {
		params.ReasoningEffort = shared.ReasoningEffort(c.cfg.Effort)
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)
	acc := openai.ChatCompletionAccumulator{}
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)
		if onText != nil && len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onText(chunk.Choices[0].Delta.Content)
		}
	}
	if err := stream.Err(); err != nil {
		return Reply{}, err
	}

	reply := Reply{Stop: StopEnd, Usage: Usage{
		Input:     acc.Usage.PromptTokens,
		Output:    acc.Usage.CompletionTokens,
		CacheRead: acc.Usage.PromptTokensDetails.CachedTokens,
	}}
	if len(acc.Choices) == 0 {
		return reply, nil
	}
	choice := acc.Choices[0]
	c.messages = append(c.messages, choice.Message.ToParam())
	reply.Text = choice.Message.Content
	for _, call := range choice.Message.ToolCalls {
		if call.Function.Name == "" {
			continue
		}
		args := call.Function.Arguments
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		reply.Calls = append(reply.Calls, ToolCall{ID: call.ID, Name: call.Function.Name, Input: []byte(args)})
	}
	switch {
	case choice.Message.Refusal != "":
		reply.Stop = StopRefusal
		if reply.Text == "" {
			reply.Text = choice.Message.Refusal
		}
	case len(reply.Calls) > 0:
		// Some compatible servers end a reply that calls tools with "stop";
		// the calls are what matter.
		reply.Stop = StopTools
	case choice.FinishReason == "length":
		reply.Stop = StopMaxTokens
	case choice.FinishReason == "content_filter":
		reply.Stop = StopRefusal
	}
	return reply, nil
}

func openAIModels(ctx context.Context, conn Connection) ([]Model, error) {
	client := openAIClient(conn)
	pager := client.Models.ListAutoPaging(ctx)
	var out []Model
	for pager.Next() {
		out = append(out, Model{ID: pager.Current().ID})
	}
	if err := pager.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
