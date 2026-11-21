// Package trust runs the axiom check and the kernel replay on a built Lean project, and writes the result for the trust report (spec 05.3 and 05.4).
//
// The axiom check runs two independent methods. The first is the Lean command #print axioms. The second is a walk of our own: Lean meta code that starts at each theorem, follows each constant in the types and values, and lists each axiom declaration that it finds. The two lists must be the same.
package trust

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Standard is the set of the three standard axioms of Mathlib.
var Standard = []string{"Classical.choice", "Quot.sound", "propext"}

// MainTheorems are the five theorems of Main.lean in riemann-lean.
var MainTheorems = []string{
	"riemannHypothesis",
	"riemannZeta_eq_zero_re",
	"LFunction_ne_zero_of_half_lt_re",
	"LFunction_eq_zero_re",
	"LFunction_eq_zero_re_of_isPrimitive",
}

// Expected gives the type that each theorem of MainTheorems must have: a Prop of Mathlib or of Statements.lean. Spec 05.5 reviews these Props.
var Expected = map[string]string{
	"riemannHypothesis":                   "RiemannHypothesis",
	"riemannZeta_eq_zero_re":              "RiemannZetaZerosOnLine",
	"LFunction_ne_zero_of_half_lt_re":     "DirichletLNonvanishingRightOfHalf",
	"LFunction_eq_zero_re":                "DirichletLZerosOnLine",
	"LFunction_eq_zero_re_of_isPrimitive": "PrimitiveDirichletLZerosOnLine",
}

// Theorem is the axiom result of one theorem.
type Theorem struct {
	Name   string   `json:"name"`
	Print  []string `json:"print_axioms"` // the axioms that #print axioms lists, sorted
	Walk   []string `json:"walk"`         // the axioms that the walk of rh finds, sorted
	Match  bool     `json:"match"`        // Print and Walk are the same
	Extra  []string `json:"extra"`        // the axioms that are not standard
	Type   string   `json:"type"`         // the type of the theorem in the environment
	Want   string   `json:"want,omitempty"`
	TypeOK bool     `json:"type_ok"` // Type is Want, or the theorem has no expected type
}

// Axioms is the result of the axiom check, records/axioms.json.
type Axioms struct {
	Date     string    `json:"date"`
	Module   string    `json:"module"`
	Lean     string    `json:"lean"`
	Seconds  float64   `json:"seconds"`
	Theorems []Theorem `json:"theorems"`
	OK       bool      `json:"ok"` // each theorem has only standard axioms, and the two methods agree
}

// Replay is the result of lean4checker, records/replay.json.
type Replay struct {
	Date    string  `json:"date"`
	Tool    string  `json:"tool"`
	Module  string  `json:"module"`
	Seconds float64 `json:"seconds"`
	Exit    int     `json:"exit"`
	Tail    string  `json:"tail"` // the last lines of the output
	OK      bool    `json:"ok"`
}

// File returns the Lean file of the axiom check. It imports module and checks each theorem in names with both methods.
func File(module string, names []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "import %s\nimport Lean\n\nopen Lean Elab Command\n\n", module)
	b.WriteString(`/-- The axioms that the constant root depends on, found by a walk of the types and values of all constants that it uses. This code does not use Lean.collectAxioms. -/
def rhAxiomWalk (env : Environment) (root : Name) : Array Name := Id.run do
  let mut seen : NameHashSet := {}
  let mut todo : Array Name := #[root]
  let mut out : Array Name := #[]
  while !todo.isEmpty do
    let n := todo.back!
    todo := todo.pop
    if seen.contains n then continue
    seen := seen.insert n
    match env.find? n with
    | some (.axiomInfo _) => out := out.push n
    | some (.inductInfo v) =>
      todo := todo ++ v.type.getUsedConstants ++ v.ctors.toArray
    | some ci =>
      todo := todo ++ ci.type.getUsedConstants
      if let some e := ci.value? (allowOpaque := true) then
        todo := todo ++ e.getUsedConstants
    | none => pure ()
  return out

`)
	for _, n := range names {
		fmt.Fprintf(&b, "#print axioms %s\n", n)
	}
	b.WriteString("\n#eval show CommandElabM Unit from do\n  let env ← getEnv\n")
	b.WriteString("  for n in [")
	for i, n := range names {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "`%s", n)
	}
	b.WriteString("] do\n")
	b.WriteString("    let ax := (rhAxiomWalk env n).map toString |>.qsort (· < ·)\n")
	b.WriteString("    IO.println s!\"rh-walk {n}: {\", \".intercalate ax.toList}\"\n")
	b.WriteString("    if let some ci := env.find? n then\n")
	b.WriteString("      IO.println s!\"rh-type {n}: {(toString ci.type).replace \"\\n\" \" \"}\"\n")
	return b.String()
}

var (
	rePrint = regexp.MustCompile(`'([^']+)' depends on axioms: \[([^\]]*)\]`)
	reNone  = regexp.MustCompile(`'([^']+)' does not depend on any axioms`)
	reWalk  = regexp.MustCompile(`^rh-walk ([^:]+): ?(.*)$`)
	reType  = regexp.MustCompile(`^rh-type ([^:]+): ?(.*)$`)
)

// Parse reads the output of the Lean file of File. It returns the result of each theorem in names, in that order.
func Parse(out []byte, names []string) ([]Theorem, error) {
	// #print axioms can wrap a long list over lines, so join the lines first.
	text := strings.Join(strings.Fields(string(out)), " ")
	pr := map[string][]string{}
	for _, m := range rePrint.FindAllStringSubmatch(text, -1) {
		pr[m[1]] = split(m[2])
	}
	for _, m := range reNone.FindAllStringSubmatch(text, -1) {
		pr[m[1]] = []string{}
	}
	walk := map[string][]string{}
	types := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		if m := reWalk.FindStringSubmatch(strings.TrimSpace(sc.Text())); m != nil {
			walk[m[1]] = split(m[2])
		} else if m := reType.FindStringSubmatch(strings.TrimSpace(sc.Text())); m != nil {
			types[m[1]] = strings.TrimSpace(m[2])
		}
	}
	var ts []Theorem
	var missing []string
	for _, n := range names {
		p, ok1 := pr[n]
		w, ok2 := walk[n]
		if !ok1 || !ok2 {
			missing = append(missing, n)
			continue
		}
		want := Expected[n]
		ts = append(ts, Theorem{Name: n, Print: p, Walk: w, Match: same(p, w), Extra: extra(p, w), Type: types[n], Want: want, TypeOK: want == "" || types[n] == want})
	}
	if len(missing) > 0 {
		return ts, fmt.Errorf("the output has no result for %s", strings.Join(missing, ", "))
	}
	return ts, nil
}

// OK reports whether each theorem has only standard axioms, the two methods agree, and the type is the expected one.
func OK(ts []Theorem) bool {
	if len(ts) == 0 {
		return false
	}
	for _, t := range ts {
		if !t.Match || len(t.Extra) > 0 || !t.TypeOK {
			return false
		}
	}
	return true
}

func split(s string) []string {
	out := []string{}
	for f := range strings.SplitSeq(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func extra(lists ...[]string) []string {
	std := map[string]bool{}
	for _, s := range Standard {
		std[s] = true
	}
	seen := map[string]bool{}
	out := []string{}
	for _, l := range lists {
		for _, a := range l {
			if !std[a] && !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Options are the options of the checks.
type Options struct {
	Dir     string        // the root of the Lean project, with lakefile.toml
	Lake    string        // the lake program; "lake" when empty
	Module  string        // the module with the theorems; "Main" when empty
	Names   []string      // the theorems; MainTheorems when empty
	Timeout time.Duration // the time limit of each run; no limit when zero
}

func (o Options) lake() string {
	if o.Lake == "" {
		return "lake"
	}
	return o.Lake
}

func (o Options) module() string {
	if o.Module == "" {
		return "Main"
	}
	return o.Module
}

func (o Options) names() []string {
	if len(o.Names) == 0 {
		return MainTheorems
	}
	return o.Names
}

func (o Options) run(ctx context.Context, args ...string) ([]byte, int, time.Duration, error) {
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, o.lake(), args...) // #nosec G204 -- the program and arguments come from the flags of rh
	cmd.Dir = o.Dir
	start := time.Now()
	out, err := cmd.CombinedOutput()
	d := time.Since(start)
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return out, ee.ExitCode(), d, nil
	}
	if err != nil {
		return out, -1, d, err
	}
	return out, 0, d, nil
}

// CheckAxioms writes the Lean file of File, runs it with lake env lean, and reads the result.
func CheckAxioms(ctx context.Context, o Options) (Axioms, []byte, error) {
	a := Axioms{Date: time.Now().UTC().Format("2006-01-02"), Module: o.module()}
	if v, _, _, err := o.run(ctx, "env", "lean", "--version"); err == nil {
		a.Lean = strings.TrimSpace(string(v))
	}
	tmp, err := os.MkdirTemp("", "rh-trust-")
	if err != nil {
		return a, nil, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	path := filepath.Join(tmp, "RhAxioms.lean")
	if err := os.WriteFile(path, []byte(File(o.module(), o.names())), 0o600); err != nil {
		return a, nil, err
	}
	out, code, d, err := o.run(ctx, "env", "lean", path)
	a.Seconds = d.Round(time.Second).Seconds()
	if err != nil {
		return a, out, err
	}
	ts, perr := Parse(out, o.names())
	a.Theorems = ts
	a.OK = perr == nil && OK(ts)
	if code != 0 && perr == nil {
		perr = fmt.Errorf("lean exited with code %d", code)
		a.OK = false
	}
	return a, out, perr
}

// CheckReplay runs lean4checker on the module with lake env. The program checker is the path of the lean4checker binary.
func CheckReplay(ctx context.Context, o Options, checker string) (Replay, error) {
	r := Replay{Date: time.Now().UTC().Format("2006-01-02"), Tool: "lean4checker --fresh", Module: o.module()}
	out, code, d, err := o.run(ctx, "env", checker, "--fresh", o.module())
	r.Seconds = d.Round(time.Second).Seconds()
	r.Exit = code
	r.Tail = tail(out, 20)
	r.OK = err == nil && code == 0
	return r, err
}

func tail(b []byte, n int) string {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
