package syntax

import (
	"sort"
	"strings"
	"testing"
)

const localsSample = `namespace A

theorem green_of_regular (hG : Pub.Green) [Field K] [inst : Ring R] : B.Green := by
  intro g hg L
  obtain ⟨τG, h1⟩ := hG g
  have hreg : ∀ P, IsPrime P → Good P := fun P hP => by
    exact foo hP
  rcases lemmaOne g with ⟨hc, hdeg⟩
  choose f hf using lemmaTwo
  simp [ext, hc.1]
  exact ⟨hc, Real⟩

theorem plain : RiemannHypothesis := rhOfAll

def rec : Nat → Nat
  | 0 => zero
  | succ n => rec n

def inst : S := { toFun := mapOne, bound := 3 }

end A
`

func localNames(d Decl) []string {
	var out []string
	for _, id := range d.Idents {
		if id.Local {
			out = append(out, id.Name)
		}
	}
	sort.Strings(out)
	return out
}

func TestLocals(t *testing.T) {
	f := Parse([]byte(localsSample))
	got := map[string]string{}
	for _, d := range f.Decls {
		got[d.Name] = strings.Join(localNames(d), " ")
	}
	want := map[string]string{
		"A.green_of_regular": "L P f g h1 hG hP hc hdeg hf hg hreg inst τG",
		"A.plain":            "",
		"A.rec":              "n",
		"A.inst":             "bound toFun",
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: local names %q, want %q", name, got[name], w)
		}
	}
	// Names that must stay uses of declarations.
	for _, d := range f.Decls {
		for _, id := range d.Idents {
			switch id.Name {
			case "Pub.Green", "Field", "Ring", "B.Green", "IsPrime", "Good", "foo", "lemmaOne", "lemmaTwo", "ext", "Real", "RiemannHypothesis", "rhOfAll", "zero", "succ", "mapOne", "S":
				if id.Local {
					t.Errorf("%s: %s is marked local", d.Name, id.Name)
				}
			}
		}
	}
}

func TestLocalsNestedType(t *testing.T) {
	f := Parse([]byte("theorem t (b : B) : ∀ (hP : Cov (faceT b (zsc b)) ok) (lam0 : ℝ), P lam0 := foo\n"))
	for _, id := range f.Decls[0].Idents {
		switch id.Name {
		case "faceT", "zsc", "Cov", "ok", "P", "foo", "B":
			if id.Local {
				t.Errorf("%s is marked local", id.Name)
			}
		case "hP", "lam0", "b":
			if !id.Local {
				t.Errorf("%s is not marked local", id.Name)
			}
		}
	}
}
