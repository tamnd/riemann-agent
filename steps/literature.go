package steps

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// Named is one entry of links/literature.yaml: a named result of the Lean code and the work of the literature that it states.
type Named struct {
	Name      string `yaml:"name"`      // the Lean declaration
	Statement string `yaml:"statement"` // the Lean type of the result, when it has a name
	File      string `yaml:"file"`      // relative to lean/src
	Line      int    `yaml:"line"`
	Result    string `yaml:"result"` // the name of the result in the literature
	Says      string `yaml:"says"`   // what the Lean statement says, in words
	// ReadsHypothesis is true when the proof of the Lean declaration uses its argument, the structure of open statements, and false when the code proves the result without it.
	ReadsHypothesis bool   `yaml:"reads_hypothesis"`
	Cite            []Cite `yaml:"cite"`
	Math2           string `yaml:"math2"`
	Note            string `yaml:"note"`
}

// Cite is one work of the literature.
type Cite struct {
	Authors  []string `yaml:"authors"`
	Title    string   `yaml:"title"`
	Venue    string   `yaml:"venue"`
	Year     int      `yaml:"year"`
	DOI      string   `yaml:"doi"`
	ArXiv    string   `yaml:"arxiv"`
	URL      string   `yaml:"url"`
	Verified bool     `yaml:"verified"`
}

// Link returns the best address of the work: the DOI, then arXiv, then the URL.
func (c Cite) Link() string {
	switch {
	case c.DOI != "":
		return "https://doi.org/" + c.DOI
	case c.ArXiv != "":
		return "https://arxiv.org/abs/" + c.ArXiv
	}
	return c.URL
}

// LoadLiterature reads links/literature.yaml. It returns no entries and no error when the file does not exist.
func LoadLiterature(root string) ([]Named, error) {
	p := filepath.Join(root, "links", "literature.yaml")
	data, err := os.ReadFile(p) // #nosec G304 -- a file of the content repository
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Named
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("links/literature.yaml: %w", err)
	}
	seen := map[string]bool{}
	for i, n := range out {
		if n.Name == "" {
			return nil, fmt.Errorf("links/literature.yaml: entry %d has no name", i+1)
		}
		if seen[n.Name] {
			return nil, fmt.Errorf("links/literature.yaml: %s is in the file twice", n.Name)
		}
		seen[n.Name] = true
	}
	return out, nil
}

// CheckLiterature checks that each entry names a declaration in the records and has at least one work with a title.
func CheckLiterature(lit []Named, env Env) []Problem {
	var out []Problem
	for _, n := range lit {
		add := func(format string, a ...any) {
			out = append(out, Problem{Step: "links/literature.yaml", Rule: "L1", Text: n.Name + ": " + fmt.Sprintf(format, a...)})
		}
		if _, ok := env.Graph.ID[n.Name]; !ok {
			add("the records have no such declaration")
		}
		if len(n.Cite) == 0 {
			add("the entry has no citation")
		}
		for _, c := range n.Cite {
			if c.Title == "" || len(c.Authors) == 0 || c.Year == 0 {
				add("a citation needs the authors, the title and the year")
			}
		}
		if n.Math2 != "" && env.Tag != nil && !env.Tag(n.Math2) {
			add("Math 2.0 has no node %s", n.Math2)
		}
	}
	return out
}
