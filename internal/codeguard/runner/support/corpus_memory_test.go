package support

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// The corpus budgets bound what a scan RETAINS, never what it analyses. Each
// test below therefore asserts two things together: retention stays inside the
// budget, and the over-budget caller still receives real content. Asserting
// only the first is what allowed budget exhaustion to silently suppress
// findings.

func TestFileCorpusServesReadsBeyondAggregateBudgetWithoutRetainingThem(t *testing.T) {
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

	data, err := corpus.read(root, "second.go")
	if err != nil {
		t.Fatalf("over-budget read error = %v, want the file contents", err)
	}
	if string(data) != string(second) {
		t.Fatalf("over-budget read = %q, want %q", data, second)
	}

	if corpus.readBytes > corpus.maxReadBytes {
		t.Fatalf("retained read bytes = %d, budget = %d", corpus.readBytes, corpus.maxReadBytes)
	}
	if got := len(corpus.reads); got != 1 {
		t.Fatalf("retained read entries = %d, want 1", got)
	}
	if !corpus.readBudgetExceeded {
		t.Fatal("read budget overflow was not recorded")
	}

	// A repeat request is served again rather than turning into a cache miss
	// that reports nothing.
	again, err := corpus.read(root, "second.go")
	if err != nil {
		t.Fatalf("repeated over-budget read error = %v, want the file contents", err)
	}
	if string(again) != string(second) {
		t.Fatalf("repeated over-budget read = %q, want %q", again, second)
	}
	if corpus.readBytes > corpus.maxReadBytes {
		t.Fatalf("retained read bytes after repeat = %d, budget = %d", corpus.readBytes, corpus.maxReadBytes)
	}
}

func TestFileCorpusAdmitsASmallerFileAfterAnOversizedOne(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, 64)
	small := []byte("x")
	for name, data := range map[string][]byte{"big.txt": big, "small.txt": small} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	corpus := newFileCorpus()
	corpus.maxReadBytes = 8
	if _, err := corpus.read(root, "big.txt"); err != nil {
		t.Fatalf("read oversized file: %v", err)
	}
	if got := len(corpus.reads); got != 0 {
		t.Fatalf("retained read entries after oversized read = %d, want 0", got)
	}
	// Overflow must not latch the cache shut: the budget still has headroom and
	// a file that fits is admitted.
	if _, err := corpus.read(root, "small.txt"); err != nil {
		t.Fatalf("read small file: %v", err)
	}
	if got := len(corpus.reads); got != 1 {
		t.Fatalf("retained read entries = %d, want the small file cached", got)
	}
}

func TestFileCorpusServesGoASTsBeyondAggregateBudgetWithoutRetainingThem(t *testing.T) {
	corpus := newFileCorpus()
	first := []byte("package first\n")
	second := []byte("package second\n")
	corpus.maxGoASTBytes = len(first) * goASTBytesPerSourceByte

	if _, _, err := corpus.parseGo("first.go", first); err != nil {
		t.Fatalf("parse first file: %v", err)
	}

	fset, file, err := corpus.parseGo("second.go", second)
	if err != nil {
		t.Fatalf("over-budget parse error = %v, want a parsed file", err)
	}
	if fset == nil || file == nil {
		t.Fatal("over-budget parse returned no tree")
	}
	if file.Name.Name != "second" {
		t.Fatalf("over-budget parse package = %q, want %q", file.Name.Name, "second")
	}

	if corpus.goASTBytes > corpus.maxGoASTBytes {
		t.Fatalf("retained Go AST bytes = %d, budget = %d", corpus.goASTBytes, corpus.maxGoASTBytes)
	}
	if got := len(corpus.asts); got != 1 {
		t.Fatalf("retained Go AST entries = %d, want 1", got)
	}
	if !corpus.goASTBudgetExceeded {
		t.Fatal("Go AST budget overflow was not recorded")
	}
}

func TestFileCorpusGoASTBudgetCountsRetainedTreeNotSource(t *testing.T) {
	corpus := newFileCorpus()
	source := []byte("package fixture\n")
	// Budget room for the source bytes but not for the tree they expand into.
	corpus.maxGoASTBytes = len(source) * (goASTBytesPerSourceByte - 1)

	if _, _, err := corpus.parseGo("fixture.go", source); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if got := len(corpus.asts); got != 0 {
		t.Fatalf("retained Go AST entries = %d, want 0: the tree does not fit the budget", got)
	}
}

func TestFileCorpusServesReadsBeyondEntryBudget(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second", "third"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
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
	data, err := corpus.read(root, "third")
	if err != nil {
		t.Fatalf("over-entry-budget read error = %v, want the file contents", err)
	}
	if string(data) != "third" {
		t.Fatalf("over-entry-budget read = %q, want %q", data, "third")
	}
	if got := len(corpus.reads); got != 2 {
		t.Fatalf("retained read entries = %d, want 2", got)
	}
}

func TestFileCorpusBudgetDiagnosticNamesTheBudgetThatTripped(t *testing.T) {
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
	for _, name := range []string{"first.go", "second.go"} {
		if _, err := corpus.read(root, name); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
	}

	diagnostics := corpus.takeDiagnostics()
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want exactly one", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.ID != "scan.corpus-budget" {
		t.Fatalf("diagnostic id = %q, want scan.corpus-budget", diagnostic.ID)
	}
	// The byte budget is what ran out; reporting the (untouched) entry budget
	// sent operators looking for 50,000 cached files that were never there.
	if got := diagnostic.Metadata["budget"]; got != "read_bytes" {
		t.Fatalf("diagnostic budget = %q, want read_bytes", got)
	}
	if got := diagnostic.Metadata["findings_affected"]; got != "no" {
		t.Fatalf("diagnostic findings_affected = %q, want no", got)
	}
}

func TestFileCorpusBoundsFileListingsAndWarnsThatFilesWereNotScanned(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first.go", "second.go", "third.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package fixture\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	corpus := newFileCorpus()
	corpus.maxFiles = 2
	files, err := corpus.list(root, nil, FileWalkOptions{})
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	if got := len(files); got != 2 {
		t.Fatalf("listed files = %d, want 2", got)
	}
	section := FinalizeSection(Context{corpus: corpus}, "quality", "Code Quality", nil)
	if len(section.Diagnostics) != 1 || section.Diagnostics[0].ID != "scan.file-limit" {
		t.Fatalf("diagnostics = %#v, want one file-limit diagnostic", section.Diagnostics)
	}
	// Walk truncation is the one corpus bound that really does drop files from
	// the scan, so unlike the cache budgets it is not informational.
	if got := section.Diagnostics[0].Level; got != "warn" {
		t.Fatalf("file-limit diagnostic level = %q, want warn", got)
	}
	if got := section.Diagnostics[0].Metadata["findings_affected"]; got != "yes" {
		t.Fatalf("file-limit findings_affected = %q, want yes", got)
	}
	if again := corpus.takeDiagnostics(); len(again) != 0 {
		t.Fatalf("diagnostic was emitted more than once: %#v", again)
	}
}

func TestFileCorpusServesConcurrentOverBudgetGoParses(t *testing.T) {
	corpus := newFileCorpus()
	corpus.maxGoASTBytes = 1
	data := []byte("package fixture\n")

	const callers = 32
	var wg sync.WaitGroup
	var parsed atomic.Int32
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, file, err := corpus.parseGo("large.go", data)
			if err == nil && file != nil {
				parsed.Add(1)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("over-budget parse error = %v, want a parsed file", err)
		}
	}
	if got := parsed.Load(); got != callers {
		t.Fatalf("callers that received a tree = %d, want %d", got, callers)
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
	results := make(chan string, callers)
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer done.Done()
			<-start
			entered.Done()
			data, err := corpus.read("root", "large.go")
			results <- string(data)
			errs <- err
		}()
	}
	close(start)
	entered.Wait()
	close(releaseRead)
	done.Wait()
	close(errs)
	close(results)

	for err := range errs {
		if err != nil {
			t.Fatalf("over-budget read error = %v, want the file contents", err)
		}
	}
	for data := range results {
		if data != "too large" {
			t.Fatalf("over-budget read = %q, want the file contents", data)
		}
	}
	// Racing callers still collapse onto one read; only callers that arrive
	// after the slot is dropped pay for their own.
	if got := readCalls.Load(); got < 1 {
		t.Fatalf("overflow file reads = %d, want at least one", got)
	}
	if got := len(corpus.reads); got != 0 {
		t.Fatalf("retained read entries = %d, want 0", got)
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
	if _, file, err := corpus.parseGo("third.go", data); err != nil || file == nil {
		t.Fatalf("third parse = (%v, %v), want a parsed file", file, err)
	}
	if got := len(corpus.asts); got != 2 {
		t.Fatalf("retained Go AST entries = %d, want 2", got)
	}
}

func TestCorpusRetentionBudgetScalesDownWithGOMEMLIMITOnly(t *testing.T) {
	previous := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })

	debug.SetMemoryLimit(math.MaxInt64)
	if got := corpusRetentionBudget(maxCorpusReadBytes, minCorpusReadBytes, 8); got != maxCorpusReadBytes {
		t.Fatalf("read budget with no memory limit = %d, want the default %d", got, maxCorpusReadBytes)
	}

	// A limit with room to spare must not inflate the default: Go cannot see a
	// container limit that was not expressed as GOMEMLIMIT, so scaling up would
	// be wrong exactly where memory is tightest.
	debug.SetMemoryLimit(64 << 30)
	if got := corpusRetentionBudget(maxCorpusReadBytes, minCorpusReadBytes, 8); got != maxCorpusReadBytes {
		t.Fatalf("read budget under a large limit = %d, want the default %d", got, maxCorpusReadBytes)
	}

	debug.SetMemoryLimit(512 << 20)
	if got, want := corpusRetentionBudget(maxCorpusReadBytes, minCorpusReadBytes, 8), 64<<20; got != want {
		t.Fatalf("read budget under a 512 MiB limit = %d, want %d", got, want)
	}

	debug.SetMemoryLimit(16 << 20)
	if got := corpusRetentionBudget(maxCorpusReadBytes, minCorpusReadBytes, 8); got != minCorpusReadBytes {
		t.Fatalf("read budget under a tiny limit = %d, want the floor %d", got, minCorpusReadBytes)
	}
}

// Cross-file passes (clone detection, import graphs) build their state from
// VisitTargetFiles, which skips any file whose read returns an error. While an
// exhausted budget produced that error, the document stream those passes saw
// depended on which files won a scheduling-dependent race for the remaining
// budget: concurrent sections share one corpus, so the same repository could
// hand the clone detector a different document set on every run and report
// different duplicate-code findings. Serving over-budget reads keeps the
// stream equal to the walk, which is sorted and single-threaded, so degraded
// scans stay reproducible instead of merely being wrong.
func TestVisitTargetFilesSeesEveryFileWhenTheReadBudgetIsFull(t *testing.T) {
	root := t.TempDir()
	want := make([]string, 0, 40)
	for i := range 40 {
		name := fmt.Sprintf("file%02d.txt", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte(strings.Repeat("x", 512)), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		want = append(want, name)
	}

	visitOrder := func(budget int) []string {
		corpus := newFileCorpus()
		corpus.maxReadBytes = budget
		sc := Context{corpus: corpus}
		var seen []string
		VisitTargetFiles(sc, core.TargetConfig{Name: "repo", Path: root}, func(string) bool { return true },
			func(rel string, data []byte) {
				if len(data) == 0 {
					t.Errorf("visited %s with no content", rel)
				}
				seen = append(seen, rel)
			})
		return seen
	}

	// A budget large enough for everything and one that runs out after a
	// couple of files must produce the same stream.
	full := visitOrder(1 << 20)
	starved := visitOrder(1024)
	if !slices.Equal(full, want) {
		t.Fatalf("unbudgeted visit order = %v, want %v", full, want)
	}
	if !slices.Equal(starved, want) {
		t.Fatalf("budget-starved visit skipped files: got %d of %d", len(starved), len(want))
	}
}
