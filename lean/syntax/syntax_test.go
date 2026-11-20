package syntax

import (
	"reflect"
	"testing"
)

const sample = `import Mathlib
import Foo.Bar

/-! A module doc. It is not a doc comment of a declaration. -/

open Complex DirichletCharacter

namespace A.B

/-- The first theorem. -/
@[simp] theorem one (h : x.re = 1) : f 1 = Nat.succ 0 := by
  rw [h.trans]
  exact (C.two .inl)

private lemma two : True := trivial

noncomputable section
open Inner in
def three : ℕ := one -- one is A.B.one
end

instance : Inhabited Nat := ⟨0⟩

instance (priority := 100) namedInst : Foo := sorry

structure S where
  mk' ::
  x y : ℕ
  z : ℕ := 0

inductive T
  | a | b (n : ℕ)

theorem _root_.top : |x| ≤ 1 := sorry

end A.B

example : True := trivial

theorem «weird name» : True := trivial
`

func TestParse(t *testing.T) {
	f := Parse([]byte(sample))
	if !reflect.DeepEqual(f.Imports, []string{"Mathlib", "Foo.Bar"}) {
		t.Errorf("imports = %v", f.Imports)
	}
	if f.Examples != 1 {
		t.Errorf("examples = %d", f.Examples)
	}
	want := []struct{ kind, name string }{
		{"theorem", "A.B.one"}, {"lemma", "A.B.two"}, {"def", "A.B.three"}, {"instance", "A.B.instance_L22"},
		{"instance", "A.B.namedInst"}, {"structure", "A.B.S"}, {"inductive", "A.B.T"}, {"theorem", "top"}, {"theorem", "weird name"},
	}
	if len(f.Decls) != len(want) {
		for _, d := range f.Decls {
			t.Logf("%s %s", d.Kind, d.Name)
		}
		t.Fatalf("got %d declarations, want %d", len(f.Decls), len(want))
	}
	for i, w := range want {
		if f.Decls[i].Kind != w.kind || f.Decls[i].Name != w.name {
			t.Errorf("decl %d = %s %s, want %s %s", i, f.Decls[i].Kind, f.Decls[i].Name, w.kind, w.name)
		}
	}
	one := f.Decls[0]
	if one.Doc != "The first theorem." || one.Attrs != "@[simp]" || one.Line != 11 {
		t.Errorf("one: doc %q attrs %q line %d", one.Doc, one.Attrs, one.Line)
	}
	if one.Sig != "(h : x.re = 1) : f 1 = Nat.succ 0" {
		t.Errorf("one: sig %q", one.Sig)
	}
	idents := map[string]Ident{}
	for _, id := range one.Idents {
		idents[id.Name] = id
	}
	if !idents["Nat.succ"].Sig || idents["h.trans"].Sig || !idents["inl"].Dot {
		t.Errorf("one: idents %v", one.Idents)
	}
	if _, ok := idents["rw"]; ok {
		t.Errorf("one: the tactic rw is not an identifier")
	}
	if !reflect.DeepEqual(one.Scope.Namespaces, []string{"A", "A.B"}) || len(one.Scope.Opens) != 2 {
		t.Errorf("one: scope %+v", one.Scope)
	}
	if !f.Decls[1].Private {
		t.Errorf("two is private")
	}
	three := f.Decls[2]
	if len(three.Scope.Opens) != 3 || three.Scope.Opens[2].Name != "Inner" || three.Scope.Opens[2].At != "A.B" {
		t.Errorf("three: opens %+v", three.Scope.Opens)
	}
	if len(f.Decls[3].Scope.Opens) != 2 {
		t.Errorf("open ... in must end after one command: %+v", f.Decls[3].Scope.Opens)
	}
	if !reflect.DeepEqual(f.Decls[5].Children, []string{"A.B.S.mk'", "A.B.S.x", "A.B.S.y", "A.B.S.z"}) {
		t.Errorf("S children = %v", f.Decls[5].Children)
	}
	if !reflect.DeepEqual(f.Decls[6].Children, []string{"A.B.T.a", "A.B.T.b"}) {
		t.Errorf("T children = %v", f.Decls[6].Children)
	}
	if f.Decls[7].Sig != ": |x| ≤ 1" {
		t.Errorf("top: sig %q", f.Decls[7].Sig)
	}
	if len(f.Decls[7].Scope.Namespaces) != 2 || len(f.Decls[8].Scope.Namespaces) != 0 {
		t.Errorf("end A.B must close the namespace")
	}
}

func TestLexUnicode(t *testing.T) {
	toks := lex([]byte("χ.IsPrimitive ℂ x₁ h' 'a' \"s -- t\" /- c /- d -/ -/ λ"))
	var got []string
	for _, x := range toks {
		got = append(got, x.text)
	}
	want := []string{"χ.IsPrimitive", "ℂ", "x₁", "h'", "'a'", "\"s -- t\"", "λ"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokens = %q", got)
	}
}
