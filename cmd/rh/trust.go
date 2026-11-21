package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/riemann-agent/site"
	"github.com/tamnd/riemann-agent/trust"
)

func runTrust(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("rh trust", flag.ContinueOnError)
	fs.SetOutput(stderr)
	lean := fs.String("lean", "", "the root of the built Lean project, with lakefile.toml (required unless -print)")
	content := fs.String("content", "", "the root of the content repository; when set, write records/axioms.json, records/replay.json and records/trust.json there")
	lake := fs.String("lake", "lake", "the lake program")
	module := fs.String("module", "Main", "the module with the theorems")
	names := fs.String("theorems", strings.Join(trust.MainTheorems, ","), "the theorems to check, separated by commas")
	checker := fs.String("lean4checker", "", "the path of the lean4checker program; empty to skip the replay")
	timeout := fs.Duration("timeout", 0, "the time limit of each run, for example 6h; 0 for no limit")
	printFile := fs.Bool("print", false, "print the Lean file of the axiom check and stop")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: trust takes no arguments", errUsage)
	}
	var list []string
	for n := range strings.SplitSeq(*names, ",") {
		if n = strings.TrimSpace(n); n != "" {
			list = append(list, n)
		}
	}
	if *printFile {
		_, err := io.WriteString(stdout, trust.File(*module, list))
		return err
	}
	if *lean == "" {
		return fmt.Errorf("%w: trust needs -lean", errUsage)
	}
	o := trust.Options{Dir: *lean, Lake: *lake, Module: *module, Names: list, Timeout: *timeout}
	ctx := context.Background()

	ax, out, err := trust.CheckAxioms(ctx, o)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", lastLines(out, 30))
		return fmt.Errorf("the axiom check: %w", err)
	}
	for _, t := range ax.Theorems {
		state := "standard"
		if len(t.Extra) > 0 {
			state = "NOT STANDARD: " + strings.Join(t.Extra, ", ")
		}
		if !t.Match {
			state += ", the two methods do not agree"
		}
		if !t.TypeOK {
			state += fmt.Sprintf(", WRONG TYPE: %s, not %s", t.Type, t.Want)
		}
		list := strings.Join(t.Print, ", ")
		if list == "" {
			list = "no axioms"
		}
		fmt.Fprintf(stdout, "%s: %s (%s)\n", t.Name, list, state)
	}

	var rp *trust.Replay
	if *checker != "" {
		r, err := trust.CheckReplay(ctx, o, *checker)
		if err != nil {
			return fmt.Errorf("the replay: %w", err)
		}
		rp = &r
		fmt.Fprintf(stdout, "lean4checker on %s: exit %d after %.0f seconds\n", r.Module, r.Exit, r.Seconds)
	}

	if *content != "" {
		if err := writeTrust(*content, ax, rp); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "wrote the records in %s\n", filepath.Join(*content, "records"))
	}
	if !ax.OK || rp != nil && !rp.OK {
		return errCheck
	}
	return nil
}

// writeTrust writes records/axioms.json and records/replay.json, and sets the rows Q2 and Q3 of records/trust.json.
func writeTrust(content string, ax trust.Axioms, rp *trust.Replay) error {
	dir := filepath.Join(content, "records")
	if err := writeJSONFile(filepath.Join(dir, "axioms.json"), ax); err != nil {
		return err
	}
	if rp != nil {
		if err := writeJSONFile(filepath.Join(dir, "replay.json"), rp); err != nil {
			return err
		}
	}
	t := site.DefaultTrust()
	path := filepath.Join(dir, "trust.json")
	if data, err := os.ReadFile(path); err == nil { // #nosec G304 -- a file of the content repository
		if err := json.Unmarshal(data, &t); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	q2 := site.TrustCheck{ID: "Q2", Question: "Which axioms does riemannHypothesis use?"}
	switch {
	case ax.OK:
		q2.State = "yes"
		q2.Note = fmt.Sprintf("Only propext, Classical.choice and Quot.sound, for each of the %d theorems of %s. #print axioms and the axiom walk of rh agree, and each theorem has the expected type. Checked on %s with %s.", len(ax.Theorems), ax.Module, ax.Date, ax.Lean)
	default:
		q2.State = "no"
		var bad []string
		for _, th := range ax.Theorems {
			if len(th.Extra) > 0 {
				bad = append(bad, fmt.Sprintf("%s uses %s", th.Name, strings.Join(th.Extra, ", ")))
			}
			if !th.Match {
				bad = append(bad, fmt.Sprintf("the two methods do not agree for %s", th.Name))
			}
			if !th.TypeOK {
				bad = append(bad, fmt.Sprintf("the type of %s is %s, not %s", th.Name, th.Type, th.Want))
			}
		}
		q2.Note = strings.Join(bad, ". ") + fmt.Sprintf(". Checked on %s.", ax.Date)
	}
	setCheck(&t, q2)
	if rp != nil {
		q3 := site.TrustCheck{ID: "Q3", Question: "Does a second kernel accept the proof?"}
		if rp.OK {
			q3.State = "yes"
			q3.Note = fmt.Sprintf("%s accepts %s and all its imports. It ran for %s on %s. A second kernel that shares no code with Lean is not run yet.", rp.Tool, rp.Module, time.Duration(rp.Seconds)*time.Second, rp.Date)
		} else {
			q3.State = "no"
			q3.Note = fmt.Sprintf("%s on %s exited with code %d on %s. See records/replay.json.", rp.Tool, rp.Module, rp.Exit, rp.Date)
		}
		setCheck(&t, q3)
	}
	t.Updated = ax.Date
	return writeJSONFile(path, t)
}

// setCheck replaces the row with the ID of c, or adds it.
func setCheck(t *site.Trust, c site.TrustCheck) {
	for i := range t.Checks {
		if t.Checks[i].ID == c.ID {
			if c.Question == "" {
				c.Question = t.Checks[i].Question
			}
			t.Checks[i] = c
			return
		}
	}
	t.Checks = append(t.Checks, c)
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func lastLines(b []byte, n int) string {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
