// Package index reads the Lean source of the release and makes the records of modules and declarations, the module graph and the syntactic declaration graph. Spec 04 gives the rules.
package index

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/tamnd/riemann-agent/lean/syntax"
)

// WeakLimit is the most declarations that one identifier in dot notation can link to. A name such as .mk or .re matches too many declarations to be useful.
const WeakLimit = 4

// Module is one Lean file.
type Module struct {
	Name    string   `json:"module"`
	Path    string   `json:"path"` // relative to the source root, with forward slashes
	Lines   int      `json:"lines"`
	Bytes   int      `json:"bytes"`
	SHA256  string   `json:"sha256"`
	Imports []string `json:"imports"`
	Decls   int      `json:"decls"`
	Depth   int      `json:"depth"` // the length of the longest import chain from Main; -1 when Main does not import the module

	decls    []syntax.Decl
	examples int
}

// Decl is the record of one declaration.
type Decl struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Module  string   `json:"module"`
	Line    int      `json:"line"`
	Span    [2]int   `json:"span"`
	Private bool     `json:"private,omitempty"`
	Attrs   string   `json:"attrs,omitempty"`
	Doc     string   `json:"doc,omitempty"`
	Sig     string   `json:"sig"`
	Uses    []string `json:"uses"`
	Weak    []string `json:"weak"`
	Graph   string   `json:"graph"`
}

// Index is the result of Build.
type Index struct {
	Modules []*Module // sorted by name
	Decls   []Decl    // sorted by name, then module
	Summary Summary
}

// Summary has the numbers of the release.
type Summary struct {
	Modules      int            `json:"modules"`
	Lines        int            `json:"lines"`
	Bytes        int            `json:"bytes"`
	Decls        int            `json:"decls"`
	Kinds        map[string]int `json:"kinds"`
	Private      int            `json:"private"`
	Examples     int            `json:"examples"`
	Libraries    []Library      `json:"libraries"`
	Imports      int            `json:"imports"`         // the import edges between modules of the release
	ExternalMods []string       `json:"external"`        // the modules that the release imports and does not have, such as Mathlib
	Unreachable  int            `json:"unreachable"`     // the modules that Main does not import
	LongestChain int            `json:"longest_chain"`   // the number of modules on the longest import chain from Main
	Uses         int            `json:"uses"`            // the strong edges of the declaration graph
	Weak         int            `json:"weak"`            // the weak edges
	Duplicates   int            `json:"duplicate_names"` // the public names that two declarations have
	Exact        int            `json:"exact,omitempty"` // the declarations whose uses come from the built environment (rh extract)
}

// Library is one top directory of the source.
type Library struct {
	Name    string `json:"name"`
	Modules int    `json:"modules"`
	Lines   int    `json:"lines"`
	Bytes   int    `json:"bytes"`
	Decls   int    `json:"decls"`
}

// Build reads the .lean files under root. root is lean/src of riemann-lean.
func Build(root string) (*Index, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != root {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".lean") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("no .lean files under " + root)
	}
	sort.Strings(paths)
	mods := make([]*Module, len(paths))
	errs := make([]error, len(paths))
	var wg sync.WaitGroup
	next := make(chan int)
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				mods[i], errs[i] = readModule(root, paths[i])
			}
		}()
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	ix := &Index{Modules: mods}
	ix.moduleGraph()
	ix.declGraph()
	ix.summarize()
	return ix, nil
}

func readModule(root, p string) (*Module, error) {
	src, err := os.ReadFile(p) // #nosec G304 -- a file under the source root
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)
	sum := sha256.Sum256(src)
	f := syntax.Parse(src)
	m := &Module{
		Name:     strings.ReplaceAll(strings.TrimSuffix(rel, ".lean"), "/", "."),
		Path:     rel,
		Lines:    lineCount(src),
		Bytes:    len(src),
		SHA256:   hex.EncodeToString(sum[:]),
		Imports:  f.Imports,
		Decls:    len(f.Decls),
		decls:    f.Decls,
		examples: f.Examples,
	}
	if m.Imports == nil {
		m.Imports = []string{}
	}
	return m, nil
}

func lineCount(src []byte) int {
	n := 0
	for _, c := range src {
		if c == '\n' {
			n++
		}
	}
	if len(src) > 0 && src[len(src)-1] != '\n' {
		n++
	}
	return n
}

// moduleGraph sets the depth of each module: the length of the longest import chain from Main.
func (ix *Index) moduleGraph() {
	byName := make(map[string]int, len(ix.Modules))
	for i, m := range ix.Modules {
		byName[m.Name] = i
		m.Depth = -1
	}
	main, ok := byName["Main"]
	if !ok {
		return
	}
	// Order the modules that Main imports so that a module comes before its imports, then relax the depths.
	order := make([]int, 0, len(ix.Modules))
	state := make([]uint8, len(ix.Modules)) // 0 new, 1 open, 2 done
	type frame struct{ m, next int }
	stack := []frame{{main, 0}}
	state[main] = 1
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		imps := ix.Modules[top.m].Imports
		if top.next < len(imps) {
			j, ok := byName[imps[top.next]]
			top.next++
			if ok && state[j] == 0 {
				state[j] = 1
				stack = append(stack, frame{j, 0})
			}
			continue
		}
		state[top.m] = 2
		order = append(order, top.m)
		stack = stack[:len(stack)-1]
	}
	// order has each module after its imports. Walk it backwards from Main.
	ix.Modules[main].Depth = 0
	for k := len(order) - 1; k >= 0; k-- {
		m := ix.Modules[order[k]]
		for _, imp := range m.Imports {
			if j, ok := byName[imp]; ok && ix.Modules[j].Depth < m.Depth+1 {
				ix.Modules[j].Depth = m.Depth + 1
			}
		}
	}
}
