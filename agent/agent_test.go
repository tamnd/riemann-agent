package agent

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/riemann-agent/index"
	"github.com/tamnd/riemann-agent/llm"
	"github.com/tamnd/riemann-agent/steps"
)

const leanSrc = "theorem low : True := trivial\ntheorem middle : True := low\n"

// fixture makes a content repository with one step, top, that lists middle in open, and a Lean source with the declarations.
func fixture(t *testing.T) (string, *Env) {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "src")
	must(t, os.MkdirAll(src, 0o750))
	must(t, os.WriteFile(filepath.Join(src, "M.lean"), []byte(leanSrc), 0o600))
	mid := strings.Index(leanSrc, "theorem middle")
	ix := &index.Index{
		Modules: []*index.Module{{Name: "M", Path: "M.lean"}},
		Decls: []index.Decl{
			{Name: "helper", Kind: "theorem", Module: "M"},
			{Name: "low", Kind: "theorem", Module: "M", Span: [2]int{0, mid}, Uses: []string{"named"}},
			{Name: "middle", Kind: "theorem", Module: "M", Span: [2]int{mid, len(leanSrc)}, Sig: ": True", Uses: []string{"low", "named"}},
			{Name: "named", Kind: "theorem", Module: "M"},
			{Name: "top", Kind: "theorem", Module: "M", Uses: []string{"middle", "helper", "named"}},
		},
	}
	dir := filepath.Join(root, "proof", "steps")
	must(t, os.MkdirAll(dir, 0o750))
	top := &steps.Step{ID: "top", Title: "The top", Decl: "top", Chapter: "trunk", Status: "drafted", Routine: []string{"helper"}, Open: []string{"middle"}, Body: "## Statement\n\nP.\n"}
	data, err := top.Marshal()
	must(t, err)
	must(t, os.WriteFile(filepath.Join(dir, "top.md"), data, 0o600))
	ss, err := steps.Load(root)
	must(t, err)
	env := &Env{
		Index:    ix,
		Graph:    index.NewGraph(ix),
		Src:      src,
		Steps:    ss,
		Glossary: []steps.Entry{{Name: "low", Kind: "theorem", Says: "a trivial fact"}},
		Lit:      []steps.Named{{Name: "named", Statement: "True", Says: "a result of the literature"}},
	}
	return root, env
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

const answer = "title: The middle fact\nroutine: `low`, nothing\n---\n## Statement\n\nTrue holds.\n\n## Proof\n\nApply `low`.\n"

func TestQueue(t *testing.T) {
	_, env := fixture(t)
	if got := env.Queue(5); !slices.Equal(got, []string{"middle"}) {
		t.Fatalf("Queue = %v", got)
	}
}

func TestDryRun(t *testing.T) {
	root, env := fixture(t)
	var out bytes.Buffer
	l, err := OpenLedger(root)
	must(t, err)
	d := &Drafter{Root: root, Env: env, Config: &Config{}, Ledger: l, Out: &out, Now: time.Now}
	res, err := d.Prove(context.Background(), "middle")
	must(t, err)
	if !res.Dry || res.Estimate <= 0 {
		t.Fatalf("result = %+v", res)
	}
	for _, want := range []string{"prover/1", "theorem middle : True := low", "`low`, a theorem.", "Glossary: a trivial fact", "Statement: True"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the prompt has no %q:\n%s", want, out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, LedgerFile)); err == nil {
		t.Fatal("a dry run must not write the ledger")
	}
}

func TestBudget(t *testing.T) {
	root, env := fixture(t)
	l, err := OpenLedger(root)
	must(t, err)
	f := &llm.Fake{Text: answer}
	d := &Drafter{Root: root, Env: env, Config: &Config{}, Ledger: l, Client: f, Now: time.Now}
	if _, err := d.Prove(context.Background(), "middle"); !errors.Is(err, ErrBudget) {
		t.Fatalf("a zero budget must stop the call, got %v", err)
	}
	if len(f.Requests) != 0 {
		t.Fatal("the call went out")
	}
	d.Config.Budget = Budget{Day: 1, Run: 1}
	must(t, l.Add(Entry{Time: time.Now().UTC(), Cost: 0.999}))
	if _, err := d.Prove(context.Background(), "middle"); !errors.Is(err, ErrBudget) {
		t.Fatalf("a spent day must stop the call, got %v", err)
	}
}

func TestProve(t *testing.T) {
	root, env := fixture(t)
	l, err := OpenLedger(root)
	must(t, err)
	f := &llm.Fake{Text: answer}
	d := &Drafter{Root: root, Env: env, Config: &Config{Budget: Budget{Day: 5, Run: 5}}, Ledger: l, Client: f, Now: time.Now}
	res, err := d.Prove(context.Background(), "middle")
	must(t, err)
	if res.Step.ID != "middle" || res.Step.Chapter != "trunk" || !slices.Equal(res.Step.Routine, []string{"low"}) || len(res.Step.Open) != 0 {
		t.Fatalf("step = %+v", res.Step)
	}
	if !slices.Equal(res.Parents, []string{"top"}) {
		t.Fatalf("parents = %v", res.Parents)
	}
	ss, err := steps.Load(root)
	must(t, err)
	by := steps.ByID(ss)
	if m := by["middle"]; m == nil || m.DraftedBy != "prover@claude-opus-5-5" || m.Prompt != "prover/1" || m.Status != "drafted" {
		t.Fatalf("the written step = %+v", m)
	}
	if top := by["top"]; len(top.Open) != 0 || !slices.Equal(top.Uses, []string{"middle"}) {
		t.Fatalf("the parent = %+v", top)
	}
	cenv := steps.Env{Graph: env.Graph, Cut: env.Cut}
	if p := steps.Check(ss, cenv); len(p) > 0 {
		t.Fatalf("the check fails: %v", p)
	}
	l2, err := OpenLedger(root)
	must(t, err)
	if len(l2.Entries) != 1 || l2.Entries[0].File != "proof/steps/middle.md" || l2.Entries[0].Cost <= 0 {
		t.Fatalf("ledger = %+v", l2.Entries)
	}
	if !f.Requests[0].Think {
		t.Fatal("the prover must think")
	}
}

func TestBadAnswer(t *testing.T) {
	root, env := fixture(t)
	l, err := OpenLedger(root)
	must(t, err)
	d := &Drafter{Root: root, Env: env, Config: &Config{Budget: Budget{Day: 5, Run: 5}}, Ledger: l, Client: &llm.Fake{Text: "no header"}, Now: time.Now}
	if _, err := d.Prove(context.Background(), "middle"); err == nil {
		t.Fatal("a bad answer must fail")
	}
	if len(l.Entries) != 1 || l.Entries[0].Error == "" {
		t.Fatalf("the ledger must record the failed call: %+v", l.Entries)
	}
	if _, err := os.Stat(filepath.Join(root, "proof", "steps", "middle.md")); err == nil {
		t.Fatal("a bad answer must not write a step")
	}
}

func TestStepID(t *testing.T) {
	taken := map[string]bool{"every-character": true}
	for _, c := range []struct{ name, kind, want string }{
		{"UpperTheory.FinalTheorems.everyCharacter", "theorem", "every-character-2"},
		{"IsRecT", "def", "is-rec-t-def"},
		{"Foo.vwin_of_row", "theorem", "vwin-of-row"},
		{"Row0b", "structure", "row0b-def"},
	} {
		if got := StepID(c.name, c.kind, taken); got != c.want {
			t.Errorf("StepID(%s) = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestParseProver(t *testing.T) {
	if _, err := ParseProver("title: x\n---\n## Statement\n\nA — B.\n\n## Proof\n\nC.\n"); err == nil {
		t.Error("an em dash must fail")
	}
	if _, err := ParseProver("title: x\n---\n## Proof\n\nC.\n"); err == nil {
		t.Error("no statement must fail")
	}
	d, err := ParseProver(answer)
	if err != nil || d.Title != "The middle fact" || !slices.Equal(d.Routine, []string{"low"}) {
		t.Fatalf("ParseProver = %+v, %v", d, err)
	}
}

func TestLoadConfig(t *testing.T) {
	root := t.TempDir()
	c, err := LoadConfig(root)
	must(t, err)
	if c.Budget.Day != 0 || c.Model("prover") != llm.ModelLarge || c.Model("namer") != llm.ModelSmall {
		t.Fatalf("defaults = %+v", c)
	}
	must(t, os.WriteFile(filepath.Join(root, "agent.yaml"), []byte("models:\n  namer: claude-sonnet-5-5\nbudget:\n  day: 20\n  run: 5\n"), 0o600))
	c, err = LoadConfig(root)
	must(t, err)
	if c.Model("namer") != "claude-sonnet-5-5" || c.Budget.Run != 5 {
		t.Fatalf("config = %+v", c)
	}
	must(t, os.WriteFile(filepath.Join(root, "agent.yaml"), []byte("models:\n  writer: x\n"), 0o600))
	if _, err := LoadConfig(root); err == nil {
		t.Fatal("an unknown role must fail")
	}
}

func TestReport(t *testing.T) {
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	l := &Ledger{Entries: []Entry{
		{Time: day.Add(time.Hour), Role: "prover", Cost: 0.5, File: "proof/steps/a.md"},
		{Time: day.Add(2 * time.Hour), Role: "prover", Cost: 0.25, Error: "no title"},
		{Time: day.Add(-48 * time.Hour), Role: "namer", Cost: 9},
	}}
	ss := []*steps.Step{
		{ID: "b", File: "proof/steps/b.md", Status: "checked", DraftedBy: "prover@m"},
		{ID: "a", File: "proof/steps/a.md", Status: "drafted", DraftedBy: "prover@m"},
		{ID: "c", File: "proof/steps/c.md", Status: "reviewed"},
	}
	edit := func(f string) (Edit, error) { return Edit{Added: len(f), Deleted: 1}, nil }
	r, err := MakeReport(l, ss, day, day.Add(24*time.Hour), edit)
	if err != nil {
		t.Fatal(err)
	}
	if r.Calls != 2 || r.Failed != 1 || r.NewDrafts != 1 || r.Cost != 0.75 || len(r.Drafted) != 2 || r.Accepted() != 1 || r.Drafted[0].ID != "a" {
		t.Fatalf("report = %+v", r)
	}
	var b strings.Builder
	r.Write(&b)
	for _, want := range []string{"2026-11-20 to 2026-11-20", "made 2 calls, and 1 of them failed", "$0.75", "People accepted 1 of them", "| b | prover@m | checked | 16 | 1 |"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the report has no %q:\n%s", want, b.String())
		}
	}
}
