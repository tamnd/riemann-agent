package steps

import (
	"strings"
	"testing"

	"github.com/tamnd/riemann-agent/index"
)

func TestPathsAndGlossary(t *testing.T) {
	ss, err := Load("testdata/ok")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := Chapters("testdata/ok")
	if err != nil || len(ch) != 1 || ch[0] != "trunk" {
		t.Fatal(ch, err)
	}
	ps, err := LoadPaths("testdata/ok")
	if err != nil || len(ps) != 2 {
		t.Fatal(ps, err)
	}
	if got := CheckPaths(ps, ch, ss, env()); len(got) != 0 {
		t.Errorf("paths: %v", got)
	}
	if o := Order([]*Path{ps[1], ps[0]}, ch); o[0].ID != "base" || o[1].ID != "next" {
		t.Errorf("order: %s %s", o[0].ID, o[1].ID)
	}
	o := Order([]*Path{{ID: "g", For: []string{"trunk"}}, {ID: "s2", Order: 2}, {ID: "s1", Order: 1}}, ch)
	if o[0].ID != "s1" || o[1].ID != "s2" || o[2].ID != "g" {
		t.Errorf("order with Order: %s %s %s", o[0].ID, o[1].ID, o[2].ID)
	}
	gl, err := LoadGlossary("testdata/ok")
	if err != nil || len(gl) != 2 {
		t.Fatal(gl, err)
	}
	e := env()
	e.Graph = index.NewGraph(&index.Index{Decls: append(append([]index.Decl(nil), graph().Decls...), index.Decl{Name: "Space.inner", Kind: "def"})})
	if got := CheckGlossary(gl, ss, e); len(got) != 0 {
		t.Errorf("glossary: %v", got)
	}
}

func TestPathRules(t *testing.T) {
	ps := []*Path{
		{ID: "a", File: "paths/b.yaml", For: []string{"none"}, Needs: []string{"c"}, Items: []Item{{Math2: "ZZZZ", Steps: []string{"nope"}}, {Concept: "x", Why: "w"}}},
		{ID: "c", File: "paths/c.yaml", Title: "C", Needs: []string{"a"}, Items: []Item{{Concept: "y", Why: "w", Math2: "AAAA"}}},
	}
	got := CheckPaths(ps, []string{"trunk"}, nil, env())
	var rules []string
	for _, p := range got {
		rules = append(rules, p.Rule)
	}
	want := "P1 P1 P2 P2 P3 P3 P4 P5 P4 P2"
	if s := strings.Join(rules, " "); s != want {
		t.Errorf("rules: %s, want %s\n%v", s, want, got)
	}
	body := []*Path{{ID: "d", File: "paths/d.yaml", BodyFile: "paths/d.md", Title: "D", Body: "## The Zeta function\n\nText.", Items: []Item{{Concept: "zeta function", Why: "w", Math2: "AAAA"}, {Concept: "Gauss sum", Why: "w", Math2: "AAAA"}}}}
	if got := CheckPaths(body, nil, nil, env()); len(got) != 1 || got[0].Rule != "P6" || !strings.Contains(got[0].Text, "Gauss sum") {
		t.Errorf("written chapter: %v", got)
	}
	gl := []Entry{{Name: "top", Kind: "def", Says: "x", Step: "none", Status: "done", Math2: "ZZZZ"}, {Name: "top", Says: "y"}, {Name: "No.", Kind: "namespace", Says: "z"}, {Name: "gone"}}
	got = CheckGlossary(gl, nil, env())
	rules = nil
	for _, p := range got {
		rules = append(rules, p.Rule)
	}
	if s := strings.Join(rules, " "); s != "G2 G3 G4 G4 G1 G2 G1 G2" {
		t.Errorf("glossary rules: %s\n%v", s, got)
	}
}
