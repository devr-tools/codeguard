package checks_test

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/quality"
)

// cloneBudgetReference is the sequential clone document collection the
// parallel implementation replaced: tokenize each file, in visit order,
// against whatever is left of the corpus budget.
func cloneBudgetReference(files []quality.CloneSourceForTest, maxSourceBytes int, maxTokens int) (paths []string, tokenCounts []int, truncated bool) {
	totalBytes, totalTokens := 0, 0
	for _, file := range files {
		if totalTokens >= maxTokens || totalBytes+len(file.Source) > maxSourceBytes {
			truncated = true
			continue
		}
		values, _, limitReached := quality.TokenizeCloneTextForTest(file.Source, maxTokens-totalTokens)
		if limitReached {
			truncated = true
		}
		if len(values) > 0 {
			paths = append(paths, file.Path)
			tokenCounts = append(tokenCounts, len(values))
			totalBytes += len(file.Source)
			totalTokens += len(values)
		}
	}
	return paths, tokenCounts, truncated
}

// The parallel collection tokenizes files on a worker pool, in batches, and
// applies the corpus budget afterwards in visit order. Across budgets that cut
// mid-file, mid-batch, at a batch boundary and not at all, it must admit
// exactly the documents (and token counts) the sequential loop admitted.
func TestCloneDocumentsParallelMatchSequentialBudget(t *testing.T) {
	rng := rand.New(rand.NewSource(7)) //nolint:gosec // deterministic fixture data
	files := make([]quality.CloneSourceForTest, 0, 700)
	for i := 0; i < 700; i++ {
		var body strings.Builder
		fmt.Fprintf(&body, "package p\n\nfunc f%d(a int) int {\n", i)
		for line := rng.Intn(12); line >= 0; line-- {
			fmt.Fprintf(&body, "\ta = a*%d + %d\n", rng.Intn(9), rng.Intn(99))
		}
		body.WriteString("\treturn a\n}\n")
		if i%97 == 0 {
			body.Reset() // empty files contribute no document
		}
		files = append(files, quality.CloneSourceForTest{Path: fmt.Sprintf("f%04d.go", i), Source: body.String()})
	}
	for _, budget := range []struct{ bytes, tokens int }{
		{1 << 30, 1 << 30},
		{1 << 30, 1},
		{1 << 30, 37},
		{1 << 30, 5_000},
		{1 << 30, 9_000},
		{1 << 30, 20_011},
		{3_000, 1 << 30},
		{40_000, 1 << 30},
		{40_000, 9_000},
		{0, 1 << 30},
		{1 << 30, 0},
	} {
		wantPaths, wantCounts, wantTruncated := cloneBudgetReference(files, budget.bytes, budget.tokens)
		gotPaths, gotCounts, gotTruncated := quality.CloneDocumentsForTest(files, budget.bytes, budget.tokens)
		if gotTruncated != wantTruncated {
			t.Fatalf("budget %+v: truncated = %v, want %v", budget, gotTruncated, wantTruncated)
		}
		if fmt.Sprint(gotPaths) != fmt.Sprint(wantPaths) || fmt.Sprint(gotCounts) != fmt.Sprint(wantCounts) {
			t.Fatalf("budget %+v: admitted %d docs %v, want %d docs %v", budget, len(gotPaths), gotCounts, len(wantPaths), wantCounts)
		}
	}
}

// cloneIndexReference is the map-of-slices window index the radix-sorted
// index replaced, flattened in the order candidate collection visits it:
// hashes ascending, each hash's windows in insertion order.
func cloneIndexReference(tokenHashes [][]uint64, threshold int, maxWindows int) ([]quality.CloneWindowForTest, bool) {
	index := map[uint64][]quality.CloneWindowForTest{}
	count := 0
	truncated := false
	top := uint64(1)
	for i := 0; i < threshold-1; i++ {
		top *= quality.CloneWindowMultiplierForTest
	}
done:
	for doc, hashes := range tokenHashes {
		if len(hashes) < threshold {
			continue
		}
		var hash uint64
		for i := 0; i < threshold; i++ {
			hash = hash*quality.CloneWindowMultiplierForTest + hashes[i]
		}
		for token := 0; token+threshold <= len(hashes); token++ {
			if token > 0 {
				hash = (hash-hashes[token-1]*top)*quality.CloneWindowMultiplierForTest + hashes[token+threshold-1]
			}
			if count >= maxWindows {
				truncated = true
				break done
			}
			index[hash] = append(index[hash], quality.CloneWindowForTest{Hash: hash, Document: doc, Token: token})
			count++
		}
	}
	keys := make([]uint64, 0, len(index))
	for hash := range index {
		keys = append(keys, hash)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	windows := make([]quality.CloneWindowForTest, 0, count)
	for _, hash := range keys {
		windows = append(windows, index[hash]...)
	}
	return windows, truncated
}

func TestCloneWindowIndexMatchesMapReference(t *testing.T) {
	rng := rand.New(rand.NewSource(11)) //nolint:gosec // deterministic fixture data
	docs := make([][]uint64, 40)
	for i := range docs {
		docs[i] = make([]uint64, rng.Intn(400))
		for j := range docs[i] {
			// A small alphabet makes many windows repeat, within and across
			// documents, so hash groups have several members; full 64-bit
			// values exercise every radix digit.
			if rng.Intn(4) == 0 {
				docs[i][j] = rng.Uint64()
			} else {
				docs[i][j] = uint64(rng.Intn(3)) << (16 * uint(rng.Intn(4)))
			}
		}
	}
	for _, threshold := range []int{1, 2, 5, 30, 1000} {
		for _, maxWindows := range []int{0, 1, 17, 2_000, 1 << 30} {
			want, wantTruncated := cloneIndexReference(docs, threshold, maxWindows)
			got, gotTruncated := quality.CloneWindowIndexForTest(docs, threshold, maxWindows)
			if gotTruncated != wantTruncated {
				t.Fatalf("threshold %d maxWindows %d: truncated = %v, want %v", threshold, maxWindows, gotTruncated, wantTruncated)
			}
			if len(got) != len(want) {
				t.Fatalf("threshold %d maxWindows %d: %d windows, want %d", threshold, maxWindows, len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("threshold %d maxWindows %d: window %d = %+v, want %+v", threshold, maxWindows, i, got[i], want[i])
				}
			}
		}
	}
}
