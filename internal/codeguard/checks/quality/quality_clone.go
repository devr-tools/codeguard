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
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.duplicate-code",
			Level:   "warn",
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
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.duplicate-code",
			Level:   "warn",
			Path:    right.Path,
			Line:    rightLine,
			Column:  1,
			Message: message,
		}))
	}
	return findings
}

func cloneDocumentsForTarget(env support.Context, target core.TargetConfig) []cloneDocument {
	docs := make([]cloneDocument, 0)
	include := cloneIncludeForLanguage(target.Language)
	env.ScanTargetFiles(target, "quality-clone", func(rel string) bool {
		return include(rel) && !cloneExcludedPath(target.Language, rel)
	}, func(file string, data []byte) []core.Finding {
		tokens := tokenizeNormalizedCloneText(string(data))
		if len(tokens) > 0 {
			docs = append(docs, cloneDocument{Path: file, Tokens: tokens})
		}
		return nil
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
	candidates := make([]cloneCandidate, 0)
	for _, occurrences := range index {
		candidates = appendClonePairs(candidates, occurrences, docs, threshold)
	}
	return candidates
}

func appendClonePairs(candidates []cloneCandidate, occurrences []cloneOccurrence, docs []cloneDocument, threshold int) []cloneCandidate {
	if len(occurrences) < 2 {
		return candidates
	}
	for i := 0; i < len(occurrences); i++ {
		for j := i + 1; j < len(occurrences); j++ {
			if next, ok := cloneCandidateForPair(occurrences[i], occurrences[j], docs, threshold); ok {
				candidates = appendOrMergeCloneCandidate(candidates, next)
			}
		}
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
	sort.Slice(candidates, func(i, j int) bool {
		leftDoc := filepath.ToSlash(docs[candidates[i].LeftDoc].Path)
		rightDoc := filepath.ToSlash(docs[candidates[j].LeftDoc].Path)
		if leftDoc != rightDoc {
			return leftDoc < rightDoc
		}
		if candidates[i].LeftStart != candidates[j].LeftStart {
			return candidates[i].LeftStart < candidates[j].LeftStart
		}
		otherLeft := filepath.ToSlash(docs[candidates[i].RightDoc].Path)
		otherRight := filepath.ToSlash(docs[candidates[j].RightDoc].Path)
		if otherLeft != otherRight {
			return otherLeft < otherRight
		}
		return candidates[i].RightStart < candidates[j].RightStart
	})
}

func appendOrMergeCloneCandidate(candidates []cloneCandidate, next cloneCandidate) []cloneCandidate {
	if next.LeftDoc > next.RightDoc {
		next.LeftDoc, next.RightDoc = next.RightDoc, next.LeftDoc
		next.LeftStart, next.RightStart = next.RightStart, next.LeftStart
	}
	for idx, existing := range candidates {
		if existing.LeftDoc != next.LeftDoc || existing.RightDoc != next.RightDoc {
			continue
		}
		if cloneRangesOverlap(existing.LeftStart, existing.Length, next.LeftStart, next.Length) &&
			cloneRangesOverlap(existing.RightStart, existing.Length, next.RightStart, next.Length) {
			if next.Length > existing.Length {
				candidates[idx] = next
			}
			return candidates
		}
	}
	return append(candidates, next)
}
