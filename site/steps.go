package site

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/riemann-agent/steps"
)

// StepURL returns the address of the page of a step.
func StepURL(id string) string { return "/step/" + id + "/" }

// chapterInfo is a chapter of the proof, for the step pages.
type chapterInfo struct {
	Slug  string
	Title string
	URL   string
	Steps []string // the ids of its steps, in reading order
	Lean  string   // the first Lean name of the chapter, for the level switch
}

// readSteps reads the step files and links/literature.yaml.
func (b *builder) readSteps() error {
	ss, err := steps.Load(b.opt.Content)
	if err != nil {
		return fmt.Errorf("site: %w", err)
	}
	lit, err := steps.LoadLiterature(b.opt.Content)
	if err != nil {
		return fmt.Errorf("site: %w", err)
	}
	b.steps, b.stepByID, b.lit = ss, steps.ByID(ss), map[string]*steps.Named{}
	b.stepOfDecl = map[string]*steps.Step{}
	for _, s := range ss {
		if _, ok := b.stepOfDecl[s.Decl]; !ok {
			b.stepOfDecl[s.Decl] = s
		}
	}
	for i := range lit {
		b.lit[lit[i].Name] = &lit[i]
		b.litList = append(b.litList, &lit[i])
	}
	return nil
}

// orderSteps puts the steps of each chapter in the order of the steps list of the chapter, and then the other steps of the chapter by id.
func (b *builder) orderSteps(chapters []*chapterInfo) error {
	bySlug := map[string]*chapterInfo{}
	for _, c := range chapters {
		bySlug[c.Slug] = c
	}
	placed := map[string]string{}
	for _, c := range chapters {
		for _, id := range c.Steps {
			if _, ok := b.stepByID[id]; !ok {
				return fmt.Errorf("site: the chapter %s lists the step %s, but proof/steps/%s.md does not exist", c.Slug, id, id)
			}
			if old, ok := placed[id]; ok {
				return fmt.Errorf("site: the step %s is in the chapters %s and %s", id, old, c.Slug)
			}
			placed[id] = c.Slug
		}
	}
	for _, s := range b.steps {
		if _, ok := placed[s.ID]; ok {
			if s.Chapter != "" && s.Chapter != placed[s.ID] {
				return fmt.Errorf("site: %s: the chapter is %s, but the chapter %s lists the step", s.File, s.Chapter, placed[s.ID])
			}
			continue
		}
		c, ok := bySlug[s.Chapter]
		if !ok {
			return fmt.Errorf("site: %s: the chapter %q does not exist", s.File, s.Chapter)
		}
		c.Steps = append(c.Steps, s.ID)
		placed[s.ID] = c.Slug
	}
	b.chapterOf = map[string]*chapterInfo{}
	for _, c := range chapters {
		for _, id := range c.Steps {
			b.chapterOf[id] = c
		}
	}
	return nil
}

// levels returns the level switch of a page: the overview, the chapter, the step and the Lean declaration.
func levels(current int, chapter, step, lean string) []Crumb {
	names := []string{"L0 Overview", "L1 Chapter", "L2 Step", "L3 Lean"}
	urls := []string{"/proof/", chapter, step, lean}
	var out []Crumb
	for i, n := range names {
		out = append(out, Crumb{Text: n, URL: urls[i], Current: i == current})
	}
	return out
}

// stepViews adds a page for each step, the list of the steps and the list of the named results.
func (b *builder) stepViews(chapters []*chapterInfo) {
	usedBy := steps.UsedBy(b.steps)
	for _, c := range chapters {
		for i, id := range c.Steps {
			s := b.stepByID[id]
			v := &view{
				Path: "step/" + id + "/", Title: s.Title, Section: "proof", tmpl: "page",
				Level: "L2", Status: s.Status,
				Description: "A step of the written proof: " + s.Title + ". The statement, the Lean declaration " + s.Decl + ", the proof in words and the steps that it uses.",
				SourceURL:   ContentRepo + "/blob/main/" + s.File,
				markdown:    b.stepMarkdown(s, usedBy[id]),
			}
			v.Trail = []Crumb{{Text: "Home", URL: "/"}, {Text: "Proof", URL: "/proof/"}, {Text: c.Title, URL: c.URL}, {Text: s.Title, Current: true}}
			lean := ""
			if b.code != nil {
				lean = DeclURL(s.Decl)
			}
			v.Levels = levels(2, c.URL, "", lean)
			side := &Side{}
			for j, o := range c.Steps {
				side.Steps = append(side.Steps, SideLink{Text: b.stepByID[o].Title, URL: StepURL(o), Current: j == i})
			}
			side.StepsTitle = "The steps of " + c.Title
			where := ""
			if b.graph != nil {
				if k, ok := b.graph.ID[s.Decl]; ok {
					d := b.graph.Decls[k]
					where = fmt.Sprintf("%s, line %d", filepath.Base(b.modPath(d.Module)), d.Line)
				}
			}
			if b.code != nil {
				side.Lean = []SideLink{{Name: s.Decl, URL: DeclURL(s.Decl), Where: where}}
			}
			for _, t := range s.Concepts {
				side.Needs = append(side.Needs, b.need(Need{Tag: t}))
			}
			for _, u := range usedBy[id] {
				side.UsedBy = append(side.UsedBy, SideLink{Text: b.stepByID[u].Title, URL: StepURL(u)})
			}
			v.Side = side
			v.Pager = &Pager{}
			if i > 0 {
				v.Pager.Prev = &SideLink{Text: b.stepByID[c.Steps[i-1]].Title, URL: StepURL(c.Steps[i-1])}
			} else {
				v.Pager.Prev = &SideLink{Text: c.Title, URL: c.URL}
			}
			if i+1 < len(c.Steps) {
				v.Pager.Next = &SideLink{Text: b.stepByID[c.Steps[i+1]].Title, URL: StepURL(c.Steps[i+1])}
			}
			b.add(v)
		}
	}
	if len(b.steps) > 0 {
		b.add(&view{
			Path: "step/", Title: "The steps", Section: "proof", tmpl: "page", Level: "L2",
			Description: "Each step of the written proof, by chapter, with its status.",
			Lead:        "A step page gives one Lean declaration of the proof: its statement in standard notation, the Lean text, the proof in words and the steps that it uses. The list follows the order of the chapters.",
			Trail:       []Crumb{{Text: "Home", URL: "/"}, {Text: "Proof", URL: "/proof/"}, {Text: "The steps", Current: true}},
			markdown:    b.stepList(chapters),
		})
	}
	if len(b.litList) > 0 {
		b.add(&view{
			Path: "proof/named-results/", Title: "Named results", Section: "proof", tmpl: "page",
			Description: "The results of the literature that the top of the Lean proof uses, with what the Lean statement says and the citation.",
			Lead:        "The Lean code gives some of its inputs the names of published theorems. This page lists the ones that the top of the proof uses. For each one it tells what the Lean statement says, whether the proof reads the hypothesis structure, and where the result is in the literature.",
			Trail:       []Crumb{{Text: "Home", URL: "/"}, {Text: "Proof", URL: "/proof/"}, {Text: "Named results", Current: true}},
			markdown:    b.namedList(),
		})
	}
}

// stepMarkdown returns the body of a step page: the text of the file, with the Lean section after the statement and the sections that the build fills at the end.
func (b *builder) stepMarkdown(s *steps.Step, usedBy []string) string {
	body := strings.TrimSpace(s.Body)
	var lean strings.Builder
	lean.WriteString("## Lean\n\n")
	header := ""
	if b.graph != nil {
		if k, ok := b.graph.ID[s.Decl]; ok {
			d := b.graph.Decls[k]
			header = steps.Header(d)
			fmt.Fprintf(&lean, "```lean\n%s\n```\n\nThe declaration is in `%s`, line %d. Its [page](%s) has the full list of what it uses and what uses it, and a link to the source.\n\n", header, b.modPath(d.Module), d.Line, DeclURL(s.Decl))
		}
	}
	if header == "" {
		fmt.Fprintf(&lean, "The declaration is `%s`.\n\n", s.Decl)
	}
	// The file can have its own lean block. The check (rule C6) makes sure that it is the current text, so the page shows the text of the records only.
	if blk := s.LeanBlock(); blk != "" && header != "" {
		body = strings.Replace(body, "```lean\n"+blk+"\n```", "", 1)
	}
	if i := strings.Index(body, "\n## Proof"); i >= 0 {
		body = body[:i+1] + "\n" + lean.String() + body[i+1:]
	} else {
		body += "\n\n" + lean.String()
	}
	var tail strings.Builder
	tail.WriteString("\n\n## Uses\n\n")
	if len(s.Uses) == 0 && len(s.Routine) == 0 {
		tail.WriteString("The proof cites no other step.\n")
	}
	for _, u := range s.Uses {
		if t, ok := b.stepByID[u]; ok {
			fmt.Fprintf(&tail, "- [%s](%s), the step for `%s`.\n", t.Title, StepURL(u), t.Decl)
		}
	}
	if b.graph != nil {
		if k, ok := b.graph.ID[s.Decl]; ok {
			var named []string
			for _, j := range b.graph.Uses[k] {
				n := b.graph.Decls[j].Name
				if l, ok := b.lit[n]; ok && !contains(named, n) {
					named = append(named, n)
					fmt.Fprintf(&tail, "- The named result %s, %s. The page of [named results](/proof/named-results/) gives its citation.\n", l.Result, b.declCode(n))
				}
			}
		}
	}
	if len(s.Routine) > 0 {
		tail.WriteString("\nThe proof also uses these declarations in a routine way, for example to unfold a definition or to pass an argument on:")
		for i, r := range s.Routine {
			sep := ","
			if i == len(s.Routine)-1 {
				sep = "."
			}
			fmt.Fprintf(&tail, " %s%s", b.declCode(r), sep)
		}
		tail.WriteString("\n")
	}
	if len(s.Open) > 0 {
		tail.WriteString("\nThe proof cites these declarations, but their steps are not written yet. The text above tells what each one gives:")
		for i, r := range s.Open {
			sep := ","
			if i == len(s.Open)-1 {
				sep = "."
			}
			fmt.Fprintf(&tail, " %s%s", b.declCode(r), sep)
		}
		tail.WriteString("\n")
	}
	tail.WriteString("\n## Used by\n\n")
	if len(usedBy) == 0 {
		tail.WriteString("No other step cites this step yet.\n")
	}
	for _, u := range usedBy {
		t := b.stepByID[u]
		fmt.Fprintf(&tail, "- [%s](%s)\n", t.Title, StepURL(u))
	}
	tail.WriteString("\n## Status\n\n")
	switch s.Status {
	case "drafted":
		tail.WriteString("Drafted. The text follows the Lean code, but no person has checked it against the code with the rules of spec 05.7 yet. If you check it, [tell us](" + ContentRepo + "/issues/new?template=error.yml).\n")
	case "checked":
		tail.WriteString("Checked. A person read the step next to the Lean code and confirmed the rules C1 to C6.\n")
	case "reviewed":
		tail.WriteString("Reviewed. A person who works in the area agrees that the text is a correct and complete proof of the statement, given the steps that it cites.\n")
	}
	if s.DraftedBy != "" {
		fmt.Fprintf(&tail, "\nA model wrote the first draft (`%s`). A person checks it before its status goes up.\n", s.DraftedBy)
	}
	for _, r := range s.Reviews {
		fmt.Fprintf(&tail, "\n- %s, %s, %s.", r.Status, r.By, r.Date)
		if r.Note != "" {
			tail.WriteString(" " + r.Note)
		}
	}
	return body + tail.String()
}

func (b *builder) stepList(chapters []*chapterInfo) string {
	var out strings.Builder
	for _, c := range chapters {
		if len(c.Steps) == 0 {
			continue
		}
		fmt.Fprintf(&out, "## [%s](%s)\n\n", c.Title, c.URL)
		for _, id := range c.Steps {
			s := b.stepByID[id]
			fmt.Fprintf(&out, "1. [%s](%s), `%s`, %s.\n", s.Title, StepURL(id), s.Decl, s.Status)
		}
		out.WriteString("\n")
	}
	return out.String()
}

func (b *builder) namedList() string {
	var out strings.Builder
	out.WriteString("Most of these theorems do not read their argument, the structure `OpenLiteratureStatements`. The code proves them from other declarations, so the name says where the idea comes from, not that the code assumes the result. The column \"reads the hypothesis\" tells which ones use the argument. The citation is the work in the literature that the name points to. A citation with the mark *not verified* is still to be checked.\n\n")
	list := append([]*steps.Named(nil), b.litList...)
	sort.SliceStable(list, func(i, j int) bool { return strings.ToLower(list[i].Result) < strings.ToLower(list[j].Result) })
	for _, n := range list {
		fmt.Fprintf(&out, "## %s\n\n", n.Result)
		fmt.Fprintf(&out, "Lean: %s", b.declCode(n.Name))
		if n.Statement != "" {
			fmt.Fprintf(&out, ", which proves %s", b.declCode(n.Statement))
		}
		out.WriteString(".")
		if n.ReadsHypothesis {
			out.WriteString(" The proof reads the hypothesis structure.")
		} else {
			out.WriteString(" The proof does not read the hypothesis structure.")
		}
		out.WriteString("\n\n")
		if n.Says != "" {
			out.WriteString(n.Says + "\n\n")
		}
		for _, c := range n.Cite {
			out.WriteString("- " + strings.Join(c.Authors, ", ") + ". ")
			if l := c.Link(); l != "" {
				fmt.Fprintf(&out, "[%s](%s).", strings.TrimSuffix(c.Title, "."), l)
			} else {
				out.WriteString("*" + strings.TrimSuffix(c.Title, ".") + "*.")
			}
			if c.Venue != "" {
				out.WriteString(" " + c.Venue + ",")
			}
			fmt.Fprintf(&out, " %d.", c.Year)
			if !c.Verified {
				out.WriteString(" *Not verified.*")
			}
			out.WriteString("\n")
		}
		if n.Math2 != "" {
			fmt.Fprintf(&out, "\nIn Math 2.0: [[%s]].\n", n.Math2)
		}
		if n.Note != "" {
			out.WriteString("\n" + n.Note + "\n")
		}
		out.WriteString("\n")
	}
	return out.String()
}

// declCode returns the Markdown of a Lean name: a link to its declaration page when the site has the code pages, and code text when it does not.
func (b *builder) declCode(name string) string {
	if b.code == nil {
		return "`" + name + "`"
	}
	return "[`" + name + "`](" + DeclURL(name) + ")"
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
