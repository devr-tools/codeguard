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
		findings = append(findings, warnFinding(env, "quality.duplicate-code", left.Path, leftLine, 1, message))
		message = fmt.Sprintf(
			"duplicate normalized token sequence of %d tokens also found in %s:%d (threshold %d)",
			candidate.Length,
			left.Path,
			leftLine,
			threshold,
		)
		findings = append(findings, warnFinding(env, "quality.duplicate-code", right.Path, rightLine, 1, message))
	}
	result.findings = findings
	return result
}

func cloneDocumentsForTargetWithBudget(env support.Context, target core.TargetConfig, budget cloneAnalysisBudget) ([]cloneDocument, bool) {
	docs := make([]cloneDocument, 0)
	include := cloneIncludeForLanguage(target.Language)
	totalSourceBytes := 0
	totalTokens := 0
	truncated := false
	// Clone detection builds cross-file state (the document list) rather than
	// per-file findings, so it must visit every file directly. Routing it
	// through the per-file findings cache would skip the tokenizer on a cache
	// hit and silently drop every clone once a warm cache exists.
	env.VisitTargetFiles(target, func(rel string) bool {
		return include(rel) && !cloneExcludedPath(target.Language, rel)
	}, func(file string, data []byte) {
		if totalTokens >= budget.maxTokens || totalSourceBytes+len(data) > budget.maxSourceBytes {
			truncated = true
			return
		}
		tokens, tokenLimitReached := tokenizeNormalizedCloneTextBounded(string(data), budget.maxTokens-totalTokens)
		if tokenLimitReached {
			truncated = true
		}
		if len(tokens) > 0 {
			docs = append(docs, cloneDocument{Path: file, Tokens: tokens})
			totalSourceBytes += len(data)
			totalTokens += len(tokens)
		}
	})
	return docs, truncated
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

// cloneWindowIndex groups every threshold-token window by a rolling polynomial
// hash over the per-token hashes computed at tokenize time. Compared with the
// previous implementation (a fresh FNV over every token's bytes per window)
// this allocates nothing and slides in O(1) per window instead of O(threshold).
// Equal windows always collide (the hash is a deterministic function of the
// normalized tokens), and unequal windows that collide are discarded by the
// token-by-token verification in sharedCloneLength, so the resulting clone
// candidates are identical to the old per-window byte hashing.
func cloneWindowIndexBounded(docs []cloneDocument, threshold int, maxOccurrences int) (cloneIndex, bool) {
	index := make(cloneIndex)
	hasWindow := false
	for _, doc := range docs {
		if len(doc.Tokens) >= threshold {
			hasWindow = true
			break
		}
	}
	if !hasWindow {
		return index, false
	}
	if maxOccurrences <= 0 {
		return index, true
	}
	// top = multiplier^(threshold-1), the weight of the token leaving the
	// window on each slide.
	top := uint64(1)
	for i := 0; i < threshold-1; i++ {
		top *= cloneWindowMultiplier
	}
	occurrences := 0
	for docIdx, doc := range docs {
		if len(doc.Tokens) < threshold {
			continue
		}
		var hash uint64
		for i := 0; i < threshold; i++ {
			hash = hash*cloneWindowMultiplier + doc.Tokens[i].Hash
		}
		if occurrences >= maxOccurrences {
			return index, true
		}
		index[hash] = append(index[hash], cloneOccurrence{DocIndex: docIdx, TokenIndex: 0})
		occurrences++
		for tokenIdx := 1; tokenIdx+threshold <= len(doc.Tokens); tokenIdx++ {
			if occurrences >= maxOccurrences {
				return index, true
			}
			hash = (hash-doc.Tokens[tokenIdx-1].Hash*top)*cloneWindowMultiplier + doc.Tokens[tokenIdx+threshold-1].Hash
			index[hash] = append(index[hash], cloneOccurrence{DocIndex: docIdx, TokenIndex: tokenIdx})
			occurrences++
		}
	}
	return index, false
}

func collectCloneCandidates(index cloneIndex, docs []cloneDocument, threshold int) []cloneCandidate {
	// Candidates are partitioned by their (LeftDoc, RightDoc) pair. The overlap
	// merge only ever compares candidates that share a pair, so bucketing by
	// pair turns the merge from a linear scan of every candidate found so far
	// into a scan of just the (usually tiny) bucket for that file pair.
	byPair := make(map[[2]int][]cloneCandidate)
	comparisons := 0
	for _, occurrences := range index {
		if addClonePairs(byPair, occurrences, docs, threshold, &comparisons) {
			break
		}
	}
	return flattenCloneCandidates(byPair)
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
