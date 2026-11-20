package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/tamnd/riemann-agent/source"
)

// runFetch downloads the release, checks the hash of each part and of the archive, and can unpack the archive and compare it with a manifest.
func runFetch(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "work/release", "the work `directory` for the parts and the archive")
	base := fs.String("base", source.BaseURL, "the base `URL` of the release")
	unpack := fs.String("unpack", "", "unpack the archive into this `directory`")
	compare := fs.String("compare", "", "compare each file of the archive with this sha256sum `manifest`, for example riemann-lean/manifests/files.sha256")
	write := fs.String("write-manifest", "", "write the sha256sum manifest of the files of the archive to this `file`")
	quiet := fs.Bool("q", false, "print only errors")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: rh fetch [flags]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Download the 14 parts of the release of 2026-11-20 and the note, check the SHA-256 of each one, and join the parts into lean.tar.gz. A file that is already correct is not downloaded again.")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: fetch takes no arguments", errUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log := stdout
	if *quiet {
		log = nil
	}
	f := &source.Fetcher{Base: *base, Log: log}
	r := source.Release20261120
	archive, err := f.Fetch(ctx, *dir, r)
	if err != nil {
		return err
	}
	if *unpack != "" {
		n, err := source.Unpack(archive, *unpack)
		if err != nil {
			return err
		}
		if !*quiet {
			fmt.Fprintf(stdout, "unpacked %d files into %s\n", n, *unpack)
		}
	}
	if *compare == "" && *write == "" {
		return nil
	}
	got, err := source.ManifestOf(archive)
	if err != nil {
		return err
	}
	if *write != "" {
		if err := writeManifest(*write, got); err != nil {
			return err
		}
	}
	if *compare == "" {
		return nil
	}
	in, err := os.Open(*compare)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	want, err := source.ReadManifest(in)
	if err != nil {
		return fmt.Errorf("%s: %w", *compare, err)
	}
	d := source.Compare(want, got)
	if d.Empty() {
		if !*quiet {
			fmt.Fprintf(stdout, "the archive has the same %d files as %s\n", len(got), *compare)
		}
		return nil
	}
	for _, n := range d.Missing {
		fmt.Fprintf(stdout, "missing in the archive: %s\n", n)
	}
	for _, n := range d.Extra {
		fmt.Fprintf(stdout, "not in the manifest: %s\n", n)
	}
	for _, n := range d.Changed {
		fmt.Fprintf(stdout, "changed: %s\n", n)
	}
	return fmt.Errorf("the archive differs from %s: %d missing, %d extra, %d changed", *compare, len(d.Missing), len(d.Extra), len(d.Changed))
}

func writeManifest(path string, m source.Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	out, err := os.Create(path) // #nosec G304 -- the user names the file
	if err != nil {
		return err
	}
	if err := source.WriteManifest(out, m); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
