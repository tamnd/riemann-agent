package trust

import (
	"strings"
	"testing"
)

// out is the output of the Lean file of File on a small project, from a run with Lean v4.33.1.
const out = `/tmp/rh-trust-1/RhAxioms.lean:28:0: warning: declaration uses 'sorry'
'good' depends on axioms: [propext, Classical.choice, Quot.sound]
'bad' depends on axioms: [sorryAx]
'uses' depends on axioms: [myAx]
'pure' does not depend on any axioms
'long' depends on axioms: [propext,
 Classical.choice,
 Quot.sound]
rh-walk good: Classical.choice, Quot.sound, propext
rh-walk bad: sorryAx
rh-walk uses: myAx
rh-walk pure: 
rh-walk long: Classical.choice, Quot.sound
rh-type riemannHypothesis: RiemannHypothesis
rh-type riemannZeta_eq_zero_re: True
`

func TestParse(t *testing.T) {
	ts, err := Parse([]byte(out), []string{"good", "bad", "uses", "pure", "long"})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name  string
		print string
		match bool
		extra string
	}{
		{"good", "Classical.choice,Quot.sound,propext", true, ""},
		{"bad", "sorryAx", true, "sorryAx"},
		{"uses", "myAx", true, "myAx"},
		{"pure", "", true, ""},
		{"long", "Classical.choice,Quot.sound,propext", false, ""},
	}
	for i, w := range want {
		th := ts[i]
		if th.Name != w.name || strings.Join(th.Print, ",") != w.print || th.Match != w.match || strings.Join(th.Extra, ",") != w.extra {
			t.Errorf("theorem %d: got %+v, want %+v", i, th, w)
		}
	}
	if OK(ts) {
		t.Error("OK is true for a list with sorryAx")
	}
	if !OK(ts[:1]) || !OK(ts[3:4]) {
		t.Error("OK is false for standard axioms only")
	}
	if OK(nil) {
		t.Error("OK is true for no theorems")
	}
}

func TestParseType(t *testing.T) {
	text := out + "'riemannHypothesis' depends on axioms: [propext]\n'riemannZeta_eq_zero_re' depends on axioms: [propext]\nrh-walk riemannHypothesis: propext\nrh-walk riemannZeta_eq_zero_re: propext\n"
	ts, err := Parse([]byte(text), []string{"riemannHypothesis", "riemannZeta_eq_zero_re", "good"})
	if err != nil {
		t.Fatal(err)
	}
	if !ts[0].TypeOK || ts[0].Type != "RiemannHypothesis" {
		t.Errorf("riemannHypothesis: got %+v, want the type RiemannHypothesis", ts[0])
	}
	if ts[1].TypeOK || ts[1].Type != "True" || ts[1].Want != "RiemannZetaZerosOnLine" {
		t.Errorf("riemannZeta_eq_zero_re: got %+v, want a wrong type", ts[1])
	}
	if !ts[2].TypeOK {
		t.Error("a theorem with no expected type must pass the type check")
	}
	if OK(ts[:2]) || !OK(ts[:1]) {
		t.Error("OK must fail on a wrong type only")
	}
}

func TestParseMissing(t *testing.T) {
	_, err := Parse([]byte(out), []string{"good", "absent"})
	if err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("got %v, want an error that names absent", err)
	}
}

func TestFile(t *testing.T) {
	f := File("Main", []string{"a", "b.c"})
	for _, s := range []string{"import Main\n", "#print axioms a\n", "#print axioms b.c\n", "[`a, `b.c]", "rh-walk {n}:", "rh-type {n}:", "def rhAxiomWalk"} {
		if !strings.Contains(f, s) {
			t.Errorf("the file has no %q", s)
		}
	}
	if strings.Contains(f, "collectAxioms ") {
		t.Error("the walk must not call Lean.collectAxioms")
	}
}
