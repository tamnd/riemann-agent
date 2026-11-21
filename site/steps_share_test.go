package site

import (
	"testing"

	"github.com/tamnd/riemann-agent/steps"
)

func TestShareText(t *testing.T) {
	st := func(ss ...string) []*steps.Step {
		var out []*steps.Step
		for _, s := range ss {
			out = append(out, &steps.Step{Status: s})
		}
		return out
	}
	for _, c := range []struct {
		in    []*steps.Step
		want  string
		state string
	}{
		{st("drafted"), "The one step is drafted. No person has checked it yet.", "no"},
		{st("reviewed"), "The one step is checked or reviewed.", "yes"},
		{st("drafted", "drafted"), "All 2 steps are drafted. No person has checked them yet.", "no"},
		{st("checked", "drafted", "drafted", "reviewed"), "2 of the 4 steps (50%) are checked or reviewed. The other 2 are drafted.", "open"},
	} {
		if got := shareText(c.in); got != c.want {
			t.Errorf("shareText = %q, want %q", got, c.want)
		}
		if got := stepCheck(c.in).State; got != c.state {
			t.Errorf("stepCheck state = %q, want %q", got, c.state)
		}
	}
}
