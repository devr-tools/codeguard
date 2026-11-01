package quality

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type cloneAnalysisResult struct {
	findings    []core.Finding
	diagnostics []core.Diagnostic
}

type cloneAnalysisBudget struct {
	maxSourceBytes int
	maxTokens      int
	maxWindows     int
}

var defaultCloneAnalysisBudget = cloneAnalysisBudget{
	maxSourceBytes: 64 << 20,
	maxTokens:      2_000_000,
	maxWindows:     1_000_000,
}

func cloneFindingsForTarget(env support.Context, target core.TargetConfig) cloneAnalysisResult {
	return cloneFindingsForTargetWithBudget(env, target, defaultCloneAnalysisBudget)
}

func cloneFindingsForTargetWithBudget(env support.Context, target core.TargetConfig, budget cloneAnalysisBudget) cloneAnalysisResult {
	threshold := env.Config.Checks.QualityRules.CloneTokenThreshold
	if threshold <= 0 {
		return cloneAnalysisResult{}
	}

	docs, truncated := cloneDocumentsForTargetWithBudget(env, target, budget)
	result := cloneAnalysisResult{}
	if len(docs) < 2 {
		if truncated {
			result.diagnostics = []core.Diagnostic{cloneBudgetDiagnostic(target, budget)}
		}
		return result
	}

	candidates, windowBudgetReached := detectCloneCandidates(docs, threshold, budget.maxWindows)
	if truncated || windowBudgetReached {
		result.diagnostics = []core.Diagnostic{cloneBudgetDiagnostic(target, budget)}
	}
	findings := make([]core.Finding, 0, len(candidates)*2)
	for _, candidate := range candidates {
		left := docs[candidate.LeftDoc]
		right := docs[candidate.RightDoc]
		leftLine := left.Tokens[candidate.LeftStart].Line
		rightLine := right.Tokens[candidate.RightStart].Line
		message := fmt.Sprintf(
			"duplicate normalized token sequence of %d tokens also found in %s:%d (threshold %d)",
			candidate.Length,
			right.Path,
			rightLine,
			threshold,
		)
		findings = append(findings, newQualityWarnFinding(env, support.WarnFindingInput{
			RuleID:  "quality.duplicate-code",
			Path:    left.Path,
			Line:    leftLine,
			Column:  1,
			Message: message,
		}))
		message = fmt.Sprintf(
			"duplicate normalized token sequence of %d tokens also found in %s:%d (threshold %d)",
			candidate.Length,
			left.Path,
			leftLine,
			threshold,
		)
		findings = append(findings, newQualityWarnFinding(env, support.WarnFindingInput{
			RuleID:  "quality.duplicate-code",
			Path:    right.Path,
			Line:    rightLine,
			Column:  1,
			Message: message,
		}))
	}
	result.findings = findings
	return result
}

func cloneDocumentsForTargetWithBudget(env support.Context, target core.TargetConfig, budget cloneAnalysisBudget) ([]cloneDocument, bool) {
	docs := make([]cloneDocument, 0)
	include := cloneIncludeForLanguage(target.Language)
	state := cloneBudgetState{budget: budget}
	// Clone detection builds cross-file state (the document list) rather than
	// per-file findings, so it must visit every file directly. Routing it
	// through the per-file findings cache would skip the tokenizer on a cache
	// hit and silently drop every clone once a warm cache exists.
	visitTargetFileBatches(env, target, func(rel string) bool {
		return include(rel) && !cloneExcludedPath(target.Language, rel)
	}, func(batch []visitedFile) {
		tokenized := tokenizeCloneBatch(batch, &state)
		for i, file := range batch {
			if tokens := state.admit(len(file.data), tokenized[i]); len(tokens) > 0 {
				docs = append(docs, cloneDocument{Path: file.rel, Tokens: tokens})
			}
		}
	})
	return docs, state.truncated
}

// clonedFileTokens is one file's tokenization under the whole-corpus token
// budget: at most budget.maxTokens tokens, and whether the file had more.
type clonedFileTokens struct {
	tokens    []cloneToken
	truncated bool
	skipped   bool
}

// tokenizeCloneBatch tokenizes a batch of files on the bounded worker pool.
// Each file is tokenized against the full corpus budget; cloneBudgetState.admit
// then applies the running budget in visit order, which yields exactly the
// documents a sequential tokenize-and-count loop would. Files that can never
// fit (the corpus token budget is already spent, or the file alone exceeds the
// byte budget) are not tokenized at all.
func tokenizeCloneBatch(batch []visitedFile, state *cloneBudgetState) []clonedFileTokens {
	tokenized := make([]clonedFileTokens, len(batch))
	if state.tokensExhausted() {
		return tokenized
	}
	support.ParallelFor(len(batch), func(i int) {
		if len(batch[i].data) > state.budget.maxSourceBytes {
			tokenized[i].skipped = true
			return
		}
		tokens, truncated := tokenizeNormalizedCloneTextBounded(string(batch[i].data), state.budget.maxTokens)
		tokenized[i] = clonedFileTokens{tokens: tokens, truncated: truncated}
	})
	return tokenized
}

// cloneBudgetState applies the clone corpus budget to files in visit order.
type cloneBudgetState struct {
	budget           cloneAnalysisBudget
	totalSourceBytes int
	totalTokens      int
	truncated        bool
}

func (s *cloneBudgetState) tokensExhausted() bool {
	return s.totalTokens >= s.budget.maxTokens
}

// admit charges one file against the budget and returns the tokens it
// contributes. It reproduces the sequential rule: a file that would overflow
// either budget is dropped, and a file with more tokens than the budget has
// left contributes only its first remaining tokens.
func (s *cloneBudgetState) admit(sourceBytes int, file clonedFileTokens) []cloneToken {
	if s.tokensExhausted() || s.totalSourceBytes+sourceBytes > s.budget.maxSourceBytes || file.skipped {
		s.truncated = true
		return nil
	}
	remaining := s.budget.maxTokens - s.totalTokens
	tokens := file.tokens
	if file.truncated || len(tokens) > remaining {
		s.truncated = true
	}
	if len(tokens) > remaining {
		tokens = tokens[:remaining:remaining]
	}
	if len(tokens) > 0 {
		s.totalSourceBytes += sourceBytes
		s.totalTokens += len(tokens)
	}
	return tokens
}

func cloneBudgetDiagnostic(target core.TargetConfig, budget cloneAnalysisBudget) core.Diagnostic {
	return core.Diagnostic{
		ID:    "quality.duplicate-code-budget",
		Level: "info",
		Kind:  "analysis",
		Message: fmt.Sprintf(
			"duplicate-code analysis for target %q reached its bounded corpus budget; add repository-specific generated paths to exclude if needed",
			target.Name,
		),
		Metadata: map[string]string{
			"target":           target.Name,
			"max_source_bytes": strconv.Itoa(budget.maxSourceBytes),
			"max_tokens":       strconv.Itoa(budget.maxTokens),
			"max_windows":      strconv.Itoa(budget.maxWindows),
		},
	}
}

func detectCloneCandidates(docs []cloneDocument, threshold int, maxWindows int) ([]cloneCandidate, bool) {
	index, truncated := cloneWindowIndexBounded(docs, threshold, maxWindows)
	candidates := collectCloneCandidates(index, docs, threshold)
	sortCloneCandidates(candidates, docs)
	return candidates, truncated
}

// cloneWindowMultiplier is the odd multiplier for the polynomial rolling
// window hash (Knuth's MMIX LCG constant). Multiplication by an odd constant
// modulo 2^64 is invertible, which lets a window's hash be updated in O(1)
// when the window slides by one token.
const cloneWindowMultiplier uint64 = 6364136223846793005

// Clone detection runs on untrusted repository contents. Keep both the work
// spent comparing hash-bucket occurrences and the resulting report size
// bounded so a repository containing many identical windows cannot make the
// pairwise scan consume unbounded CPU or memory.
const (
	maxClonePairComparisons = 100_000
	maxCloneCandidates      = 1_000
)

// cloneWindowIndexBounded hashes every threshold-token window with a rolling
// polynomial hash over the per-token hashes computed at tokenize time (O(1)
// per slide, no allocation), and groups the windows by hash. Equal windows
// always collide (the hash is a deterministic function of the normalized
// tokens), and unequal windows that collide are discarded by the
// token-by-token verification in sharedCloneLength.
//
// The grouping is a stable radix sort of a flat window slice rather than a
// map of per-hash slices: it allocates two slices instead of one slice per
// distinct window, and it yields the hashes in ascending order with each
// hash's windows in (document, token) order — exactly the visit order
// collectCloneCandidates needs, with no separate key sort.
//
// At most maxOccurrences windows are indexed, taken in (document, token)
// order; truncated reports that more windows existed.
func cloneWindowIndexBounded(docs []cloneDocument, threshold int, maxOccurrences int) (cloneIndex, bool) {
	total := 0
	for _, doc := range docs {
		if len(doc.Tokens) >= threshold {
			total += len(doc.Tokens) - threshold + 1
		}
	}
	if total == 0 {
		return cloneIndex{}, false
	}
	truncated := total > maxOccurrences
	if truncated {
		total = maxOccurrences
	}
	if total <= 0 {
		return cloneIndex{}, true
	}
	windows := make([]cloneWindow, 0, total)
	// top = multiplier^(threshold-1), the weight of the token leaving the
	// window on each slide.
	top := uint64(1)
	for i := 0; i < threshold-1; i++ {
		top *= cloneWindowMultiplier
	}
	for docIdx, doc := range docs {
		if len(doc.Tokens) < threshold || len(windows) == total {
			continue
		}
		var hash uint64
		for i := 0; i < threshold; i++ {
			hash = hash*cloneWindowMultiplier + doc.Tokens[i].Hash
		}
		windows = append(windows, cloneWindow{Hash: hash, DocIndex: int32(docIdx), TokenIndex: 0}) //nolint:gosec // bounded by the clone corpus budget
		for tokenIdx := 1; tokenIdx+threshold <= len(doc.Tokens) && len(windows) < total; tokenIdx++ {
			hash = (hash-doc.Tokens[tokenIdx-1].Hash*top)*cloneWindowMultiplier + doc.Tokens[tokenIdx+threshold-1].Hash
			windows = append(windows, cloneWindow{Hash: hash, DocIndex: int32(docIdx), TokenIndex: int32(tokenIdx)}) //nolint:gosec // bounded by the clone corpus budget
		}
	}
	return cloneIndex{windows: radixSortCloneWindows(windows)}, truncated
}

// radixSortCloneWindows stably sorts windows by hash with an LSD radix sort
// (four 16-bit digits). Stability keeps each hash's windows in the order they
// were indexed. Digits on which every window agrees are skipped.
func radixSortCloneWindows(windows []cloneWindow) []cloneWindow {
	if len(windows) < 2 {
		return windows
	}
	scratch := make([]cloneWindow, len(windows))
	counts := make([]int, 1<<16)
	for shift := uint(0); shift < 64; shift += 16 {
		clear(counts)
		for i := range windows {
			counts[(windows[i].Hash>>shift)&0xffff]++
		}
		if counts[(windows[0].Hash>>shift)&0xffff] == len(windows) {
			continue
		}
		offset := 0
		for digit, count := range counts {
			counts[digit] = offset
			offset += count
		}
		for i := range windows {
			digit := (windows[i].Hash >> shift) & 0xffff
			scratch[counts[digit]] = windows[i]
			counts[digit]++
		}
		windows, scratch = scratch, windows
	}
	return windows
}

func collectCloneCandidates(index cloneIndex, docs []cloneDocument, threshold int) []cloneCandidate {
	// Candidates are partitioned by their (LeftDoc, RightDoc) pair. The overlap
	// merge only ever compares candidates that share a pair, so bucketing by
	// pair turns the merge from a linear scan of every candidate found so far
	// into a scan of just the (usually tiny) bucket for that file pair.
	byPair := make(map[[2]int][]cloneCandidate)
	comparisons := 0
	// Walk the hash groups in a fixed (ascending hash) order. Collection stops
	// early once maxClonePairComparisons or maxCloneCandidates is reached, so
	// the iteration order decides *which* window hashes are examined before
	// the cap; a randomized order made the surviving candidate set differ
	// between runs on the same input. sortCloneCandidates orders the result
	// but cannot recover what was never collected.
	var occurrences []cloneOccurrence
	index.forEachRepeatedHash(func(group []cloneWindow) bool {
		occurrences = occurrences[:0]
		for _, window := range group {
			occurrences = append(occurrences, cloneOccurrence{DocIndex: int(window.DocIndex), TokenIndex: int(window.TokenIndex)})
		}
		return addClonePairs(byPair, occurrences, docs, threshold, &comparisons)
	})
	return flattenCloneCandidates(byPair)
}

// forEachRepeatedHash calls visit with the windows of each hash that occurs
// at least twice, in ascending hash order, until visit returns true. A hash
// seen once cannot pair with anything, so it is skipped.
func (index cloneIndex) forEachRepeatedHash(visit func(group []cloneWindow) bool) {
	windows := index.windows
	for start := 0; start < len(windows); {
		end := start + 1
		for end < len(windows) && windows[end].Hash == windows[start].Hash {
			end++
		}
		if end-start >= 2 && visit(windows[start:end]) {
			return
		}
		start = end
	}
}

// addClonePairs reports whether candidate collection has reached a safety
// limit and should stop.
func addClonePairs(byPair map[[2]int][]cloneCandidate, occurrences []cloneOccurrence, docs []cloneDocument, threshold int, comparisons *int) bool {
	if len(occurrences) < 2 {
		return false
	}
	for i := 0; i < len(occurrences); i++ {
		for j := i + 1; j < len(occurrences); j++ {
			if *comparisons >= maxClonePairComparisons {
				return true
			}
			*comparisons++
			if next, ok := cloneCandidateForPair(occurrences[i], occurrences[j], docs, threshold); ok {
				addOrMergeCloneCandidate(byPair, next)
				if cloneCandidateCount(byPair) >= maxCloneCandidates {
					return true
				}
			}
		}
	}
	return false
}

func cloneCandidateCount(byPair map[[2]int][]cloneCandidate) int {
	total := 0
	for _, bucket := range byPair {
		total += len(bucket)
	}
	return total
}

func flattenCloneCandidates(byPair map[[2]int][]cloneCandidate) []cloneCandidate {
	total := 0
	for _, bucket := range byPair {
		total += len(bucket)
	}
	candidates := make([]cloneCandidate, 0, total)
	for _, bucket := range byPair {
		candidates = append(candidates, bucket...)
	}
	return candidates
}

func cloneCandidateForPair(left cloneOccurrence, right cloneOccurrence, docs []cloneDocument, threshold int) (cloneCandidate, bool) {
	if left.DocIndex == right.DocIndex {
		return cloneCandidate{}, false
	}
	length := sharedCloneLength(docs[left.DocIndex].Tokens, left.TokenIndex, docs[right.DocIndex].Tokens, right.TokenIndex)
	if length < threshold {
		return cloneCandidate{}, false
	}
	return cloneCandidate{
		LeftDoc:    left.DocIndex,
		LeftStart:  left.TokenIndex,
		RightDoc:   right.DocIndex,
		RightStart: right.TokenIndex,
		Length:     length,
	}, true
}

func sortCloneCandidates(candidates []cloneCandidate, docs []cloneDocument) {
	// Normalize each document path once up front rather than 2-4 times inside
	// every comparator call.
	slashPaths := make([]string, len(docs))
	for i := range docs {
		slashPaths[i] = filepath.ToSlash(docs[i].Path)
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftDoc := slashPaths[candidates[i].LeftDoc]
		rightDoc := slashPaths[candidates[j].LeftDoc]
		if leftDoc != rightDoc {
			return leftDoc < rightDoc
		}
		if candidates[i].LeftStart != candidates[j].LeftStart {
			return candidates[i].LeftStart < candidates[j].LeftStart
		}
		otherLeft := slashPaths[candidates[i].RightDoc]
		otherRight := slashPaths[candidates[j].RightDoc]
		if otherLeft != otherRight {
			return otherLeft < otherRight
		}
		return candidates[i].RightStart < candidates[j].RightStart
	})
}

func addOrMergeCloneCandidate(byPair map[[2]int][]cloneCandidate, next cloneCandidate) {
	if next.LeftDoc > next.RightDoc {
		next.LeftDoc, next.RightDoc = next.RightDoc, next.LeftDoc
		next.LeftStart, next.RightStart = next.RightStart, next.LeftStart
	}
	key := [2]int{next.LeftDoc, next.RightDoc}
	bucket := byPair[key]
	for idx, existing := range bucket {
		if cloneRangesOverlap(existing.LeftStart, existing.Length, next.LeftStart, next.Length) &&
			cloneRangesOverlap(existing.RightStart, existing.Length, next.RightStart, next.Length) {
			if next.Length > existing.Length {
				bucket[idx] = next
			}
			return
		}
	}
	byPair[key] = append(bucket, next)
}
