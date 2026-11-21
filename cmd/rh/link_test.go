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
	path := "id: p\ntitle: P\nitems:\n  - concept: Complex numbers\n    why: x\n  - concept: Theta lifts\n    why: y\n"
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
	if l.Summaries["AAAA"] != "A complex number is a sum $a + bi$." || len(l.Summaries) != 1 {
		t.Errorf("summaries: %v", l.Summaries)
	}
	again, err := steps.LoadLinks(content)
	if err != nil || again.Summaries["AAAA"] != l.Summaries["AAAA"] {
		t.Errorf("the summaries do not read back: %v", err)
	}
}

func TestExpandMacros(t *testing.T) {
	m := map[string]string{`\C`: `\mathbb{C}`, `\abs`: `\left|#1\right|`, `\ip`: `\left\langle #1, #2\right\rangle`}
	for in, want := range map[string]string{
		`$z \in \C$`:             `$z \in \mathbb{C}$`,
		`$1/\abs{x}$`:            `$1/\left|x\right|$`,
		`$\abs{\abs{x}}$`:        `$\left|\left|x\right|\right|$`,
		`$\ip{u}{v}$`:            `$\left\langle u, v\right\rangle$`,
		`$\Cap \{a\} \abs$`:      `$\Cap \{a\} \abs$`,
		`$\C_v$ and \abs{\{x\}}`: `$\mathbb{C}_v$ and \left|\{x\}\right|`,
	} {
		if got := expandMacros(in, m); got != want {
			t.Errorf("expandMacros(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNodeSummary(t *testing.T) {
	root := t.TempDir()
	write := func(tag, summary string) {
		dir := filepath.Join(root, "nodes", tag[:3])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\ntitle: x\n---\n\n## Summary\n\n" + summary + "\n\n## Motivation\n\nMore.\n"
		if err := os.WriteFile(filepath.Join(dir, tag+".md"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("AAAA1", "A group is a set with a product.\nIt has more.")
	write("AAAA2", "Let $V$ be a vector space.\nA form on $V$ is a map.\nMore text.")
	write("AAAA3", "This topic is a part of algebra.")
	for tag, want := range map[string]string{
		"AAAA1": "A group is a set with a product.",
		"AAAA2": "Let $V$ be a vector space. A form on $V$ is a map.",
		"AAAA3": "",
		"AAAA4": "",
	} {
		if got := nodeSummary(root, tag); got != want {
			t.Errorf("nodeSummary(%s) = %q, want %q", tag, got, want)
		}
	}
}
