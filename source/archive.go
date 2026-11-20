package source

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

// Entry is one regular file of an archive.
type Entry struct {
	Name string // the path in the archive, with forward slashes
	Size int64
	Body io.Reader // valid only during the call of the walk function
}

// Walk reads the gzip tar archive at archivePath and calls fn for each regular file, in the order of the archive. It returns an error for a path that is absolute or that contains "..", and for a link or a device. The release has only regular files and directories.
func Walk(archivePath string, fn func(Entry) error) error {
	in, err := os.Open(archivePath) // #nosec G304 -- the caller names the archive
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	zr, err := gzip.NewReader(bufio.NewReaderSize(in, 1<<20))
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name, err := cleanName(h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			if err := fn(Entry{Name: name, Size: h.Size, Body: tr}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: the entry has type %q; only files and directories are allowed", h.Name, h.Typeflag)
		}
	}
}

// cleanName returns the clean form of an archive path, or an error when the path can leave the target directory.
func cleanName(name string) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, `\`) {
		return "", fmt.Errorf("%q: the path is not allowed", name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || !fs.ValidPath(clean) {
		return "", fmt.Errorf("%q: the path is not allowed", name)
	}
	return clean, nil
}

// Unpack writes each file of the archive under dest. It returns the number of files.
func Unpack(archivePath, dest string) (int, error) {
	n := 0
	root, err := os.OpenRoot(mustMkdir(dest))
	if err != nil {
		return 0, err
	}
	defer func() { _ = root.Close() }()
	err = Walk(archivePath, func(e Entry) error {
		if dir := path.Dir(e.Name); dir != "." {
			if err := root.MkdirAll(dir, 0o750); err != nil {
				return err
			}
		}
		out, err := root.OpenFile(e.Name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, e.Body); err != nil {
			_ = out.Close()
			return err
		}
		n++
		return out.Close()
	})
	return n, err
}

func mustMkdir(dir string) string {
	_ = os.MkdirAll(dir, 0o750)
	return dir
}

// Manifest maps a file path to its SHA-256 in hex.
type Manifest map[string]string

// ReadManifest reads a file in the format of sha256sum: one line for each file, the hash, two spaces and the path.
func ReadManifest(r io.Reader) (Manifest, error) {
	m := Manifest{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if text == "" {
			continue
		}
		hash, name, ok := strings.Cut(text, "  ")
		if !ok || len(hash) != 64 {
			return nil, fmt.Errorf("line %d: want a SHA-256, two spaces and a path", line)
		}
		m[strings.TrimPrefix(name, "*")] = hash
	}
	return m, sc.Err()
}

// ManifestOf hashes each file of the archive.
func ManifestOf(archivePath string) (Manifest, error) {
	m := Manifest{}
	err := Walk(archivePath, func(e Entry) error {
		h := sha256.New()
		if _, err := io.Copy(h, e.Body); err != nil {
			return err
		}
		m[e.Name] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	return m, err
}

// Diff is the difference between two manifests.
type Diff struct {
	Missing []string // in want, not in got
	Extra   []string // in got, not in want
	Changed []string // in both, with another hash
}

// Empty reports whether the two manifests are the same.
func (d Diff) Empty() bool { return len(d.Missing)+len(d.Extra)+len(d.Changed) == 0 }

// Compare returns the difference between want and got. Each list is sorted.
func Compare(want, got Manifest) Diff {
	var d Diff
	for name, h := range want {
		g, ok := got[name]
		switch {
		case !ok:
			d.Missing = append(d.Missing, name)
		case g != h:
			d.Changed = append(d.Changed, name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			d.Extra = append(d.Extra, name)
		}
	}
	sort.Strings(d.Missing)
	sort.Strings(d.Extra)
	sort.Strings(d.Changed)
	return d
}

// WriteManifest writes m in the format of sha256sum, sorted by path.
func WriteManifest(w io.Writer, m Manifest) error {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	bw := bufio.NewWriter(w)
	for _, n := range names {
		if _, err := fmt.Fprintf(bw, "%s  %s\n", m[n], n); err != nil {
			return err
		}
	}
	return bw.Flush()
}
