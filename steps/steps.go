// Package steps reads the step files of the written proof and checks them against the records of the Lean code, with the rules C1 to C6 of spec 05.7.
//
// A step file is proof/steps/<id>.md in the content repository. It has YAML front matter and a Markdown body with the sections Statement and Proof. The site adds the Lean source, the steps that it uses, the steps that use it and the status.
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

// Statuses are the states of a step, from the lowest to the highest.
var Statuses = []string{"drafted", "checked", "reviewed"}

// Step is one step file.
type Step struct {
	ID       string   `yaml:"id"`
	Title    string   `yaml:"title"`
	Decl     string   `yaml:"decl"`    // the Lean declaration that the step explains
	Chapter  string   `yaml:"chapter"` // the slug of the chapter, for example trunk
	Status   string   `yaml:"status"`
	Uses     []string `yaml:"uses"`     // the ids of the steps that the proof cites
	Concepts []string `yaml:"concepts"` // the tags of Math 2.0 nodes
	Routine  []string `yaml:"routine"`  // the Lean dependencies that the proof treats as routine
	Open     []string `yaml:"open"`     // the Lean dependencies that the proof cites, but that have no step yet
	Reviews  []Review `yaml:"reviews"`

	File string `yaml:"-"` // the path in the content repository, with forward slashes
	Body string `yaml:"-"` // the Markdown after the front matter
}

// Review is the record of one person who read the step.
type Review struct {
	By     string `yaml:"by"`
	Date   string `yaml:"date"`
	Status string `yaml:"status"`
	Note   string `yaml:"note"`
}

// Rank returns the place of a status in Statuses, or -1 for an unknown status.
func Rank(status string) int {
	for i, s := range Statuses {
		if s == status {
			return i
		}
	}
	return -1
}

var idRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Load reads the step files in dir/proof/steps. It returns no steps and no error when the directory does not exist. The steps are sorted by id.
func Load(root string) ([]*Step, error) {
	dir := filepath.Join(root, "proof", "steps")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Step
	seen := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		rel := "proof/steps/" + e.Name()
		s, err := read(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		s.File = rel
		if s.ID == "" {
			s.ID = strings.TrimSuffix(e.Name(), ".md")
		}
		if s.ID != strings.TrimSuffix(e.Name(), ".md") {
			return nil, fmt.Errorf("%s: the id %q is not the name of the file", rel, s.ID)
		}
		if !idRE.MatchString(s.ID) {
			return nil, fmt.Errorf("%s: the id %q must have only lower case letters, digits and hyphens", rel, s.ID)
		}
		if old, ok := seen[s.ID]; ok {
			return nil, fmt.Errorf("%s: the id %q is also in %s", rel, s.ID, old)
		}
		seen[s.ID] = rel
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func read(path string) (*Step, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- a file of the content repository
	if err != nil {
		return nil, err
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, errors.New("the file has no front matter")
	}
	rest := data[4:]
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		return nil, errors.New("the front matter has no end line ---")
	}
	s := &Step{Body: string(rest[end+5:])}
	dec := yaml.NewDecoder(bytes.NewReader(rest[:end]))
	dec.KnownFields(true)
	if err := dec.Decode(s); err != nil {
		return nil, fmt.Errorf("front matter: %w", err)
	}
	if s.Title == "" {
		return nil, errors.New("the front matter has no title")
	}
	return s, nil
}

// LeanBlock returns the text of the first lean code block in the body, or "" when the body has none.
func (s *Step) LeanBlock() string {
	i := strings.Index(s.Body, "```lean\n")
	if i < 0 {
		return ""
	}
	rest := s.Body[i+len("```lean\n"):]
	j := strings.Index(rest, "\n```")
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// ByID returns a map from the id to the step.
func ByID(steps []*Step) map[string]*Step {
	m := make(map[string]*Step, len(steps))
	for _, s := range steps {
		m[s.ID] = s
	}
	return m
}

// UsedBy returns, for each step id, the ids of the steps that cite it, sorted.
func UsedBy(steps []*Step) map[string][]string {
	m := map[string][]string{}
	for _, s := range steps {
		for _, u := range s.Uses {
			m[u] = append(m[u], s.ID)
		}
	}
	for k := range m {
		sort.Strings(m[k])
	}
	return m
}
