package site

import (
	"fmt"
	"hash/fnv"
	"html"
	"html/template"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tamnd/riemann-agent/index"
	leanread "github.com/tamnd/riemann-agent/lean/read"
)

// The code pages show the records of rh index and the spine of rh spine. The release has about a million declarations, which is too many for one static page each. So the site has one page for a declaration, /code/decl/?n=Name, and one page for a module, /code/module/?m=Name. The page reads the record from a shard of api/decl/ or api/module/. The script that does this is the same for each name, and the shards change only when the source changes.

// Shard counts. Spec 03.4 limits a site to 20,000 files, so the code pages use about 5,500.
const (
	declShards   = 4096
	moduleShards = 256
	listLimit    = 60 // the most names in the uses and the used by of one record
	weakLimit    = 20
	sigLimit     = 2000
	docLimit     = 2000
	trailLimit   = 8 // the dominators in the trail of a record; the page walks up for more
)

// DeclShard returns the shard of a declaration name: the FNV-1a hash of the name, as three hex digits. code.js computes the same hash.
func DeclShard(name string) string {
	return fmt.Sprintf("%03x", hash32(name)%declShards)
}

// ModuleShard returns the shard of a module name, as two hex digits.
func ModuleShard(name string) string {
	return fmt.Sprintf("%02x", hash32(name)%moduleShards)
}

func hash32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

// NameKey returns the key of the name shard: the first two characters of the last part of the name, in lower case. A character that is not a letter or a digit of ASCII is _.
func NameKey(name string) string {
	last := strings.ToLower(name[strings.LastIndexByte(name, '.')+1:])
	var k []byte
	for _, r := range last {
		if len(k) == 2 {
			break
		}
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			k = append(k, byte(r)) //nolint:gosec // r is ASCII here
		} else {
			k = append(k, '_')
		}
	}
	for len(k) < 2 {
		k = append(k, '_')
	}
	return string(k)
}

// CodeRow is a declaration in a table of the code page.
type CodeRow struct {
	Name   string
	Kind   string
	Module string
	Size   int
	Share  string
	From   string
	URL    string
	Source string
}

// Code is the data of the code page.
type Code struct {
	Summary    index.Summary
	Spine      *index.Spine
	Cut        *index.Spine // the spine with the cut points, or nil
	Trunk      []CodeRow
	Candidates []CodeRow
}

// declRecord is the JSON of one declaration in api/decl/. The keys are short because the shards hold about a million records.
type declRecord struct {
	N  string            `json:"n"`
	K  string            `json:"k"`
	M  string            `json:"m"`
	F  string            `json:"f"`
	L  int               `json:"l"`
	P  bool              `json:"p,omitempty"`
	A  string            `json:"a,omitempty"`
	D  string            `json:"d,omitempty"`
	S  string            `json:"s"`
	U  []string          `json:"u,omitempty"`
	NU int               `json:"nu,omitempty"`
	W  []string          `json:"w,omitempty"`
	B  []string          `json:"b,omitempty"`
	NB int               `json:"nb,omitempty"`
	X  *spineFact        `json:"x,omitempty"`
	C  string            `json:"c,omitempty"`  // literature, when the declaration is a named result of spec 04.6
	R  *leanread.Reading `json:"r,omitempty"`  // the plain reading of the statement
	T  []string          `json:"t,omitempty"`  // the nearest dominators, from the top down, with the immediate dominator last
	PK [][2]any          `json:"pk,omitempty"` // the parts: the declarations that this one dominates directly, with their sizes, the largest first
	ST []string          `json:"st,omitempty"` // the step page that explains the declaration: its id and its title
	LT *litRecord        `json:"lt,omitempty"` // the entry of links/literature.yaml
	G  *glossRecord      `json:"g,omitempty"`  // the entry of glossary/names.yaml
}

// glossRecord is the glossary entry of a declaration, as HTML with the math rendered.
type glossRecord struct {
	S string `json:"s"`
	M string `json:"m,omitempty"`
}

// litRecord is a named result on its declaration page.
type litRecord struct {
	R string      `json:"r"`           // the name in the literature
	S string      `json:"s,omitempty"` // what the Lean statement says, as HTML with the math rendered
	N string      `json:"n,omitempty"` // how the Lean statement differs from the published one, as HTML
	H bool        `json:"h,omitempty"` // the proof reads the hypothesis structure
	C [][2]string `json:"c,omitempty"` // the citations: the text and the link
}

// spineFact is the place of a declaration on the spine.
type spineFact struct {
	D int    `json:"d"`
	F int    `json:"f"`
	S int    `json:"s"`
	I string `json:"i,omitempty"`
	Y string `json:"y,omitempty"`
}

type moduleRecord struct {
	N string       `json:"n"`
	F string       `json:"f"`
	L int          `json:"l"`
	I []string     `json:"i"`
	D int          `json:"d"`
	X []moduleDecl `json:"x"`
}

type moduleDecl struct {
	N string `json:"n"`
	K string `json:"k"`
	L int    `json:"l"`
	S bool   `json:"s,omitempty"` // on the spine
}

// codeData reads the records and the spine for the code pages. It returns nil when Options.Index is empty.
func (b *builder) codeData() error {
	if b.opt.Index == "" {
		return nil
	}
	ix, err := index.Load(b.opt.Index)
	if err != nil {
		return fmt.Errorf("site: the records: %w", err)
	}
	c := &Code{Summary: ix.Summary}
	if b.opt.Spine != "" {
		sp, nodes, err := readSpine(b.opt.Spine)
		if err != nil {
			return fmt.Errorf("site: the spine: %w", err)
		}
		c.Spine = sp
		b.spine = map[string]*index.SpineNode{}
		for i := range nodes {
			b.spine[nodes[i].Name] = &nodes[i]
		}
		var cut index.Spine
		if err := readJSON(filepath.Join(b.opt.Spine, "cut", "spine.json"), &cut); err == nil {
			c.Cut = &cut
		}
	}
	b.ix, b.code = ix, c
	b.graph = index.NewGraph(ix)
	if c.Spine != nil {
		for _, n := range c.Spine.Trunk {
			c.Trunk = append(c.Trunk, b.codeRow(n, 0, ""))
		}
		for _, k := range c.Spine.Candidates {
			c.Candidates = append(c.Candidates, b.codeRow(k.Name, k.Size, k.From))
		}
	}
	return nil
}

func readSpine(dir string) (*index.Spine, []index.SpineNode, error) {
	var sp index.Spine
	if err := readJSON(filepath.Join(dir, "spine.json"), &sp); err != nil {
		return nil, nil, err
	}
	nodes, err := index.ReadNodes(filepath.Join(dir, "spine.jsonl.gz"))
	if err != nil {
		return nil, nil, err
	}
	return &sp, nodes, nil
}

func (b *builder) codeRow(name string, size int, from string) CodeRow {
	r := CodeRow{Name: name, Size: size, From: from, URL: DeclURL(name)}
	if i, ok := b.graph.ID[name]; ok {
		d := b.graph.Decls[i]
		r.Kind, r.Module = d.Kind, d.Module
		r.Source = b.leanURL(b.modPath(d.Module), d.Line)
	}
	if size > 0 && b.code.Spine.Full > 0 {
		r.Share = fmt.Sprintf("%.1f%%", 100*float64(size)/float64(b.code.Spine.Full))
	}
	return r
}

func (b *builder) modPath(module string) string {
	if b.paths == nil {
		b.paths = make(map[string]string, len(b.ix.Modules))
		for _, m := range b.ix.Modules {
			b.paths[m.Name] = m.Path
		}
	}
	return b.paths[module]
}

// DeclURL returns the address of the page of a declaration.
func DeclURL(name string) string {
	return "/code/decl/?n=" + queryEscape(name)
}

// ModuleURL returns the address of the page of a module.
func ModuleURL(name string) string {
	return "/code/module/?m=" + queryEscape(name)
}

// queryEscape escapes a Lean name for a query string. It keeps the dots and the characters that are not ASCII, so the address stays readable.
func queryEscape(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		r, w := utf8.DecodeRuneInString(s[i:])
		switch {
		case r >= 0x80, ('a' <= r && r <= 'z'), ('A' <= r && r <= 'Z'), ('0' <= r && r <= '9'), strings.ContainsRune("._-'!", r):
			out.WriteString(s[i : i+w])
		default:
			for _, c := range []byte(s[i : i+w]) {
				fmt.Fprintf(&out, "%%%02X", c)
			}
		}
		i += w
	}
	return out.String()
}

// codeFiles writes the shards of the code pages.
func (b *builder) codeFiles(w *writer) {
	if b.code == nil {
		return
	}
	g := b.graph
	strongBy := make([][]int32, len(g.Decls))
	for i, us := range g.Uses {
		for _, j := range us {
			strongBy[j] = append(strongBy[j], int32(i))
		}
	}
	names := func(ids []int32, limit int) ([]string, int) {
		seen := map[string]bool{}
		var out []string
		for _, j := range ids {
			n := g.Decls[j].Name
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
		// The declarations of the spine come first, the largest first, so the most important names lead the list.
		sort.Slice(out, func(a, c int) bool {
			sa, sc := b.spineSize(out[a]), b.spineSize(out[c])
			if sa != sc {
				return sa > sc
			}
			return out[a] < out[c]
		})
		total := len(out)
		if len(out) > limit {
			out = out[:limit]
		}
		return out, total
	}
	kids := map[string][][2]any{}
	for name, n := range b.spine {
		if n.Idom != "" {
			kids[n.Idom] = append(kids[n.Idom], [2]any{name, n.Size})
		}
	}
	for _, k := range kids {
		sort.Slice(k, func(a, c int) bool {
			if k[a][1].(int) != k[c][1].(int) {
				return k[a][1].(int) > k[c][1].(int)
			}
			return k[a][0].(string) < k[c][0].(string)
		})
	}
	decls := make(map[string][]declRecord, declShards)
	nameShards := map[string][][3]any{}
	modDecls := map[string][]moduleDecl{}
	for i, d := range g.Decls {
		r := declRecord{N: d.Name, K: d.Kind, M: d.Module, F: b.modPath(d.Module), L: d.Line, P: d.Private, A: d.Attrs, D: clip(d.Doc, docLimit), S: clip(d.Sig, sigLimit)}
		r.U, r.NU = names(g.Uses[i], listLimit)
		r.W, _ = names(g.Weak[i], weakLimit)
		r.B, r.NB = names(strongBy[i], listLimit)
		if r.NU == len(r.U) {
			r.NU = 0
		}
		if r.NB == len(r.B) {
			r.NB = 0
		}
		depth := -1
		n, on := b.spine[d.Name]
		if on && n.Module == d.Module {
			r.X = &spineFact{D: n.Depth, F: n.Fanin, S: n.Size, I: n.Idom, Y: n.Layer}
			depth = n.Depth
			r.T = b.trail(n, trailLimit)
			r.PK = kids[d.Name]
		}
		if index.IsLiterature(d.Name) {
			r.C = "literature"
		}
		if st, ok := b.stepOfDecl[d.Name]; ok {
			r.ST = []string{st.ID, st.Title}
		}
		if l, ok := b.lit[d.Name]; ok {
			r.C = "literature"
			r.LT = &litRecord{R: l.Result, S: b.inline(l.Says), N: b.inline(l.Note), H: l.ReadsHypothesis}
			for _, c := range l.Cite {
				r.LT.C = append(r.LT.C, [2]string{fmt.Sprintf("%s. %s. %s, %d.", strings.Join(c.Authors, ", "), strings.TrimSuffix(c.Title, "."), c.Venue, c.Year), c.Link()})
			}
		}
		if e, ok := b.glossOf[d.Name]; ok && e.Kind != "namespace" {
			r.G = &glossRecord{S: string(inline(template.HTML(b.inline(e.Says)))), M: string(inline(template.HTML(b.inline(e.Math))))} // #nosec G203 -- the output of the Markdown renderer
		}
		rd := leanread.Read(d.Kind, d.Name, d.Sig)
		r.R = &rd
		s := DeclShard(d.Name)
		decls[s] = append(decls[s], r)
		if !d.Private {
			k := NameKey(d.Name)
			nameShards[k] = append(nameShards[k], [3]any{d.Name, d.Kind, depth})
		}
		modDecls[d.Module] = append(modDecls[d.Module], moduleDecl{N: d.Name, K: d.Kind, L: d.Line, S: depth >= 0})
	}
	for s, rs := range decls {
		w.gzjson("api/decl/"+s+".json.gz", rs)
	}
	for k, rs := range nameShards {
		w.gzjson("api/names/"+k+".json.gz", rs)
	}
	mods := map[string][]moduleRecord{}
	for _, m := range b.ix.Modules {
		x := modDecls[m.Name]
		sort.Slice(x, func(a, c int) bool { return x[a].L < x[c].L })
		if x == nil {
			x = []moduleDecl{}
		}
		s := ModuleShard(m.Name)
		mods[s] = append(mods[s], moduleRecord{N: m.Name, F: m.Path, L: m.Lines, I: m.Imports, D: m.Depth, X: x})
	}
	for s, rs := range mods {
		w.gzjson("api/module/"+s+".json.gz", rs)
	}
	sum := map[string]any{"source": b.source.Commit, "summary": b.code.Summary}
	if b.code.Spine != nil {
		sum["spine"] = b.code.Spine
	}
	if b.code.Cut != nil {
		sum["spine_with_cuts"] = b.code.Cut
	}
	w.json("api/code.json", sum)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	k := n
	for k > 0 && !utf8.RuneStart(s[k]) {
		k--
	}
	return s[:k] + " …"
}

func (b *builder) spineSize(name string) int {
	if n, ok := b.spine[name]; ok {
		return n.Size
	}
	return 0
}

// trail returns up to limit dominators of n, from the top down. The first is the root when the chain is short enough.
func (b *builder) trail(n *index.SpineNode, limit int) []string {
	var up []string
	for p := n.Idom; p != "" && len(up) < limit; {
		up = append(up, p)
		q, ok := b.spine[p]
		if !ok {
			break
		}
		p = q.Idom
	}
	for i, j := 0, len(up)-1; i < j; i, j = i+1, j-1 {
		up[i], up[j] = up[j], up[i]
	}
	return up
}

// inline renders a short Markdown text of the content repository to HTML, with the math as MathML. A text that does not render stays as escaped text.
func (b *builder) inline(src string) string {
	if src == "" {
		return ""
	}
	out, err := b.render(src)
	if err != nil {
		return "<p>" + html.EscapeString(src) + "</p>"
	}
	return string(out)
}
