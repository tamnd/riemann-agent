package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/tamnd/riemann-agent/index"
)

func runIndex(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	fs.SetOutput(stderr)
	src := fs.String("src", "../riemann-lean/lean/src", "the Lean source: lean/src of riemann-lean")
	out := fs.String("out", "work/index", "the directory of the records")
	summary := fs.Bool("summary", true, "print the summary")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: rh index [-src DIR] [-out DIR]")
		fmt.Fprintln(stderr, "Read the Lean source without Lean and write modules.jsonl.gz, decls/<xx>.jsonl.gz and summary.json.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: index takes no arguments", errUsage)
	}
	start := time.Now()
	ix, err := index.Build(*src)
	if err != nil {
		return err
	}
	if err := ix.Write(*out); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "read %d modules and %d declarations in %s, wrote %s\n", ix.Summary.Modules, ix.Summary.Decls, time.Since(start).Round(time.Second), *out)
	if *summary {
		data, err := json.MarshalIndent(ix.Summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
	}
	return nil
}
