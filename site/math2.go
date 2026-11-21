package site

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultMath2 is the index of the nodes of Math 2.0.
const DefaultMath2 = "https://math2.tamnd.com/api/nodes.json"

// Node is one node of Math 2.0, as api/nodes.json gives it.
type Node struct {
	Tag  string `json:"tag"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	URL  string `json:"url"`
	// Aliases are the other names of the node. rh link matches the concepts of the paths against the name and the aliases.
	Aliases []string `json:"aliases"`
}

// Math2 is the index of the nodes of Math 2.0, by tag.
type Math2 struct {
	Commit string
	Nodes  map[string]Node
}

// maxIndex is the largest index that LoadMath2 reads. The index of November 2026 is near 9 MB.
const maxIndex = 256 << 20

// LoadMath2 reads the index from a file or from an http or https URL. An empty src gives an empty index, and each link to Math 2.0 is then missing.
func LoadMath2(ctx context.Context, src string) (*Math2, error) {
	m := &Math2{Nodes: map[string]Node{}}
	if src == "" {
		return m, nil
	}
	var r io.ReadCloser
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req) // #nosec G704 -- the URL is a flag of the operator
		if err != nil {
			return nil, fmt.Errorf("math2 index: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("math2 index: %s: %s", src, resp.Status)
		}
		r = resp.Body
	} else {
		f, err := os.Open(src) // #nosec G304 -- the path is a flag of the operator
		if err != nil {
			return nil, fmt.Errorf("math2 index: %w", err)
		}
		r = f
	}
	defer func() { _ = r.Close() }()
	var idx struct {
		Commit string `json:"commit"`
		Nodes  []Node `json:"nodes"`
	}
	if err := json.NewDecoder(io.LimitReader(r, maxIndex)).Decode(&idx); err != nil {
		return nil, fmt.Errorf("math2 index: %s: %w", src, err)
	}
	m.Commit = idx.Commit
	for _, n := range idx.Nodes {
		m.Nodes[n.Tag] = n
	}
	return m, nil
}

// Lookup returns the node with the tag.
func (m *Math2) Lookup(tag string) (Node, bool) {
	n, ok := m.Nodes[tag]
	return n, ok
}
