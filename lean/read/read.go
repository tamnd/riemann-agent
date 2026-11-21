// Package read makes a plain English reading of the statement of a Lean declaration. It splits the signature into the objects that the statement takes, the hypotheses and the result, and it gives a short gloss for the common forms. The reading does not elaborate, so it is a guide to the statement and not a replacement for it. The Lean text stays next to it on each page.
package read

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Item is one part of a statement: the Lean text and a gloss in English. Text is "" when the reader has no gloss.
type Item struct {
	Lean string `json:"l"`
	Text string `json:"t,omitempty"`
}

// Reading is the plain reading of a declaration.
type Reading struct {
	Summary string `json:"s"`           // one sentence that says what the declaration is
	Name    string `json:"n,omitempty"` // the words of the last part of the name
	Given   []Item `json:"g,omitempty"` // the objects
	Assume  []Item `json:"a,omitempty"` // the hypotheses
	Then    *Item  `json:"c,omitempty"` // the result of a theorem, or the type of a definition
}

// Read reads the signature of a declaration of the given kind and name.
func Read(kind, name, sig string) Reading {
	binders, concl := split(sig)
	var r Reading
	r.Name = nameWords(name)
	add := func(b binder) {
		switch {
		case b.inst:
			r.Assume = append(r.Assume, Item{Lean: "[" + b.typ + "]", Text: instText(b.typ)})
		case isProp(b.names, b.typ):
			r.Assume = append(r.Assume, Item{Lean: b.typ, Text: propText(b.typ)})
		default:
			r.Given = append(r.Given, Item{Lean: strings.Join(b.names, " ") + " : " + b.typ, Text: givenText(b.names, b.typ)})
		}
	}
	for _, b := range binders {
		add(b)
	}
	// A theorem stated as ∀ ..., P → Q: read the leading binders and premises as the binders of the signature.
	if isProof(kind) {
		for {
			more, rest, ok := leadingForall(concl)
			if ok {
				for _, b := range more {
					add(b)
				}
				concl = rest
			}
			parts := splitTop(concl, " → ")
			if len(parts) > 1 && !ok && len(more) == 0 {
				for _, p := range parts[:len(parts)-1] {
					p = strings.TrimSpace(p)
					r.Assume = append(r.Assume, Item{Lean: p, Text: propText(p)})
				}
				concl = strings.TrimSpace(parts[len(parts)-1])
				continue
			}
			if !ok {
				break
			}
		}
	}
	if concl != "" {
		t := Item{Lean: concl}
		if isProof(kind) {
			t.Text = propText(concl)
		} else {
			t.Text = typeText(concl)
		}
		r.Then = &t
	}
	r.Summary = summary(kind, &r)
	return r
}

func isProof(kind string) bool { return kind == "theorem" || kind == "lemma" }

func summary(kind string, r *Reading) string {
	n := len(r.Given)
	h := len(r.Assume)
	objs := plural(n, "object", "objects")
	hyps := plural(h, "hypothesis", "hypotheses")
	switch kind {
	case "theorem", "lemma":
		switch {
		case n == 0 && h == 0:
			return "A theorem with no hypotheses. It states the result below."
		case h == 0:
			return "A theorem about " + objs + ". It states the result below for each choice of them."
		case n == 0:
			if h == 1 {
				return "A theorem with one hypothesis. When it holds, the result below holds."
			}
			return "A theorem with " + hyps + ". When they hold, the result below holds."
		default:
			return "A theorem about " + objs + " with " + hyps + ". When the hypotheses hold, the result below holds."
		}
	case "def", "abbrev":
		w := "A definition"
		if kind == "abbrev" {
			w = "A short name (abbrev)"
		}
		if r.Then != nil {
			if n+h == 0 {
				return w + ". It makes a value of the type below."
			}
			return w + " that takes " + plural(n+h, "input", "inputs") + " and makes a value of the type below."
		}
		return w + "."
	case "structure":
		return "A structure: a record of data and properties. A value of it has the fields in the source."
	case "class":
		return "A class: a structure that Lean finds by itself when a statement needs it."
	case "inductive":
		return "An inductive type: its values are made by the constructors in the source."
	case "instance":
		return "An instance: it tells Lean how a type has the structure below, so that Lean can use it by itself."
	case "alias":
		return "A second name for another declaration."
	}
	return "A declaration of kind " + kind + "."
}

func plural(n int, one, many string) string {
	words := []string{"no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	w := ""
	if n < len(words) {
		w = words[n]
	} else {
		w = itoa(n)
	}
	if n == 1 {
		return w + " " + one
	}
	return w + " " + many
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// binder is one group of a signature: (x y : T), {x : T}, [C] or ⦃x : T⦄.
type binder struct {
	names []string
	typ   string
	inst  bool
}

var closeOf = map[rune]rune{'(': ')', '{': '}', '[': ']', '⦃': '⦄', '⟨': '⟩'}

// split cuts a signature into its binders and the text after the colon.
func split(sig string) ([]binder, string) {
	s := strings.TrimSpace(sig)
	var out []binder
	for s != "" {
		r, _ := utf8.DecodeRuneInString(s)
		if r != '(' && r != '{' && r != '[' && r != '⦃' {
			break
		}
		end := matching(s)
		if end < 0 {
			break
		}
		_, w := utf8.DecodeRuneInString(s)
		inner := strings.TrimSpace(s[w : end-len(string(closeOf[r]))])
		s = strings.TrimSpace(s[end:])
		if b, ok := parseBinder(inner, r == '['); ok {
			out = append(out, b)
		}
	}
	if strings.HasPrefix(s, ":") && !strings.HasPrefix(s, ":=") {
		s = strings.TrimSpace(s[1:])
	} else if s != "" {
		// A signature that does not start with binders and has no colon is not a statement that this reader knows.
		return out, ""
	}
	return out, collapse(s)
}

// matching returns the end of the bracket group that starts s, after the close bracket, or -1.
func matching(s string) int {
	var stack []rune
	for i, r := range s {
		if c, ok := closeOf[r]; ok {
			stack = append(stack, c)
			continue
		}
		if len(stack) > 0 && r == stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + utf8.RuneLen(r)
			}
		}
	}
	return -1
}

func parseBinder(inner string, inst bool) (binder, bool) {
	k := topIndex(inner, ":")
	if inst {
		if k >= 0 {
			return binder{names: fields(inner[:k]), typ: collapse(inner[k+1:]), inst: true}, true
		}
		return binder{typ: collapse(inner), inst: true}, true
	}
	if k < 0 {
		return binder{names: fields(inner)}, len(inner) > 0
	}
	typ := inner[k+1:]
	if d := topIndex(typ, ":="); d >= 0 { // an optional argument with a default value
		typ = typ[:d]
	}
	return binder{names: fields(inner[:k]), typ: collapse(typ)}, true
}

func fields(s string) []string { return strings.Fields(s) }

// collapse puts a text on one line with single spaces.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// topIndex finds sep at bracket depth 0, or -1. A colon in := is not a colon.
func topIndex(s, sep string) int {
	depth := 0
	for i, r := range s {
		switch r {
		case '(', '{', '[', '⦃', '⟨':
			depth++
		case ')', '}', ']', '⦄', '⟩':
			depth--
		}
		if depth == 0 && strings.HasPrefix(s[i:], sep) {
			if sep == ":" && (strings.HasPrefix(s[i:], ":=") || strings.HasPrefix(s[i:], "::") || i > 0 && s[i-1] == ':') {
				continue
			}
			return i
		}
	}
	return -1
}

// splitTop splits s at each sep at bracket depth 0, and not inside the body of a ∀, ∃ or fun, which runs to the end.
func splitTop(s, sep string) []string {
	var out []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '(', '{', '[', '⦃', '⟨':
			depth++
		case ')', '}', ']', '⦄', '⟩':
			depth--
		case '∀', '∃', 'λ':
			if depth == 0 {
				return append(out, s[start:])
			}
		}
		if depth == 0 && strings.HasPrefix(s[i:], "fun ") && (i == 0 || s[i-1] == ' ' || s[i-1] == '(') {
			return append(out, s[start:])
		}
		if depth == 0 && strings.HasPrefix(s[i:], sep) {
			out = append(out, s[start:i])
			start = i + len(sep)
		}
	}
	return append(out, s[start:])
}

// leadingForall reads ∀ binders, body. It returns the binders and the body.
func leadingForall(s string) ([]binder, string, bool) {
	if !strings.HasPrefix(s, "∀") {
		return nil, s, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, "∀"))
	comma := topIndex(rest, ",")
	if comma < 0 {
		return nil, s, false
	}
	head, body := strings.TrimSpace(rest[:comma]), strings.TrimSpace(rest[comma+1:])
	var out []binder
	for head != "" {
		r, _ := utf8.DecodeRuneInString(head)
		if c, ok := closeOf[r]; ok && r != '⟨' {
			end := matching(head)
			if end < 0 {
				return nil, s, false
			}
			_, w := utf8.DecodeRuneInString(head)
			inner := head[w : end-len(string(c))]
			if b, ok := parseBinder(inner, r == '['); ok {
				out = append(out, b)
			}
			head = strings.TrimSpace(head[end:])
			continue
		}
		// ∀ x y : T, or ∀ x y,
		k := topIndex(head, ":")
		if k >= 0 {
			out = append(out, binder{names: fields(head[:k]), typ: collapse(head[k+1:])})
		} else {
			out = append(out, binder{names: fields(head)})
		}
		break
	}
	return out, body, len(out) > 0
}

// isProp guesses whether a binder is a hypothesis. A name such as h, hx or h₁ is a hypothesis by the custom of Mathlib, and so is a type with a relation or a predicate name.
func isProp(names []string, typ string) bool {
	if len(names) > 0 && isHypName(names[0]) {
		return true
	}
	if typ == "" {
		return false
	}
	for _, rel := range []string{" = ", " ≠ ", " < ", " ≤ ", " > ", " ≥ ", " ∈ ", " ∉ ", " ⊆ ", " ↔ ", "∀", "∃", " ∧ ", " ∨ ", "¬"} {
		if strings.Contains(typ, rel) {
			return true
		}
	}
	head := firstWord(typ)
	last := head[strings.LastIndexByte(head, '.')+1:]
	return strings.HasPrefix(last, "Is") && len(last) > 2 && unicode.IsUpper(rune(last[2])) || strings.HasPrefix(last, "Has") && len(last) > 3 && unicode.IsUpper(rune(last[3]))
}

func isHypName(n string) bool {
	if n == "h" || n == "_" {
		return n == "h"
	}
	if !strings.HasPrefix(n, "h") || len(n) > 8 {
		return false
	}
	r, _ := utf8.DecodeRuneInString(n[1:])
	return unicode.IsUpper(r) || unicode.IsDigit(r) || r == '_' || r == '\'' || (r >= 0x2080 && r <= 0x209C) || unicode.IsLower(r) && len(n) <= 5 || unicode.Is(unicode.Greek, r)
}

func firstWord(s string) string {
	s = strings.TrimLeft(s, "(@")
	for i, r := range s {
		if r == ' ' || r == '(' || r == ')' {
			return s[:i]
		}
	}
	return s
}

// The glosses of the common types.
var typeWords = map[string][2]string{
	"ℕ": {"a natural number", "natural numbers"}, "Nat": {"a natural number", "natural numbers"},
	"ℤ": {"an integer", "integers"}, "Int": {"an integer", "integers"},
	"ℚ": {"a rational number", "rational numbers"},
	"ℝ": {"a real number", "real numbers"}, "Real": {"a real number", "real numbers"},
	"ℂ": {"a complex number", "complex numbers"}, "Complex": {"a complex number", "complex numbers"},
	"Prop": {"a proposition", "propositions"}, "Bool": {"a true or false value", "true or false values"},
	"Type": {"a type", "types"}, "Type*": {"a type", "types"}, "Sort*": {"a type", "types"},
	"ℝ≥0":  {"a real number that is 0 or more", "real numbers that are 0 or more"},
	"ℝ≥0∞": {"a number in [0, ∞]", "numbers in [0, ∞]"},
}

func givenText(names []string, typ string) string {
	if typ == "" {
		return ""
	}
	who := strings.Join(names, ", ")
	if len(names) > 1 {
		who = strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
	if w, ok := typeWords[typ]; ok {
		if len(names) > 1 {
			return who + " are " + w[1]
		}
		return who + " is " + w[0]
	}
	if strings.HasPrefix(typ, "Type ") || strings.HasPrefix(typ, "Sort ") {
		if len(names) > 1 {
			return who + " are types"
		}
		return who + " is a type"
	}
	g := typeText(typ)
	if g == "" {
		return ""
	}
	if len(names) > 1 {
		return "each of " + who + " is " + g
	}
	return who + " is " + g
}

// typeText glosses a type, or returns "".
func typeText(t string) string {
	if w, ok := typeWords[t]; ok {
		return w[0]
	}
	if parts := splitTop(t, " → "); len(parts) > 1 {
		from := strings.TrimSpace(strings.Join(parts[:len(parts)-1], " → "))
		return "a function from " + code(from) + " to " + code(strings.TrimSpace(parts[len(parts)-1]))
	}
	head := firstWord(t)
	arg := strings.TrimSpace(strings.TrimPrefix(t, head))
	switch head {
	case "Fin":
		return "a number less than " + code(arg)
	case "Set":
		return "a set of elements of " + code(arg)
	case "Finset":
		return "a finite set of elements of " + code(arg)
	case "List":
		return "a list of elements of " + code(arg)
	case "Multiset":
		return "a finite collection, with repeats, of elements of " + code(arg)
	case "Polynomial":
		return "a polynomial with coefficients in " + code(arg)
	case "Matrix":
		return "a matrix"
	case "Ideal":
		return "an ideal of " + code(arg)
	case "Subgroup":
		return "a subgroup of " + code(arg)
	case "Submodule":
		return "a submodule"
	}
	if strings.HasSuffix(t, "[X]") {
		return "a polynomial with coefficients in " + code(strings.TrimSuffix(t, "[X]"))
	}
	return ""
}

func code(s string) string { return "`" + unwrap(s) + "`" }

// unwrap removes the brackets around a whole text: (mdim k r i) is mdim k r i.
func unwrap(s string) string {
	for strings.HasPrefix(s, "(") && matching(s) == len(s) {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

func instText(t string) string {
	head := firstWord(t)
	arg := strings.TrimSpace(strings.TrimPrefix(t, head))
	w := words(head[strings.LastIndexByte(head, '.')+1:])
	if arg == "" || w == "" {
		return ""
	}
	return code(arg) + " has the structure " + w
}

// propText glosses a proposition, or returns "".
func propText(p string) string {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "¬") {
		if t := propText(strings.TrimSpace(strings.TrimPrefix(p, "¬"))); t != "" {
			return "it is not true that " + t
		}
		return ""
	}
	if strings.HasPrefix(p, "∀") {
		if bs, body, ok := leadingForall(p); ok {
			return "for each " + boundNames(bs) + ", " + orCode(body)
		}
	}
	if strings.HasPrefix(p, "∃") {
		rest := strings.TrimSpace(strings.TrimPrefix(p, "∃"))
		if k := topIndex(rest, ","); k > 0 {
			return "there is " + code(strings.TrimSpace(rest[:k])) + " such that " + orCode(strings.TrimSpace(rest[k+1:]))
		}
	}
	if parts := splitTop(p, " ↔ "); len(parts) == 2 {
		return code(strings.TrimSpace(parts[0])) + " holds exactly when " + code(strings.TrimSpace(parts[1])) + " holds"
	}
	if parts := splitTop(p, " ∧ "); len(parts) > 1 {
		return "all of these hold: " + joinCode(parts)
	}
	if parts := splitTop(p, " ∨ "); len(parts) > 1 {
		return "at least one of these holds: " + joinCode(parts)
	}
	if parts := splitTop(p, " → "); len(parts) > 1 {
		return "if " + joinCode(parts[:len(parts)-1]) + ", then " + code(strings.TrimSpace(parts[len(parts)-1]))
	}
	for _, rel := range []struct{ op, w string }{
		{" ≠ ", "is not equal to"}, {" ≤ ", "is at most"}, {" ≥ ", "is at least"}, {" < ", "is less than"}, {" > ", "is greater than"},
		{" ∉ ", "is not in"}, {" ∈ ", "is in"}, {" ⊆ ", "is a subset of"}, {" = ", "equals"},
	} {
		if parts := splitTop(p, rel.op); len(parts) == 2 {
			a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			switch {
			case rel.op == " < " && a == "0":
				return code(b) + " is positive"
			case rel.op == " ≤ " && a == "0":
				return code(b) + " is 0 or more"
			case rel.op == " ≠ " && b == "0":
				return code(a) + " is not zero"
			}
			return code(a) + " " + rel.w + " " + code(b)
		}
	}
	head := firstWord(p)
	last := head[strings.LastIndexByte(head, '.')+1:]
	arg := strings.TrimSpace(strings.TrimPrefix(p, head))
	if strings.HasPrefix(last, "Is") && len(last) > 2 && arg != "" && topIndex(arg, " ") < 0 {
		return code(arg) + " is " + article(words(last[2:]))
	}
	return ""
}

func boundNames(bs []binder) string {
	var out []string
	for _, b := range bs {
		if b.inst {
			continue
		}
		n := strings.Join(b.names, ", ")
		if b.typ != "" && len(b.typ) < 40 {
			n += " : " + b.typ
		}
		out = append(out, code(n))
	}
	return strings.Join(out, ", ")
}

func orCode(s string) string {
	if t := propText(s); t != "" {
		return t
	}
	return code(s)
}

func joinCode(parts []string) string {
	var out []string
	for _, p := range parts {
		out = append(out, code(strings.TrimSpace(p)))
	}
	if len(out) == 2 {
		return out[0] + " and " + out[1]
	}
	return strings.Join(out, ", ")
}

func article(w string) string {
	if w == "" {
		return w
	}
	if strings.ContainsRune("aeiou", rune(w[0])) {
		return "an " + w
	}
	return "a " + w
}

// abbrevs are the words of names in the custom of Mathlib.
var abbrevs = map[string]string{
	"eq": "equals", "ne": "is not equal to", "le": "is at most", "lt": "is less than", "ge": "is at least", "gt": "is greater than",
	"mem": "is in", "iff": "if and only if", "of": "from", "imp": "implies", "comp": "composed with", "coe": "the cast of",
	"inv": "inverse", "neg": "negative", "pos": "positive", "nonneg": "nonnegative", "nonpos": "nonpositive", "ne_zero": "is not zero",
	"zero": "zero", "one": "one", "add": "plus", "sub": "minus", "mul": "times", "div": "divided by", "pow": "power", "sq": "square",
	"abs": "absolute value", "norm": "norm", "exp": "exponential", "log": "logarithm", "re": "real part", "im": "imaginary part",
	"sum": "sum", "prod": "product", "card": "number of elements", "deriv": "derivative", "integral": "integral",
	"exists": "there exists", "forall": "for all", "def": "definition", "apply": "applied", "symm": "symmetric", "trans": "transitive",
	"refl": "reflexive", "inj": "injective", "surj": "surjective", "bij": "bijective", "cont": "continuous", "tendsto": "tends to",
	"nhds": "neighborhood", "alt": "second form", "aux": "helper", "mk": "make", "rec": "recursion", "succ": "successor", "id": "identity",
	"bound": "bound", "bounds": "bounds", "holds": "holds", "spec": "specification", "nonempty": "nonempty", "zeta": "zeta",
}

// nameWords reads the last part of a name as words: exists_split_coords is "there exists split coords", and everyCharacter is "every character".
func nameWords(name string) string {
	last := name[strings.LastIndexByte(name, '.')+1:]
	if last == "" {
		return ""
	}
	var out []string
	for _, part := range strings.Split(last, "_") {
		if part == "" {
			continue
		}
		if w, ok := abbrevs[part]; ok {
			out = append(out, w)
			continue
		}
		out = append(out, words(part))
	}
	s := strings.Join(out, " ")
	if s == strings.ToLower(last) || s == last {
		return ""
	}
	return s
}

// words splits a camel case name into lower case words: IsRegularLocalRing is "is regular local ring". A run of capitals stays as one word.
func words(s string) string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i, r := range rs {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(rs[i-1]) || i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1])) {
			out = append(out, string(cur))
			cur = nil
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	for i, w := range out {
		if !isAllUpper(w) && !(len(w) == 1 && i+1 < len(out)) {
			out[i] = strings.ToLower(w)
		}
	}
	return strings.Join(out, " ")
}

func isAllUpper(s string) bool {
	n := 0
	for _, r := range s {
		if unicode.IsLower(r) {
			return false
		}
		if unicode.IsUpper(r) {
			n++
		}
	}
	return n > 1
}
