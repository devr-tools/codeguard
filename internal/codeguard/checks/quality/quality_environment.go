package quality

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	environmentBranchPattern = regexp.MustCompile(`(?i)\b(if|switch|case|when)\b[^\n]*(prod|production|staging|stage|dev|development|test)\b|process\.env\.NODE_ENV|Rails\.env\.(production|staging|development|test)\?|os\.(Getenv|getenv)\([^)]*(ENV|ENVIRONMENT|NODE_ENV)|\b(std::)?getenv\([^)]*(ENV|ENVIRONMENT|NODE_ENV)`)
	environmentAllowedDirs   = []string{"config/", "configs/", "cmd/", "scripts/", ".github/", "deploy/", "deployment/", "k8s/", "kubernetes/", "bootstrap/"}
)

func environmentBranchingFindings(env support.Context, target core.TargetConfig) []core.Finding {
	if !localPrecisionEnabled(env) {
		return nil
	}
	return env.ScanTargetFiles(target, "quality-environment-branching", func(rel string) bool {
		return environmentBranchingEligiblePath(env.Config.Checks.DeliveryRules, rel)
	}, func(file string, data []byte) []core.Finding {
		text := string(data)
		if !environmentBranchPattern.MatchString(text) {
			return nil
		}
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:     "quality.environment-branching",
			Level:      "warn",
			Path:       file,
			Line:       environmentBranchLine(text),
			Column:     1,
			Message:    "domain/source code branches on deployment environment; move environment policy to configuration or bootstrap boundaries",
			Confidence: core.ConfidenceHigh,
			Metadata: map[string]string{
				"boundary": "source_code",
			},
		})}
	})
}

func environmentBranchingEligiblePath(cfg core.DeliveryRulesConfig, rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	if isQualityFixturePath(normalized) {
		return false
	}
	for _, pattern := range cfg.BootstrapPathPatterns {
		if support.PathMatchesPattern(pattern, normalized) {
			return false
		}
	}
	for _, prefix := range environmentAllowedDirs {
		if strings.HasPrefix(normalized, prefix) || strings.Contains(normalized, "/"+prefix) {
			return false
		}
	}
	return strings.HasSuffix(normalized, ".go") ||
		strings.HasSuffix(normalized, ".py") ||
		strings.HasSuffix(normalized, ".ts") ||
		strings.HasSuffix(normalized, ".tsx") ||
		strings.HasSuffix(normalized, ".js") ||
		strings.HasSuffix(normalized, ".jsx") ||
		strings.HasSuffix(normalized, ".cpp") ||
		strings.HasSuffix(normalized, ".cc") ||
		strings.HasSuffix(normalized, ".cxx") ||
		strings.HasSuffix(normalized, ".hpp") ||
		strings.HasSuffix(normalized, ".hh") ||
		strings.HasSuffix(normalized, ".h") ||
		strings.HasSuffix(normalized, ".rb")
}

func environmentBranchLine(text string) int {
	for idx, line := range strings.Split(text, "\n") {
		if environmentBranchPattern.MatchString(line) {
			return idx + 1
		}
	}
	return 1
}
