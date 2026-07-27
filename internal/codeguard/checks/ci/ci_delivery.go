package ci

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	deploymentMarkerPattern     = regexp.MustCompile(`(?i)\b(deploy|deployment|production|prod|release|rollout|kubectl|helm|goreleaser|terraform apply)\b`)
	destructiveMigrationPattern = regexp.MustCompile(`(?i)\b(drop\s+(table|column|index)|alter\s+table.+drop|truncate\s+table|rename\s+column|set\s+not\s+null|delete\s+from)\b`)
	migrationSafetyPattern      = regexp.MustCompile(`(?i)\b(expand|contract|backfill|dual[-_\s]?write|concurrently|safe migration|two[-_\s]?phase|reversible|rollback|roll back|down\s*\(|down:)\b`)
	highRiskBehaviorPattern     = regexp.MustCompile(`(?i)\b(payment|checkout|billing|invoice|subscription|auth|authentication|authorization|migration|backfill|delete\s+from|drop\s+table|write|charge|refund)\b`)
	sourceMutationPattern       = regexp.MustCompile(`(?i)\b(save|insert|update|delete|charge|refund|create|write|migrate|backfill)\s*\(`)
)

type fileSnapshot struct {
	rel  string
	text string
}

func RunDelivery(ctx context.Context, env support.Context) core.SectionResult {
	return support.RunTargetSection(ctx, env, "delivery", "Delivery", deliveryFindingsForTarget)
}

func deliveryFindingsForTarget(_ context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	cfg := env.Config.Checks.DeliveryRules
	files := collectFiles(env, target)
	if len(files) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	if enabled(cfg.DetectMissingRollbackStrategy) {
		findings = append(findings, missingRollbackFindings(env, cfg, files)...)
	}
	if enabled(cfg.DetectUnsafeMigrationOrder) {
		findings = append(findings, unsafeMigrationFindings(env, cfg, files)...)
	}
	if enabled(cfg.DetectHighRiskChangeWithoutKillSwitch) {
		findings = append(findings, missingKillSwitchFindings(env, cfg, files)...)
	}
	if enabled(cfg.DetectMissingPostDeployVerification) {
		findings = append(findings, missingPostDeployVerificationFindings(env, cfg, files)...)
	}
	return findings
}

func missingRollbackFindings(env support.Context, cfg core.DeliveryRulesConfig, files []fileSnapshot) []core.Finding {
	if hasAnyPattern(files, cfg.RollbackEvidencePatterns) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, file := range files {
		if !isDeploymentFile(file) && !hasDestructiveMigration(file.text) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "delivery.missing-rollback-strategy",
			Level:      "warn",
			Path:       file.rel,
			Line:       firstMarkerLine(file.text, deploymentMarkerPattern),
			Column:     1,
			Message:    "deployment or destructive migration change has no rollback strategy evidence",
			Confidence: core.ConfidenceMedium,
			Metadata: map[string]string{
				"evidence": "deployment_or_migration",
			},
		}))
	}
	return findings
}

func unsafeMigrationFindings(env support.Context, cfg core.DeliveryRulesConfig, files []fileSnapshot) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, file := range files {
		if !isMigrationPath(cfg, file.rel) || !hasDestructiveMigration(file.text) || migrationSafetyPattern.MatchString(file.text) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "delivery.unsafe-migration-order",
			Level:      "warn",
			Path:       file.rel,
			Line:       firstMarkerLine(file.text, destructiveMigrationPattern),
			Column:     1,
			Message:    "destructive migration lacks expand/backfill/contract or rollback sequencing evidence",
			Confidence: core.ConfidenceMedium,
			Metadata: map[string]string{
				"migration_risk": "destructive_change",
			},
		}))
	}
	return findings
}

func missingKillSwitchFindings(env support.Context, cfg core.DeliveryRulesConfig, files []fileSnapshot) []core.Finding {
	if hasAnyPattern(files, cfg.KillSwitchPatterns) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, file := range files {
		if isBootstrapPath(cfg, file.rel) || !isSourcePath(file.rel) || !isHighRiskChange(cfg, file) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "delivery.high-risk-change-without-kill-switch",
			Level:      "warn",
			Path:       file.rel,
			Line:       firstMarkerLine(file.text, highRiskBehaviorPattern),
			Column:     1,
			Message:    "high-risk production behavior has no feature flag or kill-switch evidence",
			Confidence: core.ConfidenceMedium,
			Metadata: map[string]string{
				"evidence": "critical_path_change",
			},
		}))
	}
	return findings
}

func missingPostDeployVerificationFindings(env support.Context, cfg core.DeliveryRulesConfig, files []fileSnapshot) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, file := range files {
		if !isDeploymentFile(file) || containsAnyFold(file.text, cfg.PostDeployVerificationPatterns) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "delivery.missing-post-deploy-verification",
			Level:      "warn",
			Path:       file.rel,
			Line:       firstMarkerLine(file.text, deploymentMarkerPattern),
			Column:     1,
			Message:    "deployment workflow lacks post-deploy smoke, health, or SLO verification evidence",
			Confidence: core.ConfidenceMedium,
			Metadata: map[string]string{
				"verification": "missing",
			},
		}))
	}
	return findings
}

func collectFiles(env support.Context, target core.TargetConfig) []fileSnapshot {
	files := make([]fileSnapshot, 0)
	if env.VisitTargetFiles == nil {
		return files
	}
	env.VisitTargetFiles(target, func(rel string) bool {
		return isPotentialDeliveryPath(env.Config.Checks.DeliveryRules, rel)
	}, func(rel string, data []byte) {
		files = append(files, fileSnapshot{rel: filepath.ToSlash(rel), text: string(data)})
	})
	return files
}

func isPotentialDeliveryPath(cfg core.DeliveryRulesConfig, rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	return strings.HasPrefix(normalized, ".github/workflows/") ||
		strings.Contains(normalized, "deploy") ||
		strings.Contains(normalized, "release") ||
		isMigrationPath(cfg, rel) ||
		isSourcePath(rel)
}

func isDeploymentFile(file fileSnapshot) bool {
	path := strings.ToLower(filepath.ToSlash(file.rel))
	if strings.HasPrefix(path, ".github/workflows/") || strings.Contains(path, "deploy") || strings.Contains(path, "release") {
		return deploymentMarkerPattern.MatchString(file.text)
	}
	return false
}

func isMigrationPath(cfg core.DeliveryRulesConfig, rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	for _, pattern := range cfg.MigrationPathPatterns {
		if support.PathMatchesPattern(pattern, normalized) {
			return true
		}
	}
	return strings.Contains(normalized, "migrations/") || strings.Contains(normalized, "db/migrate/") || strings.Contains(normalized, "alembic/")
}

func isHighRiskChange(cfg core.DeliveryRulesConfig, file fileSnapshot) bool {
	for _, pattern := range cfg.HighRiskPathPatterns {
		if support.PathMatchesPattern(pattern, strings.ToLower(filepath.ToSlash(file.rel))) {
			return highRiskBehaviorPattern.MatchString(file.text)
		}
	}
	return highRiskBehaviorPattern.MatchString(file.text) && sourceMutationPattern.MatchString(file.text)
}

func isBootstrapPath(cfg core.DeliveryRulesConfig, rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	for _, pattern := range cfg.BootstrapPathPatterns {
		if support.PathMatchesPattern(pattern, normalized) {
			return true
		}
	}
	return strings.HasPrefix(normalized, "cmd/") ||
		strings.Contains(normalized, "/config/") ||
		strings.Contains(normalized, "/bootstrap/") ||
		strings.HasPrefix(normalized, "scripts/")
}

func isSourcePath(rel string) bool {
	lowered := strings.ToLower(rel)
	return strings.HasSuffix(lowered, ".go") ||
		strings.HasSuffix(lowered, ".py") ||
		strings.HasSuffix(lowered, ".ts") ||
		strings.HasSuffix(lowered, ".tsx") ||
		strings.HasSuffix(lowered, ".js") ||
		strings.HasSuffix(lowered, ".jsx") ||
		strings.HasSuffix(lowered, ".cpp") ||
		strings.HasSuffix(lowered, ".cc") ||
		strings.HasSuffix(lowered, ".cxx") ||
		strings.HasSuffix(lowered, ".hpp") ||
		strings.HasSuffix(lowered, ".hh") ||
		strings.HasSuffix(lowered, ".h")
}

func hasDestructiveMigration(text string) bool {
	return destructiveMigrationPattern.MatchString(text)
}

func hasAnyPattern(files []fileSnapshot, patterns []string) bool {
	for _, file := range files {
		if containsAnyFold(file.text, patterns) {
			return true
		}
	}
	return false
}

func containsAnyFold(text string, patterns []string) bool {
	lowered := strings.ToLower(text)
	for _, pattern := range patterns {
		if strings.Contains(lowered, strings.ToLower(strings.TrimSpace(pattern))) {
			return true
		}
	}
	return false
}

func firstMarkerLine(text string, pattern *regexp.Regexp) int {
	lines := strings.Split(text, "\n")
	for idx, line := range lines {
		if pattern.MatchString(line) {
			return idx + 1
		}
	}
	return 1
}

func enabled(value *bool) bool {
	return value == nil || *value
}
