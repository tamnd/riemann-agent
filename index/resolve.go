package index

import (
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/tamnd/riemann-agent/lean/syntax"
)

// names is the table of the names of the release.
type names struct {
	full  map[string][]int32 // a full name or the name of a constructor or a field, to the declarations
	last  map[string][]int32 // the last part of a name, to the declarations
	space map[string]bool    // each namespace that has a declaration of the release
	decls []Decl
	mod   []int32 // the module of each declaration
}

// declGraph makes the declaration records and resolves the identifiers of each declaration.
func (ix *Index) declGraph() {
	type src struct {
		m int32
		d *syntax.Decl
	}
	var all []src
	for i, m := range ix.Modules {
		for k := range m.decls {
			all = append(all, src{int32(i), &m.decls[k]})
		}
	}
	sort.SliceStable(all, func(a, b int) bool { return all[a].d.Name < all[b].d.Name })
	t := &names{
		full:  make(map[string][]int32, len(all)*2),
		last:  make(map[string][]int32, len(all)),
		space: map[string]bool{},
		decls: make([]Decl, len(all)),
		mod:   make([]int32, len(all)),
	}
	for i, s := range all {
		d := s.d
		m := ix.Modules[s.m]
		t.decls[i] = Decl{Name: d.Name, Kind: d.Kind, Module: m.Name, Line: d.Line, Span: [2]int{d.Start, d.End}, Private: d.Private, Attrs: d.Attrs, Doc: d.Doc, Sig: d.Sig, Graph: "syntactic"}
		t.mod[i] = s.m
		id := int32(i)
		t.full[d.Name] = append(t.full[d.Name], id)
		t.last[lastPart(d.Name)] = append(t.last[lastPart(d.Name)], id)
		for _, c := range d.Children {
			t.full[c] = append(t.full[c], id)
			t.last[lastPart(c)] = append(t.last[lastPart(c)], id)
		}
		for p := d.Name; strings.Contains(p, "."); {
			p = p[:strings.LastIndexByte(p, '.')]
			if t.space[p] {
				break
			}
			t.space[p] = true
		}
	}
	var wg sync.WaitGroup
	next := make(chan int, 1024)
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				t.resolve(i, all[i].d)
			}
		}()
	}
	for i := range all {
		next <- i
	}
	close(next)
	wg.Wait()
	ix.Decls = t.decls
	// The records keep only the names. Free the parse.
	for _, m := range ix.Modules {
		m.decls = nil
	}
}

// resolve sets the uses and the weak uses of declaration i.
func (t *names) resolve(i int, d *syntax.Decl) {
	ctx := contexts(d, t.space)
	self := int32(i) //nolint:gosec // fewer than 2^31 declarations
	strong := map[int32]bool{}
	weak := map[int32]bool{}
	for _, id := range d.Idents {
		if id.Dot {
			t.addWeak(weak, lastPart(id.Name), self)
			continue
		}
		if id.Local {
			// a name that the declaration binds, or a field of it, as in hx.1 or h.trans
			if strings.Contains(id.Name, ".") {
				t.addWeak(weak, lastPart(id.Name), self)
			}
			continue
		}
		name := strings.TrimPrefix(id.Name, "_root_.")
		if j, ok := t.lookup(ctx, name, t.mod[i]); ok {
			strong[j] = true
			continue
		}
		dot := strings.IndexByte(name, '.')
		if dot < 0 {
			continue
		}
		// a.b.c with no declaration a.b.c: find the longest head that is a declaration, and read the rest as dot notation. A head in lower case is a local name, as in h.trans.
		found := false
		for k := strings.LastIndexByte(name, '.'); k > 0; k = strings.LastIndexByte(name[:k], '.') {
			if j, ok := t.lookup(ctx, name[:k], t.mod[i]); ok {
				strong[j] = true
				t.addWeak(weak, name[k+1:], self)
				found = true
				break
			}
		}
		if !found && isLower(name) {
			t.addWeak(weak, lastPart(name), self)
		}
	}
	delete(strong, self)
	for j := range strong {
		delete(weak, j)
	}
	t.decls[i].Uses = t.sorted(strong)
	t.decls[i].Weak = t.sorted(weak)
}

func (t *names) addWeak(weak map[int32]bool, part string, self int32) {
	if strings.Contains(part, ".") {
		part = lastPart(part)
	}
	c := t.last[part]
	if len(c) == 0 || len(c) > WeakLimit {
		return
	}
	for _, j := range c {
		if j != self && !t.decls[j].Private {
			weak[j] = true
		}
	}
}

// lookup finds name in the contexts, inner first. A private declaration matches only in its own module. It returns the first match.
func (t *names) lookup(ctx []string, name string, mod int32) (int32, bool) {
	for _, c := range ctx {
		full := name
		if c != "" {
			full = c + "." + name
		}
		ids := t.full[full]
		if len(ids) == 0 {
			continue
		}
		best := int32(-1)
		for _, j := range ids {
			if t.decls[j].Private {
				if t.mod[j] == mod {
					return j, true
				}
				continue
			}
			if best < 0 {
				best = j
			}
		}
		if best >= 0 {
			return best, true
		}
	}
	return 0, false
}

// contexts lists the namespaces in which an identifier of d resolves, in order: the namespace of the name of d, the current namespaces from inner to outer, the open namespaces, and the root.
func contexts(d *syntax.Decl, space map[string]bool) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for p := d.Name; strings.Contains(p, "."); {
		p = p[:strings.LastIndexByte(p, '.')]
		add(p)
	}
	for k := len(d.Scope.Namespaces) - 1; k >= 0; k-- {
		add(d.Scope.Namespaces[k])
	}
	for k := len(d.Scope.Opens) - 1; k >= 0; k-- {
		o := d.Scope.Opens[k]
		// open B inside namespace A opens A.B when it exists, and B.
		for p := o.At; p != ""; {
			if space[p+"."+o.Name] {
				add(p + "." + o.Name)
			}
			k := strings.LastIndexByte(p, '.')
			if k < 0 {
				break
			}
			p = p[:k]
		}
		add(o.Name)
	}
	add("")
	return out
}

func (t *names) sorted(set map[int32]bool) []string {
	out := make([]string, 0, len(set))
	for j := range set {
		out = append(out, t.decls[j].Name)
	}
	sort.Strings(out)
	return compact(out)
}

func compact(s []string) []string {
	if len(s) < 2 {
		return s
	}
	k := 1
	for i := 1; i < len(s); i++ {
		if s[i] != s[k-1] {
			s[k] = s[i]
			k++
		}
	}
	return s[:k]
}

func lastPart(s string) string {
	return s[strings.LastIndexByte(s, '.')+1:]
}

func isLower(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	return c == '_' || ('a' <= c && c <= 'z') || c >= 0x80 && !strings.HasPrefix(s, "ℂ") && !strings.HasPrefix(s, "ℝ")
}

// summarize sets the numbers of the release.
func (ix *Index) summarize() {
	s := Summary{Kinds: map[string]int{}}
	libs := map[string]*Library{}
	have := make(map[string]bool, len(ix.Modules))
	for _, m := range ix.Modules {
		have[m.Name] = true
	}
	ext := map[string]bool{}
	for _, m := range ix.Modules {
		s.Modules++
		s.Lines += m.Lines
		s.Bytes += m.Bytes
		s.Examples += m.examples
		if m.Depth < 0 {
			s.Unreachable++
		}
		s.LongestChain = max(s.LongestChain, m.Depth+1)
		for _, imp := range m.Imports {
			if have[imp] {
				s.Imports++
			} else {
				ext[imp] = true
			}
		}
		lib := m.Name
		if k := strings.IndexByte(lib, '.'); k >= 0 {
			lib = lib[:k]
		}
		l := libs[lib]
		if l == nil {
			l = &Library{Name: lib}
			libs[lib] = l
		}
		l.Modules++
		l.Lines += m.Lines
		l.Bytes += m.Bytes
		l.Decls += m.Decls
	}
	public := map[string]int{}
	for _, d := range ix.Decls {
		s.Decls++
		s.Kinds[d.Kind]++
		if d.Private {
			s.Private++
		} else {
			public[d.Name]++
		}
		s.Uses += len(d.Uses)
		s.Weak += len(d.Weak)
	}
	for _, n := range public {
		if n > 1 {
			s.Duplicates++
		}
	}
	for _, l := range libs {
		s.Libraries = append(s.Libraries, *l)
	}
	sort.Slice(s.Libraries, func(a, b int) bool { return s.Libraries[a].Modules > s.Libraries[b].Modules })
	for m := range ext {
		s.ExternalMods = append(s.ExternalMods, m)
	}
	sort.Strings(s.ExternalMods)
	ix.Summary = s
}
