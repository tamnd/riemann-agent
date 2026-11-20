package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/riemann-agent/index"
)

func loadGraph(dir string) (*index.Graph, error) {
	ix, err := index.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("read the records (run rh index first): %w", err)
	}
	return index.NewGraph(ix), nil
}

func find(g *index.Graph, name string) (int32, error) {
	i, hits := g.Lookup(name)
	if i >= 0 {
		return i, nil
	}
	if len(hits) == 0 {
		return 0, fmt.Errorf("no declaration %s", name)
	}
	if len(hits) > 10 {
		hits = append(hits[:10], "...")
	}
	return 0, fmt.Errorf("%s matches more than one declaration: %s", name, strings.Join(hits, ", "))
}

func runGraph(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("index", "work/index", "the records of rh index")
	weak := fs.Bool("weak", false, "follow the weak edges too")
	count := fs.Bool("count", false, "print only the number of results")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: rh graph [flags] uses NAME | by NAME | path FROM TO | closure NAME")
		fmt.Fprintln(stderr, "Query the syntactic declaration graph. uses lists what NAME uses, by lists what uses NAME, path finds a shortest chain of uses, and closure lists all that NAME uses.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	a := fs.Args()
	need := map[string]int{"uses": 2, "by": 2, "path": 3, "closure": 2}
	if len(a) == 0 || need[a[0]] != len(a) {
		fs.Usage()
		return fmt.Errorf("%w: give a query", errUsage)
	}
	g, err := loadGraph(*dir)
	if err != nil {
		return err
	}
	x, err := find(g, a[1])
	if err != nil {
		return err
	}
	var out []int32
	switch a[0] {
	case "uses":
		out = g.Next(x, *weak)
	case "by":
		out = g.By[x]
	case "closure":
		out = g.Closure(x, *weak, nil)[1:]
	case "path":
		y, err := find(g, a[2])
		if err != nil {
			return err
		}
		out = g.Path(x, y, *weak)
		if out == nil {
			return fmt.Errorf("%s does not use %s", g.Decls[x].Name, g.Decls[y].Name)
		}
	}
	if *count {
		fmt.Fprintln(stdout, len(out))
		return nil
	}
	for _, i := range out {
		d := g.Decls[i]
		fmt.Fprintf(stdout, "%s\t%s\t%s:%d\n", d.Name, d.Kind, d.Module, d.Line)
	}
	return nil
}

func runSpine(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("spine", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("index", "work/index", "the records of rh index")
	root := fs.String("root", "riemannHypothesis", "the root declaration")
	out := fs.String("out", "", "write spine.json and spine.jsonl.gz to this directory")
	weak := fs.Bool("weak", false, "follow the weak edges too")
	nocut := fs.Bool("nocut", false, "do not stop at the named results of spec 04.6")
	routine := fs.String("routine", "", "a file with the names that a person marked routine, one on each line")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: rh spine [flags]")
		fmt.Fprintln(stderr, "Find the declarations that the root uses, with the cut points of spec 04.4, the depth, fan-in and size of each, the trunk and the chapter candidates.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	opt := index.SpineOptions{Weak: *weak, NoCut: *nocut, Routine: map[string]bool{}}
	if *routine != "" {
		data, err := os.ReadFile(*routine)
		if err != nil {
			return err
		}
		for _, l := range strings.Split(string(data), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				opt.Routine[l] = true
			}
		}
	}
	g, err := loadGraph(*dir)
	if err != nil {
		return err
	}
	r, err := find(g, *root)
	if err != nil {
		return err
	}
	sp := index.BuildSpine(g, r, opt)
	data, err := json.MarshalIndent(sp, "", "  ")
	if err != nil {
		return err
	}
	if *out == "" {
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if err := os.MkdirAll(*out, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "spine.json"), append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := sp.WriteNodes(filepath.Join(*out, "spine.jsonl.gz")); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "the spine of %s has %d declarations (%d with no cut points), wrote %s\n", sp.Root, sp.Size, sp.Full, *out)
	return nil
}
