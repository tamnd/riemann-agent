package read

import (
	"encoding/json"
	"testing"
)

func TestRead(t *testing.T) {
	r := Read("theorem", "Foo.exp_pos", "(x : ℝ) (hx : 0 < x) {n m : ℕ} [Field K] : Real.exp x ≠ 0")
	if len(r.Given) != 2 || r.Given[0].Text != "x is a real number" || r.Given[1].Text != "n and m are natural numbers" {
		t.Errorf("given: %+v", r.Given)
	}
	if len(r.Assume) != 2 || r.Assume[0].Text != "`x` is positive" || r.Assume[1].Text != "`K` has the structure field" {
		t.Errorf("assume: %+v", r.Assume)
	}
	if r.Then == nil || r.Then.Text != "`Real.exp x` is not zero" {
		t.Errorf("then: %+v", r.Then)
	}
	if r.Name != "exponential positive" {
		t.Errorf("name: %q", r.Name)
	}
}

func TestReadForall(t *testing.T) {
	r := Read("theorem", "T.x", ": ∀ (F E : Type) [Field F] (σ : E →ₐ[F] E), IsGood σ → ∀ e, P e → Q e")
	b, _ := json.Marshal(r)
	if len(r.Given) != 3 || len(r.Assume) != 3 || r.Then == nil || r.Then.Lean != "Q e" {
		t.Errorf("reading: %s", b)
	}
	if r.Given[0].Text != "F and E are types" {
		t.Errorf("given: %+v", r.Given[0])
	}
}

func TestReadDef(t *testing.T) {
	r := Read("def", "A.f", "(n : ℕ) : Fin n → ℝ")
	if r.Then == nil || r.Then.Text != "a function from `Fin n` to `ℝ`" {
		t.Errorf("then: %+v", r.Then)
	}
	if r.Summary != "A definition that takes one input and makes a value of the type below." {
		t.Errorf("summary: %q", r.Summary)
	}
	if got := Read("theorem", "M.meanVal_holds", ": MeanVal"); got.Summary != "A theorem with no hypotheses. It states the result below." {
		t.Errorf("summary: %q", got.Summary)
	}
}

func TestWords(t *testing.T) {
	for in, want := range map[string]string{"IsRegularLocalRing": "is regular local ring", "everyCharacter": "every character", "GL": "GL", "LSeries": "L series"} {
		if got := words(in); got != want {
			t.Errorf("words(%q) = %q, want %q", in, got, want)
		}
	}
}
