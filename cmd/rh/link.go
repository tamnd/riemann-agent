package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/riemann-agent/site"
	"github.com/tamnd/riemann-agent/steps"
)

func runLink(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("rh link", flag.ContinueOnError)
	fs.SetOutput(stderr)
	from := fs.String("from", "../math2", "a checkout of tamnd/math2")
	content := fs.String("content", "../riemann", "the root of the content repository")
	math2 := fs.String("math2", site.DefaultMath2, "the index of Math 2.0, a file or a URL")
	dry := fs.Bool("n", false, "print the counts and do not write the file")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: link takes no arguments", errUsage)
	}
	m, err := site.LoadMath2(context.Background(), *math2)
	if err != nil {
		return err
	}
	found, bad, err := scanMath2(*from)
	if err != nil {
		return err
	}
	old, err := steps.LoadLinks(*content)
	if err != nil {
		return err
	}
	out := &steps.Links{Decls: map[string]string{}, Concepts: map[string]string{}}
	dropped := 0
	for d, tag := range found {
		if _, ok := m.Lookup(tag); ok {
			out.Decls[d] = tag
		} else {
			dropped++
		}
	}
	kept := 0
	for d, tag := range old.Decls {
		if found[d] != tag {
			kept++
		}
		out.Decls[d] = tag
	}
	for c, tag := range old.Concepts {
		out.Concepts[c] = tag
	}
	paths, err := steps.LoadPaths(*content)
	if err != nil {
		return err
	}
	byName := map[string][]string{}
	for _, n := range m.Nodes {
		names := append([]string{n.Name}, n.Aliases...)
		for _, s := range names {
			k := conceptKey(s)
			if k != "" && !contains(byName[k], n.Tag) {
				byName[k] = append(byName[k], n.Tag)
			}
		}
	}
	filled, missing := 0, []string{}
	for _, p := range paths {
		for _, it := range p.Items {
			if it.Math2 != "" || out.Concepts[it.Concept] != "" {
				continue
			}
			if tags := byName[conceptKey(it.Concept)]; len(tags) == 1 {
				out.Concepts[it.Concept] = tags[0]
				filled++
			} else {
				missing = append(missing, fmt.Sprintf("%s: %s", p.File, it.Concept))
			}
		}
	}
	fmt.Fprintf(stdout, "%d Lean names with a node in Math 2.0 (%d with a tag that the index does not have, %d not read), %d kept from the file, %d concepts filled, %d concepts with no node\n", len(out.Decls), dropped, bad, kept, filled, len(missing))
	for _, s := range missing {
		fmt.Fprintln(stdout, "  no node:", s)
	}
	if *dry {
		return nil
	}
	dir := filepath.Join(*content, "links")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "math2.yaml"), out.Marshal(), 0o600)
}

// conceptKey is the form of a name that rh link compares: lower case, with no plural s and no punctuation.
func conceptKey(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '-' || r == '_' || r == '–':
			return ' '
		case r == '\'' || r == '’' || r == ',' || r == '.':
			return -1
		}
		return r
	}, s)
	ws := strings.Fields(s)
	for i, w := range ws {
		if len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
			ws[i] = strings.TrimSuffix(w, "s")
		}
	}
	return strings.Join(ws, " ")
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// scanMath2 reads the Lean names of a checkout of math2: the formal.decl fields of the nodes and the decl fields of the items of the seed list and the skeletons. It returns the tag for each name. When two sources give two tags for one name, the node wins. bad counts the files that it cannot read.
func scanMath2(root string) (map[string]string, int, error) {
	out := map[string]string{}
	bad := 0
	nodes := filepath.Join(root, "nodes")
	if _, err := os.Stat(nodes); err != nil {
		return nil, 0, fmt.Errorf("rh link: %s is not a checkout of math2: %w", root, err)
	}
	err := filepath.WalkDir(nodes, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		data, err := os.ReadFile(p) // #nosec G304 -- a file of the math2 checkout
		if err != nil {
			return err
		}
		fm, ok := frontMatter(data)
		if !ok || !bytes.Contains(fm, []byte("formal:")) {
			return nil
		}
		var n struct {
			Tag    string `yaml:"tag"`
			Formal []struct {
				System string `yaml:"system"`
				Decl   string `yaml:"decl"`
			} `yaml:"formal"`
		}
		if yaml.Unmarshal(fm, &n) != nil {
			bad++
			return nil
		}
		for _, f := range n.Formal {
			if f.Decl != "" && n.Tag != "" && (f.System == "" || f.System == "lean4") {
				out[f.Decl] = n.Tag
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	lists, _ := filepath.Glob(filepath.Join(root, "topics", "skeletons", "*.yaml"))
	more, _ := filepath.Glob(filepath.Join(root, "topics", "curricula", "*.yaml"))
	for _, f := range append(lists, more...) {
		data, err := os.ReadFile(f) // #nosec G304 -- a file of the math2 checkout
		if err != nil {
			return nil, 0, err
		}
		var v any
		if yaml.Unmarshal(data, &v) != nil {
			bad++
			continue
		}
		walkItems(v, func(decl, tag string) {
			if _, ok := out[decl]; !ok {
				out[decl] = tag
			}
		})
	}
	return out, bad, nil
}

// walkItems calls f for each map in v with a decl and a node field.
func walkItems(v any, f func(decl, tag string)) {
	switch x := v.(type) {
	case map[string]any:
		d, _ := x["decl"].(string)
		t, _ := x["node"].(string)
		if d != "" && t != "" {
			f(d, t)
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkItems(x[k], f)
		}
	case []any:
		for _, e := range x {
			walkItems(e, f)
		}
	}
}

// frontMatter returns the YAML between the first two lines of three dashes.
func frontMatter(data []byte) ([]byte, bool) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, false
	}
	rest := data[4:]
	i := bytes.Index(rest, []byte("\n---"))
	if i < 0 {
		return nil, false
	}
	return rest[:i+1], true
}
