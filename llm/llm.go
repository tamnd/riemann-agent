// Package llm is the model client of rh draft. It sends one prompt to a model and returns the text, the token counts and the cost in dollars.
//
// The type Claude calls the Claude API. The type Fake returns fixed text, and the tests use it, so a test never spends money.
package llm

import (
	"context"
	"errors"
	"fmt"
)

// The models of spec 09.4: the newest model for the prover and the chapterer, and a small model for the namer and the linker.
const (
	ModelLarge = "claude-opus-5-5"
	ModelSmall = "claude-haiku-5-5"
)

// Request is one prompt.
type Request struct {
	Model     string
	System    string // the fixed part of the prompt; the client marks it for the prompt cache
	Prompt    string // the part of the prompt for this step
	MaxTokens int64
	Effort    string // low, medium, high, xhigh or max; empty for the default of the model
	Think     bool   // let the model think before it answers (adaptive thinking); the thinking text is not kept
}

// Usage is the token count of one response.
type Usage struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}

// Response is the answer of the model.
type Response struct {
	Text       string
	Model      string
	StopReason string
	Usage      Usage
}

// Client sends a request to a model.
type Client interface {
	Complete(ctx context.Context, r Request) (*Response, error)
}

// ErrRefused is the error for a response that the model refused to give.
var ErrRefused = errors.New("llm: the model refused the request")

// ErrTruncated is the error for a response that stopped at MaxTokens.
var ErrTruncated = errors.New("llm: the response stopped at the token limit")

// price is the price of a model in dollars for one million tokens.
type price struct{ in, out, read, write float64 }

// prices are the list prices of the models, for the 5-minute cache.
var prices = map[string]price{
	"claude-opus-5-5":   {in: 4, out: 20, read: 0.20, write: 5},
	"claude-sonnet-5-5": {in: 2, out: 10, read: 0.10, write: 2.5},
	"claude-haiku-5-5":  {in: 0.10, out: 0.50, read: 0.01, write: 0.125},
}

// haikuLong is the prompt size above which Claude Haiku 5.5 has a higher price.
const haikuLong = 100_000

func priceOf(model string, input int64) (price, error) {
	p, ok := prices[model]
	if !ok {
		return price{}, fmt.Errorf("llm: no price for the model %q", model)
	}
	if model == "claude-haiku-5-5" && input > haikuLong {
		p = price{in: 0.50, out: 2.50, read: 0.05, write: 0.625}
	}
	return p, nil
}

// Cost returns the cost in dollars of a response.
func Cost(model string, u Usage) (float64, error) {
	p, err := priceOf(model, u.Input+u.CacheRead+u.CacheWrite)
	if err != nil {
		return 0, err
	}
	return (float64(u.Input)*p.in + float64(u.Output)*p.out + float64(u.CacheRead)*p.read + float64(u.CacheWrite)*p.write) / 1e6, nil
}

// Estimate returns the largest cost of a request: the prompt with no cache hit, and an answer of MaxTokens. The budget uses it before a call.
func Estimate(r Request) (float64, error) {
	in := Tokens(r.System) + Tokens(r.Prompt)
	p, err := priceOf(r.Model, in)
	if err != nil {
		return 0, err
	}
	return (float64(in)*p.write + float64(r.MaxTokens)*p.out) / 1e6, nil
}

// Tokens returns a high guess of the token count of a text: one token for each three bytes. Lean text has many symbols of more than one byte, so the guess is high.
func Tokens(s string) int64 { return int64(len(s)/3 + 1) }
