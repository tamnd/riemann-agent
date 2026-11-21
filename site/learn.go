package site

import (
	"fmt"
	"strings"

	"github.com/tamnd/riemann-agent/steps"
)

// LearnURL returns the address of the page of a learning path.
func LearnURL(id string) string { return "/learn/" + id + "/" }

// readLearn reads the paths, the glossary and the links to Math 2.0.
func (b *builder) readLearn() error {
	ps, err := steps.LoadPaths(b.opt.Content)
	if err != nil {
		return fmt.Errorf("site: %w", err)
	}
	gl, err := steps.LoadGlossary(b.opt.Content)
	if err != nil {
		return fmt.Errorf("site: %w", err)
	}
	links, err := steps.LoadLinks(b.opt.Content)
	if err != nil {
		return fmt.Errorf("site: %w", err)
	}
	b.learn, b.glossary, b.links = ps, gl, links
	b.glossOf = map[string]*steps.Entry{}
	for i := range gl {
		b.glossOf[gl[i].Name] = &gl[i]
	}
	return nil
}

// pathsFor returns the paths that prepare for the chapter, in reading order.
func (b *builder) pathsFor(slug string) []*steps.Path {
	var out []*steps.Path
	for _, p := range b.learn {
		if contains(p.For, slug) {
			out = append(out, p)
		}
	}
	return out
}

// learnViews adds a page for each path, the concept map and the glossary. It adds the list of the paths to the page learn/.
func (b *builder) learnViews(chapters []*chapterInfo) {
	var slugs []string
	bySlug := map[string]*chapterInfo{}
	for _, c := range chapters {
		slugs = append(slugs, c.Slug)
		bySlug[c.Slug] = c
	}
	b.learn = steps.Order(b.learn, slugs)
	byID := map[string]*steps.Path{}
	for _, p := range b.learn {
		byID[p.ID] = p
	}
	trail := func(t string) []Crumb {
		return []Crumb{{Text: "Home", URL: "/"}, {Text: "Learn", URL: "/learn/"}, {Text: t, Current: true}}
	}
	var contents []SideLink
	for _, p := range b.learn {
		contents = append(contents, SideLink{Text: p.Title, URL: LearnURL(p.ID)})
	}
	contents = append(contents, SideLink{Text: "The concept map", URL: "/learn/map/"})
	if len(b.glossary) > 0 {
		contents = append(contents, SideLink{Text: "The glossary", URL: "/glossary/"})
	}
	side := func(current string) *Side {
		s := &Side{Contents: make([]SideLink, len(contents))}
		copy(s.Contents, contents)
		for i := range s.Contents {
			s.Contents[i].Current = s.Contents[i].URL == current
		}
		return s
	}
	for i, p := range b.learn {
		v := &view{
			Path: "learn/" + p.ID + "/", Title: p.Title, Section: "learn", tmpl: "page", Lead: p.Lead,
			Description: "A learning path of riemann: " + p.Title + ". The concepts in the order to learn them, with the reason for each one and a link to Math 2.0.",
			SourceURL:   ContentRepo + "/blob/main/" + p.File,
			Trail:       trail(p.Title),
			markdown:    b.pathMarkdown(p, byID, bySlug),
		}
		v.Side = side(LearnURL(p.ID))
		for _, c := range p.For {
			if ci := bySlug[c]; ci != nil {
				v.Side.Next = append(v.Side.Next, SideLink{Text: ci.Title, URL: ci.URL, Sub: "the chapter that this path prepares for"})
			}
		}
		v.Pager = &Pager{}
		if i > 0 {
			v.Pager.Prev = &SideLink{Text: b.learn[i-1].Title, URL: LearnURL(b.learn[i-1].ID)}
		} else {
			v.Pager.Prev = &SideLink{Text: "What to learn first", URL: "/learn/"}
		}
		if i+1 < len(b.learn) {
			v.Pager.Next = &SideLink{Text: b.learn[i+1].Title, URL: LearnURL(b.learn[i+1].ID)}
		} else {
			v.Pager.Next = &SideLink{Text: "The concept map", URL: "/learn/map/"}
		}
		b.add(v)
	}
	if len(b.learn) == 0 {
		return
	}
	for _, v := range b.views {
		for _, c := range chapters {
			if "/"+v.Path != c.URL || v.Side == nil {
				continue
			}
			for _, p := range b.pathsFor(c.Slug) {
				v.Side.Needs = append(v.Side.Needs, SideLink{Text: p.Title, URL: LearnURL(p.ID), Sub: fmt.Sprintf("a path of %d concepts", len(p.Items))})
			}
		}
		if v.Path == "learn/" {
			v.markdown = strings.TrimRight(v.markdown, "\n") + "\n\n" + b.learnList(bySlug)
			v.Side = side("/learn/")
		}
	}
	m := &view{
		Path: "learn/map/", Title: "The concept map", Section: "learn", tmpl: "page",
		Description: "Each chapter of the proof with the concepts that it needs, and for each concept its node in Math 2.0.",
		Lead:        "The map gives each chapter of the proof, the paths that prepare for it and the concepts of each path. A concept links to its node in Math 2.0. A concept that Math 2.0 does not have yet links to the proposal for a new node.",
		Trail:       trail("The concept map"),
		markdown:    b.mapMarkdown(chapters),
	}
	m.Side = side("/learn/map/")
	m.Pager = &Pager{Prev: &SideLink{Text: b.learn[len(b.learn)-1].Title, URL: LearnURL(b.learn[len(b.learn)-1].ID)}}
	if len(b.glossary) > 0 {
		m.Pager.Next = &SideLink{Text: "The glossary", URL: "/glossary/"}
	}
	b.add(m)
	if len(b.glossary) > 0 {
		g := &view{
			Path: "glossary/", Title: "The glossary", Section: "learn", tmpl: "page",
			Description: "The Lean names that a reader of the proof meets, with what each one means in words and in standard notation.",
			Lead:        "The Lean code uses long names. This page gives the names that a reader meets on the chapters and the steps, with what each one means. The name links to its declaration page, and the step links to the page that explains it.",
			SourceURL:   ContentRepo + "/blob/main/glossary/names.yaml",
			Trail:       []Crumb{{Text: "Home", URL: "/"}, {Text: "Learn", URL: "/learn/"}, {Text: "The glossary", Current: true}},
			markdown:    b.glossaryMarkdown(),
		}
		g.Side = side("/glossary/")
		g.Pager = &Pager{Prev: &SideLink{Text: "The concept map", URL: "/learn/map/"}}
		b.add(g)
	}
}

// conceptLink returns the Markdown of the Math 2.0 link of an item.
func (b *builder) conceptLink(it steps.Item) string {
	if tag := b.links.Concept(it); tag != "" {
		return "[[" + it.Concept + "|" + tag + "]]"
	}
	return it.Concept
}

func (b *builder) pathMarkdown(p *steps.Path, byID map[string]*steps.Path, bySlug map[string]*chapterInfo) string {
	var out strings.Builder
	var before, after []string
	for _, n := range p.Needs {
		if q := byID[n]; q != nil {
			before = append(before, fmt.Sprintf("[%s](%s)", q.Title, LearnURL(q.ID)))
		}
	}
	for _, c := range p.For {
		if ci := bySlug[c]; ci != nil {
			after = append(after, fmt.Sprintf("[%s](%s)", ci.Title, ci.URL))
		}
	}
	if len(before) > 0 {
		fmt.Fprintf(&out, "Read first: %s.\n\n", strings.Join(before, ", "))
	}
	if len(after) > 0 {
		word := "chapter"
		if len(after) > 1 {
			word = "chapters"
		}
		fmt.Fprintf(&out, "This path prepares you for the %s %s.\n\n", word, strings.Join(after, ", "))
	}
	fmt.Fprintf(&out, "The path has %d concepts. Learn them in order. Mark a concept as known when you know it, and the mark stays in this browser.\n\n", len(p.Items))
	for i, it := range p.Items {
		fmt.Fprintf(&out, "## %d. %s\n\n", i+1, it.Concept)
		tag := b.links.Concept(it)
		says := strings.TrimSpace(it.Says)
		if says == "" && b.links != nil {
			says = b.links.Summaries[tag]
		}
		if says != "" {
			out.WriteString(says + "\n\n")
		}
		if w := strings.TrimSpace(it.Why); w != "" {
			out.WriteString("**Where the proof uses it.** " + w + "\n\n")
		}
		var refs []string
		if tag != "" {
			refs = append(refs, "Learn it in Math 2.0: [["+it.Concept+"|"+tag+"]].")
		} else if it.Proposal != "" {
			refs = append(refs, fmt.Sprintf("Math 2.0 has no node for it yet. The [proposal](%s) asks for one.", it.Proposal))
		} else {
			refs = append(refs, "Math 2.0 has no node for it yet.")
		}
		if t := strings.TrimSpace(it.Text); t != "" {
			refs = append(refs, "A standard text: "+strings.TrimSuffix(t, ".")+".")
		}
		if len(it.Steps) > 0 {
			var ls []string
			for _, s := range it.Steps {
				if st := b.stepByID[s]; st != nil {
					ls = append(ls, fmt.Sprintf("[%s](%s)", st.Title, StepURL(s)))
				}
			}
			if len(ls) > 0 {
				refs = append(refs, "The steps that use it: "+strings.Join(ls, ", ")+".")
			}
		}
		out.WriteString(strings.Join(refs, " ") + "\n\n")
	}
	return out.String()
}

func (b *builder) learnList(bySlug map[string]*chapterInfo) string {
	var out strings.Builder
	out.WriteString("## The paths\n\nEach path prepares for one or more chapters of the proof. A path that needs another path comes after it.\n\n")
	for _, p := range b.learn {
		var ch []string
		for _, c := range p.For {
			if ci := bySlug[c]; ci != nil {
				ch = append(ch, ci.Title)
			}
		}
		fmt.Fprintf(&out, "1. [%s](%s), %d concepts", p.Title, LearnURL(p.ID), len(p.Items))
		if len(ch) > 0 {
			fmt.Fprintf(&out, ", for %s", strings.Join(ch, ", "))
		}
		out.WriteString(".\n")
	}
	out.WriteString("\nThe [concept map](/learn/map/) shows all the concepts by chapter.")
	if len(b.glossary) > 0 {
		out.WriteString(" The [glossary](/glossary/) gives the Lean names.")
	}
	out.WriteString("\n")
	return out.String()
}

func (b *builder) mapMarkdown(chapters []*chapterInfo) string {
	var out strings.Builder
	have, proposed, none := 0, 0, 0
	for _, c := range chapters {
		ps := b.pathsFor(c.Slug)
		fmt.Fprintf(&out, "## [%s](%s)\n\n", c.Title, c.URL)
		if len(ps) == 0 {
			out.WriteString("No path prepares for this chapter yet.\n\n")
			continue
		}
		for _, p := range ps {
			fmt.Fprintf(&out, "**[%s](%s).** ", p.Title, LearnURL(p.ID))
			var items []string
			for _, it := range p.Items {
				s := b.conceptLink(it)
				switch {
				case b.links.Concept(it) != "":
				case it.Proposal != "":
					s += fmt.Sprintf(" ([proposed](%s))", it.Proposal)
				default:
					s += " (no node yet)"
				}
				items = append(items, s)
			}
			out.WriteString(strings.Join(items, ", ") + ".\n\n")
		}
	}
	for _, p := range b.learn {
		for _, it := range p.Items {
			switch {
			case b.links.Concept(it) != "":
				have++
			case it.Proposal != "":
				proposed++
			default:
				none++
			}
		}
	}
	fmt.Fprintf(&out, "## The count\n\nThe paths have %d concepts.\n\n- With a node in Math 2.0: %d.\n- With a proposal for a node: %d.\n- With neither: %d.\n", have+proposed+none, have, proposed, none)
	return out.String()
}

func (b *builder) glossaryMarkdown() string {
	var out strings.Builder
	var spaces, names []*steps.Entry
	for i := range b.glossary {
		e := &b.glossary[i]
		if e.Kind == "namespace" {
			spaces = append(spaces, e)
		} else {
			names = append(names, e)
		}
	}
	entry := func(e *steps.Entry) {
		name := "`" + e.Name + "`"
		if b.code != nil && e.Kind != "namespace" {
			if _, ok := b.graph.ID[e.Name]; ok {
				name = b.declCode(e.Name)
			}
		}
		fmt.Fprintf(&out, "### %s\n\n", strings.TrimSuffix(e.Name, "."))
		meta := []string{name}
		if e.Kind != "" {
			meta = append(meta, e.Kind)
		}
		if e.Status == "open" {
			meta = append(meta, "the meaning is not known yet")
		}
		out.WriteString(strings.Join(meta, ", ") + ".\n\n")
		out.WriteString(strings.TrimSpace(e.Says) + "\n\n")
		var refs []string
		if e.Math != "" {
			refs = append(refs, "In standard notation: "+e.Math+".")
		}
		if e.Step != "" {
			if st := b.stepByID[e.Step]; st != nil {
				refs = append(refs, fmt.Sprintf("The step: [%s](%s).", st.Title, StepURL(e.Step)))
			}
		}
		if e.Math2 != "" {
			refs = append(refs, "In Math 2.0: "+b.m2ref(e.Math2)+".")
		}
		if len(refs) > 0 {
			out.WriteString(strings.Join(refs, " ") + "\n\n")
		}
	}
	if len(spaces) > 0 {
		out.WriteString("## Name spaces\n\nA name space is the first part of a long name. It tells which part of the code a name belongs to.\n\n")
		for _, e := range spaces {
			entry(e)
		}
	}
	if len(names) > 0 {
		out.WriteString("## Names\n\n")
		for _, e := range names {
			entry(e)
		}
	}
	return out.String()
}

// m2ref gives the Markdown link to the Math 2.0 node tag, with the name of the node as the text. A bare [[TAG]] is not a link of the Markdown.
func (b *builder) m2ref(tag string) string {
	name := tag
	if n, ok := b.math2.Lookup(tag); ok && n.Name != "" {
		name = n.Name
	}
	return "[[" + name + "|" + tag + "]]"
}
