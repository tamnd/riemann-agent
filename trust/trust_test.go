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

func TestParseMissing(t *testing.T) {
	_, err := Parse([]byte(out), []string{"good", "absent"})
	if err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("got %v, want an error that names absent", err)
	}
}

func TestFile(t *testing.T) {
	f := File("Main", []string{"a", "b.c"})
	for _, s := range []string{"import Main\n", "#print axioms a\n", "#print axioms b.c\n", "[`a, `b.c]", "rh-walk {n}:", "def rhAxiomWalk"} {
		if !strings.Contains(f, s) {
			t.Errorf("the file has no %q", s)
		}
	}
	if strings.Contains(f, "collectAxioms ") {
		t.Error("the walk must not call Lean.collectAxioms")
	}
}
