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
	environmentAllowedDirs   = []string{"config/", "configs/", "cmd/", "scripts/", ".github/", "deploy/", "deployment/", "k8s/", "kubernetes/", "bootstrap/", "infra/"}
)

func environmentBranchingFindings(env support.Context, target core.TargetConfig) []core.Finding {
	if !localPrecisionEnabled(env) {
		return nil
	}
	return env.ScanTargetFiles(target, "quality-environment-branching", func(rel string) bool {
		return environmentBranchingEligiblePath(env.Config.Checks.DeliveryRules, rel)
	}, func(file string, data []byte) []core.Finding {
		text := string(data)
		line := environmentBranchLine(text)
		if line == 0 {
			return nil
		}
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:     "quality.environment-branching",
			Level:      "warn",
			Path:       file,
			Line:       line,
			Column:     1,
			Message:    "domain/source code branches on deployment environment; move environment policy to configuration or bootstrap boundaries",
			Confidence: core.ConfidenceHigh,
			Metadata: map[string]string{
				"boundary": "source_code",
			},
		})}
	})
}

// environmentExcludedPath* mark API, auth, script, ORM, integration, and
// shared-library paths where environment checks are boundary concerns.
var (
	environmentExcludedPathFragments = []string{
		"/app/api/", "/api/", "/auth/", "/scripts/", "/prisma/", "/integrations/", "/packages/db/src/", "/apps/web/lib/",
	}
	environmentExcludedPathPrefixes = []string{"apps/web/lib/", "packages/db/src/"}
	environmentSourceExtensions     = []string{
		".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".h", ".rb",
	}
)

func environmentBranchingEligiblePath(cfg core.DeliveryRulesConfig, rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	if isQualityFixturePath(normalized) || isLikelyUIFile(normalized) ||
		containsAny(normalized, environmentExcludedPathFragments) || hasAnyPrefix(normalized, environmentExcludedPathPrefixes) {
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
	return hasAnySuffix(normalized, environmentSourceExtensions)
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func hasAnySuffix(value string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func environmentBranchLine(text string) int {
	for idx, line := range strings.Split(text, "\n") {
		if environmentBranchLineIsDeploymentDecision(line) {
			return idx + 1
		}
	}
	return 0
}

// branchKeywords replaces `\b(if|switch|case|when)\b`, which ran on every
// source line; containsWordAny applies the same ASCII word boundaries without
// entering the regexp engine.
var branchKeywords = []string{"if", "switch", "case", "when"}

func environmentBranchLineIsDeploymentDecision(line string) bool {
	if !environmentBranchPattern.MatchString(line) {
		return false
	}
	lowered := strings.ToLower(line)
	if containsAny(lowered, []string{
		"process.env", "node_env", "rails.env", "os.getenv", "os.getenv", "getenv(", "std::getenv",
	}) {
		return true
	}
	if !containsWordAny(lowered, branchKeywords) {
		return false
	}
	return containsAny(lowered, []string{" env", "env.", "env[", "environment", "deployment"}) &&
		containsAny(lowered, []string{"prod", "production", "staging", "stage", "dev", "development", "test"})
}
