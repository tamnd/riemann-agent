package index

import (
	"reflect"
	"testing"
)

func decl(t *testing.T, ix *Index, name string) Decl {
	t.Helper()
	for _, d := range ix.Decls {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("no declaration %s", name)
	return Decl{}
}

func TestBuild(t *testing.T) {
	ix, err := Build("testdata/src")
	if err != nil {
		t.Fatal(err)
	}
	s := ix.Summary
	if s.Modules != 4 || s.Decls != 9 || s.LongestChain != 4 || s.Unreachable != 0 || !reflect.DeepEqual(s.ExternalMods, []string{"Mathlib"}) {
		t.Errorf("summary = %+v", s)
	}
	if got := decl(t, ix, "Lib.top").Uses; !reflect.DeepEqual(got, []string{"Lib.Lit.big_of_literature", "Lib.mid"}) {
		t.Errorf("top uses %v", got)
	}
	if got := decl(t, ix, "Lib.top").Weak; !reflect.DeepEqual(got, []string{"Lib.Low.trans"}) {
		t.Errorf("top weak %v", got)
	}
	if got := decl(t, ix, "Lib.Lit.big_of_literature").Uses; !reflect.DeepEqual(got, []string{"Lib.Low.deep"}) {
		t.Errorf("big uses %v", got)
	}
	if got := decl(t, ix, "Lib.Low.deep").Uses; !reflect.DeepEqual(got, []string{"Lib.Low.base"}) {
		t.Errorf("deep uses %v", got)
	}
	dir := t.TempDir()
	if err := ix.Write(dir); err != nil {
		t.Fatal(err)
	}
	back, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Decls, ix.Decls) || len(back.Modules) != 4 {
		t.Errorf("the records do not read back the same")
	}
}
