package steps

import (
	"fmt"
	"strings"

	"github.com/tamnd/riemann-agent/index"
)

// Problem is one failed rule for one step.
type Problem struct {
	Step string // the file of the step
	Rule string // C1 to C6
	Text string
}

func (p Problem) String() string { return fmt.Sprintf("%s: %s: %s", p.Step, p.Rule, p.Text) }

// Env is what Check checks against.
type Env struct {
	Graph *index.Graph
	// Cut reports whether a Lean name is a cut point: a named result with a citation, or a name that the spine stops at. It can be nil.
	Cut func(name string) bool
	// Tag reports whether Math 2.0 has a node with the tag. It can be nil, and then rule C4 does not run.
	Tag func(tag string) bool
}

// Check runs the rules C1 to C6 on the steps and returns the problems in the order of the steps.
func Check(steps []*Step, env Env) []Problem {
	var out []Problem
	byID := ByID(steps)
	g := env.Graph
	for _, s := range steps {
		add := func(rule, format string, a ...any) {
			out = append(out, Problem{Step: s.File, Rule: rule, Text: fmt.Sprintf(format, a...)})
		}
		// C1: the declaration exists in the records.
		id, ok := g.ID[s.Decl]
		if s.Decl == "" {
			add("C1", "the step names no declaration")
		} else if !ok {
			add("C1", "the records have no declaration %s", s.Decl)
		}
		// C2: each step in uses exists, and the declaration reaches its declaration in the graph.
		for _, u := range s.Uses {
			t, ok := byID[u]
			if !ok {
				add("C2", "the step %s in uses does not exist", u)
				continue
			}
			if u == s.ID {
				add("C2", "the step uses itself")
				continue
			}
			tid, tok := g.ID[t.Decl]
			if ok && tok && !reaches(g, id, tid) {
				add("C2", "%s does not use %s, the declaration of the step %s", s.Decl, t.Decl, u)
			}
		}
		// C3: each Lean dependency is covered.
		if ok {
			covered := map[string]bool{}
			for _, u := range s.Uses {
				if t, ok := byID[u]; ok {
					covered[t.Decl] = true
				}
			}
			for _, list := range [][]string{s.Routine, s.Open} {
				for _, r := range list {
					if _, known := g.ID[r]; !known {
						add("C3", "the name %s in routine or open is not in the records", r)
					}
					covered[r] = true
				}
			}
			for _, j := range unique(g.Uses[id]) {
				name := g.Decls[j].Name
				if j == id || covered[name] || (env.Cut != nil && env.Cut(name)) {
					continue
				}
				add("C3", "the dependency %s is not covered: cite its step in uses, or list it in routine or open", name)
			}
		}
		// C4: each concept is a node of Math 2.0.
		if env.Tag != nil {
			for _, t := range s.Concepts {
				if !env.Tag(t) {
					add("C4", "Math 2.0 has no node %s", t)
				}
			}
		}
		// C5: the status is known, and a status above drafted has a review by a person.
		r := Rank(s.Status)
		switch {
		case r < 0:
			add("C5", "the status %q is not one of %s", s.Status, strings.Join(Statuses, ", "))
		case r > 0:
			best := -1
			for _, rv := range s.Reviews {
				if strings.TrimSpace(rv.By) != "" && rv.Date != "" {
					best = max(best, Rank(rv.Status))
				}
			}
			if best < r {
				add("C5", "the status is %s, but no review with a person and a date gives it", s.Status)
			}
		}
		for _, rv := range s.Reviews {
			if Rank(rv.Status) < 1 {
				add("C5", "a review has the status %q; use checked or reviewed", rv.Status)
			}
		}
		// C6: the Lean text on the page is the text of the source.
		if lean := s.LeanBlock(); lean != "" && ok {
			d := g.Decls[id]
			if l := norm(lean); l != norm(d.Sig) && l != Header(d) {
				add("C6", "the lean block is not the current signature of %s; remove the block, and the site shows the signature from the records", s.Decl)
			}
		}
	}
	return out
}

// reaches reports whether a chain of strong uses goes from a to b.
func reaches(g *index.Graph, a, b int32) bool {
	if a == b {
		return true
	}
	seen := map[int32]bool{a: true}
	queue := []int32{a}
	for len(queue) > 0 && len(seen) < 2_000_000 {
		v := queue[0]
		queue = queue[1:]
		for _, w := range g.Uses[v] {
			if w == b {
				return true
			}
			if !seen[w] {
				seen[w] = true
				queue = append(queue, w)
			}
		}
	}
	return false
}

func unique(xs []int32) []int32 {
	seen := map[int32]bool{}
	var out []int32
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func norm(s string) string { return strings.Join(strings.Fields(s), " ") }

// Header returns the Lean text of the statement of d: its kind, its name and its signature, on one line.
func Header(d index.Decl) string { return norm(d.Kind + " " + d.Name + " " + d.Sig) }
