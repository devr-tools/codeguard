package support

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestFileCorpusDoesNotRetainReadsBeyondAggregateBudget(t *testing.T) {
	root := t.TempDir()
	first := []byte("package first\n")
	second := []byte("package second\n")
	for name, data := range map[string][]byte{"first.go": first, "second.go": second} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	corpus := newFileCorpus()
	corpus.maxReadBytes = len(first)
	if _, err := corpus.read(root, "first.go"); err != nil {
		t.Fatalf("read first file: %v", err)
	}
	if _, err := corpus.read(root, "second.go"); !errors.Is(err, errCorpusReadBudget) {
		t.Fatalf("read second file error = %v, want corpus budget error", err)
	}

	if corpus.readBytes > corpus.maxReadBytes {
		t.Fatalf("retained read bytes = %d, budget = %d", corpus.readBytes, corpus.maxReadBytes)
	}
	if got := len(corpus.reads); got != 1 {
		t.Fatalf("retained read entries = %d, want 1", got)
	}

	if _, err := corpus.read(root, "second.go"); !errors.Is(err, errCorpusReadBudget) {
		t.Fatalf("repeated overflow read error = %v, want corpus budget error", err)
	}
}

func TestFileCorpusDoesNotRetainGoASTsBeyondAggregateBudget(t *testing.T) {
	corpus := newFileCorpus()
	first := []byte("package first\n")
	second := []byte("package second\n")
	corpus.maxGoASTBytes = len(first)

	if _, _, err := corpus.parseGo("first.go", first); err != nil {
		t.Fatalf("parse first file: %v", err)
	}
	if _, _, err := corpus.parseGo("second.go", second); !errors.Is(err, errCorpusGoASTBudget) {
		t.Fatalf("parse second file error = %v, want corpus budget error", err)
	}

	if corpus.goASTBytes > corpus.maxGoASTBytes {
		t.Fatalf("retained Go AST source bytes = %d, budget = %d", corpus.goASTBytes, corpus.maxGoASTBytes)
	}
	if got := len(corpus.asts); got != 1 {
		t.Fatalf("retained Go AST entries = %d, want 1", got)
	}
}

func TestFileCorpusBoundsZeroByteReadEntries(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second", "third"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	corpus := newFileCorpus()
	corpus.maxReadEntries = 2
	for _, name := range []string{"first", "second"} {
		if _, err := corpus.read(root, name); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
	}
	if _, err := corpus.read(root, "third"); !errors.Is(err, errCorpusReadBudget) {
		t.Fatalf("third read error = %v, want corpus budget error", err)
	}
	if got := len(corpus.reads); got != 2 {
		t.Fatalf("retained read entries = %d, want 2", got)
	}
}

func TestFileCorpusBoundsFileListingsAndReportsDiagnostic(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first.go", "second.go", "third.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package fixture\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	corpus := newFileCorpus()
	corpus.maxFiles = 2
	files, err := corpus.list(root, nil)
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	if got := len(files); got != 2 {
		t.Fatalf("listed files = %d, want 2", got)
	}
	section := FinalizeSection(Context{corpus: corpus}, "quality", "Code Quality", nil)
	if len(section.Diagnostics) != 1 || section.Diagnostics[0].ID != "scan.corpus-budget" {
		t.Fatalf("diagnostics = %#v, want one corpus budget diagnostic", section.Diagnostics)
	}
	if again := corpus.takeDiagnostics(); len(again) != 0 {
		t.Fatalf("diagnostic was emitted more than once: %#v", again)
	}
}

func TestFileCorpusRejectsConcurrentGoParsesBeyondBudget(t *testing.T) {
	corpus := newFileCorpus()
	corpus.maxGoASTBytes = 1
	data := []byte("package fixture\n")

	const callers = 32
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := corpus.parseGo("large.go", data)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if !errors.Is(err, errCorpusGoASTBudget) {
			t.Fatalf("parse error = %v, want corpus budget error", err)
		}
	}
	if got := len(corpus.asts); got != 0 {
		t.Fatalf("retained Go AST entries = %d, want 0", got)
	}
}

func TestFileCorpusSingleflightsConcurrentOverflowReads(t *testing.T) {
	corpus := newFileCorpus()
	corpus.maxReadBytes = 1
	var readCalls atomic.Int32
	releaseRead := make(chan struct{})
	corpus.readFile = func(string) ([]byte, error) {
		readCalls.Add(1)
		<-releaseRead
		return []byte("too large"), nil
	}

	const callers = 32
	start := make(chan struct{})
	var entered sync.WaitGroup
	var done sync.WaitGroup
	entered.Add(callers)
	done.Add(callers)
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer done.Done()
			<-start
			entered.Done()
			_, err := corpus.read("root", "large.go")
			errs <- err
		}()
	}
	close(start)
	entered.Wait()
	close(releaseRead)
	done.Wait()
	close(errs)

	for err := range errs {
		if !errors.Is(err, errCorpusReadBudget) {
			t.Fatalf("read error = %v, want corpus budget error", err)
		}
	}
	if got := readCalls.Load(); got != 1 {
		t.Fatalf("overflow file reads = %d, want one singleflight read", got)
	}
}

func TestFileCorpusBoundsGoASTEntriesIndependentOfBytes(t *testing.T) {
	corpus := newFileCorpus()
	corpus.maxGoASTBytes = 1 << 20
	corpus.maxGoASTEntries = 2
	data := []byte("package fixture\n")

	for _, name := range []string{"first.go", "second.go"} {
		if _, _, err := corpus.parseGo(name, data); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}
	if _, _, err := corpus.parseGo("third.go", data); !errors.Is(err, errCorpusGoASTBudget) {
		t.Fatalf("third parse error = %v, want corpus budget error", err)
	}
	if got := len(corpus.asts); got != 2 {
		t.Fatalf("retained Go AST entries = %d, want 2", got)
	}
}
