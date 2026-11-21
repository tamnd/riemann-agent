package steps

import (
	"strings"
	"testing"

	"github.com/tamnd/riemann-agent/index"
)

func graph() *index.Graph {
	return index.NewGraph(&index.Index{Decls: []index.Decl{
		{Name: "helper", Kind: "theorem"},
		{Name: "low", Kind: "theorem", Uses: []string{"named"}},
		{Name: "middle", Kind: "theorem", Sig: ": Q", Uses: []string{"low"}},
		{Name: "named", Kind: "theorem"},
		{Name: "top", Kind: "theorem", Sig: ": P", Uses: []string{"middle", "helper", "named"}},
	}})
}

func env() Env {
	return Env{
		Graph: graph(),
		Cut:   func(n string) bool { return n == "named" },
		Tag:   func(t string) bool { return t == "AAAA" },
	}
}

func TestLoadAndCheck(t *testing.T) {
	ss, err := Load("testdata/ok")
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 2 || ss[0].ID != "middle" || ss[1].ID != "top" {
		t.Fatalf("steps: %+v", ss)
	}
	ps := Check(ss, env())
	// middle uses low, which no step and no routine list covers.
	if len(ps) != 1 || ps[0].Rule != "C3" || !strings.Contains(ps[0].Text, "low") {
		t.Fatalf("problems: %v", ps)
	}
	if got := UsedBy(ss)["middle"]; len(got) != 1 || got[0] != "top" {
		t.Errorf("used by: %v", got)
	}
	lit, err := LoadLiterature("testdata/ok")
	if err != nil || len(lit) != 1 {
		t.Fatal(lit, err)
	}
	if ps := CheckLiterature(lit, env()); len(ps) != 0 {
		t.Errorf("literature: %v", ps)
	}
	if lit[0].Cite[0].Link() != "https://doi.org/10.1000/x" {
		t.Errorf("link: %s", lit[0].Cite[0].Link())
	}
}

func TestRules(t *testing.T) {
	ss := []*Step{
		{ID: "a", File: "a.md", Decl: "nothing", Status: "drafted"},
		{ID: "b", File: "b.md", Decl: "low", Status: "checked", Uses: []string{"c", "d"}, Concepts: []string{"ZZZZ"}, Body: "```lean\ntheorem low : R\n```\n"},
		{ID: "d", File: "d.md", Decl: "top", Status: "done"},
	}
	got := map[string]bool{}
	for _, p := range Check(ss, env()) {
		got[p.Step+" "+p.Rule] = true
	}
	for _, want := range []string{"a.md C1", "b.md C2", "b.md C4", "b.md C5", "b.md C6", "d.md C5", "d.md C3"} {
		if !got[want] {
			t.Errorf("no problem %s in %v", want, got)
		}
	}
}
