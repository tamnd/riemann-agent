package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// Claude is the client of the Claude API.
type Claude struct {
	c anthropic.Client
}

// NewClaude returns a client with the API key. An empty key means the key in the environment variable ANTHROPIC_API_KEY.
func NewClaude(key string) *Claude {
	var opts []option.RequestOption
	if key != "" {
		opts = append(opts, option.WithAPIKey(key))
	}
	return &Claude{c: anthropic.NewClient(opts...)}
}

// Complete sends the request as a stream, so that a long answer does not hit a time limit, and returns the full answer.
func (c *Claude) Complete(ctx context.Context, r Request) (*Response, error) {
	p := anthropic.MessageNewParams{
		Model:     r.Model,
		MaxTokens: r.MaxTokens,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(r.Prompt))},
	}
	if r.System != "" {
		p.System = []anthropic.TextBlockParam{{Text: r.System, CacheControl: anthropic.NewCacheControlEphemeralParam()}}
	}
	if r.Think {
		p.Thinking = anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{Display: "omitted"}}
	}
	if r.Effort != "" {
		p.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(r.Effort)}
	}
	stream := c.c.Messages.NewStreaming(ctx, p)
	m := anthropic.Message{}
	for stream.Next() {
		if err := m.Accumulate(stream.Current()); err != nil {
			return nil, fmt.Errorf("llm: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}
	var text strings.Builder
	for _, b := range m.Content {
		if t, ok := b.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	out := &Response{
		Text:       text.String(),
		Model:      m.Model,
		StopReason: string(m.StopReason),
		Usage: Usage{
			Input:      m.Usage.InputTokens,
			Output:     m.Usage.OutputTokens,
			CacheRead:  m.Usage.CacheReadInputTokens,
			CacheWrite: m.Usage.CacheCreationInputTokens,
		},
	}
	switch m.StopReason {
	case anthropic.StopReasonRefusal:
		return out, ErrRefused
	case anthropic.StopReasonMaxTokens:
		return out, ErrTruncated
	}
	return out, nil
}
