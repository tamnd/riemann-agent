package prompts

import (
	"io/fs"
	"strings"
	"testing"
)

func TestLoadAll(t *testing.T) {
	files, err := fs.Glob(Files, "*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no prompt files: %v", err)
	}
	for _, f := range files {
		role := strings.TrimSuffix(f, ".md")
		p, err := Load(role)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if p.System == "" {
			t.Errorf("%s: the system prompt is empty", f)
		}
		if strings.Contains(p.System, "—") || strings.Contains(p.System, "–") {
			t.Errorf("%s: the system prompt has a dash", f)
		}
	}
}

func TestParse(t *testing.T) {
	p, err := parse("x", []byte("---\nrole: x\nversion: 2\n---\nBe short.\n=== prompt ===\nName: {{.Name}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID() != "x/2" || p.System != "Be short." {
		t.Fatalf("got %q and %q", p.ID(), p.System)
	}
	got, err := p.Render(map[string]string{"Name": "zeta"})
	if err != nil || got != "Name: zeta\n" {
		t.Fatalf("Render = %q, %v", got, err)
	}
	if _, err := parse("x", []byte("---\nrole: y\nversion: 1\n---\na\n=== prompt ===\nb\n")); err == nil {
		t.Fatal("a wrong role must fail")
	}
}
