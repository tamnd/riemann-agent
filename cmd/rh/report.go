package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/riemann-agent/agent"
	"github.com/tamnd/riemann-agent/steps"
)

func runReport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("rh report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	content := fs.String("content", "../riemann", "the root of the content repository")
	days := fs.Int("days", 7, "the number of days that the report covers, up to today")
	until := fs.String("until", "", "the last day of the report, as YYYY-MM-DD (the default is today, UTC)")
	noGit := fs.Bool("no-git", false, "do not read the git history, so the report has no edit size")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: rh report [flags]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "rh report writes the agent report of spec 09.4 as Markdown: the calls and the cost from agent/ledger.jsonl, the steps that a model drafted, the steps that people accepted and the lines that people changed since the draft.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() > 0 || *days < 1 {
		return fmt.Errorf("%w: report takes no arguments, and -days must be 1 or more", errUsage)
	}
	end := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	if *until != "" {
		t, err := time.Parse("2006-01-02", *until)
		if err != nil {
			return fmt.Errorf("%w: -until: %v", errUsage, err)
		}
		end = t.Add(24 * time.Hour)
	}
	l, err := agent.OpenLedger(*content)
	if err != nil {
		return err
	}
	ss, err := steps.Load(*content)
	if err != nil {
		return err
	}
	var edit agent.EditFunc
	if !*noGit {
		edit = func(file string) (agent.Edit, error) { return gitEdit(*content, file) }
	}
	r, err := agent.MakeReport(l, ss, end.Add(-time.Duration(*days)*24*time.Hour), end, edit)
	if err != nil {
		return err
	}
	r.Write(stdout)
	return nil
}

// gitEdit counts the lines that changed in a file since the commit that added it. A file that git does not know yet has no edit.
func gitEdit(root, file string) (agent.Edit, error) {
	out, err := exec.Command("git", "-C", root, "log", "--diff-filter=A", "--format=%H", "--", file).Output() // #nosec G204 -- git with a path of the content repository
	if err != nil {
		return agent.Edit{}, err
	}
	shas := strings.Fields(string(out))
	if len(shas) == 0 {
		return agent.Edit{}, nil
	}
	out, err = exec.Command("git", "-C", root, "diff", "--numstat", shas[len(shas)-1], "--", file).Output() // #nosec G204 -- git with a commit that git gave
	if err != nil {
		return agent.Edit{}, err
	}
	var e agent.Edit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		a, _ := strconv.Atoi(f[0])
		d, _ := strconv.Atoi(f[1])
		e.Added += a
		e.Deleted += d
	}
	return e, nil
}
