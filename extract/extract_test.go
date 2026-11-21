package extract

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/tamnd/riemann-agent/index"
)

// consts is a part of the output of the Lean file on a small project, from a run with Lean v4.33.1.
const consts = `{"kind":"inductive","module":"T.A","name":"T.Pt","private":false,"lines":[2],"uses":["T.Pt.mk"]}
{"kind":"ctor","module":"T.A","name":"T.Pt.mk","private":false,"uses":["Nat","T.Pt"]}
{"kind":"def","module":"T.A","name":"T.Pt.x","private":false,"uses":["T.Pt","Nat"]}
{"kind":"def","module":"T.A","name":"T.norm1","private":false,"lines":[6],"uses":["T.Pt","Nat","T.Pt.x"]}
{"kind":"def","module":"T.A","name":"T.pick.match_1","private":false,"uses":["Nat","Nat.casesOn"]}
{"kind":"def","module":"T.A","name":"T.pick","private":false,"lines":[8],"uses":["Nat","T.pick.match_1"]}
{"kind":"def","module":"T.A","name":"T.helper","private":true,"lines":[12],"uses":["Nat"]}
{"kind":"def","module":"T.A","name":"T.useHelper","private":false,"lines":[14],"uses":["Nat","T.helper"]}
{"kind":"def","module":"T.A","name":"T.instInhabitedPt","private":false,"lines":[16],"uses":["Inhabited","T.Pt","T.Pt.mk"]}
{"kind":"def","module":"T.A","name":"T.origin","private":false,"lines":[18],"uses":["T.Pt","Inhabited.default","T.instInhabitedPt"]}
{"kind":"def","module":"T.B","name":"T.helper","private":true,"lines":[3],"uses":["T.norm1"]}
{"kind":"def","module":"T.B","name":"T.top","private":false,"lines":[4],"uses":["T.helper","T.Pt.mk"]}
`

func decl(name, module, kind string, line int, private bool, uses ...string) index.Decl {
	return index.Decl{Name: name, Module: module, Kind: kind, Line: line, Private: private, Uses: uses, Weak: []string{"T.weak"}, Graph: "syntactic"}
}

func TestMerge(t *testing.T) {
	cs, err := Read(strings.NewReader(consts))
	if err != nil {
		t.Fatal(err)
	}
	ix := &index.Index{Decls: []index.Decl{
		decl("T.Pt", "T.A", "structure", 2, false),
		decl("T.norm1", "T.A", "def", 6, false, "T.Pt"),
		decl("T.pick", "T.A", "def", 8, false),
		decl("T.helper", "T.A", "def", 12, true),
		decl("T.useHelper", "T.A", "def", 14, false, "T.helper"),
		decl("T.instance_L16", "T.A", "instance", 16, false, "T.Pt"),
		decl("T.origin", "T.A", "def", 18, false, "T.Pt"),
		decl("T.helper", "T.B", "def", 3, true, "T.norm1"),
		decl("T.top", "T.B", "theorem", 4, false, "T.helper", "T.gone"),
		decl("T.notBuilt", "T.B", "def", 9, false, "T.Pt"),
	}}
	st := Merge(ix, cs)
	want := map[string]string{
		"T.Pt":           "",
		"T.norm1":        "T.Pt",
		"T.pick":         "",
		"T.useHelper":    "T.helper",
		"T.instance_L16": "T.Pt",
		"T.origin":       "T.Pt,T.instance_L16",
		"T.top":          "T.Pt,T.helper",
		"T.notBuilt":     "T.Pt",
		"T.helper@T.B":   "T.norm1",
		"T.helper@T.A":   "",
	}
	for _, d := range ix.Decls {
		k := d.Name
		if d.Private {
			k += "@" + d.Module
		}
		if got := strings.Join(d.Uses, ","); got != want[k] {
			t.Errorf("%s: uses %q, want %q", k, got, want[k])
		}
		wantGraph := "exact"
		if d.Name == "T.notBuilt" {
			wantGraph = "syntactic"
		}
		if d.Graph != wantGraph {
			t.Errorf("%s: graph %s, want %s", k, d.Graph, wantGraph)
		}
		if d.Graph == "exact" && len(d.Weak) != 0 {
			t.Errorf("%s: an exact record keeps weak edges %v", k, d.Weak)
		}
	}
	if st.Matched != 9 || st.Missing != 1 || st.Constants != 12 || st.Auxiliary != 3 {
		t.Errorf("stats %+v", st)
	}
	// Added: origin -> instance_L16, top -> T.Pt. Removed: top -> T.gone.
	if st.Added != 2 || st.Removed != 1 {
		t.Errorf("added %d and removed %d, want 2 and 1", st.Added, st.Removed)
	}
	if ix.Summary.Weak != 1 {
		t.Errorf("summary weak %d, want 1 (the record with no constant)", ix.Summary.Weak)
	}
}

func TestReadGzip(t *testing.T) {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	_, _ = zw.Write([]byte(consts))
	_ = zw.Close()
	cs, err := Read(&b)
	if err != nil || len(cs) != 12 || cs[6].Name != "T.helper" || !cs[6].Private {
		t.Fatalf("got %d constants, %v", len(cs), err)
	}
}

func TestFile(t *testing.T) {
	f := File("Main", []string{"A", "B"}, "/tmp/out.jsonl")
	for _, s := range []string{"import Main\n", "[`A, `B]", `"/tmp/out.jsonl"`, "findDeclarationRanges?", "privateToUserName?"} {
		if !strings.Contains(f, s) {
			t.Errorf("the file has no %q", s)
		}
	}
}
