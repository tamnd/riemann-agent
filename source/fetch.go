package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Fetcher downloads the files of a release into a directory.
type Fetcher struct {
	Base   string       // the base URL; BaseURL when empty
	Client *http.Client // http.DefaultClient when nil
	Log    io.Writer    // progress messages; none when nil
}

// HashError is a file with the wrong size or the wrong SHA-256.
type HashError struct {
	Name      string
	Want, Got string
}

func (e *HashError) Error() string {
	return fmt.Sprintf("%s: the hash is %s, want %s", e.Name, e.Got, e.Want)
}

// Get makes sure that dir has the file f with the right hash. It keeps a file that is already correct, and it downloads the file again when the hash is wrong. It writes to a temporary file first, so a stopped download leaves no bad file.
func (f *Fetcher) Get(ctx context.Context, dir string, file File) error {
	path := filepath.Join(dir, file.Name)
	if err := Check(path, file); err == nil {
		f.logf("%s: present, hash ok\n", file.Name)
		return nil
	}
	base := f.Base
	if base == "" {
		base = BaseURL
	}
	url := strings.TrimSuffix(base, "/") + "/" + file.Name
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	tmp, err := os.CreateTemp(dir, file.Name+".*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	h := sha256.New()
	// Read one byte more than the size, so that a longer file fails the size check.
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, file.Size+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}
	if n != file.Size {
		return fmt.Errorf("%s: the size is %d bytes, want %d", file.Name, n, file.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != file.SHA256 {
		return &HashError{Name: file.Name, Want: file.SHA256, Got: got}
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	f.logf("%s: downloaded, hash ok\n", file.Name)
	return nil
}

// Fetch gets each part and the note of r into dir, then joins the parts into the archive and checks its hash. It returns the path of the archive.
func (f *Fetcher) Fetch(ctx context.Context, dir string, r Release) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	for _, p := range r.Parts {
		if err := f.Get(ctx, dir, p); err != nil {
			return "", err
		}
	}
	if r.Note.Name != "" {
		if err := f.Get(ctx, dir, r.Note); err != nil {
			return "", err
		}
	}
	path, err := Join(dir, r)
	if err != nil {
		return "", err
	}
	f.logf("%s: joined from %d parts, hash ok\n", r.Archive.Name, len(r.Parts))
	return path, nil
}

func (f *Fetcher) logf(format string, args ...any) {
	if f.Log != nil {
		fmt.Fprintf(f.Log, format, args...)
	}
}

// Join concatenates the parts of r in dir into the archive and checks its hash. It keeps an archive that is already correct.
func Join(dir string, r Release) (string, error) {
	path := filepath.Join(dir, r.Archive.Name)
	if Check(path, r.Archive) == nil {
		return path, nil
	}
	tmp, err := os.CreateTemp(dir, r.Archive.Name+".*.tmp")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	h := sha256.New()
	w := io.MultiWriter(tmp, h)
	for _, p := range r.Parts {
		if err := appendFile(w, filepath.Join(dir, p.Name)); err != nil {
			_ = tmp.Close()
			return "", err
		}
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != r.Archive.SHA256 {
		return "", &HashError{Name: r.Archive.Name, Want: r.Archive.SHA256, Got: got}
	}
	return path, os.Rename(tmp.Name(), path)
}

func appendFile(w io.Writer, path string) error {
	in, err := os.Open(path) // #nosec G304 -- the path is a part name in the work directory
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	_, err = io.Copy(w, in)
	return err
}

// Check returns nil when the file at path has the size and the SHA-256 of f.
func Check(path string, f File) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.Size() != f.Size {
		return fmt.Errorf("%s: the size is %d bytes, want %d", f.Name, st.Size(), f.Size)
	}
	got, err := HashFile(path)
	if err != nil {
		return err
	}
	if got != f.SHA256 {
		return &HashError{Name: f.Name, Want: f.SHA256, Got: got}
	}
	return nil
}

// HashFile returns the SHA-256 of the file at path, in hex.
func HashFile(path string) (string, error) {
	in, err := os.Open(path) // #nosec G304 -- the caller names the file
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, in); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// IsHashError reports whether err is a HashError.
func IsHashError(err error) bool {
	var h *HashError
	return errors.As(err, &h)
}
