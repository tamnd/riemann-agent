package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/tamnd/riemann-agent/index"
	"github.com/tamnd/riemann-agent/steps"
)

// Env is what the roles read: the records, the Lean source and the content repository.
type Env struct {
	Index    *index.Index
	Graph    *index.Graph
	Src      string // the root of the Lean source, lean/src of riemann-lean
	Steps    []*steps.Step
	Glossary []steps.Entry
	Lit      []steps.Named
	Spine    []index.SpineNode // can be nil; the queue then uses the order of the names
}

// Cut reports whether the proof stops at a name: a named result of the literature.
func (e *Env) Cut(name string) bool {
	for _, n := range e.Lit {
		if n.Name == name {
			return true
		}
	}
	return false
}

// Dep is one direct dependency of the declaration, as the prover prompt shows it.
type Dep struct {
	Name, Kind string
	Step       *steps.Step // the step that explains it, or nil
	Says       string      // the glossary text, or the text of the named result
	Statement  string      // the Lean statement of a named result
	Sig        string
}

// ProverInput is the data of the prover template.
type ProverInput struct {
	Decl     index.Decl
	Source   string
	Doc      string
	Deps     []Dep
	Glossary []steps.Entry
}

// maxSource is the largest Lean text that the prompt shows. A longer text is cut, and the prompt says so.
const maxSource = 60_000

// Source returns the Lean text of a declaration, from its span in the source file.
func (e *Env) Source(d index.Decl) (string, error) {
	path := ""
	for _, m := range e.Index.Modules {
		if m.Name == d.Module {
			path = m.Path
			break
		}
	}
	if path == "" {
		return "", fmt.Errorf("the records have no module %s", d.Module)
	}
	data, err := os.ReadFile(filepath.Join(e.Src, filepath.FromSlash(path))) // #nosec G304 -- a file of the Lean source
	if err != nil {
		return "", err
	}
	a, b := d.Span[0], d.Span[1]
	if a < 0 || b > len(data) || a >= b {
		return "", fmt.Errorf("the span %v of %s is not in %s", d.Span, d.Name, path)
	}
	s := string(data[a:b])
	if len(s) > maxSource {
		s = s[:maxSource] + "\n-- (the rest of the text is cut)"
	}
	return s, nil
}

// Deps returns the direct strong dependencies of a declaration, sorted by name, without the declaration itself.
func (e *Env) Deps(id int32) []int32 {
	seen := map[int32]bool{id: true}
	var out []int32
	for _, j := range e.Graph.Uses[id] {
		if !seen[j] {
			seen[j] = true
			out = append(out, j)
		}
	}
	sort.Slice(out, func(a, b int) bool { return e.Graph.Decls[out[a]].Name < e.Graph.Decls[out[b]].Name })
	return out
}

// Prover returns the input of the prover for a Lean name.
func (e *Env) Prover(name string) (*ProverInput, error) {
	id, ok := e.Graph.ID[name]
	if !ok {
		return nil, fmt.Errorf("the records have no declaration %s", name)
	}
	d := e.Graph.Decls[id]
	src, err := e.Source(d)
	if err != nil {
		return nil, err
	}
	byDecl := map[string]*steps.Step{}
	for _, s := range e.Steps {
		byDecl[s.Decl] = s
	}
	gloss := map[string]steps.Entry{}
	for _, g := range e.Glossary {
		gloss[g.Name] = g
	}
	in := &ProverInput{Decl: d, Source: src, Doc: d.Doc}
	inDeps := map[string]bool{}
	for _, j := range e.Deps(id) {
		x := e.Graph.Decls[j]
		dep := Dep{Name: x.Name, Kind: x.Kind, Step: byDecl[x.Name], Sig: x.Sig}
		if g, ok := gloss[x.Name]; ok && g.Status != "open" {
			dep.Says = g.Says
		}
		for _, n := range e.Lit {
			if n.Name == x.Name {
				dep.Statement = n.Statement
				if dep.Says == "" {
					dep.Says = n.Says
				}
			}
		}
		inDeps[x.Name] = true
		in.Deps = append(in.Deps, dep)
	}
	// The glossary names that the text uses but that are not direct dependencies, such as the names of fields.
	for _, g := range e.Glossary {
		if inDeps[g.Name] || g.Status == "open" || g.Says == "" || strings.HasSuffix(g.Name, ".") {
			continue
		}
		if strings.Contains(src, lastPart(g.Name)) {
			in.Glossary = append(in.Glossary, g)
		}
	}
	return in, nil
}

// Draft is the parsed answer of the prover.
type Draft struct {
	Title   string
	Routine []string
	Body    string
}

// ParseProver reads the answer of the prover: the lines title and routine, the line ---, then the Markdown.
func ParseProver(text string) (*Draft, error) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	head, body, ok := strings.Cut(text, "\n---\n")
	if !ok {
		return nil, errors.New("the answer has no line --- after the header")
	}
	d := &Draft{Body: strings.TrimSpace(body) + "\n"}
	for _, l := range strings.Split(head, "\n") {
		k, v, _ := strings.Cut(l, ":")
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "title":
			d.Title = v
		case "routine":
			for _, r := range strings.Split(v, ",") {
				if r = strings.Trim(strings.TrimSpace(r), "`"); r != "" && r != "nothing" {
					d.Routine = append(d.Routine, r)
				}
			}
		}
	}
	if d.Title == "" {
		return nil, errors.New("the answer has no title")
	}
	if !strings.Contains(d.Body, "## Statement") || !strings.Contains(d.Body, "## Proof") {
		return nil, errors.New("the answer must have the sections Statement and Proof")
	}
	if strings.ContainsAny(d.Title+d.Body, "—–") {
		return nil, errors.New("the answer has an em dash or an en dash")
	}
	return d, nil
}

// lastPart returns the part of a Lean name after the last dot.
func lastPart(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}

var camelRE = regexp.MustCompile(`[^a-z0-9]+`)

// StepID returns the id of a new step for a Lean name: the last part in kebab case, with -def for a definition or a structure, as the steps of the trunk do. It adds -2, -3 and so on when the id is taken.
func StepID(name, kind string, taken map[string]bool) string {
	var b strings.Builder
	prev := rune(0)
	for _, r := range lastPart(name) {
		if unicode.IsUpper(r) && prev != 0 && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
		prev = r
	}
	id := strings.Trim(camelRE.ReplaceAllString(b.String(), "-"), "-")
	if id == "" {
		id = "step"
	}
	switch kind {
	case "def", "abbrev", "structure", "class", "inductive", "instance", "opaque":
		id += "-def"
	}
	base := id
	for n := 2; taken[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// Build makes the step of a draft. The code fills the front matter from the records, so that the model cannot name a step or a dependency that does not exist: uses are the steps of the direct dependencies, routine is the part of the list of the model that is a direct dependency with no step, and open is the rest, without the cut points.
func (e *Env) Build(name string, dr *Draft, draftedBy, prompt string) (*steps.Step, error) {
	id, ok := e.Graph.ID[name]
	if !ok {
		return nil, fmt.Errorf("the records have no declaration %s", name)
	}
	d := e.Graph.Decls[id]
	byDecl := map[string]*steps.Step{}
	taken := map[string]bool{}
	for _, s := range e.Steps {
		byDecl[s.Decl] = s
		taken[s.ID] = true
	}
	if s, ok := byDecl[name]; ok && s.Status != "drafted" {
		return nil, fmt.Errorf("the step %s of %s has the status %s; a draft must not replace a checked text", s.ID, name, s.Status)
	}
	routine := map[string]bool{}
	for _, r := range dr.Routine {
		routine[r] = true
	}
	st := &steps.Step{Title: dr.Title, Decl: name, Status: "drafted", DraftedBy: draftedBy, Prompt: prompt, Body: dr.Body}
	if old, ok := byDecl[name]; ok {
		st.ID, st.Chapter, st.Concepts = old.ID, old.Chapter, old.Concepts
	} else {
		st.ID = StepID(name, d.Kind, taken)
	}
	for _, j := range e.Deps(id) {
		x := e.Graph.Decls[j].Name
		switch {
		case byDecl[x] != nil && byDecl[x].ID != st.ID:
			st.Uses = append(st.Uses, byDecl[x].ID)
		case e.Cut(x):
		case routine[x]:
			st.Routine = append(st.Routine, x)
		default:
			st.Open = append(st.Open, x)
		}
	}
	if st.Chapter == "" {
		st.Chapter = e.parentChapter(name)
	}
	st.File = "proof/steps/" + st.ID + ".md"
	return st, nil
}

// parentChapter returns the chapter of the first step that lists the name in open, or "".
func (e *Env) parentChapter(name string) string {
	for _, s := range e.Steps {
		for _, o := range s.Open {
			if o == name {
				return s.Chapter
			}
		}
	}
	return ""
}

// Parents returns the steps that list the name in open. When the draft is written, each of them cites the new step in uses.
func (e *Env) Parents(name string) []*steps.Step {
	var out []*steps.Step
	for _, s := range e.Steps {
		for _, o := range s.Open {
			if o == name {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// Queue returns the next n names to draft: the names in the open lists of the steps that have no step and are not cut points, in the order of spec 09.3: the depth on the spine from the top, then the fan-in, highest first.
func (e *Env) Queue(n int) []string {
	have := map[string]bool{}
	for _, s := range e.Steps {
		have[s.Decl] = true
	}
	want := map[string]bool{}
	for _, s := range e.Steps {
		for _, o := range s.Open {
			if !have[o] && !e.Cut(o) {
				want[o] = true
			}
		}
	}
	type cand struct {
		name         string
		depth, fanin int
	}
	var cs []cand
	pos := map[string]index.SpineNode{}
	for _, sn := range e.Spine {
		pos[sn.Name] = sn
	}
	for w := range want {
		c := cand{name: w, depth: 1 << 30}
		if sn, ok := pos[w]; ok {
			c.depth, c.fanin = sn.Depth, sn.Fanin
		}
		cs = append(cs, c)
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].depth != cs[j].depth {
			return cs[i].depth < cs[j].depth
		}
		if cs[i].fanin != cs[j].fanin {
			return cs[i].fanin > cs[j].fanin
		}
		return cs[i].name < cs[j].name
	})
	var out []string
	for i := 0; i < len(cs) && i < n; i++ {
		out = append(out, cs[i].name)
	}
	return out
}
