package index

import (
	"encoding/json"
	"sort"
	"strings"
)

// LiteratureSpaces are the namespaces of the named results of spec 04.6. A name in one of them, or a name with a suffix in LiteratureSuffixes, is a cut point of the spine.
var LiteratureSpaces = []string{
	"GlobalTheory.Published",
	"UpperTheory.LiteratureTheorems",
	"PositivityTheory.PublishedTheorems",
	"Development.WeilBound.CoreStatements.PublishedTheorems",
}

// LiteratureSuffixes are the ends of the names of the named results.
var LiteratureSuffixes = []string{"_of_literature", "_holds"}

// IsLiterature reports whether name is a named result of spec 04.6.
func IsLiterature(name string) bool {
	for _, s := range LiteratureSpaces {
		if strings.HasPrefix(name, s+".") {
			return true
		}
	}
	for _, s := range LiteratureSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

// SpineNode is one declaration of the spine.
type SpineNode struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Module string `json:"module"`
	Depth  int    `json:"depth"`           // the length of the longest chain of uses from the root, with the edges that close a cycle left out
	Fanin  int    `json:"fanin"`           // the declarations of the spine that use it
	Size   int    `json:"size"`            // the declarations of the spine that only it leads to: its subtree in the dominator tree, with itself
	Idom   string `json:"idom"`            // the immediate dominator: the nearest declaration that each chain from the root goes through
	Cut    string `json:"cut,omitempty"`   // literature or routine, when the spine stops at it
	Layer  string `json:"layer,omitempty"` // trunk, or the name of the chapter candidate that holds it
}

// Spine is the result of BuildSpine.
type Spine struct {
	Root       string         `json:"root"`
	Weak       bool           `json:"weak"` // the walk follows the weak edges
	Nodes      []SpineNode    `json:"-"`
	Size       int            `json:"size"`
	Full       int            `json:"full"` // the size of the closure with no cut points
	Cuts       map[string]int `json:"cuts"`
	Libraries  map[string]int `json:"libraries"`
	Kinds      map[string]int `json:"kinds"`
	MaxDepth   int            `json:"max_depth"`
	Trunk      []string       `json:"trunk"`
	Candidates []Candidate    `json:"candidates"`
}

// Candidate is a chapter candidate of spec 04.5: a branch off the trunk with more than 1% of the spine.
type Candidate struct {
	Name   string `json:"name"`
	Module string `json:"module"`
	Size   int    `json:"size"`
	From   string `json:"from"` // the trunk node that it hangs from
}

// SpineOptions sets the cut points.
type SpineOptions struct {
	Weak    bool            // follow the weak edges too
	NoCut   bool            // do not stop at the named results
	Routine map[string]bool // the declarations that a person marked routine
	Share   float64         // the share of the spine that makes a chapter candidate; 0 means 0.01
}

// BuildSpine computes the spine of g from root.
func BuildSpine(g *Graph, root int32, opt SpineOptions) *Spine {
	cut := func(i int32) string {
		n := g.Decls[i].Name
		switch {
		case opt.Routine[n]:
			return "routine"
		case !opt.NoCut && IsLiterature(n):
			return "literature"
		}
		return ""
	}
	full := g.Closure(root, opt.Weak, nil)
	order := g.Closure(root, opt.Weak, func(i int32) bool { return cut(i) != "" })
	pos := make(map[int32]int32, len(order))
	for k, i := range order {
		pos[i] = int32(k)
	}
	n := len(order)
	// succ and pred in the numbers of the spine.
	succ := make([][]int32, n)
	pred := make([][]int32, n)
	for k, i := range order {
		if k > 0 && cut(i) != "" {
			continue
		}
		for _, j := range g.Next(i, opt.Weak) {
			if p, ok := pos[j]; ok {
				succ[k] = append(succ[k], p)
				pred[p] = append(pred[p], int32(k))
			}
		}
	}
	post := postorder(succ)
	depth := longest(succ, post)
	idom := dominators(succ, pred, post)
	size := make([]int, n)
	// A node comes after its dominator in reverse postorder, so a walk in postorder adds each subtree before its parent.
	for _, k := range post {
		size[k]++
		if k != 0 {
			size[idom[k]] += size[k]
		}
	}
	sp := &Spine{Root: g.Decls[root].Name, Weak: opt.Weak, Size: n, Full: len(full), Cuts: map[string]int{}, Libraries: map[string]int{}, Kinds: map[string]int{}}
	sp.Nodes = make([]SpineNode, n)
	for k, i := range order {
		d := g.Decls[i]
		node := SpineNode{Name: d.Name, Kind: d.Kind, Module: d.Module, Depth: depth[k], Fanin: len(pred[k]), Size: size[k], Cut: cut(i)}
		if k > 0 {
			node.Idom = g.Decls[order[idom[k]]].Name
		}
		if k == 0 {
			node.Cut = ""
		}
		sp.Nodes[k] = node
		if node.Cut != "" {
			sp.Cuts[node.Cut]++
		}
		lib, _, _ := strings.Cut(d.Module, ".")
		sp.Libraries[lib]++
		sp.Kinds[d.Kind]++
		sp.MaxDepth = max(sp.MaxDepth, node.Depth)
	}
	sp.layers(idom, size, opt.Share)
	return sp
}

// layers finds the trunk and the chapter candidates of spec 04.5 in the dominator tree.
func (sp *Spine) layers(idom []int32, size []int, share float64) {
	if share == 0 {
		share = 0.01
	}
	n := len(sp.Nodes)
	kids := make([][]int32, n)
	for k := 1; k < n; k++ {
		kids[idom[k]] = append(kids[idom[k]], int32(k))
	}
	for k := range kids {
		sort.Slice(kids[k], func(a, b int) bool { return size[kids[k][a]] > size[kids[k][b]] })
	}
	limit := int(share * float64(n))
	trunk := []int32{0}
	for k := int32(0); ; {
		if len(kids[k]) == 0 || 2*size[kids[k][0]] <= size[k] {
			break
		}
		k = kids[k][0]
		trunk = append(trunk, k)
	}
	onTrunk := map[int32]bool{}
	for _, k := range trunk {
		onTrunk[k] = true
		sp.Nodes[k].Layer = "trunk"
		sp.Trunk = append(sp.Trunk, sp.Nodes[k].Name)
	}
	for _, t := range trunk {
		for _, c := range kids[t] {
			if onTrunk[c] || size[c] <= limit {
				continue
			}
			sp.Candidates = append(sp.Candidates, Candidate{Name: sp.Nodes[c].Name, Module: sp.Nodes[c].Module, Size: size[c], From: sp.Nodes[t].Name})
		}
	}
	sort.SliceStable(sp.Candidates, func(a, b int) bool { return sp.Candidates[a].Size > sp.Candidates[b].Size })
	// Each node below a candidate takes the name of the candidate.
	label := make([]string, n)
	for _, c := range sp.Candidates {
		label[indexOf(sp.Nodes, c.Name)] = c.Name
	}
	// A node comes after its dominator in the order of the walk, so one pass in that order gives each node the label of its dominator.
	for k := 1; k < n; k++ {
		if label[k] == "" && !onTrunk[int32(k)] {
			label[k] = label[idom[k]]
		}
		if !onTrunk[int32(k)] {
			sp.Nodes[k].Layer = label[k]
		}
	}
}

func indexOf(nodes []SpineNode, name string) int {
	for k := range nodes {
		if nodes[k].Name == name {
			return k
		}
	}
	return -1
}

// postorder returns the nodes reachable from 0 in the postorder of a depth-first walk.
func postorder(succ [][]int32) []int32 {
	n := len(succ)
	state := make([]uint8, n)
	type frame struct{ k, next int32 }
	out := make([]int32, 0, n)
	stack := []frame{{0, 0}}
	state[0] = 1
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if int(top.next) < len(succ[top.k]) {
			j := succ[top.k][top.next]
			top.next++
			if state[j] == 0 {
				state[j] = 1
				stack = append(stack, frame{j, 0})
			}
			continue
		}
		state[top.k] = 2
		out = append(out, top.k)
		stack = stack[:len(stack)-1]
	}
	return out
}

// longest returns the longest chain from 0 to each node in the graph with the back edges of the walk left out.
func longest(succ [][]int32, post []int32) []int {
	n := len(succ)
	rank := make([]int32, n) // the place in reverse postorder
	for k, v := range post {
		rank[v] = int32(len(post) - 1 - k) //nolint:gosec // fewer than 2^31 nodes
	}
	depth := make([]int, n)
	for k := len(post) - 1; k >= 0; k-- {
		v := post[k]
		for _, w := range succ[v] {
			if rank[w] > rank[v] && depth[w] < depth[v]+1 {
				depth[w] = depth[v] + 1
			}
		}
	}
	return depth
}

// dominators returns the immediate dominator of each node, with the iterative method of Cooper, Harvey and Kennedy (2001). idom[0] is 0.
func dominators(succ, pred [][]int32, post []int32) []int32 {
	n := len(succ)
	rank := make([]int32, n) // the place in postorder
	for k, v := range post {
		rank[v] = int32(k)
	}
	idom := make([]int32, n)
	for k := range idom {
		idom[k] = -1
	}
	idom[0] = 0
	intersect := func(a, b int32) int32 {
		for a != b {
			for rank[a] < rank[b] {
				a = idom[a]
			}
			for rank[b] < rank[a] {
				b = idom[b]
			}
		}
		return a
	}
	for changed := true; changed; {
		changed = false
		for k := len(post) - 1; k >= 0; k-- {
			v := post[k]
			if v == 0 {
				continue
			}
			nd := int32(-1)
			for _, p := range pred[v] {
				if idom[p] < 0 {
					continue
				}
				if nd < 0 {
					nd = p
				} else {
					nd = intersect(p, nd)
				}
			}
			if nd >= 0 && idom[v] != nd {
				idom[v] = nd
				changed = true
			}
		}
	}
	return idom
}

// WriteNodes writes the nodes of the spine as gzip JSON lines, in the order of the walk.
func (sp *Spine) WriteNodes(path string) error {
	return writeGzipLines(path, len(sp.Nodes), func(i int) any { return &sp.Nodes[i] })
}

// ReadNodes reads the nodes that WriteNodes wrote.
func ReadNodes(path string) ([]SpineNode, error) {
	var out []SpineNode
	err := readGzipLines(path, func(line []byte) error {
		var n SpineNode
		if err := json.Unmarshal(line, &n); err != nil {
			return err
		}
		out = append(out, n)
		return nil
	})
	return out, err
}
