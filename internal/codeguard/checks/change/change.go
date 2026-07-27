// Package change implements diff-level change-safety checks.
package change

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	sectionID   = "change"
	sectionName = "Change Safety"
)

type evidence struct {
	files              []changedFile
	fileCount          int
	directories        []string
	layers             []string
	concerns           []string
	productionFiles    []string
	testFiles          []string
	verificationFiles  []string
	publicSurfaceFiles []string
	changedLines       int
	movePairs          []movePair
	behaviorFiles      []string
}

type changedFile struct {
	path   string
	status core.ChangedFileStatus
}

type movePair struct {
	from string
	to   string
}

// Run evaluates change-safety rules. The section is intentionally diff-first:
// full scans have no review unit to measure and therefore emit no findings.
func Run(_ context.Context, env support.Context) core.SectionResult {
	if env.Mode != core.ScanModeDiff {
		return env.FinalizeSection(sectionID, sectionName, nil)
	}
	return env.FinalizeSection(sectionID, sectionName, findings(env))
}

func findings(env support.Context) []core.Finding {
	ev := collectEvidence(env)
	if ev.fileCount == 0 {
		return nil
	}

	rules := env.Config.Checks.ChangeRules
	findings := make([]core.Finding, 0, 6)
	if enabled(rules.DetectOversizedDiff) {
		if finding, ok := oversizedDiffFinding(env, rules, ev); ok {
			findings = append(findings, finding)
		}
	}
	if enabled(rules.DetectMixedConcerns) {
		if finding, ok := mixedConcernsFinding(env, ev); ok {
			findings = append(findings, finding)
		}
	}
	if enabled(rules.DetectTooManyConcerns) {
		if finding, ok := tooManyConcernsFinding(env, rules, ev); ok {
			findings = append(findings, finding)
		}
	}
	if enabled(rules.DetectMixedRefactorAndBehavior) {
		if finding, ok := mixedRefactorAndBehaviorFinding(env, ev); ok {
			findings = append(findings, finding)
		}
	}
	if enabled(rules.DetectUnnecessarySurfaceArea) {
		if finding, ok := unnecessarySurfaceAreaFinding(env, rules, ev); ok {
			findings = append(findings, finding)
		}
	}
	if enabled(rules.DetectMoveWithoutVerification) {
		if finding, ok := moveWithoutVerificationFinding(env, ev); ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func collectEvidence(env support.Context) evidence {
	files := collectChangedFiles(env)
	ev := evidence{
		files:     files,
		fileCount: len(files),
	}
	dirSet := map[string]struct{}{}
	layerSet := map[string]struct{}{}
	concernSet := map[string]struct{}{}

	for _, file := range files {
		dir := directoryOf(file.path)
		dirSet[dir] = struct{}{}
		layerSet[layerCategory(file.path)] = struct{}{}
		concernSet[concernFamily(file.path)] = struct{}{}
		if isTestFile(file.path) {
			ev.testFiles = append(ev.testFiles, file.path)
		} else if isProductionFile(file.path) {
			ev.productionFiles = append(ev.productionFiles, file.path)
		}
		if isVerificationFile(file.path) {
			ev.verificationFiles = append(ev.verificationFiles, file.path)
		}
		if isPublicSurfaceFile(env, file.path) {
			ev.publicSurfaceFiles = append(ev.publicSurfaceFiles, file.path)
		}
	}

	ev.directories = sortedKeys(dirSet)
	ev.layers = sortedKeys(layerSet)
	ev.concerns = sortedKeys(concernSet)
	sort.Strings(ev.productionFiles)
	sort.Strings(ev.testFiles)
	sort.Strings(ev.verificationFiles)
	sort.Strings(ev.publicSurfaceFiles)
	ev.changedLines = changedLineCount(env)
	ev.movePairs = detectMovePairs(files)
	ev.behaviorFiles = detectBehaviorFiles(env, files)
	return ev
}

func collectChangedFiles(env support.Context) []changedFile {
	seen := map[string]core.ChangedFileStatus{}
	for _, target := range env.Config.Targets {
		if env.ListChangedFiles == nil {
			continue
		}
		changed, err := env.ListChangedFiles(target)
		if err != nil {
			continue
		}
		for _, file := range changed {
			rel := normalizePath(file.Path)
			if rel == "" {
				continue
			}
			seen[rel] = file.Status
		}
	}
	if len(seen) == 0 {
		for _, rel := range env.ChangedFiles {
			rel = normalizePath(rel)
			if rel != "" {
				seen[rel] = core.ChangedFileModified
			}
		}
	}

	out := make([]changedFile, 0, len(seen))
	for rel, status := range seen {
		out = append(out, changedFile{path: rel, status: status})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func changedLineCount(env support.Context) int {
	if env.DiffScope == nil {
		return 0
	}
	scope := env.DiffScope()
	total := 0
	for rel, ranges := range scope {
		if ranges.AllChanged {
			if lines := lineCountForChangedFile(env, rel); lines > 0 {
				total += lines
			}
			continue
		}
		for _, r := range ranges.Ranges {
			if r[1] >= r[0] {
				total += r[1] - r[0] + 1
			}
		}
	}
	return total
}

func lineCountForChangedFile(env support.Context, rel string) int {
	for _, target := range env.Config.Targets {
		if env.ReadTargetFile != nil {
			if data, err := env.ReadTargetFile(target, rel); err == nil {
				return env.CountLines(data)
			}
		}
		if env.ReadBaseFile != nil {
			if data, err := env.ReadBaseFile(target, rel); err == nil {
				return env.CountLines(data)
			}
		}
	}
	return 0
}

func oversizedDiffFinding(env support.Context, rules core.ChangeRulesConfig, ev evidence) (core.Finding, bool) {
	reasons := make([]string, 0, 5)
	if rules.MaxChangedFiles > 0 && ev.fileCount > rules.MaxChangedFiles {
		reasons = append(reasons, fmt.Sprintf("files touched %d > %d", ev.fileCount, rules.MaxChangedFiles))
	}
	if rules.MaxChangedDirectories > 0 && len(ev.directories) > rules.MaxChangedDirectories {
		reasons = append(reasons, fmt.Sprintf("directories touched %d > %d", len(ev.directories), rules.MaxChangedDirectories))
	}
	if rules.MaxChangedLines > 0 && ev.changedLines > rules.MaxChangedLines {
		reasons = append(reasons, fmt.Sprintf("changed lines %d > %d", ev.changedLines, rules.MaxChangedLines))
	}
	if rules.MaxPublicInterfacesChanged > 0 && len(ev.publicSurfaceFiles) > rules.MaxPublicInterfacesChanged {
		reasons = append(reasons, fmt.Sprintf("public-surface files %d > %d", len(ev.publicSurfaceFiles), rules.MaxPublicInterfacesChanged))
	}
	if poorTestRatio(rules, ev) {
		reasons = append(reasons, fmt.Sprintf("test-to-production file ratio %d%% < %d%%", testToProductionRatio(ev), rules.MinTestToProductionRatioPercent))
	}
	if len(reasons) == 0 {
		return core.Finding{}, false
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     "change.oversized-diff",
		Level:      "warn",
		Confidence: confidenceForReasonCount(len(reasons)),
		Message:    "change is difficult to review safely: " + strings.Join(reasons, "; "),
		Metadata:   evidenceMetadata(ev),
	}), true
}

func mixedConcernsFinding(env support.Context, ev evidence) (core.Finding, bool) {
	nonTestConcerns := nonTestConcernCount(ev)
	if nonTestConcerns < 2 || len(ev.layers) < 2 || len(ev.directories) < 2 {
		return core.Finding{}, false
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     "change.mixed-concerns",
		Level:      "warn",
		Confidence: "medium",
		Message: fmt.Sprintf(
			"change spans multiple concerns (%s) across layers (%s) and directories (%s)",
			strings.Join(ev.concerns, ", "),
			strings.Join(ev.layers, ", "),
			strings.Join(limitStrings(ev.directories, 6), ", "),
		),
		Metadata: evidenceMetadata(ev),
	}), true
}

func tooManyConcernsFinding(env support.Context, rules core.ChangeRulesConfig, ev evidence) (core.Finding, bool) {
	maxConcerns := rules.MaxConcernFamilies
	if maxConcerns <= 0 || len(ev.concerns) <= maxConcerns {
		return core.Finding{}, false
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     "change.too-many-concerns",
		Level:      "warn",
		Confidence: confidenceForOverage(len(ev.concerns), maxConcerns),
		Message:    fmt.Sprintf("change touches %d concern families (%s), above the configured limit of %d", len(ev.concerns), strings.Join(ev.concerns, ", "), maxConcerns),
		Metadata:   evidenceMetadata(ev),
	}), true
}

func mixedRefactorAndBehaviorFinding(env support.Context, ev evidence) (core.Finding, bool) {
	if len(ev.movePairs) == 0 || len(ev.behaviorFiles) == 0 {
		return core.Finding{}, false
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     "change.mixed-refactor-and-behavior",
		Level:      "warn",
		Confidence: "high",
		Message: fmt.Sprintf(
			"change combines file movement (%s) with behavior-bearing production edits (%s)",
			movePairSummary(ev.movePairs),
			strings.Join(limitStrings(ev.behaviorFiles, 4), ", "),
		),
		Metadata: evidenceMetadata(ev),
	}), true
}

func unnecessarySurfaceAreaFinding(env support.Context, rules core.ChangeRulesConfig, ev evidence) (core.Finding, bool) {
	maxPublic := rules.MaxPublicInterfacesChanged
	if maxPublic <= 0 || len(ev.publicSurfaceFiles) <= maxPublic {
		return core.Finding{}, false
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     "change.unnecessary-surface-area",
		Level:      "warn",
		Confidence: confidenceForOverage(len(ev.publicSurfaceFiles), maxPublic),
		Message:    fmt.Sprintf("change touches %d public-surface files (%s), above the configured limit of %d", len(ev.publicSurfaceFiles), strings.Join(limitStrings(ev.publicSurfaceFiles, 6), ", "), maxPublic),
		Metadata:   evidenceMetadata(ev),
	}), true
}

func moveWithoutVerificationFinding(env support.Context, ev evidence) (core.Finding, bool) {
	if len(ev.movePairs) == 0 || len(ev.verificationFiles) > 0 {
		return core.Finding{}, false
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     "change.move-without-verification",
		Level:      "warn",
		Confidence: "high",
		Message:    fmt.Sprintf("change moves production files (%s) without changed tests or verification files", movePairSummary(ev.movePairs)),
		Metadata:   evidenceMetadata(ev),
	}), true
}

func evidenceMetadata(ev evidence) map[string]string {
	return map[string]string{
		"files_touched":                    strconv.Itoa(ev.fileCount),
		"directories_touched":              strconv.Itoa(len(ev.directories)),
		"layers_touched":                   strings.Join(ev.layers, ","),
		"concern_families_touched":         strings.Join(ev.concerns, ","),
		"production_files_touched":         strconv.Itoa(len(ev.productionFiles)),
		"test_files_touched":               strconv.Itoa(len(ev.testFiles)),
		"test_to_production_ratio_percent": strconv.Itoa(testToProductionRatio(ev)),
		"public_surface_files_touched":     strconv.Itoa(len(ev.publicSurfaceFiles)),
		"changed_lines":                    strconv.Itoa(ev.changedLines),
		"move_pairs":                       strconv.Itoa(len(ev.movePairs)),
	}
}

func detectMovePairs(files []changedFile) []movePair {
	added := make([]string, 0)
	deleted := make([]string, 0)
	for _, file := range files {
		if !isProductionFile(file.path) {
			continue
		}
		switch file.status {
		case core.ChangedFileAdded:
			added = append(added, file.path)
		case core.ChangedFileDeleted:
			deleted = append(deleted, file.path)
		}
	}
	sort.Strings(added)
	sort.Strings(deleted)
	pairs := make([]movePair, 0)
	usedAdded := map[string]struct{}{}
	for _, from := range deleted {
		fromKey := moveKey(from)
		for _, to := range added {
			if _, used := usedAdded[to]; used {
				continue
			}
			if fromKey == moveKey(to) {
				pairs = append(pairs, movePair{from: from, to: to})
				usedAdded[to] = struct{}{}
				break
			}
		}
	}
	return pairs
}

func moveKey(rel string) string {
	return strings.ToLower(filepath.Base(rel))
}

func detectBehaviorFiles(env support.Context, files []changedFile) []string {
	seen := map[string]struct{}{}
	for _, file := range files {
		if file.status == core.ChangedFileDeleted || !isProductionFile(file.path) || !isSourceFile(file.path) {
			continue
		}
		if fileHasBehaviorChange(env, file.path) {
			seen[file.path] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func fileHasBehaviorChange(env support.Context, rel string) bool {
	if env.DiffScope == nil || env.ReadTargetFile == nil {
		return false
	}
	scope, ok := env.DiffScope()[rel]
	if !ok {
		return false
	}
	for _, target := range env.Config.Targets {
		data, err := env.ReadTargetFile(target, rel)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		if scope.AllChanged {
			return linesContainBehavior(lines)
		}
		for _, r := range scope.Ranges {
			start := max(1, r[0])
			end := min(len(lines), r[1])
			if start > end {
				continue
			}
			if linesContainBehavior(lines[start-1 : end]) {
				return true
			}
		}
	}
	return false
}

func linesContainBehavior(lines []string) bool {
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "//") || strings.HasPrefix(text, "#") || strings.HasPrefix(text, "*") {
			continue
		}
		lower := strings.ToLower(text)
		for _, token := range []string{
			"return ", "if ", "else", "switch ", "case ", "for ", "while ",
			"throw ", "panic(", "error", "err", "validate", "auth", "permission",
			"save", "update", "delete", "insert", "publish", "emit", "send",
			"http", "sql", "query", "exec", "time.", "rand.", "math.random",
		} {
			if strings.Contains(lower, token) {
				return true
			}
		}
	}
	return false
}

func poorTestRatio(rules core.ChangeRulesConfig, ev evidence) bool {
	if rules.MinTestToProductionRatioPercent <= 0 || len(ev.productionFiles) < 3 {
		return false
	}
	return testToProductionRatio(ev) < rules.MinTestToProductionRatioPercent
}

func testToProductionRatio(ev evidence) int {
	if len(ev.productionFiles) == 0 {
		return 100
	}
	return len(ev.testFiles) * 100 / len(ev.productionFiles)
}

func nonTestConcernCount(ev evidence) int {
	count := 0
	for _, concern := range ev.concerns {
		if concern != "tests" {
			count++
		}
	}
	return count
}

func isProductionFile(rel string) bool {
	return !isTestFile(rel) && !isDocsOnlyFile(rel) && !isGeneratedOrVendorFile(rel) && !isConfigOnlyFile(rel)
}

func isSourceFile(rel string) bool {
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".java", ".kt", ".rs", ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".hh":
		return true
	default:
		return false
	}
}

func isTestFile(rel string) bool {
	lower := strings.ToLower(normalizePath(rel))
	base := path.Base(lower)
	return strings.Contains(lower, "/test/") ||
		strings.Contains(lower, "/tests/") ||
		strings.Contains(lower, "/testdata/") ||
		strings.Contains(lower, "/fixtures/") ||
		strings.Contains(lower, "/__tests__/") ||
		strings.Contains(lower, "/__fixtures__/") ||
		strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.tsx") ||
		strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".test.jsx") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.tsx") ||
		strings.HasSuffix(base, ".spec.js") ||
		strings.HasSuffix(base, ".spec.jsx")
}

func isVerificationFile(rel string) bool {
	lower := strings.ToLower(normalizePath(rel))
	base := path.Base(lower)
	return isTestFile(rel) ||
		strings.Contains(lower, ".github/workflows/") ||
		strings.Contains(lower, ".buildkite/") ||
		strings.Contains(lower, "/ci/") ||
		base == "makefile" ||
		base == "justfile" ||
		base == "taskfile.yml" ||
		base == "taskfile.yaml" ||
		strings.HasPrefix(base, "dockerfile") ||
		strings.HasSuffix(base, ".bats")
}

func isDocsOnlyFile(rel string) bool {
	lower := strings.ToLower(normalizePath(rel))
	ext := strings.ToLower(filepath.Ext(lower))
	return strings.HasPrefix(lower, "docs/") ||
		strings.HasPrefix(lower, ".claude/") ||
		strings.HasPrefix(lower, ".github/") && (ext == ".md" || ext == ".txt") ||
		ext == ".md" || ext == ".mdx" || ext == ".rst" || ext == ".adoc" || ext == ".txt"
}

func isConfigOnlyFile(rel string) bool {
	lower := strings.ToLower(normalizePath(rel))
	base := path.Base(lower)
	ext := strings.ToLower(filepath.Ext(lower))
	if base == "go.mod" || base == "go.sum" || base == "package.json" || base == "package-lock.json" ||
		base == "pnpm-lock.yaml" || base == "yarn.lock" || base == "cargo.toml" || base == "cargo.lock" ||
		base == "requirements.txt" || base == "poetry.lock" || base == "pyproject.toml" {
		return true
	}
	return strings.HasPrefix(lower, ".github/") ||
		strings.HasPrefix(lower, ".buildkite/") ||
		strings.HasPrefix(lower, "ci/") ||
		ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".toml"
}

func isGeneratedOrVendorFile(rel string) bool {
	lower := strings.ToLower(normalizePath(rel))
	base := path.Base(lower)
	return strings.Contains(lower, "/vendor/") ||
		strings.Contains(lower, "/node_modules/") ||
		strings.Contains(lower, "/dist/") ||
		strings.Contains(lower, "/build/") ||
		strings.HasSuffix(base, ".pb.go") ||
		strings.HasSuffix(base, ".generated.go") ||
		strings.HasSuffix(base, ".gen.go") ||
		strings.Contains(base, ".generated.")
}

func isPublicSurfaceFile(env support.Context, rel string) bool {
	lower := strings.ToLower(normalizePath(rel))
	base := path.Base(lower)
	ext := strings.ToLower(filepath.Ext(lower))
	if env.IsSDKFacadeFile != nil && env.IsSDKFacadeFile(rel) {
		return true
	}
	if env.IsPublicPackageFile != nil && env.IsPublicPackageFile(rel) {
		return true
	}
	return hasPathSegment(lower, "api") ||
		hasPathSegment(lower, "apis") ||
		hasPathSegment(lower, "public") ||
		hasPathSegment(lower, "pkg") ||
		hasPathSegment(lower, "sdk") ||
		hasPathSegment(lower, "include") ||
		hasPathSegment(lower, "proto") ||
		hasPathSegment(lower, "graphql") ||
		strings.Contains(base, "openapi") ||
		strings.Contains(base, "swagger") ||
		strings.HasSuffix(base, ".proto") ||
		ext == ".graphql" || ext == ".gql" ||
		base == "index.ts" || base == "index.tsx" || base == "index.js" || base == "index.jsx"
}

func layerCategory(rel string) string {
	lower := strings.ToLower(normalizePath(rel))
	switch {
	case isTestFile(lower):
		return "tests"
	case strings.HasPrefix(lower, ".github/") || strings.HasPrefix(lower, ".buildkite/") || strings.HasPrefix(lower, "ci/"):
		return "delivery"
	case strings.HasPrefix(lower, "docs/") || strings.HasSuffix(lower, ".md"):
		return "docs"
	case hasPathSegment(lower, "ui") || hasPathSegment(lower, "view") || strings.Contains(lower, "/component") || hasPathSegment(lower, "frontend"):
		return "presentation"
	case hasPathSegment(lower, "api") || strings.Contains(lower, "/handler") || strings.Contains(lower, "/controller") || strings.Contains(lower, "/route"):
		return "application"
	case hasPathSegment(lower, "domain") || strings.Contains(lower, "/model/") || strings.Contains(lower, "/service/"):
		return "domain"
	case hasPathSegment(lower, "db") || hasPathSegment(lower, "data") || strings.Contains(lower, "/store/") || strings.Contains(lower, "/repo"):
		return "data"
	case hasPathSegment(lower, "infra") || hasPathSegment(lower, "platform") || strings.Contains(lower, "/adapter/"):
		return "infrastructure"
	default:
		return "core"
	}
}

func concernFamily(rel string) string {
	lower := strings.ToLower(normalizePath(rel))
	base := path.Base(lower)
	ext := strings.ToLower(filepath.Ext(lower))
	switch {
	case isTestFile(lower):
		return "tests"
	case strings.HasPrefix(lower, ".github/") || strings.HasPrefix(lower, ".buildkite/") || strings.HasPrefix(lower, "ci/"):
		return "ci"
	case strings.HasPrefix(lower, "docs/") || ext == ".md" || ext == ".mdx" || ext == ".rst":
		return "docs"
	case base == "go.mod" || base == "go.sum" || base == "package.json" || strings.Contains(base, "lock"):
		return "dependencies"
	case ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".toml":
		return "config"
	case hasPathSegment(lower, "auth") || strings.Contains(lower, "auth") || strings.Contains(lower, "permission"):
		return "auth"
	case hasPathSegment(lower, "billing") || strings.Contains(lower, "payment") || strings.Contains(lower, "invoice"):
		return "billing"
	case hasPathSegment(lower, "api") || strings.Contains(base, "openapi") || strings.Contains(base, "swagger") || ext == ".proto" || ext == ".graphql" || ext == ".gql":
		return "api"
	case hasPathSegment(lower, "ui") || hasPathSegment(lower, "frontend") || strings.Contains(lower, "/component") || hasPathSegment(lower, "view"):
		return "ui"
	case hasPathSegment(lower, "db") || hasPathSegment(lower, "data") || strings.Contains(lower, "/store/") || strings.Contains(lower, "/repo") || strings.Contains(lower, "migration"):
		return "data"
	case hasPathSegment(lower, "infra") || hasPathSegment(lower, "platform") || hasPathSegment(lower, "deploy") || strings.Contains(lower, "docker"):
		return "infra"
	default:
		layer := layerCategory(lower)
		if layer != "core" {
			return layer
		}
		return firstPathSegment(lower)
	}
}

func directoryOf(rel string) string {
	dir := path.Dir(normalizePath(rel))
	if dir == "." || dir == "" {
		return "."
	}
	return dir
}

func firstPathSegment(rel string) string {
	rel = strings.Trim(normalizePath(rel), "/")
	if rel == "" {
		return "."
	}
	if idx := strings.Index(rel, "/"); idx >= 0 {
		return rel[:idx]
	}
	return "."
}

func hasPathSegment(rel string, segment string) bool {
	for _, part := range strings.Split(strings.Trim(normalizePath(rel), "/"), "/") {
		if part == segment {
			return true
		}
	}
	return false
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func normalizePath(rel string) string {
	return strings.Trim(filepath.ToSlash(strings.TrimSpace(rel)), "/")
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return append([]string(nil), values...)
	}
	out := append([]string(nil), values[:limit]...)
	out = append(out, fmt.Sprintf("+%d more", len(values)-limit))
	return out
}

func movePairSummary(pairs []movePair) string {
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, pair.from+" -> "+pair.to)
	}
	return strings.Join(limitStrings(parts, 3), ", ")
}

func confidenceForReasonCount(count int) string {
	if count >= 2 {
		return "high"
	}
	return "medium"
}

func confidenceForOverage(value int, limit int) string {
	if limit > 0 && value >= limit*2 {
		return "high"
	}
	return "medium"
}

func enabled(toggle *bool) bool {
	return toggle == nil || *toggle
}
