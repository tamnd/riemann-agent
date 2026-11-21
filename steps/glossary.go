package steps

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Entry is one name of glossary/names.yaml: a Lean name of the proof and what it means (spec 07).
type Entry struct {
	Name   string `yaml:"name"`   // the Lean name, or a name space that ends with a dot
	Kind   string `yaml:"kind"`   // the kind in the records, or namespace
	Says   string `yaml:"says"`   // what it means, in words
	Math   string `yaml:"math"`   // the usual notation, in TeX
	Math2  string `yaml:"math2"`  // the tag of the Math 2.0 node, when there is one
	Step   string `yaml:"step"`   // the id of the step that explains it
	Status string `yaml:"status"` // open when the meaning is not known yet, or drafted, checked or reviewed
}

// LoadGlossary reads glossary/names.yaml. It returns no entries and no error when the file does not exist.
func LoadGlossary(root string) ([]Entry, error) {
	data, err := os.ReadFile(filepath.Join(root, "glossary", "names.yaml")) // #nosec G304 -- a file of the content repository
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("glossary/names.yaml: %w", err)
	}
	return out, nil
}

// CheckGlossary runs the rules G1 to G4 on the glossary.
//
//	G1: each name is in the file once, and has a text.
//	G2: the name is a declaration of the records with that kind, or a name space of declarations.
//	G3: the tag of Math 2.0 exists.
//	G4: the step exists, and the status is open or a status of a step.
func CheckGlossary(es []Entry, ss []*Step, env Env) []Problem {
	const file = "glossary/names.yaml"
	var out []Problem
	add := func(rule, format string, a ...any) {
		out = append(out, Problem{Step: file, Rule: rule, Text: fmt.Sprintf(format, a...)})
	}
	seen := map[string]bool{}
	stepIDs := ByID(ss)
	var prefixes map[string]bool
	for _, e := range es {
		if e.Name == "" {
			add("G1", "an entry has no name")
			continue
		}
		if seen[e.Name] {
			add("G1", "%s is in the file twice", e.Name)
		}
		seen[e.Name] = true
		if strings.TrimSpace(e.Says) == "" {
			add("G1", "%s has no text", e.Name)
		}
		if g := env.Graph; g != nil {
			if e.Kind == "namespace" {
				if prefixes == nil {
					prefixes = namespaces(env)
				}
				if !prefixes[strings.TrimSuffix(e.Name, ".")] {
					add("G2", "no declaration is in the name space %s", e.Name)
				}
			} else if id, ok := g.ID[e.Name]; !ok {
				add("G2", "%s is not in the records", e.Name)
			} else if k := g.Decls[id].Kind; e.Kind != "" && e.Kind != k {
				add("G2", "%s is a %s in the records, not a %s", e.Name, k, e.Kind)
			}
		}
		if e.Math2 != "" && env.Tag != nil && !env.Tag(e.Math2) {
			add("G3", "%s: Math 2.0 has no node %s", e.Name, e.Math2)
		}
		if e.Step != "" && stepIDs[e.Step] == nil {
			add("G4", "%s: the step %q does not exist", e.Name, e.Step)
		}
		if e.Status != "" && e.Status != "open" && Rank(e.Status) < 0 {
			add("G4", "%s: the status %q is not known", e.Name, e.Status)
		}
	}
	return out
}

// namespaces returns each proper prefix of each declaration name.
func namespaces(env Env) map[string]bool {
	out := map[string]bool{}
	for _, d := range env.Graph.Decls {
		n := d.Name
		for i := strings.LastIndexByte(n, '.'); i > 0; i = strings.LastIndexByte(n, '.') {
			n = n[:i]
			if out[n] {
				break
			}
			out[n] = true
		}
	}
	return out
}
