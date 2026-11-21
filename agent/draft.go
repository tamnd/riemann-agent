package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/tamnd/riemann-agent/llm"
	"github.com/tamnd/riemann-agent/prompts"
	"github.com/tamnd/riemann-agent/steps"
)

// Drafter runs the prover for one name at a time.
type Drafter struct {
	Root   string // the content repository
	Env    *Env
	Config *Config
	Ledger *Ledger
	Client llm.Client // nil for a dry run
	Out    io.Writer  // the dry run prints the prompt here
	Now    func() time.Time

	spent float64 // the spend of this run
}

// Result is the outcome of one draft.
type Result struct {
	Name     string
	Step     *steps.Step
	Parents  []string // the ids of the steps that now cite the new step
	Estimate float64
	Cost     float64
	Dry      bool
}

// Request returns the prover request for a name.
func (d *Drafter) Request(name string) (*llm.Request, *prompts.Prompt, error) {
	p, err := prompts.Load("prover")
	if err != nil {
		return nil, nil, err
	}
	in, err := d.Env.Prover(name)
	if err != nil {
		return nil, nil, err
	}
	text, err := p.Render(in)
	if err != nil {
		return nil, nil, err
	}
	return &llm.Request{
		Model:     d.Config.Model("prover"),
		System:    p.System,
		Prompt:    text,
		MaxTokens: d.Config.Tokens("prover"),
		Effort:    d.Config.Effort["prover"],
		Think:     d.Config.Think("prover"),
	}, p, nil
}

// Prove drafts the step of a name. With no client, it prints the prompt and the estimate and calls nothing.
func (d *Drafter) Prove(ctx context.Context, name string) (*Result, error) {
	req, p, err := d.Request(name)
	if err != nil {
		return nil, err
	}
	est, err := llm.Estimate(*req)
	if err != nil {
		return nil, err
	}
	res := &Result{Name: name, Estimate: est}
	if d.Client == nil {
		res.Dry = true
		if d.Out != nil {
			fmt.Fprintf(d.Out, "=== system (%s, %s) ===\n%s\n=== prompt ===\n%s=== end: about %d tokens in, at most %d out, at most $%.4f ===\n", p.ID(), req.Model, req.System, req.Prompt, llm.Tokens(req.System)+llm.Tokens(req.Prompt), req.MaxTokens, est)
		}
		return res, nil
	}
	now := d.Now()
	if err := d.Config.Allow(d.Ledger, now, d.spent, est); err != nil {
		return nil, err
	}
	resp, callErr := d.Client.Complete(ctx, *req)
	if resp == nil {
		return nil, callErr
	}
	cost, err := llm.Cost(req.Model, resp.Usage)
	if err != nil {
		return nil, err
	}
	d.spent += cost
	res.Cost = cost
	entry := Entry{Time: now.UTC(), Role: "prover", Target: name, Model: req.Model, Prompt: p.ID(), Usage: resp.Usage, Cost: cost}
	var st *steps.Step
	if callErr == nil {
		st, callErr = d.write(name, resp.Text, "prover@"+req.Model, p.ID(), res)
	}
	if callErr != nil {
		entry.Error = callErr.Error()
	} else {
		entry.File = st.File
	}
	if err := d.Ledger.Add(entry); err != nil {
		return nil, errors.Join(callErr, err)
	}
	if callErr != nil {
		return nil, callErr
	}
	res.Step = st
	return res, nil
}

// write parses the answer, writes the step and adds it to the uses of the steps that listed the name in open.
func (d *Drafter) write(name, text, draftedBy, prompt string, res *Result) (*steps.Step, error) {
	dr, err := ParseProver(text)
	if err != nil {
		return nil, fmt.Errorf("the answer for %s: %w", name, err)
	}
	st, err := d.Env.Build(name, dr, draftedBy, prompt)
	if err != nil {
		return nil, err
	}
	if err := d.save(st); err != nil {
		return nil, err
	}
	for _, p := range d.Env.Parents(name) {
		if p.ID == st.ID {
			continue
		}
		p.Open = slices.DeleteFunc(p.Open, func(o string) bool { return o == name })
		if !slices.Contains(p.Uses, st.ID) {
			p.Uses = append(p.Uses, st.ID)
		}
		if err := d.save(p); err != nil {
			return nil, err
		}
		res.Parents = append(res.Parents, p.ID)
	}
	// The next draft of this run sees the new step.
	d.Env.Steps = slices.DeleteFunc(d.Env.Steps, func(s *steps.Step) bool { return s.ID == st.ID })
	d.Env.Steps = append(d.Env.Steps, st)
	return st, nil
}

func (d *Drafter) save(s *steps.Step) error {
	data, err := s.Marshal()
	if err != nil {
		return err
	}
	path := filepath.Join(d.Root, filepath.FromSlash(s.File))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
