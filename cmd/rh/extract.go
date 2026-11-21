package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/riemann-agent/extract"
	"github.com/tamnd/riemann-agent/index"
)

func runExtract(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("rh extract", flag.ContinueOnError)
	fs.SetOutput(stderr)
	indexDir := fs.String("index", "", "the records of rh index (required)")
	out := fs.String("out", "", "the directory for the new records (required unless -print)")
	lean := fs.String("lean", "", "the root of the built Lean project; rh runs the Lean file of the extract there")
	exactFile := fs.String("exact", "", "the output of an earlier run of the Lean file, plain or gzip; use it instead of -lean")
	keep := fs.String("keep", "", "with -lean, also keep the output of the Lean file at this path")
	lake := fs.String("lake", "lake", "the lake program")
	module := fs.String("module", "Main", "the module to import")
	timeout := fs.Duration("timeout", 0, "the time limit of the Lean run, for example 6h; 0 for no limit")
	printFile := fs.Bool("print", false, "print the Lean file and stop; the roots come from -index")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if *indexDir == "" || fs.NArg() > 0 {
		return fmt.Errorf("%w: extract needs -index and takes no arguments", errUsage)
	}
	if !*printFile && (*out == "" || (*lean == "") == (*exactFile == "")) {
		return fmt.Errorf("%w: extract needs -out and one of -lean and -exact", errUsage)
	}
	ix, err := index.Load(*indexDir)
	if err != nil {
		return err
	}
	roots := map[string]bool{}
	for _, m := range ix.Modules {
		roots[strings.SplitN(m.Name, ".", 2)[0]] = true
	}
	var rs []string
	for r := range roots {
		rs = append(rs, r)
	}
	sort.Strings(rs)
	if *printFile {
		_, err := io.WriteString(stdout, extract.File(*module, rs, "rh-extract.jsonl"))
		return err
	}

	path := *exactFile
	if *lean != "" {
		tmp, err := os.MkdirTemp("", "rh-extract-")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		path = filepath.Join(tmp, "constants.jsonl")
		if *keep != "" {
			if path, err = filepath.Abs(*keep); err != nil {
				return err
			}
		}
		src := filepath.Join(tmp, "RhExtract.lean")
		if err := os.WriteFile(src, []byte(extract.File(*module, rs, path)), 0o600); err != nil {
			return err
		}
		ctx := context.Background()
		if *timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *timeout)
			defer cancel()
		}
		cmd := exec.CommandContext(ctx, *lake, "env", "lean", src) // #nosec G204 -- the program comes from the flags of rh
		cmd.Dir = *lean
		start := time.Now()
		res, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(stderr, "%s\n", lastLines(res, 30))
			return fmt.Errorf("the Lean run: %w", err)
		}
		fmt.Fprintf(stdout, "%s (%s)\n", strings.TrimSpace(lastLines(res, 1)), time.Since(start).Round(time.Second))
	}
	cs, err := extract.ReadFile(path)
	if err != nil {
		return err
	}
	st := extract.Merge(ix, cs)
	ix.Summary.Exact = st.Matched
	if err := ix.Write(*out); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "extract.json"), append(data, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d constants, %d declarations with the exact graph, %d with the syntactic graph, %d auxiliary constants; %d edges added and %d removed\n", st.Constants, st.Matched, st.Missing, st.Auxiliary, st.Added, st.Removed)
	return nil
}
