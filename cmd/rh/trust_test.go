package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/riemann-agent/site"
	"github.com/tamnd/riemann-agent/trust"
)

func TestWriteTrust(t *testing.T) {
	dir := t.TempDir()
	ax := trust.Axioms{Date: "2026-11-21", Module: "Main", Lean: "Lean (version 4.33.1)", OK: true, Theorems: []trust.Theorem{{Name: "riemannHypothesis", Print: trust.Standard, Walk: trust.Standard, Match: true, Extra: []string{}}}}
	rp := &trust.Replay{Date: "2026-11-21", Tool: "lean4checker --fresh", Module: "Main", Seconds: 3600, Exit: 1}
	if err := writeTrust(dir, ax, rp); err != nil {
		t.Fatal(err)
	}
	var tr site.Trust
	data, err := os.ReadFile(filepath.Join(dir, "records", "trust.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, c := range tr.Checks {
		states[c.ID] = c.State
	}
	if states["Q2"] != "yes" || states["Q3"] != "no" || states["Q1"] != "not run" || len(tr.Checks) != 6 {
		t.Fatalf("got %v", states)
	}
	if tr.Updated != "2026-11-21" {
		t.Errorf("updated is %q", tr.Updated)
	}
	for _, f := range []string{"axioms.json", "replay.json"} {
		if _, err := os.Stat(filepath.Join(dir, "records", f)); err != nil {
			t.Error(err)
		}
	}

	// A second run keeps the other rows and changes Q2.
	ax.OK = false
	ax.Theorems[0].Extra = []string{"sorryAx"}
	if err := writeTrust(dir, ax, nil); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "records", "trust.json"))
	if !strings.Contains(string(data), "riemannHypothesis uses sorryAx") || !strings.Contains(string(data), "lean4checker --fresh on Main exited with code 1") {
		t.Fatalf("got %s", data)
	}
}

func TestTrustPrint(t *testing.T) {
	var out, errb strings.Builder
	if code := run([]string{"trust", "-print", "-theorems", "x,y"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "#print axioms y") {
		t.Fatalf("got %s", out.String())
	}
	if code := run([]string{"trust"}, &out, &errb); code != 2 {
		t.Fatalf("exit %d without -lean, want 2", code)
	}
}
