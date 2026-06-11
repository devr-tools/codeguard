package quality

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var cloneTokenPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|\d+|==|!=|<=|>=|&&|\|\||[{}()[\].,:;+*/%<>=!-]`)

type cloneToken struct {
	Value string
	Line  int
}

type cloneDocument struct {
	Path   string
	Tokens []cloneToken
}

type cloneOccurrence struct {
	DocIndex   int
	TokenIndex int
}

type cloneCandidate struct {
	LeftDoc    int
	LeftStart  int
	RightDoc   int
	RightStart int
	Length     int
}

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
	env.ScanTargetFiles(target, "quality-clone", cloneIncludeForLanguage(target.Language), func(file string, data []byte) []core.Finding {
		tokens := tokenizeNormalizedCloneText(string(data))
		if len(tokens) > 0 {
			docs = append(docs, cloneDocument{Path: file, Tokens: tokens})
		}
		return nil
	})
	return docs
}

func cloneIncludeForLanguage(language string) func(string) bool {
	switch normalizedLanguage(language) {
	case "", "go":
		return func(rel string) bool { return strings.HasSuffix(strings.ToLower(rel), ".go") }
	case "python", "py":
		return func(rel string) bool { return strings.HasSuffix(strings.ToLower(rel), ".py") }
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return isTypeScriptLikeFile
	case "rust", "rs":
		return isRustFile
	case "java":
		return isJavaFile
	case "csharp", "c#", "cs", "dotnet":
		return isCSharpFile
	case "ruby", "rb":
		return isRubyFile
	default:
		return func(string) bool { return false }
	}
}

func tokenizeNormalizedCloneText(source string) []cloneToken {
	matches := cloneTokenPattern.FindAllStringIndex(source, -1)
	if len(matches) == 0 {
		return nil
	}
	tokens := make([]cloneToken, 0, len(matches))
	line := 1
	prev := 0
	for _, match := range matches {
		line += strings.Count(source[prev:match[0]], "\n")
		value := strings.ToLower(source[match[0]:match[1]])
		tokens = append(tokens, cloneToken{Value: value, Line: line})
		prev = match[1]
	}
	return tokens
}

func detectCloneCandidates(docs []cloneDocument, threshold int) []cloneCandidate {
	index := make(map[uint64][]cloneOccurrence)
	for docIdx, doc := range docs {
		for tokenIdx := 0; tokenIdx+threshold <= len(doc.Tokens); tokenIdx++ {
			hash := cloneWindowHash(doc.Tokens[tokenIdx : tokenIdx+threshold])
			index[hash] = append(index[hash], cloneOccurrence{DocIndex: docIdx, TokenIndex: tokenIdx})
		}
	}

	candidates := make([]cloneCandidate, 0)
	for _, occurrences := range index {
		if len(occurrences) < 2 {
			continue
		}
		for i := 0; i < len(occurrences); i++ {
			for j := i + 1; j < len(occurrences); j++ {
				left := occurrences[i]
				right := occurrences[j]
				if left.DocIndex == right.DocIndex {
					continue
				}
				length := sharedCloneLength(docs[left.DocIndex].Tokens, left.TokenIndex, docs[right.DocIndex].Tokens, right.TokenIndex)
				if length < threshold {
					continue
				}
				candidates = appendOrMergeCloneCandidate(candidates, cloneCandidate{
					LeftDoc:    left.DocIndex,
					LeftStart:  left.TokenIndex,
					RightDoc:   right.DocIndex,
					RightStart: right.TokenIndex,
					Length:     length,
				})
			}
		}
	}

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
	return candidates
}

func cloneWindowHash(tokens []cloneToken) uint64 {
	hasher := fnv.New64a()
	for _, token := range tokens {
		_, _ = hasher.Write([]byte(token.Value))
		_, _ = hasher.Write([]byte{0})
	}
	return hasher.Sum64()
}

func sharedCloneLength(left []cloneToken, leftStart int, right []cloneToken, rightStart int) int {
	length := 0
	for leftStart+length < len(left) && rightStart+length < len(right) {
		if left[leftStart+length].Value != right[rightStart+length].Value {
			break
		}
		length++
	}
	return length
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

func cloneRangesOverlap(startA int, lenA int, startB int, lenB int) bool {
	endA := startA + lenA
	endB := startB + lenB
	return startA < endB && startB < endA
}
