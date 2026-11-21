package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/riemann-agent/steps"
)

func TestLink(t *testing.T) {
	content := t.TempDir()
	if err := os.MkdirAll(filepath.Join(content, "paths"), 0o750); err != nil {
		t.Fatal(err)
	}
	path := "id: p\ntitle: P\nitems:\n  - concept: Complex numbers\n    text: x\n  - concept: Theta lifts\n    text: y\n"
	if err := os.WriteFile(filepath.Join(content, "paths", "p.yaml"), []byte(path), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(content, "links"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(content, "links", "math2.yaml"), []byte("decls:\n  ours: BBBB\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	err := runLink([]string{"-from", "testdata/math2", "-math2", "testdata/math2/nodes.json", "-content", content}, &out, &errb)
	if err != nil {
		t.Fatal(err, errb.String())
	}
	if !strings.Contains(out.String(), "3 Lean names") || !strings.Contains(out.String(), "1 concepts filled") || !strings.Contains(out.String(), "no node: paths/p.yaml: Theta lifts") {
		t.Errorf("output: %s", out.String())
	}
	l, err := steps.LoadLinks(content)
	if err != nil {
		t.Fatal(err)
	}
	if l.Decls["Complex"] != "AAAA" || l.Decls["riemannZeta"] != "BBBB" || l.Decls["ours"] != "BBBB" || l.Decls["Old"] != "" {
		t.Errorf("decls: %v", l.Decls)
	}
	if l.Concepts["Complex numbers"] != "AAAA" {
		t.Errorf("concepts: %v", l.Concepts)
	}
}
