// Package extract reads the exact declaration graph from a built Lean environment and puts it into the records of rh index (spec 04, the exact graph).
//
// The syntactic graph of rh index comes from the text of the source. It can miss a use that the elaborator adds, such as an instance or a coercion, and it can add a use that the elaborator does not keep. The exact graph comes from the terms in the environment: the constants in the type and the value of each declaration.
//
// The environment has more constants than the source has declarations. Lean adds auxiliary constants, for example foo.match_1, foo._proof_1, foo.eq_1 and the projections and recursors of a structure. Merge follows the edges through these constants until it gets to a declaration of the source, so each record of the index keeps the names of the source.
package extract

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/tamnd/riemann-agent/index"
)

// Const is one constant of the environment, one line of the output of the Lean file of File.
type Const struct {
	Name    string   `json:"name"`   // the name as the source writes it; for a private constant, the name without the _private prefix
	Module  string   `json:"module"` // the module that declares it
	Kind    string   `json:"kind"`   // axiom, def, theorem, opaque, quot, inductive, ctor or rec
	Private bool     `json:"private,omitempty"`
	Lines   []int    `json:"lines,omitempty"` // the first line of the declaration and the line of its name, when Lean has the declaration range
	Uses    []string `json:"uses"`            // the constants in the type and the value, with the private prefix removed
}

// File returns the Lean file that writes the constants of the modules whose first component is in roots, one JSON line each, to the file out.
func File(module string, roots []string, out string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "import %s\nimport Lean\n\nopen Lean Elab Command\n\n", module)
	b.WriteString(`def rhKind : ConstantInfo → String
  | .axiomInfo _ => "axiom"
  | .defnInfo _ => "def"
  | .thmInfo _ => "theorem"
  | .opaqueInfo _ => "opaque"
  | .quotInfo _ => "quot"
  | .inductInfo _ => "inductive"
  | .ctorInfo _ => "ctor"
  | .recInfo _ => "rec"

def rhName (n : Name) : String :=
  (privateToUserName? n |>.getD n).toString (escape := false)

def rhUses (ci : ConstantInfo) : Array Name := Id.run do
  let mut a := ci.type.getUsedConstants
  if let some e := ci.value? (allowOpaque := true) then
    a := a ++ e.getUsedConstants
  if let .inductInfo v := ci then
    a := a ++ v.ctors.toArray
  return a

`)
	b.WriteString("def rhRoots : List Name := [")
	for i, r := range roots {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "`%s", r)
	}
	b.WriteString("]\n\n")
	fmt.Fprintf(&b, "#eval show CommandElabM Unit from do\n  let env ← getEnv\n  let h ← IO.FS.Handle.mk %q .write\n", out)
	b.WriteString(`  let mods := env.header.moduleNames
  let mut n := 0
  for i in [0:mods.size] do
    let m := mods[i]!
    if !rhRoots.contains m.getRoot then continue
    for ci in env.header.moduleData[i]!.constants do
      let uses := (rhUses ci).map rhName |>.toList.eraseDups
      let lines := match (← findDeclarationRanges? ci.name) with
        | some r => [r.range.pos.line, r.selectionRange.pos.line].eraseDups
        | none => []
      let j := Json.mkObj [("name", toJson (rhName ci.name)), ("module", toJson m.toString), ("kind", toJson (rhKind ci)), ("private", toJson (isPrivateName ci.name)), ("lines", toJson lines), ("uses", toJson uses)]
      h.putStrLn j.compress
      n := n + 1
  h.flush
  IO.println s!"rh-extract: {n} constants"
`)
	return b.String()
}

// Read reads the output of the Lean file, plain or gzip.
func Read(r io.Reader) ([]Const, error) {
	br := bufio.NewReader(r)
	if head, err := br.Peek(2); err == nil && head[0] == 0x1f && head[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return nil, err
		}
		defer func() { _ = zr.Close() }()
		br = bufio.NewReader(zr)
	}
	var cs []Const
	dec := json.NewDecoder(br)
	for {
		var c Const
		if err := dec.Decode(&c); err == io.EOF {
			return cs, nil
		} else if err != nil {
			return cs, fmt.Errorf("constant %d: %w", len(cs)+1, err)
		}
		cs = append(cs, c)
	}
}

// ReadFile reads the output of the Lean file from path.
func ReadFile(path string) ([]Const, error) {
	f, err := os.Open(path) // #nosec G304 -- a path from the flags of rh
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return Read(f)
}

// Stats tells how well the environment and the index match.
type Stats struct {
	Constants int `json:"constants"` // the constants of the environment in the modules of the release
	Matched   int `json:"matched"`   // the declarations of the index that have a constant
	Missing   int `json:"missing"`   // the declarations of the index that have no constant; they keep the syntactic graph
	Auxiliary int `json:"auxiliary"` // the constants that are not declarations of the index
	Added     int `json:"added"`     // the edges of the exact graph that the syntactic graph did not have
	Removed   int `json:"removed"`   // the edges of the syntactic graph that the exact graph does not have
}

// Merge sets the uses of each declaration of ix that has a constant in cs to its exact uses, and sets its graph to "exact". A declaration with no constant keeps its syntactic uses.
func Merge(ix *index.Index, cs []Const) Stats {
	var st Stats
	st.Constants = len(cs)
	type key = struct{ name, module string }
	decl := map[key]int{}
	public := map[string]int{} // the public name to the declaration, when only one declaration has it
	for i, d := range ix.Decls {
		decl[key{d.Name, d.Module}] = i
		if !d.Private {
			if _, ok := public[d.Name]; ok {
				public[d.Name] = -1
			} else {
				public[d.Name] = i
			}
		}
	}
	// The node of a constant: the index of its declaration, or -1 for an auxiliary constant.
	byName := map[string][]int{} // the name to the constants with that name
	node := make([]int, len(cs))
	for i, c := range cs {
		byName[c.Name] = append(byName[c.Name], i)
		node[i] = -1
		if j, ok := decl[key{c.Name, c.Module}]; ok {
			node[i] = j
		}
	}
	// resolve finds the constant of a use from constant i: a constant of the same module first (a private name is only visible there), then a public one.
	resolve := func(i int, name string) int {
		ids := byName[name]
		if len(ids) == 1 {
			return ids[0]
		}
		for _, k := range ids {
			if cs[k].Module == cs[i].Module {
				return k
			}
		}
		for _, k := range ids {
			if !cs[k].Private {
				return k
			}
		}
		return -1
	}
	// A declaration of the index with a name that the environment does not have, such as an anonymous instance, matches the constant that starts on the same line of the same module.
	taken := make([]bool, len(ix.Decls))
	for _, j := range node {
		if j >= 0 {
			taken[j] = true
		}
	}
	type at = struct {
		module string
		line   int
	}
	byLine := map[at][]int{}
	for i, c := range cs {
		if node[i] < 0 && c.Kind != "ctor" && c.Kind != "rec" {
			for _, l := range c.Lines {
				byLine[at{c.Module, l}] = append(byLine[at{c.Module, l}], i)
			}
		}
	}
	for j, d := range ix.Decls {
		if taken[j] {
			continue
		}
		best := -1
		for _, i := range byLine[at{d.Module, d.Line}] {
			if node[i] < 0 && (best < 0 || len(cs[i].Name) < len(cs[best].Name)) {
				best = i
			}
		}
		if best >= 0 {
			node[best] = j
			taken[j] = true
		}
	}
	exact := make([]map[string]bool, len(ix.Decls))
	for i := range cs {
		j := node[i]
		if j < 0 {
			st.Auxiliary++
			continue
		}
		if exact[j] != nil {
			continue // a second constant for the same declaration
		}
		// Walk from the constant through auxiliary constants to the declarations of the index.
		uses := map[string]bool{}
		seen := map[int]bool{i: true}
		todo := []int{i}
		for len(todo) > 0 {
			k := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			for _, u := range cs[k].Uses {
				t := resolve(k, u)
				if t < 0 || seen[t] {
					continue
				}
				seen[t] = true
				if node[t] >= 0 {
					if node[t] != j {
						uses[ix.Decls[node[t]].Name] = true
					}
					continue
				}
				// An auxiliary constant of the release. When it belongs to this declaration, for example foo.match_1 for foo, follow it. When it belongs to another declaration, for example the projection S.x of the structure S, it is a use of that declaration. Else follow it.
				switch b := ownerOf(cs[t].Name, cs[t].Module, decl); {
				case b == j || b < 0:
					todo = append(todo, t)
				default:
					uses[ix.Decls[b].Name] = true
				}
			}
		}
		exact[j] = uses
	}
	for j := range ix.Decls {
		d := &ix.Decls[j]
		if exact[j] == nil {
			st.Missing++
			continue
		}
		st.Matched++
		old := map[string]bool{}
		for _, u := range d.Uses {
			old[u] = true
		}
		var list []string
		for u := range exact[j] {
			list = append(list, u)
			if !old[u] {
				st.Added++
			}
		}
		for u := range old {
			if !exact[j][u] {
				st.Removed++
			}
		}
		sort.Strings(list)
		if list == nil {
			list = []string{}
		}
		d.Uses, d.Weak, d.Graph = list, []string{}, "exact"
	}
	ix.Summary.Uses, ix.Summary.Weak = 0, 0
	for _, d := range ix.Decls {
		ix.Summary.Uses += len(d.Uses)
		ix.Summary.Weak += len(d.Weak)
	}
	return st
}

// ownerOf returns the declaration of the index in module whose name is the longest prefix of the auxiliary constant aux, or -1.
func ownerOf(aux, module string, decl map[struct{ name, module string }]int) int {
	for i := strings.LastIndexByte(aux, '.'); i > 0; i = strings.LastIndexByte(aux[:i], '.') {
		if j, ok := decl[struct{ name, module string }{aux[:i], module}]; ok {
			return j
		}
	}
	return -1
}
