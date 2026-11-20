package index

import (
	"reflect"
	"testing"
)

func TestPrivate(t *testing.T) {
	ix, err := Build("testdata/src")
	if err != nil {
		t.Fatal(err)
	}
	g := NewGraph(ix)
	mid := g.ID["Lib.mid"]
	if len(g.Uses[mid]) != 1 || g.Decls[g.Uses[mid][0]].Module != "Lib.Mid" {
		t.Errorf("mid must use the helper of its own module, got %v", g.Uses[mid])
	}
}

func TestSpine(t *testing.T) {
	ix, err := Build("testdata/src")
	if err != nil {
		t.Fatal(err)
	}
	g := NewGraph(ix)
	root, _ := g.Lookup("riemannHypothesis")
	sp := BuildSpine(g, root, SpineOptions{})
	// riemannHypothesis, top, mid, helper (Mid), base, and the cut big_of_literature. deep is under the cut.
	if sp.Size != 6 || sp.Full != 7 || sp.Cuts["literature"] != 1 {
		t.Errorf("spine size %d full %d cuts %v", sp.Size, sp.Full, sp.Cuts)
	}
	if !reflect.DeepEqual(sp.Trunk, []string{"riemannHypothesis", "Lib.top", "Lib.mid", "Lib.helper"}) {
		t.Errorf("trunk = %v", sp.Trunk)
	}
	full := BuildSpine(g, root, SpineOptions{NoCut: true})
	p := g.Path(root, g.ID["Lib.Low.base"], false)
	if len(p) != 5 {
		t.Errorf("path = %v", p)
	}
	for _, n := range full.Nodes {
		if n.Name == "Lib.Low.base" && (n.Idom != "Lib.top" || n.Fanin != 2) {
			t.Errorf("base: %+v", n)
		}
	}
}
