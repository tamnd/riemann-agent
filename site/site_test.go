package site

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func build(t *testing.T) (string, *Result) {
	t.Helper()
	idx, err := LoadMath2(context.Background(), filepath.Join("testdata", "math2.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	res, err := Build(context.Background(), Options{Content: filepath.Join("testdata", "content"), Out: out, Commit: "c0ffee0", Date: "2026-11-21", Version: "test", Math2: idx})
	if err != nil {
		t.Fatal(err)
	}
	return out, res
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLearn(t *testing.T) {
	out, _ := build(t)
	path := read(t, out, "learn/top/index.html")
	for _, want := range []string{
		`1. Riemann hypothesis`,
		`Where the proof uses it.`,
		`href="https://math2.tamnd.com/wiki/riemann-hypothesis/"`,
		`href="https://math2.tamnd.com/wiki/dirichlet-l-function/"`,
		`href="https://github.com/tamnd/math2/issues/9"`,
		`href="/step/top/"`,
		`<math`,
		`A Dirichlet L-function is the series of a character.`,
		`A standard text: Kudla, Notes on the local theta correspondence, sec. 1.`,
	} {
		if !strings.Contains(path, want) {
			t.Errorf("learn/top/index.html has no %s", want)
		}
	}
	w := read(t, out, "learn/written/index.html")
	for _, want := range []string{`The zeros of zeta`, `<strong>Definition.</strong>`, `href="/learn/top/"`, `href="https://math2.tamnd.com/wiki/riemann-hypothesis/"`, `blob/main/paths/written.md`} {
		if !strings.Contains(w, want) {
			t.Errorf("learn/written/index.html has no %s", want)
		}
	}
	if strings.Contains(w, "Where the proof uses it.") {
		t.Error("learn/written/index.html shows the list of the items")
	}
	if s := read(t, out, "learn/index.html"); !strings.Contains(s, `href="/learn/top/"`) || !strings.Contains(s, `href="/learn/map/"`) {
		t.Error("learn/index.html does not list the paths")
	}
	if s := read(t, out, "learn/map/index.html"); !strings.Contains(s, "With a node in Math 2.0: 3.") {
		t.Errorf("the map has no count")
	}
	gl := read(t, out, "glossary/index.html")
	for _, want := range []string{`The Riemann Hypothesis, as Mathlib states it.`, `href="/step/top/"`, `UpperTheory`, `<math`} {
		if !strings.Contains(gl, want) {
			t.Errorf("glossary/index.html has no %s", want)
		}
	}
	if s := read(t, out, "proof/index.html"); !strings.Contains(s, `href="/learn/top/"`) {
		t.Error("the overview does not link to its path")
	}
}

func TestBuild(t *testing.T) {
	out, res := build(t)
	if len(res.Missing) != 1 || res.Missing[0] != "ZZZZZZZZZZ" {
		t.Errorf("missing = %v, want [ZZZZZZZZZZ]", res.Missing)
	}
	for _, f := range []string{"index.html", "index.md", "proof/index.html", "proof/index.md", "proof/timeline/index.html", "about/index.html", "trust/index.html", "search/index.html", "404.html", "api/pages.json", "api/search.json", "api/trust.json", "llms.txt", "sitemap.xml", "robots.txt", "_headers", "assets/site.css", "assets/riemann.css", "assets/fonts/STIXTwoMath-Regular.woff2"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "assets", "UPSTREAM.md")); err == nil {
		t.Error("assets/UPSTREAM.md is in the site")
	}

	home := read(t, out, "index.html")
	for _, want := range []string{
		`href="https://math2.tamnd.com/wiki/riemann-hypothesis/"`,
		`<math`,
		`class="claim-bar"`,
		`Is the source the same as the release?`,
		`<a href="/proof/">Read the proof</a>`,
		`riemann-lean 9110120`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("index.html has no %s", want)
		}
	}

	proof := read(t, out, "proof/index.html")
	for _, want := range []string{
		`https://github.com/tamnd/riemann-lean/blob/9110120cdc195ee8d2fb08d2adac8b1747912d8f/lean/src/Main.lean#L4`,
		`<aside class="side"`,
		`Dirichlet L-function`,
		`class="missing"`,
		`class="math display"`,
		`<span class="level"`,
		`aria-current="page"`,
	} {
		if !strings.Contains(proof, want) {
			t.Errorf("proof/index.html has no %s", want)
		}
	}
	step := read(t, out, "step/top/index.html")
	for _, want := range []string{"The top statement.", "<h2 id=\"lean\">Lean</h2>", `href="/step/low/"`, `class="levels"`, "L2 Step", "Drafted."} {
		if !strings.Contains(step, want) {
			t.Errorf("step/top/index.html has no %s", want)
		}
	}
	if low := read(t, out, "step/low/index.html"); !strings.Contains(low, "checked, A Person, 2026-11-21") || !strings.Contains(low, `href="/step/top/"`) {
		t.Error("step/low/index.html has no review or no used-by link")
	}
	if list := read(t, out, "step/index.html"); !strings.Contains(list, `href="/step/top/"`) {
		t.Error("step/index.html does not list the steps")
	}
	if named := read(t, out, "proof/named-results/index.html"); !strings.Contains(named, `href="https://doi.org/10.1000/x"`) {
		t.Error("the named results page has no citation link")
	}
	if !strings.Contains(proof, `href="/step/top/"`) {
		t.Error("the overview does not list its steps")
	}
	if tl := read(t, out, "proof/timeline/index.html"); !strings.Contains(tl, `<a href="/proof/">`) || !strings.Contains(tl, `<a href="/learn/top/">The top of the proof</a>`) {
		t.Error("the timeline does not link the overview")
	}
	if md := read(t, out, "proof/index.md"); !strings.HasPrefix(md, "# The proof in one page\n\nThe top theorem") {
		t.Errorf("proof/index.md starts with %q", md[:min(len(md), 40)])
	}
	if !strings.Contains(read(t, out, "llms.txt"), "https://riemann.tamnd.com/proof/index.md") {
		t.Error("llms.txt does not list the Markdown copy of the proof")
	}
}

func TestCheck(t *testing.T) {
	out, _ := build(t)
	r, err := Check(out)
	if err != nil {
		t.Fatal(err)
	}
	if !r.OK() {
		t.Fatalf("problems: %v", r.Problems)
	}
	page := filepath.Join(out, "about", "index.html")
	data := read(t, out, "about/index.html")
	data = strings.Replace(data, `href="/proof/"`, `href="/nowhere/"`, 1) + `<code class="tex-error">x</code>`
	if err := os.WriteFile(page, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err = Check(out)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(r.Problems, "\n")
	if !strings.Contains(text, "/nowhere/") || !strings.Contains(text, "formula") {
		t.Errorf("problems = %v", r.Problems)
	}
}

func TestServe(t *testing.T) {
	out, _ := build(t)
	h, err := Handler(out)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	for path, code := range map[string]int{"/": 200, "/proof/": 200, "/proof/index.md": 200, "/nowhere/": 404, "/../etc/passwd": 404} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != code {
			t.Errorf("%s: status %d, want %d", path, resp.StatusCode, code)
		}
	}
}

func TestChapterPath(t *testing.T) {
	for in, want := range map[string]string{"00-overview.md": "proof/", "overview.md": "proof/", "03-theta.md": "proof/theta/", "x-y.md": "proof/x-y/"} {
		if got := chapterPath(in); got != want {
			t.Errorf("chapterPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitFront(t *testing.T) {
	fm, body, err := splitFront([]byte("---\r\ntitle: A\r\n---\r\nText\r\n"))
	if err != nil || string(fm) != "title: A" || body != "Text\n" {
		t.Errorf("got %q %q %v", fm, body, err)
	}
	if _, _, err := splitFront([]byte("---\ntitle: A\n")); err == nil {
		t.Error("no error for front matter with no end")
	}
}
