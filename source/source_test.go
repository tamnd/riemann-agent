package source

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarball returns a gzip tar archive with the given entries.
func tarball(t *testing.T, entries []tar.Header, bodies []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for i, h := range entries {
		h.Size = int64(len(bodies[i]))
		if h.Typeflag == 0 {
			h.Typeflag = tar.TypeReg
		}
		h.Mode = 0o644
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(bodies[i])); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// fixture splits an archive into three parts and returns the release and a server for it.
func fixture(t *testing.T) (Release, *httptest.Server, []byte) {
	t.Helper()
	archive := tarball(t,
		[]tar.Header{{Name: "lean/", Typeflag: tar.TypeDir}, {Name: "lean/Main.lean"}, {Name: "lean/src/A.lean"}},
		[]string{"", "theorem riemannHypothesis : RiemannHypothesis := rhOfAll\n", "def a := 1\n"})
	files := map[string][]byte{}
	r := Release{Name: "test", Archive: File{"lean.tar.gz", int64(len(archive)), sum(archive)}}
	step := len(archive)/3 + 1
	for i := 0; i*step < len(archive); i++ {
		part := archive[i*step : min((i+1)*step, len(archive))]
		name := "lean.tar.gz.part0" + string(rune('1'+i))
		files[name] = part
		r.Parts = append(r.Parts, File{name, int64(len(part)), sum(part)})
	}
	note := []byte("%PDF note")
	files["remark.pdf"] = note
	r.Note = File{"remark.pdf", int64(len(note)), sum(note)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, ok := files[strings.TrimPrefix(req.URL.Path, "/")]
		if !ok {
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return r, srv, archive
}

func TestFetchJoinUnpackCompare(t *testing.T) {
	r, srv, _ := fixture(t)
	dir := t.TempDir()
	var log bytes.Buffer
	f := &Fetcher{Base: srv.URL, Log: &log}
	archive, err := f.Fetch(context.Background(), dir, r)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(archive, r.Archive); err != nil {
		t.Fatal(err)
	}
	// A second run keeps the files and downloads nothing.
	log.Reset()
	if _, err := f.Fetch(context.Background(), dir, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(log.String(), "downloaded") {
		t.Fatalf("the second run downloaded again:\n%s", log.String())
	}

	n, err := Unpack(archive, filepath.Join(dir, "tree"))
	if err != nil || n != 2 {
		t.Fatalf("Unpack: %d files, %v", n, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "tree", "lean", "src", "A.lean"))
	if err != nil || string(b) != "def a := 1\n" {
		t.Fatalf("unpacked file: %q, %v", b, err)
	}

	got, err := ManifestOf(archive)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteManifest(&buf, got); err != nil {
		t.Fatal(err)
	}
	want, err := ReadManifest(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if d := Compare(want, got); !d.Empty() {
		t.Fatalf("a manifest differs from itself: %+v", d)
	}
	want["lean/src/A.lean"] = strings.Repeat("0", 64)
	want["lean/src/B.lean"] = strings.Repeat("1", 64)
	delete(want, "lean/Main.lean")
	d := Compare(want, got)
	if len(d.Changed) != 1 || len(d.Missing) != 1 || len(d.Extra) != 1 {
		t.Fatalf("Compare: %+v", d)
	}
}

func TestFetchRejectsWrongHash(t *testing.T) {
	r, srv, _ := fixture(t)
	r.Parts[1].SHA256 = strings.Repeat("a", 64)
	_, err := (&Fetcher{Base: srv.URL}).Fetch(context.Background(), t.TempDir(), r)
	if !IsHashError(err) {
		t.Fatalf("want a hash error, got %v", err)
	}
}

func TestFetchRejectsWrongSize(t *testing.T) {
	r, srv, _ := fixture(t)
	r.Parts[0].Size--
	if _, err := (&Fetcher{Base: srv.URL}).Fetch(context.Background(), t.TempDir(), r); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("want a size error, got %v", err)
	}
}

func TestFetchReplacesBadFile(t *testing.T) {
	r, srv, _ := fixture(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, r.Parts[0].Name), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Fetcher{Base: srv.URL}).Fetch(context.Background(), dir, r); err != nil {
		t.Fatal(err)
	}
}

func TestWalkRejectsUnsafeEntries(t *testing.T) {
	for _, h := range []tar.Header{
		{Name: "../evil"},
		{Name: "/etc/passwd"},
		{Name: "a/../../evil"},
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc"},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "a.tar.gz")
		if err := os.WriteFile(path, tarball(t, []tar.Header{h}, []string{""}), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Unpack(path, filepath.Join(dir, "out")); err == nil {
			t.Errorf("%s: Unpack accepted the entry", h.Name)
		}
	}
}

func TestReadManifestRejectsBadLine(t *testing.T) {
	if _, err := ReadManifest(strings.NewReader("abc lean/x\n")); err == nil {
		t.Fatal("want an error")
	}
}

func TestReleaseSizes(t *testing.T) {
	var total int64
	for _, p := range Release20261120.Parts {
		total += p.Size
	}
	if total != Release20261120.Archive.Size {
		t.Fatalf("the parts have %d bytes, the archive %d", total, Release20261120.Archive.Size)
	}
}
