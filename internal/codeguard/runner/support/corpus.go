package support

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// readCappedFile reads path but refuses to buffer more than maxScanFileBytes,
// bounding memory even if a file grew past the walk-time size filter (TOCTOU) or
// is read outside the walk. It reads one byte past the cap to distinguish an
// exactly-cap-sized file from an oversized one.
func readCappedFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // path enumerated by WalkFiles under the scan root
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxScanFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxScanFileBytes {
		return nil, fmt.Errorf("file %q exceeds the %d byte scan limit", path, maxScanFileBytes)
	}
	return data, nil
}

// fileCorpus memoizes, for the lifetime of a single scan, the expensive work
// that would otherwise be repeated by every check section: the per-target
// directory walk, the individual file reads, and Go AST parses. Every by-value
// copy of Context shares one *fileCorpus, so a file is walked, read, and parsed
// at most once per scan no matter how many sections inspect it.
//
// All methods are safe for concurrent use so that sections can run in parallel.
// Each cached slot carries its own sync.Once, so concurrent callers racing on a
// cold slot compute it exactly once and every caller observes the same result.
type fileCorpus struct {
	mu      sync.Mutex
	targets map[string]*targetListing
	reads   map[string]*fileRead
	asts    map[string]*goParse
}

type targetListing struct {
	once  sync.Once
	files []string
	err   error
}

type fileRead struct {
	once sync.Once
	data []byte
	err  error
}

type goParse struct {
	once sync.Once
	fset *token.FileSet
	file *ast.File
	err  error
}

func newFileCorpus() *fileCorpus {
	return &fileCorpus{
		targets: map[string]*targetListing{},
		reads:   map[string]*fileRead{},
		asts:    map[string]*goParse{},
	}
}

// list returns every non-excluded file under root, walking the tree only once
// per target. Callers apply their own include filter to the returned slice; the
// walk itself is identical regardless of the filter, so sharing it is safe.
func (c *fileCorpus) list(root string, excludes []string) ([]string, error) {
	key := filepath.Clean(root)
	c.mu.Lock()
	entry, ok := c.targets[key]
	if !ok {
		entry = &targetListing{}
		c.targets[key] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		entry.files, entry.err = WalkFiles(root, excludes, includeAll)
	})
	return entry.files, entry.err
}

// read returns the bytes of root/rel, reading each file at most once per scan.
func (c *fileCorpus) read(root string, rel string) ([]byte, error) {
	key := filepath.Clean(root) + "\x00" + rel
	c.mu.Lock()
	entry, ok := c.reads[key]
	if !ok {
		entry = &fileRead{}
		c.reads[key] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		entry.data, entry.err = readCappedFile(filepath.Join(root, rel))
	})
	return entry.data, entry.err
}

// parseGo returns a shared, read-only Go AST for the given source. The cache key
// includes the content hash so patched (diff-mode) content is reparsed rather
// than serving a stale tree. Callers must treat the returned *ast.File and
// *token.FileSet as immutable, which the AST inspection in the check sections
// already does.
func (c *fileCorpus) parseGo(path string, data []byte) (*token.FileSet, *ast.File, error) {
	key := path + "\x00" + hashBytes(data)
	c.mu.Lock()
	entry, ok := c.asts[key]
	if !ok {
		entry = &goParse{}
		c.asts[key] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
		entry.fset, entry.file, entry.err = fset, file, err
	})
	return entry.fset, entry.file, entry.err
}

func includeAll(string) bool { return true }

// corpusFiles lists every non-excluded file under root, using the shared
// per-scan corpus when present and falling back to a direct walk otherwise
// (e.g. for a Context assembled in a unit test).
func (sc Context) corpusFiles(root string) ([]string, error) {
	if sc.corpus != nil {
		return sc.corpus.list(root, sc.Cfg.Exclude)
	}
	return WalkFiles(root, sc.Cfg.Exclude, includeAll)
}

// corpusRead returns the bytes of root/rel via the shared per-scan corpus,
// falling back to a direct read when no corpus is attached.
func (sc Context) corpusRead(root string, rel string) ([]byte, error) {
	if sc.corpus != nil {
		return sc.corpus.read(root, rel)
	}
	return readCappedFile(filepath.Join(root, rel))
}

// ParseGoFile returns a shared, read-only Go AST for the given source, parsed at
// most once per scan across every section. It falls back to a fresh parse when
// no corpus is attached.
func ParseGoFile(sc Context, path string, data []byte) (*token.FileSet, *ast.File, error) {
	if sc.corpus != nil {
		return sc.corpus.parseGo(path, data)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	return fset, file, err
}
