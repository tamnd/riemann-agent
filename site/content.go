package site

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Source is manifests/source.yaml of the content repository: the commit of riemann-lean that the pages describe.
type Source struct {
	Repository    string `yaml:"repository"`
	Commit        string `yaml:"commit"`
	Release       string `yaml:"release"`
	ArchiveSHA256 string `yaml:"archive_sha256"`
	URL           string `yaml:"url"`
}

// Front is the front matter of a Markdown page.
type Front struct {
	Title       string    `yaml:"title"`
	Description string    `yaml:"description"`
	Lead        string    `yaml:"lead"`
	Level       string    `yaml:"level"`
	Status      string    `yaml:"status"`
	Eyebrow     string    `yaml:"eyebrow"`
	Doors       []Door    `yaml:"doors"`
	Lean        []LeanRef `yaml:"lean"`
	Needs       []Need    `yaml:"needs"`
	Next        []Need    `yaml:"next"`
	Steps       []string  `yaml:"steps"` // the step files of a chapter, in reading order
}

// Door is an entry point on the home page, for one kind of reader.
type Door struct {
	Title string `yaml:"title"`
	Path  string `yaml:"path"`
	Text  string `yaml:"text"`
}

// LeanRef names a Lean declaration and the place of its source.
type LeanRef struct {
	Name string `yaml:"name"`
	File string `yaml:"file"` // relative to lean/src, for example Main.lean
	Line int    `yaml:"line"`
}

// Need is a concept or a page that a page needs. Tag is a node of Math 2.0; Path is a page of this site; URL is any other address.
type Need struct {
	Text string `yaml:"text"`
	Tag  string `yaml:"tag"`
	Path string `yaml:"path"`
	URL  string `yaml:"url"`
	Sub  string `yaml:"sub"`
}

// Timeline is proof/timeline.yaml: the steps of the note in the order of the note.
type Timeline struct {
	Title       string          `yaml:"title"`
	Description string          `yaml:"description"`
	Lead        string          `yaml:"lead"`
	Intro       string          `yaml:"intro"`
	Steps       []TimelineEntry `yaml:"steps"`
}

// TimelineEntry is one step of the timeline.
type TimelineEntry struct {
	N       int    `yaml:"n"`
	Date    string `yaml:"date"`
	Text    string `yaml:"text"`
	Chapter string `yaml:"chapter"`

	HTML template.HTML `yaml:"-"` // Text, rendered by the build
}

// Trust is the trust report, records/trust.json. Until milestone M2 the file does not exist, and the site shows each check as not run.
type Trust struct {
	Updated string       `json:"updated,omitempty"`
	Checks  []TrustCheck `json:"checks"`
}

// TrustCheck is one question of the trust report.
type TrustCheck struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	State    string `json:"state"` // yes, no, open or "not run"
	Note     string `json:"note,omitempty"`
}

// Class returns the CSS class of the state.
func (c TrustCheck) Class() string {
	switch c.State {
	case "yes":
		return "yes"
	case "no":
		return "no"
	case "open":
		return "open"
	}
	return ""
}

// DefaultTrust is the trust report before any check runs. The questions are those of spec 05.1.
func DefaultTrust() Trust {
	return Trust{Checks: []TrustCheck{
		{ID: "Q0", Question: "Is the source the same as the release?", State: "not run", Note: "Run rh fetch -compare."},
		{ID: "Q1", Question: "Does the project build with the pinned Lean and Mathlib?", State: "not run", Note: "Milestone M2."},
		{ID: "Q2", Question: "Which axioms does riemannHypothesis use?", State: "not run", Note: "Milestone M2, with #print axioms."},
		{ID: "Q3", Question: "Does a second kernel accept the proof?", State: "not run", Note: "Milestone M2, with lean4checker."},
		{ID: "Q4", Question: "Do the Lean statements say what the standard statements say?", State: "not run", Note: "Milestone M2, a review by a named person."},
		{ID: "Q5", Question: "How much of the proof is written in words?", State: "not run", Note: "Milestone M3 and later."},
	}}
}

// Claimed reports whether each check of the report says yes.
func (t Trust) Claimed() bool {
	if len(t.Checks) == 0 {
		return false
	}
	for _, c := range t.Checks {
		if c.State != "yes" {
			return false
		}
	}
	return true
}

// doc is one Markdown file with its front matter.
type doc struct {
	Rel   string // the path in the content repository, with forward slashes
	Front Front
	Body  string // the Markdown after the front matter
}

// splitFront splits a Markdown file into its YAML front matter and its body.
func splitFront(data []byte) ([]byte, string, error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, string(data), nil
	}
	rest := data[4:]
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		if bytes.HasSuffix(rest, []byte("\n---")) {
			return rest[:len(rest)-4], "", nil
		}
		return nil, "", errors.New("the front matter has no end line ---")
	}
	return rest[:end], string(rest[end+5:]), nil
}

func readDoc(root, rel string) (doc, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a file of the content repository
	if err != nil {
		return doc{}, err
	}
	fm, body, err := splitFront(data)
	if err != nil {
		return doc{}, fmt.Errorf("%s: %w", rel, err)
	}
	d := doc{Rel: rel, Body: body}
	if fm != nil {
		dec := yaml.NewDecoder(bytes.NewReader(fm))
		dec.KnownFields(true)
		if err := dec.Decode(&d.Front); err != nil {
			return doc{}, fmt.Errorf("%s: front matter: %w", rel, err)
		}
	}
	if d.Front.Title == "" {
		return doc{}, fmt.Errorf("%s: the front matter has no title", rel)
	}
	return d, nil
}

func readYAML(path string, v any) error {
	data, err := os.ReadFile(path) // #nosec G304 -- a file of the content repository
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// readTrust reads records/trust.json, or returns the default report when the file does not exist.
func readTrust(root string) (Trust, error) {
	data, err := os.ReadFile(filepath.Join(root, "records", "trust.json")) // #nosec G304 -- a file of the content repository
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultTrust(), nil
	}
	if err != nil {
		return Trust{}, err
	}
	var t Trust
	if err := json.Unmarshal(data, &t); err != nil {
		return Trust{}, fmt.Errorf("records/trust.json: %w", err)
	}
	return t, nil
}

// sitePages lists the Markdown files in site/ of the content repository, sorted.
func sitePages(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "site"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, "site/"+e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// Link returns the path of the chapter of the step, for example proof/ for the overview.
func (e TimelineEntry) Link() string {
	return chapterPath(e.Chapter + ".md")
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path) // #nosec G304 -- an input file of the build
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
