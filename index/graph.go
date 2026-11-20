package index

import (
	"sort"
	"strings"
)

// Graph is the declaration graph of an index, with numbers in place of names.
type Graph struct {
	Decls []Decl
	ID    map[string]int32 // a public name, or a private name when no public declaration has it
	Uses  [][]int32        // the strong edges
	Weak  [][]int32        // the weak edges
	By    [][]int32        // the reverse of the strong and the weak edges
}

// NewGraph makes the graph of ix.
func NewGraph(ix *Index) *Graph {
	g := &Graph{Decls: ix.Decls, ID: make(map[string]int32, len(ix.Decls))}
	for i, d := range ix.Decls {
		if old, ok := g.ID[d.Name]; ok && !ix.Decls[old].Private {
			continue
		}
		g.ID[d.Name] = int32(i)
	}
	g.Uses = make([][]int32, len(ix.Decls))
	g.Weak = make([][]int32, len(ix.Decls))
	g.By = make([][]int32, len(ix.Decls))
	modOf := func(i int32) string { return ix.Decls[i].Module }
	// A name can be private in more than one module. Find the declaration in the same module first.
	private := map[[2]string]int32{}
	for i, d := range ix.Decls {
		if d.Private {
			private[[2]string{d.Module, d.Name}] = int32(i)
		}
	}
	find := func(from int32, name string) (int32, bool) {
		if j, ok := private[[2]string{modOf(from), name}]; ok {
			return j, true
		}
		j, ok := g.ID[name]
		return j, ok
	}
	for i, d := range ix.Decls {
		from := int32(i)
		for _, u := range d.Uses {
			if j, ok := find(from, u); ok {
				g.Uses[i] = append(g.Uses[i], j)
				g.By[j] = append(g.By[j], from)
			}
		}
		for _, u := range d.Weak {
			if j, ok := find(from, u); ok {
				g.Weak[i] = append(g.Weak[i], j)
				g.By[j] = append(g.By[j], from)
			}
		}
	}
	return g
}

// Lookup finds a declaration by its full name, or by the end of its name when that is unique.
func (g *Graph) Lookup(name string) (int32, []string) {
	if j, ok := g.ID[name]; ok {
		return j, nil
	}
	var hits []string
	for n := range g.ID {
		if strings.HasSuffix(n, "."+name) {
			hits = append(hits, n)
		}
	}
	sort.Strings(hits)
	if len(hits) == 1 {
		return g.ID[hits[0]], nil
	}
	return -1, hits
}

// Next returns the edges out of i. With weak, it includes the weak edges.
func (g *Graph) Next(i int32, weak bool) []int32 {
	if !weak || len(g.Weak[i]) == 0 {
		return g.Uses[i]
	}
	return append(append([]int32(nil), g.Uses[i]...), g.Weak[i]...)
}

// Closure returns the declarations that root uses, with root, in the order of a breadth-first walk. stop tells where the walk does not go down; the walk keeps the stop node.
func (g *Graph) Closure(root int32, weak bool, stop func(int32) bool) []int32 {
	seen := make([]bool, len(g.Decls))
	seen[root] = true
	out := []int32{root}
	for k := 0; k < len(out); k++ {
		i := out[k]
		if k > 0 && stop != nil && stop(i) {
			continue
		}
		for _, j := range g.Next(i, weak) {
			if !seen[j] {
				seen[j] = true
				out = append(out, j)
			}
		}
	}
	return out
}

// Path returns a shortest chain of uses from a to b, or nil.
func (g *Graph) Path(a, b int32, weak bool) []int32 {
	prev := make([]int32, len(g.Decls))
	for i := range prev {
		prev[i] = -2
	}
	prev[a] = -1
	queue := []int32{a}
	for k := 0; k < len(queue); k++ {
		i := queue[k]
		if i == b {
			var out []int32
			for j := b; j >= 0; j = prev[j] {
				out = append(out, j)
			}
			for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
				out[l], out[r] = out[r], out[l]
			}
			return out
		}
		for _, j := range g.Next(i, weak) {
			if prev[j] == -2 {
				prev[j] = i
				queue = append(queue, j)
			}
		}
	}
	return nil
}
