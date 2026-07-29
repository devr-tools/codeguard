package change

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	goPublicSigPattern      = regexp.MustCompile(`(?m)^\s*(?:func\s+(?:\([^)]*\)\s*)?|type\s+|var\s+|const\s+)([A-Z][A-Za-z0-9_]*)[^{\n]*`)
	goPrivateSigPattern     = regexp.MustCompile(`(?m)^\s*(?:func\s+(?:\([^)]*\)\s*)?|type\s+|var\s+|const\s+)([a-z_][A-Za-z0-9_]*)[^{\n]*`)
	tsPublicSigPattern      = regexp.MustCompile(`(?m)^\s*export\s+(?:async\s+)?(?:function|class|interface|type|const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)[^{;=\n]*`)
	tsPrivateSigPattern     = regexp.MustCompile(`(?m)^\s*(?:async\s+)?(?:function|class|interface|type|const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)[^{;=\n]*`)
	pythonPublicSigPattern  = regexp.MustCompile(`(?m)^\s*(?:def|class)\s+([A-Za-z][A-Za-z0-9_]*)\s*[(:]`)
	pythonPrivateSigPattern = regexp.MustCompile(`(?m)^\s*(?:def|class)\s+(_[A-Za-z0-9_]+|[a-z][A-Za-z0-9_]*)\s*[(:]`)
	cppPublicSigPattern     = regexp.MustCompile(`(?m)^\s*(?:template\s*<[^>]+>\s*)?(?:class|struct|enum|using|typedef|[A-Za-z_:<>~*&\s]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\([^;{}]*\)|[:;{=])`)
	importPattern           = regexp.MustCompile(`(?m)^\s*(?:import\s+(?:[^'"\n]+from\s+)?["']([^"']+)["']|(?:from|import)\s+([A-Za-z0-9_./-]+)|#include\s+[<"]([^>"]+)[>"]|import\s+(?:\([^)]*?"([^"]+)"[^)]*?\)|"([^"]+)"))`)
)

type refactorFilePair struct {
	beforePath string
	afterPath  string
	status     core.ChangedFileStatus
	base       []byte
	after      []byte
	ranges     core.ChangedLineRanges
	moved      bool
}

type refactorFindingEvidence struct {
	ruleID     string
	level      string
	confidence string
	path       string
	line       int
	message    string
	metadata   map[string]string
}

func refactorFindings(ctx context.Context, env support.Context) []core.Finding {
	if env.Mode != core.ScanModeDiff || env.ListChangedFiles == nil || env.ReadTargetFile == nil || env.ReadBaseFile == nil {
		return nil
	}
	rules := env.Config.Checks.ChangeRules
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		select {
		case <-ctx.Done():
			return findings
		default:
		}
		pairs := refactorPairs(env, target)
		for _, pair := range pairs {
			if !isSourceFile(pair.afterPath) && !isSourceFile(pair.beforePath) {
				continue
			}
			for _, evidence := range refactorPairEvidence(rules, pair) {
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID:     evidence.ruleID,
					Level:      evidence.level,
					Path:       evidence.path,
					Line:       evidence.line,
					Message:    evidence.message,
					Confidence: evidence.confidence,
					Metadata:   evidence.metadata,
				}))
			}
		}
		if enabled(rules.DetectRefactorDuplicateLeftBehind) {
			findings = append(findings, duplicateImplementationFindings(env, target, pairs)...)
		}
	}
	return findings
}

func refactorPairs(env support.Context, target core.TargetConfig) []refactorFilePair {
	changed, err := env.ListChangedFiles(target)
	if err != nil {
		return nil
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Path < changed[j].Path })
	scope := map[string]core.ChangedLineRanges{}
	if env.DiffScope != nil {
		scope = env.DiffScope()
	}
	moves := detectMovePairs(changedFilesFromCore(changed))
	movedTo := map[string]movePair{}
	for _, pair := range moves {
		movedTo[pair.to] = pair
	}
	out := make([]refactorFilePair, 0, len(changed))
	for _, file := range changed {
		afterPath := normalizePath(file.Path)
		if afterPath == "" || file.Status == core.ChangedFileDeleted || !isSourceFile(afterPath) ||
			isDocsOnlyFile(afterPath) || isConfigOnlyFile(afterPath) || isGeneratedOrVendorFile(afterPath) {
			continue
		}
		beforePath := afterPath
		moved := false
		if pair, ok := movedTo[afterPath]; ok {
			beforePath = pair.from
			moved = true
		}
		after, err := env.ReadTargetFile(target, afterPath)
		if err != nil {
			continue
		}
		base, err := env.ReadBaseFile(target, beforePath)
		if err != nil {
			continue
		}
		out = append(out, refactorFilePair{
			beforePath: beforePath,
			afterPath:  afterPath,
			status:     file.Status,
			base:       base,
			after:      after,
			ranges:     scope[afterPath],
			moved:      moved,
		})
	}
	return out
}

func changedFilesFromCore(files []core.ChangedFile) []changedFile {
	out := make([]changedFile, 0, len(files))
	for _, file := range files {
		out = append(out, changedFile{path: normalizePath(file.Path), status: file.Status})
	}
	return out
}

func refactorPairEvidence(rules core.ChangeRulesConfig, pair refactorFilePair) []refactorFindingEvidence {
	baseText := string(pair.base)
	afterText := string(pair.after)
	production := isProductionFile(pair.afterPath)
	refactorLike := pair.moved || fileLooksRefactored(pair.beforePath, baseText, afterText)
	out := make([]refactorFindingEvidence, 0, 8)

	if production && enabled(rules.DetectRefactorPublicContract) {
		if evidence, ok := publicContractEvidence(pair, baseText, afterText); ok {
			out = append(out, evidence)
		}
	}
	if production && enabled(rules.DetectRefactorVisibilityExpand) {
		if evidence, ok := visibilityExpandedEvidence(pair, baseText, afterText); ok {
			out = append(out, evidence)
		}
	}
	if enabled(rules.DetectRefactorTestCoverageDrop) && isTestFile(pair.afterPath) {
		if evidence, ok := testCoverageReducedEvidence(pair, baseText, afterText); ok {
			out = append(out, evidence)
		}
	}
	if production && enabled(rules.DetectRefactorDependencyWorsened) {
		if evidence, ok := dependencyDirectionEvidence(pair, baseText, afterText); ok {
			out = append(out, evidence)
		}
	}
	if production && enabled(rules.DetectRefactorDeadPathLeftBehind) {
		if evidence, ok := deadPathEvidence(pair); ok {
			out = append(out, evidence)
		}
	}
	if !production || !refactorLike {
		return out
	}
	if enabled(rules.DetectRefactorBehaviorChange) {
		if evidence, ok := behaviorChangedEvidence(pair, baseText, afterText); ok {
			out = append(out, evidence)
		}
	}
	if enabled(rules.DetectRefactorErrorPathChange) {
		if evidence, ok := errorPathChangedEvidence(pair, baseText, afterText); ok {
			out = append(out, evidence)
		}
	}
	if enabled(rules.DetectRefactorSideEffectReorder) {
		if evidence, ok := sideEffectOrderEvidence(pair, baseText, afterText); ok {
			out = append(out, evidence)
		}
	}
	return out
}

func fileLooksRefactored(path string, baseText string, afterText string) bool {
	if strings.Contains(strings.ToLower(path), "refactor") {
		return true
	}
	basePublic := publicSignatures(path, baseText)
	afterPublic := publicSignatures(path, afterText)
	if len(basePublic) == 0 || signatureOverlap(basePublic, afterPublic) == 0 {
		return false
	}
	return signatureSetChanged(privateSignatures(path, baseText), privateSignatures(path, afterText))
}

func publicContractEvidence(pair refactorFilePair, baseText string, afterText string) (refactorFindingEvidence, bool) {
	before := publicSignatures(pair.beforePath, baseText)
	after := publicSignatures(pair.afterPath, afterText)
	if len(before) == 0 && len(after) == 0 {
		return refactorFindingEvidence{}, false
	}
	changed := signatureDiff(before, after)
	if len(changed) == 0 {
		return refactorFindingEvidence{}, false
	}
	return refactorFindingEvidence{
		ruleID:     "refactor.public-contract-changed",
		level:      "fail",
		confidence: "high",
		path:       pair.afterPath,
		line:       firstUsefulChangedLine(pair),
		message:    fmt.Sprintf("Public contract changed during refactor-sensitive diff: %s.", strings.Join(limitStrings(changed, 4), ", ")),
		metadata:   refactorMetadata(pair, "public-contract", len(changed)),
	}, true
}

func visibilityExpandedEvidence(pair refactorFilePair, baseText string, afterText string) (refactorFindingEvidence, bool) {
	beforePrivate := privateSignatureKeys(pair.beforePath, baseText)
	afterPublic := publicSignatures(pair.afterPath, afterText)
	expanded := make([]string, 0)
	for name := range afterPublic {
		if _, ok := beforePrivate[visibilityKey(name)]; ok {
			expanded = append(expanded, name)
		}
	}
	sort.Strings(expanded)
	if len(expanded) == 0 {
		return refactorFindingEvidence{}, false
	}
	return refactorFindingEvidence{
		ruleID:     "refactor.visibility-expanded",
		level:      "warn",
		confidence: "high",
		path:       pair.afterPath,
		line:       firstUsefulChangedLine(pair),
		message:    fmt.Sprintf("Visibility expanded for formerly private symbol(s): %s.", strings.Join(limitStrings(expanded, 4), ", ")),
		metadata:   refactorMetadata(pair, "visibility-expanded", len(expanded)),
	}, true
}

func testCoverageReducedEvidence(pair refactorFilePair, baseText string, afterText string) (refactorFindingEvidence, bool) {
	beforeTests := testEvidenceCount(baseText)
	afterTests := testEvidenceCount(afterText)
	if beforeTests == 0 || afterTests >= beforeTests {
		return refactorFindingEvidence{}, false
	}
	return refactorFindingEvidence{
		ruleID:     "refactor.test-coverage-reduced",
		level:      "warn",
		confidence: "high",
		path:       pair.afterPath,
		line:       firstUsefulChangedLine(pair),
		message:    fmt.Sprintf("Changed test file has fewer test/assertion markers after refactor (%d -> %d).", beforeTests, afterTests),
		metadata:   refactorMetadata(pair, "test-coverage-reduced", beforeTests-afterTests),
	}, true
}

func behaviorChangedEvidence(pair refactorFilePair, baseText string, afterText string) (refactorFindingEvidence, bool) {
	before := behaviorFingerprint(baseText)
	after := behaviorFingerprint(afterText)
	if strings.Join(before, "|") == strings.Join(after, "|") {
		return refactorFindingEvidence{}, false
	}
	return refactorFindingEvidence{
		ruleID:     "refactor.behavior-change-detected",
		level:      "fail",
		confidence: "high",
		path:       pair.afterPath,
		line:       firstUsefulChangedLine(pair),
		message:    "Behavior-preservation evidence changed in a refactor-shaped diff: return paths, branches, calls, or mutations differ.",
		metadata:   refactorMetadata(pair, "behavior-fingerprint-changed", len(before)+len(after)),
	}, true
}

func errorPathChangedEvidence(pair refactorFilePair, baseText string, afterText string) (refactorFindingEvidence, bool) {
	before := errorFingerprint(baseText)
	after := errorFingerprint(afterText)
	if (len(before) == 0 && len(after) == 0) || strings.Join(before, "|") == strings.Join(after, "|") {
		return refactorFindingEvidence{}, false
	}
	return refactorFindingEvidence{
		ruleID:     "refactor.error-path-changed",
		level:      "fail",
		confidence: "high",
		path:       pair.afterPath,
		line:       firstUsefulChangedLine(pair),
		message:    "Error-path evidence changed in a refactor-shaped diff: returned errors, thrown exceptions, wrapping, panic, retry, fallback, or rollback behavior differs.",
		metadata:   refactorMetadata(pair, "error-path-changed", len(before)+len(after)),
	}, true
}

func sideEffectOrderEvidence(pair refactorFilePair, baseText string, afterText string) (refactorFindingEvidence, bool) {
	before := sideEffectSequence(baseText)
	after := sideEffectSequence(afterText)
	if len(before) < 2 || len(before) != len(after) || strings.Join(before, "|") == strings.Join(after, "|") || strings.Join(sortedCopy(before), "|") != strings.Join(sortedCopy(after), "|") {
		return refactorFindingEvidence{}, false
	}
	return refactorFindingEvidence{
		ruleID:     "refactor.side-effect-order-changed",
		level:      "fail",
		confidence: "high",
		path:       pair.afterPath,
		line:       firstUsefulChangedLine(pair),
		message:    fmt.Sprintf("Side-effect order changed in a refactor-shaped diff (%s -> %s).", strings.Join(before, " then "), strings.Join(after, " then ")),
		metadata:   refactorMetadata(pair, "side-effect-order-changed", len(before)),
	}, true
}

func dependencyDirectionEvidence(pair refactorFilePair, baseText string, afterText string) (refactorFindingEvidence, bool) {
	layer := layerCategory(pair.afterPath)
	baseImports := stringSet(importsFor(baseText))
	afterImports := importsFor(afterText)
	worse := make([]string, 0)
	for _, imp := range afterImports {
		if _, existed := baseImports[imp]; existed {
			continue
		}
		if dependencyWorsensLayer(layer, imp) {
			worse = append(worse, imp)
		}
	}
	if len(worse) == 0 {
		return refactorFindingEvidence{}, false
	}
	sort.Strings(worse)
	return refactorFindingEvidence{
		ruleID:     "refactor.dependency-direction-worsened",
		level:      "warn",
		confidence: "high",
		path:       pair.afterPath,
		line:       firstUsefulChangedLine(pair),
		message:    fmt.Sprintf("Refactor introduced dependency direction from %s code toward outer infrastructure/framework dependency: %s.", layer, strings.Join(limitStrings(worse, 4), ", ")),
		metadata:   refactorMetadata(pair, "dependency-direction-worsened", len(worse)),
	}, true
}

func deadPathEvidence(pair refactorFilePair) (refactorFindingEvidence, bool) {
	lines := strings.Split(string(pair.after), "\n")
	for idx, raw := range lines {
		lineNo := idx + 1
		if !pair.ranges.AllChanged && len(pair.ranges.Ranges) > 0 && !pair.ranges.Contains(lineNo) {
			continue
		}
		line := strings.ToLower(maskLineComments(strings.TrimSpace(raw)))
		if line == "" {
			continue
		}
		if strings.Contains(line, "todo") && strings.Contains(line, "remove") ||
			strings.Contains(line, "deprecated") ||
			strings.Contains(line, "obsolete") ||
			strings.Contains(line, "compatibility path") ||
			strings.Contains(line, "legacy path") ||
			strings.Contains(line, "remove after") ||
			strings.Contains(line, "if false") ||
			strings.Contains(line, "if (false") {
			return refactorFindingEvidence{
				ruleID:     "refactor.dead-path-left-behind",
				level:      "warn",
				confidence: "medium",
				path:       pair.afterPath,
				line:       lineNo,
				message:    "Refactor leaves an explicit obsolete, deprecated, disabled, or TODO-remove path behind.",
				metadata:   refactorMetadata(pair, "dead-path-marker", 1),
			}, true
		}
	}
	return refactorFindingEvidence{}, false
}

func duplicateImplementationFindings(env support.Context, target core.TargetConfig, _ []refactorFilePair) []core.Finding {
	if env.ListTargetFiles == nil || env.ReadTargetFile == nil {
		return nil
	}
	changedFiles, err := env.ListChangedFiles(target)
	if err != nil {
		return nil
	}
	changed := map[string]struct{}{}
	for _, file := range changedFiles {
		rel := normalizePath(file.Path)
		if rel != "" && file.Status != core.ChangedFileDeleted {
			changed[rel] = struct{}{}
		}
	}
	allFiles, err := env.ListTargetFiles(target)
	if err != nil {
		return nil
	}
	bodies := map[string][]codeBlock{}
	for _, rel := range allFiles {
		rel = normalizePath(rel)
		if !isProductionFile(rel) || !isSourceFile(rel) {
			continue
		}
		data, err := env.ReadTargetFile(target, rel)
		if err != nil {
			continue
		}
		for _, block := range extractCodeBlocks(rel, string(data)) {
			if block.key != "" {
				bodies[block.key] = append(bodies[block.key], block)
			}
		}
	}
	out := make([]core.Finding, 0)
	for _, matches := range bodies {
		if len(matches) < 2 {
			continue
		}
		hasChanged := false
		for _, match := range matches {
			if _, ok := changed[match.path]; ok {
				hasChanged = true
			}
		}
		if !hasChanged {
			continue
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].path == matches[j].path {
				return matches[i].line < matches[j].line
			}
			return matches[i].path < matches[j].path
		})
		paths := make([]string, 0, len(matches))
		for _, match := range matches {
			paths = append(paths, fmt.Sprintf("%s:%d", match.path, match.line))
		}
		out = append(out, env.NewFinding(support.FindingInput{
			RuleID:     "refactor.duplicate-implementation-left-behind",
			Level:      "warn",
			Path:       matches[0].path,
			Line:       matches[0].line,
			Confidence: "high",
			Message:    fmt.Sprintf("Refactor leaves duplicate implementation bodies active at %s.", strings.Join(limitStrings(paths, 4), ", ")),
			Metadata: map[string]string{
				"evidence":        "duplicate-implementation-body",
				"duplicate_count": strconv.Itoa(len(matches)),
			},
		}))
		break
	}
	return out
}

type codeBlock struct {
	path string
	line int
	key  string
}

func extractCodeBlocks(path string, text string) []codeBlock {
	lines := strings.Split(text, "\n")
	out := make([]codeBlock, 0)
	for idx, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if !looksLikeFunctionStart(path, trimmed) {
			continue
		}
		body := collectBlock(lines[idx:])
		key := normalizedImplementation(body)
		if len(key) >= 45 {
			out = append(out, codeBlock{path: path, line: idx + 1, key: key})
		}
	}
	return out
}

func looksLikeFunctionStart(path string, line string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return strings.HasPrefix(line, "func ") && strings.Contains(line, "{")
	case ".py":
		return strings.HasPrefix(line, "def ") || strings.HasPrefix(line, "async def ")
	default:
		return strings.Contains(line, "(") && strings.Contains(line, "{") && !strings.HasPrefix(line, "if ") && !strings.HasPrefix(line, "for ") && !strings.HasPrefix(line, "while ") && !strings.HasPrefix(line, "switch ")
	}
}

func collectBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	braceDepth := 0
	seenBrace := false
	for idx, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
		braceDepth += strings.Count(line, "{")
		braceDepth -= strings.Count(line, "}")
		if strings.Contains(line, "{") {
			seenBrace = true
		}
		if seenBrace && idx > 0 && braceDepth <= 0 {
			break
		}
		if !seenBrace && idx > 0 && strings.TrimSpace(line) == "" {
			break
		}
		if idx >= 80 {
			break
		}
	}
	return b.String()
}

func publicSignatures(path string, text string) map[string]string {
	return signatureMap(path, text, true)
}

func privateSignatures(path string, text string) map[string]string {
	return signatureMap(path, text, false)
}

func privateSignatureKeys(path string, text string) map[string]struct{} {
	out := map[string]struct{}{}
	for name := range privateSignatures(path, text) {
		out[visibilityKey(name)] = struct{}{}
	}
	return out
}

func signatureMap(path string, text string, public bool) map[string]string {
	patterns := signaturePatterns(path, public)
	out := map[string]string{}
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			name := strings.TrimSpace(match[1])
			if name == "" {
				continue
			}
			if !public && isPublicNameForPath(path, name) {
				continue
			}
			out[name] = normalizeSignature(match[0])
		}
	}
	return out
}

func signaturePatterns(path string, public bool) []*regexp.Regexp {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		if public {
			return []*regexp.Regexp{goPublicSigPattern}
		}
		return []*regexp.Regexp{goPrivateSigPattern}
	case ".py":
		if public {
			return []*regexp.Regexp{pythonPublicSigPattern}
		}
		return []*regexp.Regexp{pythonPrivateSigPattern}
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		if public {
			return []*regexp.Regexp{tsPublicSigPattern}
		}
		return []*regexp.Regexp{tsPrivateSigPattern}
	default:
		return []*regexp.Regexp{cppPublicSigPattern}
	}
}

func isPublicNameForPath(path string, name string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".go" {
		r := []rune(name)
		return len(r) > 0 && r[0] >= 'A' && r[0] <= 'Z'
	}
	return !strings.HasPrefix(name, "_")
}

func normalizeSignature(sig string) string {
	sig = maskLineComments(sig)
	sig = strings.TrimSpace(sig)
	return strings.Join(strings.Fields(sig), " ")
}

func signatureDiff(before map[string]string, after map[string]string) []string {
	diff := make([]string, 0)
	for name, beforeSig := range before {
		afterSig, ok := after[name]
		if !ok {
			diff = append(diff, "removed "+name)
			continue
		}
		if beforeSig != afterSig {
			diff = append(diff, "changed "+name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			diff = append(diff, "added "+name)
		}
	}
	sort.Strings(diff)
	return diff
}

func signatureOverlap(a map[string]string, b map[string]string) int {
	count := 0
	for name := range a {
		if _, ok := b[name]; ok {
			count++
		}
	}
	return count
}

func signatureSetChanged(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return true
	}
	for name, sig := range a {
		if b[name] != sig {
			return true
		}
	}
	return false
}

func visibilityKey(name string) string {
	name = strings.TrimLeft(name, "_")
	if name == "" {
		return ""
	}
	return strings.ToLower(name[:1]) + name[1:]
}

func behaviorFingerprint(text string) []string {
	return fingerprint(text, func(line string) string {
		switch {
		case strings.Contains(line, "return"):
			return "return:" + compactLine(line)
		case strings.Contains(line, "if ") || strings.Contains(line, "if(") || strings.Contains(line, "switch") || strings.Contains(line, "case ") || strings.Contains(line, "for ") || strings.Contains(line, "while "):
			return "branch:" + compactLine(line)
		case strings.Contains(line, "=") || strings.Contains(line, ":=") || strings.Contains(line, "+=") || strings.Contains(line, "-="):
			return "mutation:" + compactLine(line)
		default:
			if effect := sideEffectKind(line); effect != "" {
				return "effect:" + effect
			}
		}
		return ""
	})
}

func errorFingerprint(text string) []string {
	return fingerprint(text, func(line string) string {
		switch {
		case strings.Contains(line, "return") && (strings.Contains(line, "err") || strings.Contains(line, "error")):
			return "return-error:" + compactLine(line)
		case strings.Contains(line, "fmt.errorf") || strings.Contains(line, "errors.new") || strings.Contains(line, "new error") || strings.Contains(line, "throw") || strings.Contains(line, "raise"):
			return "new-error:" + compactLine(line)
		case strings.Contains(line, "catch") || strings.Contains(line, "except"):
			return "catch:" + compactLine(line)
		case strings.Contains(line, "panic") || strings.Contains(line, "retry") || strings.Contains(line, "fallback") || strings.Contains(line, "rollback"):
			return "failure-action:" + compactLine(line)
		default:
			return ""
		}
	})
}

func sideEffectSequence(text string) []string {
	return fingerprint(text, sideEffectKind)
}

func fingerprint(text string, classify func(string) string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0)
	for _, raw := range lines {
		line := strings.ToLower(maskLineComments(strings.TrimSpace(raw)))
		if line == "" || isCommentOnly(line) {
			continue
		}
		if item := classify(line); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func sideEffectKind(line string) string {
	switch {
	case strings.Contains(line, "authorize") || strings.Contains(line, "permission") || strings.Contains(line, "auth."):
		return "auth"
	case strings.Contains(line, ".save") || strings.Contains(line, ".insert") || strings.Contains(line, ".update") || strings.Contains(line, ".delete") || strings.Contains(line, ".exec") || strings.Contains(line, ".query") || strings.Contains(line, "sql."):
		return "write"
	case strings.Contains(line, "publish") || strings.Contains(line, "emit") || strings.Contains(line, ".send") || strings.Contains(line, "enqueue"):
		return "event"
	case strings.Contains(line, "http.") || strings.Contains(line, "fetch(") || strings.Contains(line, "requests.") || strings.Contains(line, "axios."):
		return "network"
	case strings.Contains(line, "os.") || strings.Contains(line, "fs.") || strings.Contains(line, "std::filesystem") || strings.Contains(line, "open("):
		return "filesystem"
	case strings.Contains(line, "defer ") || strings.Contains(line, "finally") || strings.Contains(line, "close()"):
		return "cleanup"
	default:
		return ""
	}
}

func importsFor(text string) []string {
	out := make([]string, 0)
	for _, match := range importPattern.FindAllStringSubmatch(text, -1) {
		for _, item := range match[1:] {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, strings.ToLower(item))
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func dependencyWorsensLayer(layer string, imp string) bool {
	imp = strings.ToLower(imp)
	if strings.Contains(imp, "test") || strings.HasPrefix(imp, ".") {
		return false
	}
	outer := strings.Contains(imp, "infra") || strings.Contains(imp, "adapter") || strings.Contains(imp, "db") ||
		strings.Contains(imp, "sql") || strings.Contains(imp, "http") || strings.Contains(imp, "axios") ||
		strings.Contains(imp, "requests") || strings.Contains(imp, "react") || strings.Contains(imp, "express") ||
		strings.Contains(imp, "boto") || strings.Contains(imp, "aws") || strings.Contains(imp, "filesystem")
	switch layer {
	case "domain", "core":
		return outer
	case "application":
		return strings.Contains(imp, "ui") || strings.Contains(imp, "react") || strings.Contains(imp, "view")
	default:
		return false
	}
}

func testEvidenceCount(text string) int {
	count := 0
	for _, line := range strings.Split(strings.ToLower(text), "\n") {
		line = strings.TrimSpace(maskLineComments(line))
		if strings.Contains(line, "func test") || strings.Contains(line, "def test_") ||
			strings.Contains(line, "test(") || strings.Contains(line, "it(") ||
			strings.Contains(line, "assert") || strings.Contains(line, "expect(") ||
			strings.Contains(line, ".fatal") || strings.Contains(line, "require.") {
			count++
		}
	}
	return count
}

func normalizedImplementation(text string) string {
	lines := make([]string, 0)
	skippedSignature := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.ToLower(maskLineComments(strings.TrimSpace(raw)))
		if line == "" || isCommentOnly(line) {
			continue
		}
		if !skippedSignature {
			skippedSignature = true
			continue
		}
		lines = append(lines, compactLine(line))
	}
	return strings.Join(lines, ";")
}

func compactLine(line string) string {
	replacer := strings.NewReplacer(" ", "", "\t", "", ";", "", "{", "", "}", "")
	return replacer.Replace(line)
}

func stringSet(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func firstUsefulChangedLine(pair refactorFilePair) int {
	lines := strings.Split(string(pair.after), "\n")
	for idx, raw := range lines {
		lineNo := idx + 1
		if !pair.ranges.AllChanged && len(pair.ranges.Ranges) > 0 && !pair.ranges.Contains(lineNo) {
			continue
		}
		if strings.TrimSpace(raw) != "" {
			return lineNo
		}
	}
	return 1
}

func refactorMetadata(pair refactorFilePair, evidence string, count int) map[string]string {
	return map[string]string{
		"evidence":    evidence,
		"before_path": pair.beforePath,
		"after_path":  pair.afterPath,
		"count":       strconv.Itoa(count),
		"moved":       strconv.FormatBool(pair.moved),
	}
}
