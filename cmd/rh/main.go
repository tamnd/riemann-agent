// Command rh is the toolchain of riemann. It fetches and checks the Lean source of the claimed proof of the Riemann Hypothesis, maps the proof, checks the written proof and builds the site riemann.tamnd.com.
//
// Exit codes: 0 for success, 1 for a failed check or a failed run, 2 for a usage error, and 3 for a command that is not written yet.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

// Version is the version of rh. The release build sets it with -ldflags "-X main.Version=v0.1.0".
var Version = "dev"

// errUsage is a usage error. main exits with code 2.
var errUsage = errors.New("usage error")

// command is one subcommand of rh.
type command struct {
	name      string
	summary   string
	milestone string // the milestone that adds the command, when it is not written yet
	run       func(args []string, stdout, stderr io.Writer) error
}

// commands returns the subcommands of rh, in the order of the help text.
func commands() []command {
	return []command{
		{name: "version", summary: "print the version of rh", run: runVersion},
		{name: "fetch", summary: "download the release, check its hashes and unpack it", run: runFetch},
		{name: "site", summary: "build, check or serve the site", run: runSite},
		{name: "index", summary: "read the Lean source and write the records of modules and declarations", run: runIndex},
		{name: "graph", summary: "query the declaration graph: uses, used by, path and closure", run: runGraph},
		{name: "spine", summary: "find the declarations that the top theorem needs", run: runSpine},
		{name: "trust", summary: "run the axiom check and the kernel replay, and write the trust report", milestone: "M2"},
		{name: "extract", summary: "read the exact declaration graph from the built environment", milestone: "M2"},
		{name: "check", summary: "check the written proof against the records, with the rules C1 to C6, P1 to P5 and G1 to G4", run: runCheck},
		{name: "link", summary: "match the names of the proof with the nodes of Math 2.0", run: runLink},
		{name: "draft", summary: "draft one step of the written proof with a model", milestone: "M5"},
		{name: "deploy", summary: "upload the site to the Cloudflare Worker", milestone: "M0"},
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs rh with the arguments args and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "-help" || args[0] == "--help" {
		usage(stdout)
		return 0
	}
	if args[0] == "-version" || args[0] == "--version" {
		args = []string{"version"}
	}
	for _, c := range commands() {
		if c.name != args[0] {
			continue
		}
		if c.run == nil {
			fmt.Fprintf(stderr, "rh %s: this command is not written yet. Milestone %s adds it.\n", c.name, c.milestone)
			return 3
		}
		err := c.run(args[1:], stdout, stderr)
		switch {
		case err == nil:
			return 0
		case errors.Is(err, flag.ErrHelp):
			return 0
		case errors.Is(err, errUsage):
			fmt.Fprintf(stderr, "rh %s: %v\n", c.name, err)
			return 2
		default:
			fmt.Fprintf(stderr, "rh %s: %v\n", c.name, err)
			return 1
		}
	}
	fmt.Fprintf(stderr, "rh: unknown command %q. Run rh help to see the commands.\n", args[0])
	return 2
}

// usage writes the help text.
func usage(w io.Writer) {
	fmt.Fprintln(w, "rh is the toolchain of riemann: index the Lean source, check it, map the proof and build riemann.tamnd.com.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: rh <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	cs := commands()
	width := 0
	for _, c := range cs {
		width = max(width, len(c.name))
	}
	for _, c := range cs {
		note := ""
		if c.run == nil {
			note = " (not written yet, milestone " + c.milestone + ")"
		}
		fmt.Fprintf(w, "  %-*s  %s%s\n", width, c.name, c.summary, note)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run rh <command> -h to see the flags of a command.")
}

func runVersion(args []string, stdout, _ io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("%w: version takes no arguments", errUsage)
	}
	fmt.Fprintln(stdout, "rh", Version)
	return nil
}

// names returns the names of the commands, sorted. The tests use it.
func names() []string {
	var out []string
	for _, c := range commands() {
		out = append(out, c.name)
	}
	sort.Strings(out)
	return out
}
