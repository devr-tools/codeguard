package quality

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func cloneFindingsForTarget(env support.Context, target core.TargetConfig) []core.Finding {
	threshold := env.Config.Checks.QualityRules.CloneTokenThreshold
	if threshold <= 0 {
		return nil
	}

	docs := cloneDocumentsForTarget(env, target)
	if len(docs) < 2 {
		return nil
	}

	candidates := detectCloneCandidates(docs, threshold)
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
	return findings
}

func cloneDocumentsForTarget(env support.Context, target core.TargetConfig) []cloneDocument {
	docs := make([]cloneDocument, 0)
	include := cloneIncludeForLanguage(target.Language)
	// Clone detection builds cross-file state (the document list) rather than
	// per-file findings, so it must visit every file directly. Routing it
	// through the per-file findings cache would skip the tokenizer on a cache
	// hit and silently drop every clone once a warm cache exists.
	env.VisitTargetFiles(target, func(rel string) bool {
		return include(rel) && !cloneExcludedPath(target.Language, rel)
	}, func(file string, data []byte) {
		tokens := tokenizeNormalizedCloneText(string(data))
		if len(tokens) > 0 {
			docs = append(docs, cloneDocument{Path: file, Tokens: tokens})
		}
	})
	return docs
}

func detectCloneCandidates(docs []cloneDocument, threshold int) []cloneCandidate {
	index := cloneWindowIndex(docs, threshold)
	candidates := collectCloneCandidates(index, docs, threshold)
	sortCloneCandidates(candidates, docs)
	return candidates
}

func cloneWindowIndex(docs []cloneDocument, threshold int) cloneIndex {
	index := make(cloneIndex)
	for docIdx, doc := range docs {
		for tokenIdx := 0; tokenIdx+threshold <= len(doc.Tokens); tokenIdx++ {
			hash := cloneWindowHash(doc.Tokens[tokenIdx : tokenIdx+threshold])
			index[hash] = append(index[hash], cloneOccurrence{DocIndex: docIdx, TokenIndex: tokenIdx})
		}
	}
	return index
}

func collectCloneCandidates(index cloneIndex, docs []cloneDocument, threshold int) []cloneCandidate {
	// Candidates are partitioned by their (LeftDoc, RightDoc) pair. The overlap
	// merge only ever compares candidates that share a pair, so bucketing by
	// pair turns the merge from a linear scan of every candidate found so far
	// into a scan of just the (usually tiny) bucket for that file pair.
	byPair := make(map[[2]int][]cloneCandidate)
	for _, occurrences := range index {
		addClonePairs(byPair, occurrences, docs, threshold)
	}
	return flattenCloneCandidates(byPair)
}

func addClonePairs(byPair map[[2]int][]cloneCandidate, occurrences []cloneOccurrence, docs []cloneDocument, threshold int) {
	if len(occurrences) < 2 {
		return
	}
	for i := 0; i < len(occurrences); i++ {
		for j := i + 1; j < len(occurrences); j++ {
			if next, ok := cloneCandidateForPair(occurrences[i], occurrences[j], docs, threshold); ok {
				addOrMergeCloneCandidate(byPair, next)
			}
		}
	}
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
