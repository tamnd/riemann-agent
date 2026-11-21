package steps

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Path is one file paths/<id>.yaml: the concepts that a reader needs for one or more chapters, in the order to learn them (spec 07).
type Path struct {
	ID    string   `yaml:"id"`
	Title string   `yaml:"title"`
	For   []string `yaml:"for"`   // the slugs of the chapters that the path prepares for
	Needs []string `yaml:"needs"` // the ids of the paths to read first
	Lead  string   `yaml:"lead"`
	// Order is the place of the path in the reading order. The paths with an order come first, from the lowest order; the other paths follow in the order of their chapters.
	Order int    `yaml:"order"`
	Items []Item `yaml:"items"`

	File string `yaml:"-"`
	// Body is the written chapter of the path, the file paths/<id>.md. It is empty when the path has no such file. When it is not empty, the site shows it in place of the list of the items.
	Body     string `yaml:"-"`
	BodyFile string `yaml:"-"`
}

// Item is one concept of a path.
type Item struct {
	Concept string `yaml:"concept"`
	// Math2 is the tag of the Math 2.0 node. It is empty when Math 2.0 has no node yet, and then Proposal tells where the node is proposed.
	Math2    string   `yaml:"math2"`
	Proposal string   `yaml:"proposal"`
	Says     string   `yaml:"says"`  // what the concept is, in one sentence; for a concept with a node, the site shows the summary of the node
	Text     string   `yaml:"text"`  // a standard text to learn it from: a book or a paper, with the section
	Why      string   `yaml:"why"`   // where the proof uses it
	Steps    []string `yaml:"steps"` // the ids of the steps that use it
}

// LoadPaths reads the files paths/*.yaml. It returns no paths and no error when the directory does not exist. The paths are sorted by id.
func LoadPaths(root string) ([]*Path, error) {
	files, err := filepath.Glob(filepath.Join(root, "paths", "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []*Path
	for _, f := range files {
		data, err := os.ReadFile(f) // #nosec G304 -- a file of the content repository
		if err != nil {
			return nil, err
		}
		rel := "paths/" + filepath.Base(f)
		p := &Path{File: rel}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(p); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		md := strings.TrimSuffix(f, ".yaml") + ".md"
		if body, err := os.ReadFile(md); err == nil { // #nosec G304 -- a file of the content repository
			p.Body = string(body)
			p.BodyFile = strings.TrimSuffix(rel, ".yaml") + ".md"
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

var chapterRE = regexp.MustCompile(`^[0-9]+-([a-z0-9-]+)\.md$`)

// Chapters returns the slugs of the chapters in proof/, in the order of the file names.
func Chapters(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "proof"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if m := chapterRE.FindStringSubmatch(e.Name()); m != nil && !e.IsDir() {
			out = append(out, m[1])
		}
	}
	return out, nil
}

// CheckPaths runs the rules P1 to P5 on the paths.
//
//	P1: the id is the name of the file, and the path has a title and items.
//	P2: each chapter of For exists, and each path of Needs exists, with no cycle.
//	P3: each item has a concept and says where the proof uses it.
//	P4: the tag of Math 2.0 exists, or the item has a proposal or a standard text.
//	P5: each step of an item exists.
//	P6: when the path has a written chapter, the chapter names each concept of the items.
func CheckPaths(paths []*Path, chapters []string, ss []*Step, env Env) []Problem {
	var out []Problem
	chap := map[string]bool{}
	for _, c := range chapters {
		chap[c] = true
	}
	byID := map[string]*Path{}
	for _, p := range paths {
		byID[p.ID] = p
	}
	stepIDs := ByID(ss)
	for _, p := range paths {
		add := func(rule, format string, a ...any) {
			out = append(out, Problem{Step: p.File, Rule: rule, Text: fmt.Sprintf(format, a...)})
		}
		if want := strings.TrimSuffix(filepath.Base(p.File), ".yaml"); p.ID != want {
			add("P1", "the id is %q, but the file name says %q", p.ID, want)
		}
		if p.Title == "" {
			add("P1", "the path has no title")
		}
		if len(p.Items) == 0 {
			add("P1", "the path has no items")
		}
		for _, c := range p.For {
			if !chap[c] {
				add("P2", "the chapter %q does not exist", c)
			}
		}
		for _, n := range p.Needs {
			if byID[n] == nil {
				add("P2", "the path %q does not exist", n)
			}
		}
		if needsCycle(p.ID, byID) {
			add("P2", "the needs of the path make a cycle")
		}
		for i, it := range p.Items {
			name := it.Concept
			if name == "" {
				name = fmt.Sprintf("item %d", i+1)
				add("P3", "%s has no concept", name)
			}
			if strings.TrimSpace(it.Why) == "" {
				add("P3", "%s does not say where the proof uses it (the field why)", name)
			}
			switch {
			case it.Math2 != "" && env.Tag != nil && !env.Tag(it.Math2):
				add("P4", "%s: Math 2.0 has no node %s", name, it.Math2)
			case it.Math2 == "" && it.Proposal == "" && strings.TrimSpace(it.Text) == "":
				add("P4", "%s has no Math 2.0 node, no proposal and no standard text", name)
			}
			for _, s := range it.Steps {
				if stepIDs[s] == nil {
					add("P5", "%s: the step %q does not exist", name, s)
				}
			}
			if p.Body != "" && it.Concept != "" && !strings.Contains(strings.ToLower(p.Body), strings.ToLower(it.Concept)) {
				add("P6", "the chapter %s does not name the concept %q", p.BodyFile, it.Concept)
			}
		}
	}
	return out
}

// needsCycle reports whether the path id can reach itself through Needs.
func needsCycle(id string, byID map[string]*Path) bool {
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(cur string) bool {
		p := byID[cur]
		if p == nil {
			return false
		}
		for _, n := range p.Needs {
			if n == id {
				return true
			}
			if !seen[n] {
				seen[n] = true
				if walk(n) {
					return true
				}
			}
		}
		return false
	}
	return walk(id)
}

// Order returns the paths so that each path comes after the paths it needs. Paths with no order between them keep the order of the chapters they are for.
func Order(paths []*Path, chapters []string) []*Path {
	rank := map[string]int{}
	for i, c := range chapters {
		rank[c] = i
	}
	first := func(p *Path) int {
		if p.Order > 0 {
			return p.Order - 1<<20
		}
		r := len(chapters)
		for _, c := range p.For {
			if v, ok := rank[c]; ok && v < r {
				r = v
			}
		}
		return r
	}
	sorted := append([]*Path(nil), paths...)
	sort.SliceStable(sorted, func(i, j int) bool { return first(sorted[i]) < first(sorted[j]) })
	byID := map[string]*Path{}
	for _, p := range sorted {
		byID[p.ID] = p
	}
	var out []*Path
	done := map[string]bool{}
	var visit func(p *Path)
	visit = func(p *Path) {
		if done[p.ID] {
			return
		}
		done[p.ID] = true
		for _, n := range p.Needs {
			if q := byID[n]; q != nil {
				visit(q)
			}
		}
		out = append(out, p)
	}
	for _, p := range sorted {
		visit(p)
	}
	return out
}
