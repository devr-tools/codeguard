// Package change implements diff-aware change-safety and testability checks.
package change

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	behaviorEvidencePattern = regexp.MustCompile(`\b(return|if|else|switch|case|throw|raise|panic|except|catch|for|while|await|yield|emit|publish|send|save|insert|update|delete|authorize|permission|status|fallback|retry)\b|=>|:=|\+=|-=|=`)
	failurePathPattern      = regexp.MustCompile(`\b(error|err|fail|failure|exception|except|catch|throw|raise|panic|retry|fallback|timeout|unauthorized|forbidden|denied|invalid|rollback|partial|circuit|backoff)\b`)
	failureTestPattern      = regexp.MustCompile(`\b(error|err|fail|failure|exception|except|throw|raise|reject|timeout|unauthorized|forbidden|denied|fallback|retry|rollback|mock|stub|fake)\b`)
	hardwiredPattern        = regexp.MustCompile(`\b(http\.defaultclient|http\.(get|post|do)|httpx\.client|requests\.(get|post|put|delete)|axios\.|fetch\(|sql\.open|boto3\.client|new\s+[a-z0-9_]*client|new[a-z0-9_]*client\(|os\.(open|create|readfile|writefile)|open\(|fs\.(readfilesync|writefilesync)|std::(ifstream|ofstream|filesystem)|exec\.command|subprocess\.(run|popen)|process\.env|std::getenv)`)
	nondeterministicPattern = regexp.MustCompile(`\b(time\.now|date\.now|new\s+date\(|math\.random|rand\.|random\.|uuid\.|datetime\.(now|today)|time\.time|os\.getenv|process\.env|std::chrono::system_clock::now|std::random_device|std::getenv|getenv\()`)
)

type testabilityEvidence struct {
	path          string
	line          int
	ruleID        string
	level         string
	confidence    string
	evidenceKind  string
	messageDetail string
}

func testabilityFindings(ctx context.Context, env support.Context) []core.Finding {
	if env.Mode != core.ScanModeDiff || env.ListChangedFiles == nil {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		findings = append(findings, testabilityTargetFindings(ctx, env, target)...)
	}
	return findings
}

func testabilityTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	changed, err := env.ListChangedFiles(target)
	if err != nil || len(changed) == 0 {
		return nil
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Path < changed[j].Path })

	diffScope := map[string]core.ChangedLineRanges{}
	if env.DiffScope != nil {
		diffScope = env.DiffScope()
	}

	testFiles, testHasFailureEvidence := changedTestEvidence(ctx, env, target, changed)
	hasChangedTests := len(testFiles) > 0

	findings := make([]core.Finding, 0)
	for _, file := range changed {
		path := filepath.ToSlash(file.Path)
		if file.Status == core.ChangedFileDeleted || isTestPath(path) || !isSupportedProductionPath(path) {
			continue
		}
		data, err := readTargetFile(env, target, path)
		if err != nil {
			continue
		}
		evidences := fileTestabilityEvidence(env, path, data, diffScope[path], hasChangedTests, testHasFailureEvidence)
		for _, evidence := range evidences {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:     evidence.ruleID,
				Level:      evidence.level,
				Path:       evidence.path,
				Line:       evidence.line,
				Message:    evidence.messageDetail,
				Confidence: evidence.confidence,
				Metadata: map[string]string{
					"evidence": evidence.evidenceKind,
				},
			}))
		}
	}

	// TODO(testing.legacy-hotspot-uncovered): emit only after the change section
	// receives reliable per-file history/churn inputs. The current diff context
	// can identify touched files, but cannot distinguish genuine legacy hotspots
	// from ordinary modified code without risking misleading findings.

	return findings
}

func fileTestabilityEvidence(env support.Context, path string, data []byte, ranges core.ChangedLineRanges, hasChangedTests bool, testHasFailureEvidence bool) []testabilityEvidence {
	lines := strings.Split(string(data), "\n")
	out := make([]testabilityEvidence, 0, 4)

	behaviorLine := firstChangedLineMatching(lines, ranges, func(line string) bool {
		return isBehaviorLine(line)
	})
	if enabled(env.Config.Checks.ChangeRules.DetectBehaviorChangeWithoutTest) && behaviorLine > 0 && !hasChangedTests {
		out = append(out, testabilityEvidence{
			path:          path,
			line:          behaviorLine,
			ruleID:        "testing.behavior-change-without-test",
			level:         "fail",
			confidence:    "high",
			evidenceKind:  "changed-production-behavior-without-test-file",
			messageDetail: "Changed production behavior without any changed test file in the same diff.",
		})
	}

	failureLine := firstChangedLineMatching(lines, ranges, func(line string) bool {
		return isFailurePathLine(line)
	})
	if enabled(env.Config.Checks.ChangeRules.DetectFailurePathMissing) && failureLine > 0 && !testHasFailureEvidence {
		out = append(out, testabilityEvidence{
			path:          path,
			line:          failureLine,
			ruleID:        "testing.failure-path-missing",
			level:         "warn",
			confidence:    confidenceWithTests(hasChangedTests),
			evidenceKind:  "changed-failure-path-without-failure-test-evidence",
			messageDetail: "Changed failure-path logic without changed tests that exercise an error, retry, fallback, or denial path.",
		})
	}

	hardwiredLine := firstChangedLineMatching(lines, ranges, func(line string) bool {
		return isHardwiredDependencyLine(line)
	})
	if enabled(env.Config.Checks.ChangeRules.DetectHardwiredDependency) && hardwiredLine > 0 {
		out = append(out, testabilityEvidence{
			path:          path,
			line:          hardwiredLine,
			ruleID:        "testing.hardwired-dependency",
			level:         "warn",
			confidence:    "high",
			evidenceKind:  "changed-direct-dependency-construction",
			messageDetail: "Changed business logic directly wires an external dependency; inject a fakeable boundary for deterministic tests.",
		})
	}

	nondeterministicLine := firstChangedLineMatching(lines, ranges, func(line string) bool {
		return isDomainPath(path) && isNondeterministicLine(line)
	})
	if enabled(env.Config.Checks.ChangeRules.DetectNondeterministicDomain) && nondeterministicLine > 0 {
		out = append(out, testabilityEvidence{
			path:          path,
			line:          nondeterministicLine,
			ruleID:        "testing.nondeterministic-domain-logic",
			level:         "warn",
			confidence:    "high",
			evidenceKind:  "changed-domain-nondeterminism",
			messageDetail: "Changed domain logic reads time, randomness, environment, or process state directly, making behavior hard to test deterministically.",
		})
	}

	return out
}

func changedTestEvidence(_ context.Context, env support.Context, target core.TargetConfig, changed []core.ChangedFile) ([]string, bool) {
	testFiles := make([]string, 0)
	hasFailureEvidence := false
	for _, file := range changed {
		path := filepath.ToSlash(file.Path)
		if file.Status == core.ChangedFileDeleted || !isTestPath(path) || !isSupportedTestPath(path) {
			continue
		}
		testFiles = append(testFiles, path)
		data, err := readTargetFile(env, target, path)
		if err != nil && env.ReadBaseFile != nil {
			data, err = env.ReadBaseFile(target, path)
		}
		if err == nil && failureTestPattern.MatchString(strings.ToLower(maskLineComments(string(data)))) {
			hasFailureEvidence = true
		}
	}
	return testFiles, hasFailureEvidence
}

func readTargetFile(env support.Context, target core.TargetConfig, path string) ([]byte, error) {
	if env.ReadTargetFile != nil {
		return env.ReadTargetFile(target, path)
	}
	return nil, errors.New("read target file callback is not configured")
}

func firstChangedLineMatching(lines []string, ranges core.ChangedLineRanges, match func(string) bool) int {
	for idx, raw := range lines {
		lineNo := idx + 1
		if !ranges.AllChanged && len(ranges.Ranges) > 0 && !ranges.Contains(lineNo) {
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || isCommentOnly(trimmed) {
			continue
		}
		if match(strings.ToLower(trimmed)) {
			return lineNo
		}
	}
	return 0
}

func isBehaviorLine(line string) bool {
	line = maskLineComments(line)
	if strings.Contains(line, "logger.") || strings.Contains(line, "log.") || strings.Contains(line, "fmt.print") {
		return false
	}
	return behaviorEvidencePattern.MatchString(line)
}

func isFailurePathLine(line string) bool {
	line = maskLineComments(line)
	return failurePathPattern.MatchString(line)
}

func isHardwiredDependencyLine(line string) bool {
	line = maskLineComments(line)
	return hardwiredPattern.MatchString(line)
}

func isNondeterministicLine(line string) bool {
	line = maskLineComments(line)
	return nondeterministicPattern.MatchString(line)
}

func confidenceWithTests(hasChangedTests bool) string {
	if hasChangedTests {
		return "medium"
	}
	return "high"
}

func isSupportedProductionPath(path string) bool {
	return isSupportedSourcePath(path) && !isGeneratedPath(path)
}

func isSupportedTestPath(path string) bool {
	return isSupportedSourcePath(path)
}

func isSupportedSourcePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".cc", ".cpp", ".cxx", ".c++", ".hh", ".hpp", ".hxx", ".h++":
		return true
	default:
		if ext == ".h" || ext == ".inc" {
			return true
		}
		return false
	}
}

func isTestPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	if strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") || strings.Contains(lower, "/__tests__/") || strings.Contains(lower, "/testdata/") {
		return true
	}
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasSuffix(base, "_test.cpp") ||
		strings.HasSuffix(base, "_test.cc") ||
		strings.HasSuffix(base, "_spec.cpp") ||
		strings.HasSuffix(base, "_spec.cc")
}

func isDomainPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") ||
		strings.Contains(lower, "/cmd/") || strings.Contains(lower, "/cli/") ||
		strings.Contains(lower, "/infra/") || strings.Contains(lower, "/infrastructure/") ||
		strings.Contains(lower, "/adapter/") || strings.Contains(lower, "/adapters/") ||
		strings.Contains(lower, "/migration") || strings.Contains(lower, "/script") ||
		strings.HasSuffix(lower, "/main.go") {
		return false
	}
	for _, token := range []string{"/domain/", "/service/", "/services/", "/usecase/", "/usecases/", "/business/", "/model/", "/models/", "/core/", "/internal/", "/pkg/", "/app/", "/src/"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return !strings.Contains(lower, "/config/")
}

func isGeneratedPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "generated") ||
		strings.HasSuffix(lower, ".pb.go") ||
		strings.HasSuffix(lower, ".gen.go") ||
		strings.HasSuffix(lower, ".generated.ts") ||
		strings.HasSuffix(lower, ".generated.js")
}

func isCommentOnly(line string) bool {
	return strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "/*")
}

func maskLineComments(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = line[:idx]
	}
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}
