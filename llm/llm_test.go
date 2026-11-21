package llm

import (
	"context"
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCost(t *testing.T) {
	c, err := Cost(ModelLarge, Usage{Input: 1_000_000, Output: 1_000_000})
	if err != nil || !near(c, 24) {
		t.Fatalf("Cost = %v, %v; want 24", c, err)
	}
	c, err = Cost(ModelSmall, Usage{Input: 30_000, CacheRead: 50_000})
	if err != nil || !near(c, (30_000*0.10+50_000*0.01)/1e6) {
		t.Fatalf("Cost of a short prompt = %v, %v", c, err)
	}
	c, err = Cost(ModelSmall, Usage{Input: 200_000})
	if err != nil || !near(c, 0.1) {
		t.Fatalf("Cost of a long prompt = %v, %v; want 0.1", c, err)
	}
	if _, err := Cost("no-such-model", Usage{}); err == nil {
		t.Fatal("an unknown model must fail")
	}
}

func TestEstimate(t *testing.T) {
	r := Request{Model: ModelLarge, Prompt: string(make([]byte, 2994)), MaxTokens: 1000}
	e, err := Estimate(r)
	if err != nil || !near(e, (1000*5.0+1000*20.0)/1e6) {
		t.Fatalf("Estimate = %v, %v", e, err)
	}
}

func TestFake(t *testing.T) {
	f := &Fake{Text: "ok"}
	r, err := f.Complete(context.Background(), Request{Model: ModelSmall, Prompt: "abc"})
	if err != nil || r.Text != "ok" || len(f.Requests) != 1 || r.Usage.Input != 3 {
		t.Fatalf("Fake = %+v, %v", r, err)
	}
}
