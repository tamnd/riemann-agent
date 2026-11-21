package syntax

import (
	"strconv"
	"strings"
)

// File is what the reader finds in one Lean file.
type File struct {
	Imports []string
	Decls   []Decl
	// Examples counts the example commands. An example has no name.
	Examples int
}

// Decl is one declaration.
type Decl struct {
	Kind      string   // theorem, lemma, def, abbrev, instance, structure, class, inductive, axiom, opaque, alias
	Name      string   // the full name: the namespaces and the name as written, with no _root_
	Written   string   // the name as written in the source; empty for an instance with no name
	Private   bool     // the declaration is private, so only its own file can use it
	Modifiers []string // private, protected, noncomputable, partial, unsafe, local, scoped
	Attrs     string   // the text of the attributes, for example @[simp]
	Doc       string   // the doc comment, with no /-- and -/
	Line      int      // the line of the keyword, 1-based
	Start     int      // the byte offset where the command starts, with its doc comment and attributes
	End       int      // the byte offset where the command ends
	Sig       string   // the signature: the text from the name to :=, where or the first |
	Children  []string // the full names of the constructors and the fields
	Scope     Scope    // the namespaces and the open commands at the declaration
	Idents    []Ident  // the identifiers in the signature and the body, in order, with no repeats
}

// Ident is an identifier in a declaration.
type Ident struct {
	Name string
	Dot  bool // written as .name or (e).name, so the reader cannot resolve it
	Sig  bool // the identifier is in the signature
	// Local is true when the declaration binds the name itself, or when the head of a dotted name is such a name, as in hx.1. A binder of fun, ∀, ∃, intro, obtain, have or let, a binder of the signature, a name in the pattern of a match arm, a name before := and a field of the declaration are local. The resolver does not take a local name as a use of a declaration.
	Local bool
}

// Scope is the context of names at a declaration.
type Scope struct {
	// Namespaces is the stack of the current namespaces, outer first, as full names: A, A.B, A.B.C.
	Namespaces []string
	// Opens are the namespaces of the open and export commands in force, in the order of the file, as written.
	Opens []Open
}

// Open is one namespace of an open command.
type Open struct {
	Name string
	// At is the namespace that was current at the open command. Lean also tries At.Name.
	At string
}

var declKinds = map[string]bool{
	"theorem": true, "lemma": true, "def": true, "abbrev": true, "instance": true, "structure": true,
	"class": true, "inductive": true, "axiom": true, "opaque": true, "example": true, "alias": true,
	"irreducible_def": true,
}

var modifiers = map[string]bool{
	"private": true, "protected": true, "noncomputable": true, "partial": true, "unsafe": true,
	"nonrec": true, "local": true, "scoped": true, "public": true, "meta": true,
}

// commands are the words that start a command at column 0. A word at column 0 that is not here, or a symbol, goes on the command before it.
var commands = map[string]bool{
	"import": true, "open": true, "end": true, "namespace": true, "section": true, "variable": true,
	"set_option": true, "attribute": true, "omit": true, "include": true, "universe": true, "export": true,
	"mutual": true, "macro": true, "macro_rules": true, "elab": true, "elab_rules": true, "syntax": true,
	"notation": true, "notation3": true, "infix": true, "infixl": true, "infixr": true, "prefix": true,
	"postfix": true, "unseal": true, "seal": true, "run_cmd": true, "initialize": true,
	"declare_syntax_cat": true, "add_decl_doc": true, "suppress_compilation": true, "compile_inductive": true,
}

type scope struct {
	kind  string // namespace, section or mutual
	parts int    // the number of name parts that a namespace adds
	opens int    // the number of opens when the scope started
}

// Parse reads the Lean file src.
func Parse(src []byte) *File {
	toks := lex(src)
	f := &File{}
	// Find the start of each command.
	var starts []int
	for i, t := range toks {
		if t.col != 0 || !t.bol {
			continue
		}
		if !isCommandStart(toks, i) {
			continue
		}
		// A doc comment or an attribute on its own line goes with the command after it.
		if k := len(starts); k > 0 && prefixOnly(toks[starts[k-1]:i]) {
			continue
		}
		starts = append(starts, i)
	}
	var ns []string // the name parts of the current namespace
	var opens []Open
	var scopes []scope
	var openIn []Open // the opens of an open ... in, for the next command only
	for k, s := range starts {
		e := len(toks)
		if k+1 < len(starts) {
			e = starts[k+1]
		}
		cmd := toks[s:e]
		endOff := len(src)
		if e < len(toks) {
			endOff = toks[e].off
		}
		endOff = trimEnd(src, cmd[0].off, endOff)
		pendingIn := openIn
		openIn = nil
		// Read the doc comment, the attributes and the modifiers.
		i := 0
		doc, attrs := "", ""
		var mods []string
		for i < len(cmd) {
			t := cmd[i]
			switch {
			case t.kind == tDoc:
				doc = cleanDoc(t.text)
				i++
				continue
			case t.kind == tSymbol && t.text == "@[":
				j := closeBracket(cmd, i)
				attrs = strings.TrimSpace(attrs + " " + string(src[t.off:cmd[j-1].end]))
				i = j
				continue
			case t.kind == tIdent && modifiers[t.text]:
				// noncomputable section and local notation are not declarations.
				mods = append(mods, t.text)
				i++
				continue
			}
			break
		}
		if i >= len(cmd) {
			continue
		}
		t := cmd[i]
		if t.kind != tIdent {
			continue
		}
		switch t.text {
		case "import":
			for _, x := range cmd[i+1:] {
				if x.kind == tIdent {
					f.Imports = append(f.Imports, x.text)
				}
			}
		case "namespace":
			if i+1 < len(cmd) && cmd[i+1].kind == tIdent {
				parts := strings.Split(unquote(cmd[i+1].text), ".")
				ns = append(ns, parts...)
				scopes = append(scopes, scope{kind: "namespace", parts: len(parts), opens: len(opens)})
			}
		case "section":
			scopes = append(scopes, scope{kind: "section", opens: len(opens)})
		case "mutual":
			scopes = append(scopes, scope{kind: "mutual", opens: len(opens)})
		case "end":
			if len(scopes) > 0 {
				top := scopes[len(scopes)-1]
				scopes = scopes[:len(scopes)-1]
				ns = ns[:len(ns)-top.parts]
				opens = opens[:min(top.opens, len(opens))]
			}
		case "open", "export":
			at := strings.Join(ns, ".")
			var got []Open
			in := false
			paren := 0
			for _, x := range cmd[i+1:] {
				if x.kind == tSymbol && (x.text == "(" || x.text == "[") {
					paren++
				}
				if x.kind == tSymbol && (x.text == ")" || x.text == "]") {
					paren--
				}
				if x.kind != tIdent || paren > 0 {
					continue
				}
				switch x.text {
				case "scoped", "hiding", "renaming":
					continue
				case "in":
					in = true
					continue
				}
				got = append(got, Open{Name: unquote(x.text), At: at})
			}
			if in {
				openIn = append(pendingIn, got...)
			} else {
				opens = append(opens, got...)
			}
		default:
			if !declKinds[t.text] {
				continue
			}
			kind := t.text
			i++
			if kind == "class" && i < len(cmd) && cmd[i].kind == tIdent && (cmd[i].text == "inductive" || cmd[i].text == "abbrev") {
				kind = cmd[i].text
				i++
			}
			if kind == "example" {
				f.Examples++
				continue
			}
			d := Decl{Kind: kind, Doc: doc, Attrs: attrs, Modifiers: mods, Line: t.line, Start: cmd[0].off, End: endOff}
			for _, m := range mods {
				if m == "private" {
					d.Private = true
				}
			}
			if kind == "instance" && i < len(cmd) && cmd[i].text == "(" {
				i = closeParen(cmd, i)
			}
			nameTok := -1
			if i < len(cmd) && cmd[i].kind == tIdent {
				nameTok = i
				d.Written = unquote(cmd[i].text)
				i++
			}
			if kind == "alias" && nameTok < 0 {
				continue // alias ⟨a, b⟩ := h names two declarations; the reader skips it
			}
			current := strings.Join(ns, ".")
			switch {
			case d.Written == "":
				d.Name = join(current, "instance_L"+strconv.Itoa(t.line))
			case strings.HasPrefix(d.Written, "_root_."):
				d.Name = strings.TrimPrefix(d.Written, "_root_.")
			default:
				d.Name = join(current, d.Written)
			}
			scopeOpens := append(append([]Open(nil), opens...), pendingIn...)
			d.Scope = Scope{Namespaces: prefixes(ns), Opens: scopeOpens}
			// The signature goes to := or where or | at depth 0.
			sigEnd := len(cmd)
			depth := 0
			for j := i; j < len(cmd); j++ {
				x := cmd[j]
				if x.kind == tSymbol {
					switch x.text {
					case "(", "[", "{", "⟨", "⦃":
						depth++
					case ")", "]", "}", "⟩", "⦄":
						depth--
					}
				}
				if depth == 0 && ((x.kind == tSymbol && (x.text == ":=" || (x.text == "|" && (x.bol || kind == "inductive")))) || (x.kind == tIdent && (x.text == "where" || x.text == "extends"))) {
					sigEnd = j
					break
				}
			}
			if i < len(cmd) && sigEnd > i {
				d.Sig = squash(string(src[cmd[i].off:cmd[sigEnd-1].end]))
			}
			var own []string
			if kind == "structure" || kind == "class" || kind == "inductive" {
				own = children(cmd[sigEnd:], d.Name, kind)
			}
			loc := locals(cmd, i, sigEnd, own)
			seen := map[string]bool{}
			for j := i; j < len(cmd); j++ {
				x := cmd[j]
				if x.kind != tIdent || keywords[x.text] {
					continue
				}
				name := unquote(x.text)
				key := name
				if x.dot {
					key = "." + name
				}
				if seen[key] || name == d.Written {
					continue
				}
				seen[key] = true
				head, _, _ := strings.Cut(name, ".")
				d.Idents = append(d.Idents, Ident{Name: name, Dot: x.dot, Sig: j < sigEnd, Local: !x.dot && loc[head]})
			}
			d.Children = own
			f.Decls = append(f.Decls, d)
		}
	}
	return f
}

// binderStart are the tokens after which a list of binders comes, up to =>, ↦, a comma or a colon at the same depth.
var binderStart = map[string]bool{"fun": true, "λ": true, "∀": true, "∃": true, "∃!": true, "Π": true, "Σ": true, "∑": true, "∏": true, "⋃": true, "⋂": true, "forall": true, "exists": true}

// lineBinders are the tactics that bind the names that follow them on the same line.
var lineBinders = map[string]bool{"intro": true, "intros": true, "rintro": true, "introv": true, "obtain": true, "rcases": true, "rename_i": true, "next": true, "case": true, "choose": true, "set": true, "generalize": true, "ext": true, "funext": true}

// headBinders are the words that bind the names after them up to the first colon or :=, as in have h : p := e or let x := e.
var headBinders = map[string]bool{"have": true, "let": true, "suffices": true, "haveI": true, "letI": true, "obtain": true}

// locals returns the names that a declaration binds itself. The scan is a heuristic over the tokens: it reads the binders of the signature, the binders after fun, ∀ and ∃, the names of the tactics that introduce hypotheses, the patterns of the match arms, the names before :=, and the fields of the declaration. It does not follow the scope of a binder, so a name that is local anywhere in the declaration is local in all of it.
// i is the index of the token after the name, and sigEnd the end of the signature.
func locals(cmd []token, i, sigEnd int, own []string) map[string]bool {
	out := map[string]bool{}
	for _, c := range own {
		out[lastPart(c)] = true
	}
	add := func(x token) {
		if x.kind == tIdent && !x.dot && !keywords[x.text] && !strings.Contains(x.text, ".") && x.text != "_" {
			out[unquote(x.text)] = true
		}
	}
	isOpen := func(s string) bool { return s == "(" || s == "[" || s == "{" || s == "⟨" || s == "⦃" }
	isClose := func(s string) bool { return s == ")" || s == "]" || s == "}" || s == "⟩" || s == "⦄" }
	// binders reads a list of binders from k, as in x y (z : T) ⟨a, b⟩, and stops at =>, ↦, := or a colon or comma at depth 0.
	binders := func(k int) {
		depth := 0
		typed := map[int]bool{} // the depths at which a colon came, so the rest of the group is a type
		for ; k < len(cmd); k++ {
			x := cmd[k]
			if x.kind == tSymbol {
				switch {
				case isOpen(x.text):
					depth++
					// an instance binder with no name, as in [Field K], binds no name
					typed[depth] = typed[depth-1] || x.text == "[" && !named(cmd, k)
					continue
				case isClose(x.text):
					depth--
					if depth < 0 {
						return
					}
					continue
				case x.text == "=>" || x.text == "↦" || x.text == ":=":
					return
				case depth == 0 && (x.text == "," || x.text == ":" || x.text == "∈" || x.text == "<" || x.text == "≤" || x.text == ">" || x.text == "≥" || x.text == "≠" || x.text == "|"):
					return
				case x.text == ":" || x.text == "∈":
					typed[depth] = true
				}
				continue
			}
			if x.kind == tIdent && (x.text == "in" || x.text == "with" || x.text == "by") {
				return
			}
			if x.kind == tIdent && depth > 0 && !typed[depth] {
				add(x)
			} else if x.kind == tIdent && depth == 0 {
				add(x)
			}
		}
	}
	// The binders of the signature are the groups at depth 0 before the colon of the type.
	depth := 0
	for k := i; k < sigEnd; k++ {
		x := cmd[k]
		if x.kind != tSymbol {
			continue
		}
		switch {
		case isOpen(x.text):
			if depth == 0 && (x.text != "[" || named(cmd, k)) {
				binders(k + 1)
			}
			depth++
		case isClose(x.text):
			depth--
		case depth == 0 && x.text == ":":
			k = sigEnd
		}
	}
	for k := i; k < len(cmd); k++ {
		x := cmd[k]
		switch {
		case (x.kind == tIdent || x.kind == tSymbol) && binderStart[x.text]:
			binders(k + 1)
		case x.kind == tIdent && headBinders[x.text]:
			binders(k + 1)
		case x.kind == tIdent && lineBinders[x.text] && !x.dot && (k == 0 || !inList(cmd[k-1])):
			m := k + 1
			if x.text == "rcases" {
				// rcases e with pat: the names come after with.
				for m < len(cmd) && cmd[m].line == x.line && (cmd[m].kind != tIdent || cmd[m].text != "with") {
					m++
				}
				m++
			}
			skip := x.text == "case"
			for ; m < len(cmd) && cmd[m].line == x.line; m++ {
				y := cmd[m]
				if y.kind == tSymbol && (y.text == "=>" || y.text == ":=" || y.text == ";" || y.text == ":" || y.text == "<|" || y.text == "|>") {
					break
				}
				if y.kind == tIdent && (y.text == "using" || y.text == "with" || y.text == "at" || y.text == "from") {
					break
				}
				if skip && y.kind == tIdent {
					skip = false
					continue
				}
				add(y)
			}
		case x.kind == tIdent && x.text == "with" && k+1 < len(cmd) && cmd[k+1].kind == tSymbol && cmd[k+1].text == "⟨":
			for m := k + 1; m < len(cmd) && cmd[m].line == x.line; m++ {
				add(cmd[m])
			}
		case x.kind == tSymbol && x.text == "|" && x.bol:
			// A match arm: | pat => e. The first name is the constructor, the other plain names are binders.
			first := true
			for m := k + 1; m < len(cmd); m++ {
				y := cmd[m]
				if y.kind == tSymbol && (y.text == "=>" || y.text == ":=" || y.text == ":") {
					break
				}
				if y.kind == tIdent {
					if first {
						first = false
						if !y.dot && !strings.Contains(y.text, ".") && m+1 < len(cmd) && cmd[m+1].kind == tSymbol && (cmd[m+1].text == "=>" || cmd[m+1].text == ",") {
							add(y) // a pattern that is one name, as in | x => e
						}
						continue
					}
					add(y)
				}
			}
		case x.kind == tIdent && k >= i && k > 0 && k+1 < len(cmd) && cmd[k+1].kind == tSymbol && cmd[k+1].text == ":=" && (x.bol || cmd[k-1].kind == tSymbol && (cmd[k-1].text == "{" || cmd[k-1].text == ",")):
			add(x) // a field of a structure instance, as in { f := e, g := e } or f := e on its own line
		}
	}
	return out
}

// named reports whether the group that opens at k has a colon at its top level, as in [inst : Field K] or (x : T).
func named(cmd []token, k int) bool {
	depth := 0
	for m := k + 1; m < len(cmd); m++ {
		x := cmd[m]
		if x.kind != tSymbol {
			continue
		}
		switch x.text {
		case "(", "[", "{", "⟨", "⦃":
			depth++
		case ")", "]", "}", "⟩", "⦄":
			if depth == 0 {
				return false
			}
			depth--
		case ":":
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

// inList reports whether a token opens or continues a list, as in simp [foo, ext] or @[ext].
func inList(t token) bool {
	return t.kind == tSymbol && (t.text == "[" || t.text == "," || t.text == "@[")
}

// keywords are the words of the term language that look like identifiers.
var keywords = map[string]bool{
	"fun": true, "by": true, "at": true, "with": true, "match": true, "if": true, "then": true, "else": true,
	"let": true, "have": true, "show": true, "from": true, "do": true, "in": true, "where": true, "extends": true,
	"Type": true, "Sort": true, "Prop": true, "calc": true, "using": true, "only": true, "deriving": true,
	"termination_by": true, "decreasing_by": true, "suffices": true, "return": true, "for": true, "mut": true,
	"this": true, "forall": true, "exists": true, "obtain": true, "rcases": true, "intro": true, "exact": true,
	"rw": true, "simp": true, "theorem": true, "def": true, "lemma": true, "instance": true, "structure": true,
	"class": true, "inductive": true, "abbrev": true, "noncomputable": true, "private": true, "protected": true,
}

// children finds the constructors of an inductive and the fields of a structure, after where or the first |.
func children(toks []token, full, kind string) []string {
	var out []string
	add := func(s string) {
		out = append(out, full+"."+unquote(s))
	}
	if kind == "inductive" {
		for j := 0; j+1 < len(toks); j++ {
			if toks[j].kind == tSymbol && toks[j].text == "|" && toks[j+1].kind == tIdent {
				add(toks[j+1].text)
			}
		}
		return out
	}
	ctor := false
	for j := 0; j < len(toks); j++ {
		x := toks[j]
		if !x.bol || x.kind != tIdent || keywords[x.text] {
			continue
		}
		// Read the names at the start of a line, up to :, ::, := or a binder.
		k := j
		for k < len(toks) && toks[k].kind == tIdent && toks[k].line == x.line {
			k++
		}
		if k < len(toks) && toks[k].line == x.line && toks[k].kind == tSymbol {
			switch toks[k].text {
			case "::":
				add(toks[k-1].text)
				ctor = true
			case ":", ":=", "(", "{", "[", "⦃":
				for _, y := range toks[j:k] {
					add(y.text)
				}
			}
		}
		j = k - 1
	}
	if !ctor {
		out = append(out, full+".mk")
	}
	return out
}

func isCommandStart(toks []token, i int) bool {
	t := toks[i]
	switch t.kind {
	case tDoc:
		return true
	case tSymbol:
		return t.text == "@[" || (t.text == "#" && i+1 < len(toks) && toks[i+1].kind == tIdent)
	case tIdent:
		return declKinds[t.text] || modifiers[t.text] || commands[t.text]
	}
	return false
}

// prefixOnly reports whether toks has only doc comments and attributes.
func prefixOnly(toks []token) bool {
	for i := 0; i < len(toks); {
		switch {
		case toks[i].kind == tDoc:
			i++
		case toks[i].kind == tSymbol && toks[i].text == "@[":
			i = closeBracket(toks, i)
		default:
			return false
		}
	}
	return true
}

// closeBracket returns the index after the ] that closes the @[ at i.
func closeBracket(toks []token, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		switch toks[j].text {
		case "@[", "[":
			depth++
		case "]":
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return len(toks)
}

// closeParen returns the index after the ) that closes the ( at i.
func closeParen(toks []token, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		switch toks[j].text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return len(toks)
}

func prefixes(ns []string) []string {
	out := make([]string, len(ns))
	for i := range ns {
		out[i] = strings.Join(ns[:i+1], ".")
	}
	return out
}

func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

// unquote removes the « » of a name.
func unquote(s string) string {
	if !strings.Contains(s, "«") {
		return s
	}
	return strings.NewReplacer("«", "", "»", "").Replace(s)
}

// squash puts each run of white space as one space.
func squash(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func cleanDoc(s string) string {
	s = strings.TrimPrefix(s, "/--")
	s = strings.TrimSuffix(s, "-/")
	return strings.TrimSpace(s)
}

// trimEnd moves end back over the white space at the end of a span.
func trimEnd(src []byte, start, end int) int {
	for end > start && (src[end-1] == ' ' || src[end-1] == '\n' || src[end-1] == '\t' || src[end-1] == '\r') {
		end--
	}
	return end
}

// lastPart returns the part of a name after the last dot.
func lastPart(s string) string {
	if k := strings.LastIndexByte(s, '.'); k >= 0 {
		return s[k+1:]
	}
	return s
}
