// Package site builds riemann.tamnd.com from the content repository.
//
// The site has a main page, the proof pages, the timeline of the note, the learning page, the code page, the trust report, an about page and a search page. People read the HTML. Agents read the same text as Markdown next to each page, the JSON in api/ and llms.txt. The style is the style of Math 2.0, with a side panel on the proof pages that names the Lean declarations, the concepts that the page needs and the page to read next. The build renders the math to MathML with the katex package of math2-agent, so a page shows its math with JavaScript off.
package site

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tamnd/math2-agent/katex"
	"github.com/tamnd/math2-agent/markup"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed all:assets
var assetFS embed.FS

// Options are the inputs of Build.
type Options struct {
	Content string // the root of the content repository
	Out     string // the output directory; Build empties it first
	URL     string // the public address of the site, for example https://riemann.tamnd.com
	Commit  string // the commit of the content repository, or ""
	Date    string // the build date, or ""
	Version string // the version of rh
	Math2   *Math2 // the index of Math 2.0; nil is an empty index
}

// Repositories of the project.
const (
	LeanRepo    = "https://github.com/tamnd/riemann-lean"
	ContentRepo = "https://github.com/tamnd/riemann"
	AgentRepo   = "https://github.com/tamnd/riemann-agent"
	math2Search = "https://math2.tamnd.com/search/?q="
)

// MenuItem is an entry of the main menu. Question is the question that the section answers.
type MenuItem struct {
	Path     string
	Section  string
	Question string
	Text     string
}

// Menu is the main menu. Each entry answers one question of a reader.
var Menu = []MenuItem{
	{Path: "proof/", Section: "proof", Question: "How does the proof go?", Text: "Proof"},
	{Path: "learn/", Section: "learn", Question: "What do I need to know first?", Text: "Learn"},
	{Path: "code/", Section: "code", Question: "Where is it in the Lean code?", Text: "Code"},
	{Path: "trust/", Section: "trust", Question: "What is checked?", Text: "Trust"},
	{Path: "about/", Section: "about", Question: "Who made this and how?", Text: "About"},
}

// Info is the data of the site that each page shows.
type Info struct {
	Name         string
	URL          string
	Menu         []MenuItem
	Claim        string
	Trust        Trust
	LeanRepo     string
	ContentRepo  string
	AgentRepo    string
	SourceCommit string
	Release      string
	Version      string
	Commit       string
	Built        string
}

// Canonical returns the public address of a page.
func (i *Info) Canonical(p string) string { return strings.TrimSuffix(i.URL, "/") + "/" + p }

// Crumb is a step of the trail at the top of a page.
type Crumb struct {
	Text    string
	URL     string
	Current bool
}

// SideLink is a link in the side panel.
type SideLink struct {
	Text    string
	Name    string
	URL     string
	Where   string
	Sub     string
	Missing bool
}

// Side is the side panel of a proof page.
type Side struct {
	Lean   []SideLink
	Needs  []SideLink
	UsedBy []SideLink
	Next   []SideLink
}

// HomeData is the extra data of the main page.
type HomeData struct {
	Eyebrow string
	Doors   []Door
}

// view is the data of one page for the templates.
type view struct {
	Site        *Info
	Path        string // the path of the page, for example proof/; "" is the main page
	Title       string
	Description string
	Lead        string
	Level       string
	Status      string
	Section     string
	Trail       []Crumb
	Side        *Side
	Home        *HomeData
	Timeline    []TimelineEntry
	Body        template.HTML
	Content     template.HTML
	SourceURL   string
	MarkdownURL string

	tmpl     string
	markdown string // the Markdown copy, or ""
}

// PageRecord is an entry of api/pages.json.
type PageRecord struct {
	Path        string `json:"path"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Section     string `json:"section,omitempty"`
	Level       string `json:"level,omitempty"`
	Markdown    string `json:"markdown,omitempty"`
}

// searchEntry is an entry of api/search.json, in the form that site.js of Math 2.0 reads.
type searchEntry struct {
	T string   `json:"t"`
	N string   `json:"n"`
	K string   `json:"k"`
	U string   `json:"u"`
	A []string `json:"a,omitempty"`
	M []string `json:"m,omitempty"`
}

// Result tells what Build wrote.
type Result struct {
	Pages   int
	Files   int
	Bytes   int64
	Missing []string // links to Math 2.0 tags that the index does not have
}

type builder struct {
	opt    Options
	info   *Info
	tmpl   *template.Template
	md     *markup.Markdown
	tex    *katex.Renderer
	math2  *Math2
	views  []*view
	source Source
	miss   map[string]bool
}

// Build builds the site in opt.Out.
func Build(ctx context.Context, opt Options) (*Result, error) {
	if opt.Content == "" || opt.Out == "" {
		return nil, errors.New("site: the content directory and the output directory are necessary")
	}
	if opt.URL == "" {
		opt.URL = "https://riemann.tamnd.com"
	}
	if opt.Version == "" {
		opt.Version = "dev"
	}
	b := &builder{opt: opt, math2: opt.Math2, miss: map[string]bool{}}
	if b.math2 == nil {
		b.math2 = &Math2{Nodes: map[string]Node{}}
	}
	if err := readYAML(filepath.Join(opt.Content, "manifests", "source.yaml"), &b.source); err != nil {
		return nil, fmt.Errorf("site: %w", err)
	}
	if b.source.Commit == "" {
		return nil, errors.New("site: manifests/source.yaml has no commit")
	}
	trust, err := readTrust(opt.Content)
	if err != nil {
		return nil, fmt.Errorf("site: %w", err)
	}
	b.info = &Info{
		Name: "riemann", URL: strings.TrimSuffix(opt.URL, "/"), Menu: Menu, Trust: trust,
		LeanRepo: LeanRepo, ContentRepo: ContentRepo, AgentRepo: AgentRepo,
		SourceCommit: b.source.Commit, Release: b.source.Release,
		Version: opt.Version, Commit: opt.Commit, Built: opt.Date,
	}
	if !trust.Claimed() {
		b.info.Claim = "The Lean code claims a proof of the Riemann Hypothesis. This site has not checked the claim yet."
	}
	b.tmpl, err = template.New("").Funcs(template.FuncMap{"url": func(p string) string { return "/" + p }}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("site: templates: %w", err)
	}
	if b.tex, err = katex.New(nil); err != nil {
		return nil, fmt.Errorf("site: katex: %w", err)
	}
	b.md = markup.New(b.math)
	if err := b.collect(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return b.write()
}

// math returns the MathML of a formula, as the site of Math 2.0 does. A formula that KaTeX cannot parse shows as its TeX source, with the error as the title.
func (b *builder) math(tex string, display bool) string {
	out, err := b.tex.Render(tex, display)
	if err != nil {
		return `<code class="tex-error" title="` + html.EscapeString(err.Error()) + `">` + html.EscapeString(tex) + `</code>`
	}
	if m := katex.MathML(out); m != "" {
		return `<span class="katex">` + m + `</span>`
	}
	return out
}

// leanURL returns the address of a file of lean/src in riemann-lean at the pinned commit. line 0 means the whole file.
func (b *builder) leanURL(file string, line int) string {
	u := LeanRepo + "/blob/" + b.source.Commit + "/lean/src/" + strings.TrimPrefix(file, "/")
	if line > 0 {
		u += fmt.Sprintf("#L%d", line)
	}
	return u
}

// link resolves a link of the Markdown. lean:Main.lean#L4 goes to riemann-lean, m2node:TAG goes to Math 2.0 and /path/ goes to a page of the site.
func (b *builder) link(dest string) (markup.Target, bool) {
	switch {
	case strings.HasPrefix(dest, markup.NodeScheme):
		tag := strings.TrimPrefix(dest, markup.NodeScheme)
		if n, ok := b.math2.Lookup(tag); ok {
			return markup.Target{Dest: n.URL, Title: n.Name + " in Math 2.0", Class: "m2"}, true
		}
		b.miss[tag] = true
		return markup.Target{Dest: math2Search + tag}, false
	case strings.HasPrefix(dest, "lean:"):
		file, frag, _ := strings.Cut(strings.TrimPrefix(dest, "lean:"), "#")
		u := LeanRepo + "/blob/" + b.source.Commit + "/lean/src/" + file
		if frag != "" {
			u += "#" + frag
		}
		return markup.Target{Dest: u, Class: "lean"}, true
	}
	return markup.Target{Dest: dest}, true
}

func (b *builder) render(src string) (template.HTML, error) {
	out, err := b.md.Render(src, b.link)
	if err != nil {
		return "", err
	}
	return template.HTML(out), nil // #nosec G203 -- the content repository is reviewed
}

// side builds the side panel from the front matter. It returns nil when the panel is empty.
func (b *builder) side(f Front) *Side {
	s := &Side{}
	for _, l := range f.Lean {
		where := path.Base(l.File)
		if l.Line > 0 {
			where = fmt.Sprintf("%s, line %d", where, l.Line)
		}
		s.Lean = append(s.Lean, SideLink{Name: l.Name, URL: b.leanURL(l.File, l.Line), Where: where})
	}
	for _, n := range f.Needs {
		s.Needs = append(s.Needs, b.need(n))
	}
	for _, n := range f.Next {
		s.Next = append(s.Next, b.need(n))
	}
	if len(s.Lean)+len(s.Needs)+len(s.Next) == 0 {
		return nil
	}
	return s
}

func (b *builder) need(n Need) SideLink {
	l := SideLink{Text: n.Text, Sub: n.Sub}
	switch {
	case n.Tag != "":
		if node, ok := b.math2.Lookup(n.Tag); ok {
			l.URL = node.URL
			if l.Text == "" {
				l.Text = node.Name
			}
			if l.Sub == "" {
				l.Sub = "Math 2.0, " + node.Kind
			}
		} else {
			b.miss[n.Tag] = true
			l.URL, l.Missing = math2Search+n.Tag, true
			if l.Text == "" {
				l.Text = n.Tag
			}
		}
	case n.Path != "":
		l.URL = "/" + strings.TrimPrefix(n.Path, "/")
	default:
		l.URL = n.URL
	}
	return l
}

// chapterPath returns the path of a proof page: proof/00-overview.md is proof/, and proof/03-theta.md is proof/theta/.
func chapterPath(name string) string {
	stem := strings.TrimSuffix(name, ".md")
	if i := strings.IndexByte(stem, '-'); i > 0 && strings.Trim(stem[:i], "0123456789") == "" {
		stem = stem[i+1:]
	}
	if stem == "overview" {
		return "proof/"
	}
	return "proof/" + stem + "/"
}

// collect reads the content and makes the views.
func (b *builder) collect() error {
	pages, err := sitePages(b.opt.Content)
	if err != nil {
		return fmt.Errorf("site: %w", err)
	}
	for _, rel := range pages {
		d, err := readDoc(b.opt.Content, rel)
		if err != nil {
			return fmt.Errorf("site: %w", err)
		}
		name := strings.TrimSuffix(path.Base(rel), ".md")
		v := b.docView(d)
		switch name {
		case "home":
			v.Path, v.tmpl = "", "home"
			v.Home = &HomeData{Eyebrow: d.Front.Eyebrow, Doors: d.Front.Doors}
		case "trust":
			v.Path, v.Section, v.tmpl = "trust/", "trust", "trust"
		default:
			v.Path, v.Section, v.tmpl = name+"/", name, "page"
			v.Trail = []Crumb{{Text: "Home", URL: "/"}, {Text: d.Front.Title, Current: true}}
		}
		b.add(v)
	}
	chapters, err := filepath.Glob(filepath.Join(b.opt.Content, "proof", "*.md"))
	if err != nil {
		return err
	}
	sort.Strings(chapters)
	for _, f := range chapters {
		d, err := readDoc(b.opt.Content, "proof/"+filepath.Base(f))
		if err != nil {
			return fmt.Errorf("site: %w", err)
		}
		v := b.docView(d)
		v.Path, v.Section, v.tmpl = chapterPath(filepath.Base(f)), "proof", "page"
		v.Side = b.side(d.Front)
		if v.Path == "proof/" {
			v.Trail = []Crumb{{Text: "Home", URL: "/"}, {Text: "Proof", Current: true}}
		} else {
			v.Trail = []Crumb{{Text: "Home", URL: "/"}, {Text: "Proof", URL: "/proof/"}, {Text: d.Front.Title, Current: true}}
		}
		b.add(v)
	}
	if err := b.timeline(); err != nil {
		return err
	}
	b.add(&view{Path: "search/", Title: "Search", Description: "Search the pages of riemann.", Section: "search", tmpl: "search"})
	b.add(&view{Path: "404.html", Title: "Page not found", Description: "This page does not exist.", tmpl: "notfound"})
	return b.unique()
}

func (b *builder) docView(d doc) *view {
	return &view{
		Title: d.Front.Title, Description: d.Front.Description, Lead: d.Front.Lead,
		Level: d.Front.Level, Status: d.Front.Status,
		SourceURL: ContentRepo + "/blob/main/" + d.Rel,
		markdown:  d.Body,
	}
}

func (b *builder) timeline() error {
	p := filepath.Join(b.opt.Content, "proof", "timeline.yaml")
	if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	var t Timeline
	if err := readYAML(p, &t); err != nil {
		return fmt.Errorf("site: %w", err)
	}
	if t.Title == "" {
		t.Title = "The timeline of the note"
	}
	v := &view{
		Path: "proof/timeline/", Title: t.Title, Description: t.Description, Lead: t.Lead,
		Section: "proof", tmpl: "timeline", Timeline: t.Steps, markdown: t.Intro,
		SourceURL: ContentRepo + "/blob/main/proof/timeline.yaml",
		Trail:     []Crumb{{Text: "Home", URL: "/"}, {Text: "Proof", URL: "/proof/"}, {Text: "Timeline", Current: true}},
	}
	b.add(v)
	return nil
}

func (b *builder) add(v *view) {
	v.Site = b.info
	if v.Description == "" {
		v.Description = v.Lead
	}
	if v.markdown != "" && !strings.HasSuffix(v.Path, ".html") {
		v.MarkdownURL = "/" + v.Path + "index.md"
	}
	b.views = append(b.views, v)
}

func (b *builder) unique() error {
	seen := map[string]bool{}
	for _, v := range b.views {
		if seen[v.Path] {
			return fmt.Errorf("site: two pages have the path /%s", v.Path)
		}
		seen[v.Path] = true
	}
	if !seen[""] {
		return errors.New("site: the content has no site/home.md")
	}
	return nil
}

// write empties the output directory and writes the site.
func (b *builder) write() (*Result, error) {
	out := b.opt.Out
	if err := os.RemoveAll(out); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(out, 0o750); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(out)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	w := &writer{root: root}

	for _, v := range b.views {
		for i := range v.Timeline {
			h, err := b.render(v.Timeline[i].Text)
			if err != nil {
				return nil, fmt.Errorf("site: timeline step %d: %w", v.Timeline[i].N, err)
			}
			v.Timeline[i].HTML = inline(h)
		}
		if v.markdown != "" {
			body, err := b.render(v.markdown)
			if err != nil {
				return nil, fmt.Errorf("site: /%s: %w", v.Path, err)
			}
			v.Body = body
		}
		var inner bytes.Buffer
		if err := b.tmpl.ExecuteTemplate(&inner, v.tmpl, v); err != nil {
			return nil, fmt.Errorf("site: /%s: %w", v.Path, err)
		}
		v.Content = template.HTML(inner.String()) // #nosec G203 -- the output of a template
		var page bytes.Buffer
		if err := b.tmpl.ExecuteTemplate(&page, "layout", v); err != nil {
			return nil, fmt.Errorf("site: /%s: %w", v.Path, err)
		}
		file := v.Path
		if file == "" || strings.HasSuffix(file, "/") {
			file += "index.html"
		}
		w.file(file, page.Bytes())
		if v.MarkdownURL != "" {
			w.file(strings.TrimPrefix(v.MarkdownURL, "/"), []byte("# "+v.Title+"\n\n"+strings.TrimLeft(v.markdown, "\n")))
		}
	}
	if err := fs.WalkDir(assetFS, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(p, ".md") {
			return err
		}
		data, err := assetFS.ReadFile(p)
		if err != nil {
			return err
		}
		w.file(p, data)
		return nil
	}); err != nil {
		return nil, err
	}
	b.machine(w)
	if w.err != nil {
		return nil, fmt.Errorf("site: %w", w.err)
	}
	res := &Result{Pages: len(b.views), Files: w.files, Bytes: w.bytes}
	for t := range b.miss {
		res.Missing = append(res.Missing, t)
	}
	sort.Strings(res.Missing)
	return res, nil
}

// inline removes the paragraph around a short text.
func inline(h template.HTML) template.HTML {
	s := strings.TrimSpace(string(h))
	if strings.HasPrefix(s, "<p>") && strings.HasSuffix(s, "</p>") && strings.Count(s, "<p>") == 1 {
		s = s[3 : len(s)-4]
	}
	return template.HTML(s) // #nosec G203 -- the output of the Markdown renderer
}

// machine writes the files for agents and for the host: the JSON in api/, llms.txt, the sitemap, robots.txt and _headers.
func (b *builder) machine(w *writer) {
	var pages []PageRecord
	var search []searchEntry
	for _, v := range b.views {
		if strings.HasSuffix(v.Path, ".html") {
			continue
		}
		pages = append(pages, PageRecord{
			Path: "/" + v.Path, URL: b.info.Canonical(v.Path), Title: v.Title, Description: v.Description,
			Section: v.Section, Level: v.Level, Markdown: v.MarkdownURL,
		})
		kind := v.Section
		if kind == "" {
			kind = "page"
		}
		search = append(search, searchEntry{T: v.Path, N: v.Title, K: kind, U: v.Path, A: aliases(v)})
	}
	w.json("api/pages.json", map[string]any{"name": "riemann", "commit": b.opt.Commit, "built": b.opt.Date, "source": b.source, "pages": pages})
	w.json("api/search.json", search)
	w.json("api/trust.json", b.info.Trust)

	var l strings.Builder
	fmt.Fprintf(&l, "# riemann\n\n> A written proof, for people, of the Lean proof of the Riemann Hypothesis that Levent Alpöge released on 2026-11-20. The site describes riemann-lean at commit %s. The Lean code claims the proof. The trust report tells what this site has checked.\n\n", b.source.Commit)
	l.WriteString("Each page has a Markdown copy at index.md next to it. The JSON is in /api/.\n\n## Pages\n\n")
	for _, p := range pages {
		u := p.URL
		if p.Markdown != "" {
			u = b.info.Canonical(strings.TrimPrefix(p.Markdown, "/"))
		}
		fmt.Fprintf(&l, "- [%s](%s)", p.Title, u)
		if p.Description != "" {
			fmt.Fprintf(&l, ": %s", p.Description)
		}
		l.WriteString("\n")
	}
	fmt.Fprintf(&l, "\n## Data\n\n- [Pages](%s)\n- [Trust report](%s)\n- [Lean source](%s/tree/%s)\n", b.info.Canonical("api/pages.json"), b.info.Canonical("api/trust.json"), LeanRepo, b.source.Commit)
	w.file("llms.txt", []byte(l.String()))

	var s strings.Builder
	s.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range pages {
		if p.Path == "/search/" {
			continue
		}
		fmt.Fprintf(&s, "<url><loc>%s</loc></url>\n", html.EscapeString(p.URL))
	}
	s.WriteString("</urlset>\n")
	w.file("sitemap.xml", []byte(s.String()))
	w.file("robots.txt", []byte("User-agent: *\nAllow: /\n\nSitemap: "+b.info.Canonical("sitemap.xml")+"\n"))
	w.file("_headers", []byte(headers))
}

// headers are the HTTP headers for Cloudflare Pages.
const headers = `/*
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  X-Frame-Options: DENY
  Permissions-Policy: interest-cohort=()

/assets/fonts/*
  Cache-Control: public, max-age=31536000, immutable

/*.md
  Content-Type: text/markdown; charset=utf-8

/llms.txt
  Content-Type: text/plain; charset=utf-8
`

var wordRE = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]+`)

// aliases returns the words of the lead of a page, for the search.
func aliases(v *view) []string {
	if v.Lead == "" {
		return nil
	}
	return []string{strings.Join(wordRE.FindAllString(v.Lead, -1), " ")}
}

// writer writes files under a root and keeps the first error.
type writer struct {
	root  *os.Root
	files int
	bytes int64
	err   error
}

func (w *writer) file(name string, data []byte) {
	if w.err != nil {
		return
	}
	if dir := path.Dir(name); dir != "." {
		if err := w.root.MkdirAll(dir, 0o750); err != nil {
			w.err = err
			return
		}
	}
	if err := w.root.WriteFile(name, data, 0o644); err != nil { // #nosec G306 -- the files of a public site
		w.err = err
		return
	}
	w.files++
	w.bytes += int64(len(data))
}

func (w *writer) json(name string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		w.err = err
		return
	}
	w.file(name, append(data, '\n'))
}
