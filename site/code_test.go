package site

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/riemann-agent/index"
)

func TestNameKey(t *testing.T) {
	for in, want := range map[string]string{"Lib.everyChar": "ev", "x": "x_", "A.ℂpow": "_p", "Foo.Bar_9": "ba"} {
		if got := NameKey(in); got != want {
			t.Errorf("NameKey(%q) = %q, want %q", in, got, want)
		}
	}
	// The hash must match code.js: FNV-1a of "a" is 0xe40c292c.
	if got := DeclShard("a"); got != "92c" {
		t.Errorf("DeclShard(a) = %s, want 92c", got)
	}
	if got := queryEscape("Lib.a b&c'"); got != "Lib.a%20b%26c'" {
		t.Errorf("queryEscape = %s", got)
	}
}

func TestCodePages(t *testing.T) {
	ix, err := index.Build(filepath.Join("..", "index", "testdata", "src"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	recs, sp := filepath.Join(dir, "index"), filepath.Join(dir, "spine")
	if err := ix.Write(recs); err != nil {
		t.Fatal(err)
	}
	g := index.NewGraph(ix)
	s := index.BuildSpine(g, g.ID["riemannHypothesis"], index.SpineOptions{NoCut: true})
	data, _ := json.Marshal(s)
	if err := os.MkdirAll(sp, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sp, "spine.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteNodes(filepath.Join(sp, "spine.jsonl.gz")); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "public")
	if _, err := Build(context.Background(), Options{Content: filepath.Join("testdata", "content"), Out: out, Index: recs, Spine: sp}); err != nil {
		t.Fatal(err)
	}
	var recsOut []declRecord
	raw, err := os.ReadFile(filepath.Join(out, "api", "decl", DeclShard("Lib.top")+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &recsOut); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range recsOut {
		if r.N == "Lib.top" {
			found = true
			if r.X == nil || r.X.D != 1 {
				t.Errorf("Lib.top must be on the spine at depth 1, got %+v", r.X)
			}
		}
	}
	if !found {
		t.Error("the shard has no record of Lib.top")
	}
	for _, f := range []string{"code/decl/index.html", "code/module/index.html", "api/code.json", "api/names/to.json", "assets/code.js"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Error(err)
		}
	}
	if rep, err := Check(out); err != nil || len(rep.Problems) > 0 {
		t.Errorf("check: %v %v", err, rep)
	}
}
