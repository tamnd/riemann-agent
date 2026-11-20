package index

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Shard returns the shard of a declaration name: the first two hex characters of the SHA-256 of the name.
func Shard(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:1])
}

// Write writes the records to dir: modules.jsonl.gz, decls/<xx>.jsonl.gz and summary.json. The files are gzip files, because the records of the release are about 1 GB of JSON.
func (ix *Index) Write(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, "decls"), 0o750); err != nil {
		return err
	}
	if err := writeGzipLines(filepath.Join(dir, "modules.jsonl.gz"), len(ix.Modules), func(i int) any { return ix.Modules[i] }); err != nil {
		return err
	}
	shards := map[string][]int{}
	for i, d := range ix.Decls {
		s := Shard(d.Name)
		shards[s] = append(shards[s], i)
	}
	for s, ids := range shards {
		err := writeGzipLines(filepath.Join(dir, "decls", s+".jsonl.gz"), len(ids), func(k int) any { return &ix.Decls[ids[k]] })
		if err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(ix.Summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "summary.json"), append(data, '\n'), 0o600)
}

func writeGzipLines(path string, n int, item func(int) any) (err error) {
	f, err := os.Create(path) // #nosec G304 -- a path under the output directory
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	// The gzip header has no time and no name, so the same records give the same bytes.
	zw, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(zw, 1<<20)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	for i := range n {
		if err := enc.Encode(item(i)); err != nil {
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return zw.Close()
}

// Load reads the records that Write wrote.
func Load(dir string) (*Index, error) {
	ix := &Index{}
	data, err := os.ReadFile(filepath.Join(dir, "summary.json")) // #nosec G304 -- the records directory
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &ix.Summary); err != nil {
		return nil, fmt.Errorf("summary.json: %w", err)
	}
	err = readGzipLines(filepath.Join(dir, "modules.jsonl.gz"), func(line []byte) error {
		m := &Module{}
		if err := json.Unmarshal(line, m); err != nil {
			return err
		}
		ix.Modules = append(ix.Modules, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(dir, "decls"))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jsonl.gz") {
			continue
		}
		err := readGzipLines(filepath.Join(dir, "decls", e.Name()), func(line []byte) error {
			var d Decl
			if err := json.Unmarshal(line, &d); err != nil {
				return err
			}
			ix.Decls = append(ix.Decls, d)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(ix.Modules, func(a, b int) bool { return ix.Modules[a].Name < ix.Modules[b].Name })
	sort.SliceStable(ix.Decls, func(a, b int) bool { return ix.Decls[a].Name < ix.Decls[b].Name })
	if len(ix.Decls) == 0 {
		return nil, errors.New(dir + ": no declarations")
	}
	return ix, nil
}

func readGzipLines(path string, fn func([]byte) error) (err error) {
	f, err := os.Open(path) // #nosec G304 -- a file of the records directory
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	br := bufio.NewReaderSize(zr, 1<<20)
	for n := 1; ; n++ {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if err := fn(line); err != nil {
				return fmt.Errorf("%s:%d: %w", path, n, err)
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
}
