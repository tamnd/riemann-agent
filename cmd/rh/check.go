package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/tamnd/riemann-agent/index"
	"github.com/tamnd/riemann-agent/site"
	"github.com/tamnd/riemann-agent/steps"
)

// errCheck is a failed check. main exits with code 1.
var errCheck = errors.New("the check failed")

func runCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("rh check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	content := fs.String("content", "../riemann", "the root of the content repository")
	indexDir := fs.String("index", "", "the records of rh index (required)")
	math2 := fs.String("math2", site.DefaultMath2, "the index of Math 2.0, a file or a URL; empty to skip rule C4")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if *indexDir == "" || fs.NArg() > 0 {
		return fmt.Errorf("%w: check needs -index and takes no arguments", errUsage)
	}
	ss, err := steps.Load(*content)
	if err != nil {
		return err
	}
	lit, err := steps.LoadLiterature(*content)
	if err != nil {
		return err
	}
	ix, err := index.Load(*indexDir)
	if err != nil {
		return err
	}
	env := steps.Env{Graph: index.NewGraph(ix)}
	cited := map[string]bool{}
	for _, n := range lit {
		cited[n.Name] = true
	}
	env.Cut = func(name string) bool { return cited[name] }
	if *math2 != "" {
		m, err := site.LoadMath2(context.Background(), *math2)
		if err != nil {
			return err
		}
		env.Tag = func(t string) bool { _, ok := m.Lookup(t); return ok }
	}
	paths, err := steps.LoadPaths(*content)
	if err != nil {
		return err
	}
	chapters, err := steps.Chapters(*content)
	if err != nil {
		return err
	}
	gl, err := steps.LoadGlossary(*content)
	if err != nil {
		return err
	}
	problems := append(steps.Check(ss, env), steps.CheckLiterature(lit, env)...)
	problems = append(problems, steps.CheckPaths(paths, chapters, ss, env)...)
	problems = append(problems, steps.CheckGlossary(gl, ss, env)...)
	for _, p := range problems {
		fmt.Fprintln(stdout, p)
	}
	counts := map[string]int{}
	open := 0
	for _, s := range ss {
		counts[s.Status]++
		open += len(s.Open)
	}
	fmt.Fprintf(stdout, "%d steps (%d drafted, %d checked, %d reviewed), %d dependencies with no step yet, %d named results, %d paths, %d glossary names, %d problems\n", len(ss), counts["drafted"], counts["checked"], counts["reviewed"], open, len(lit), len(paths), len(gl), len(problems))
	if len(problems) > 0 {
		return errCheck
	}
	return nil
}
