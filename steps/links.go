package steps

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Links is links/math2.yaml: the tables from Lean names and from concept names to Math 2.0 tags (spec 07.3).
type Links struct {
	Decls    map[string]string `yaml:"decls"`
	Concepts map[string]string `yaml:"concepts"`
}

// LoadLinks reads links/math2.yaml. It returns empty tables and no error when the file does not exist.
func LoadLinks(root string) (*Links, error) {
	l := &Links{Decls: map[string]string{}, Concepts: map[string]string{}}
	data, err := os.ReadFile(filepath.Join(root, "links", "math2.yaml")) // #nosec G304 -- a file of the content repository
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(l); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("links/math2.yaml: %w", err)
	}
	if l.Decls == nil {
		l.Decls = map[string]string{}
	}
	if l.Concepts == nil {
		l.Concepts = map[string]string{}
	}
	return l, nil
}

// Concept returns the tag of the concept: the tag of the item, then the tag of the table.
func (l *Links) Concept(it Item) string {
	if it.Math2 != "" || l == nil {
		return it.Math2
	}
	return l.Concepts[it.Concept]
}

const linksHeader = `# The links from Lean names and from the concepts of the paths to the nodes of Math 2.0 (spec 07.3).
# The command rh link writes this file. It reads the formal.decl fields of the nodes of Math 2.0 and the decl fields of the seed list and the skeletons. An entry that a person adds stays, and it wins over an entry that rh link finds.
`

// Marshal returns the file, with the keys in order.
func (l *Links) Marshal() []byte {
	var b bytes.Buffer
	b.WriteString(linksHeader)
	write := func(name string, m map[string]string) {
		if len(m) == 0 {
			fmt.Fprintf(&b, "%s: {}\n", name)
			return
		}
		fmt.Fprintf(&b, "%s:\n", name)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  %s: %s\n", yamlKey(k), m[k])
		}
	}
	write("decls", l.Decls)
	write("concepts", l.Concepts)
	return b.Bytes()
}

// yamlKey quotes a key when YAML would read it as something other than the plain string.
func yamlKey(k string) string {
	out, err := yaml.Marshal(map[string]int{k: 0})
	if err != nil {
		return fmt.Sprintf("%q", k)
	}
	return strings.TrimSuffix(string(out), ": 0\n")
}
