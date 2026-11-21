package steps

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMarshalRoundTrip(t *testing.T) {
	s := &Step{ID: "zeta-def", Title: "The zeta function: a sum", Decl: "Riemann.zeta", Chapter: "trunk", Status: "drafted", DraftedBy: "prover@claude-opus-5-5", Prompt: "prover/1", Uses: []string{"a", "b"}, Routine: []string{"Nat.succ"}, Open: []string{"Foo.bar'"}, Body: "## Statement\n\nText.\n"}
	data, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "zeta-def.md")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := read(p)
	if err != nil {
		t.Fatalf("read: %v\n%s", err, data)
	}
	got.File = ""
	if !reflect.DeepEqual(got, s) {
		t.Fatalf("round trip:\n got %+v\nwant %+v\n%s", got, s, data)
	}
}
