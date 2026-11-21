package llm

import (
	"context"
	"sync"
)

// Fake is a client that returns fixed text and spends nothing. The tests use it.
type Fake struct {
	// Reply returns the text for a request. When it is nil, the reply is Text.
	Reply func(r Request) string
	Text  string

	mu       sync.Mutex
	Requests []Request // the requests, in the order of the calls
}

// Complete records the request and returns the reply, with a token count from Tokens.
func (f *Fake) Complete(_ context.Context, r Request) (*Response, error) {
	f.mu.Lock()
	f.Requests = append(f.Requests, r)
	f.mu.Unlock()
	text := f.Text
	if f.Reply != nil {
		text = f.Reply(r)
	}
	return &Response{
		Text:       text,
		Model:      r.Model,
		StopReason: "end_turn",
		Usage:      Usage{Input: Tokens(r.System) + Tokens(r.Prompt), Output: Tokens(text)},
	}, nil
}
