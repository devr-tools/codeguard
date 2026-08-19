package ci

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	workflowUsesPattern       = regexp.MustCompile(`(?i)\buses:\s*['"]?([^@\s'"]+)(?:@([^\s#'"]+))?`)
	immutableActionRefPattern = regexp.MustCompile(`(?i)^[0-9a-f]{40}$`)
	latestImagePattern        = regexp.MustCompile(`(?i)\b(?:image:|from)\s+['"]?[^@\s'"]+:latest\b`)
	deployImageRunPattern     = regexp.MustCompile(`(?i)\b(?:docker|kubectl|helm)\b.*:latest\b`)
)

func missingRequiredGateFindings(env support.Context, target core.TargetConfig) []core.Finding {
	gates := normalizedNonEmpty(env.Config.Checks.CIRules.RequiredGates)
	if len(gates) == 0 {
		return nil
	}
	workflows := workflowFiles(env, target)
	if len(workflows) == 0 {
		return nil
	}
	allContent := strings.Builder{}
	for _, file := range workflows {
		allContent.WriteString("\n")
		allContent.WriteString(strings.ToLower(string(file.data)))
	}
	content := allContent.String()
	findings := make([]core.Finding, 0)
	for _, gate := range gates {
		if workflowHasGate(content, gate) {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "ci.missing-required-gate",
			Level:      "fail",
			Path:       ".github/workflows",
			Line:       1,
			Column:     1,
			Message:    fmt.Sprintf("required CI gate %q is missing from configured workflows", gate),
			Confidence: core.ConfidenceHigh,
			Metadata: map[string]string{
				"gate": gate,
			},
		}))
	}
	return findings
}

func workflowHasGate(content string, gate string) bool {
	gate = strings.ToLower(strings.TrimSpace(gate))
	if gate == "" {
		return true
	}
	switch gate {
	case "test", "tests":
		return strings.Contains(content, "go test") ||
			strings.Contains(content, "npm test") ||
			strings.Contains(content, "pnpm test") ||
			strings.Contains(content, "yarn test") ||
			strings.Contains(content, "pytest") ||
			strings.Contains(content, "cargo test") ||
			strings.Contains(content, "dotnet test") ||
			strings.Contains(content, "mvn test") ||
			strings.Contains(content, "gradle test") ||
			strings.Contains(content, "\n  test:") ||
			strings.Contains(content, "\n    name: test")
	case "security":
		return strings.Contains(content, "govulncheck") ||
			strings.Contains(content, "gosec") ||
			strings.Contains(content, "codeql") ||
			strings.Contains(content, "trivy") ||
			strings.Contains(content, "snyk") ||
			strings.Contains(content, "semgrep")
	default:
		return strings.Contains(content, gate)
	}
}

func mutableDeploymentReferenceFindings(env support.Context, target core.TargetConfig) []core.Finding {
	files := deliveryReferenceFiles(env, target)
	findings := make([]core.Finding, 0)
	for _, file := range files {
		lines := strings.Split(string(file.data), "\n")
		for idx, line := range lines {
			if finding, ok := mutableActionRefFinding(env, file.rel, idx+1, line); ok {
				findings = append(findings, finding)
			}
			if finding, ok := latestImageFinding(env, file.rel, idx+1, line); ok {
				findings = append(findings, finding)
			}
		}
	}
	return findings
}

func mutableActionRefFinding(env support.Context, rel string, lineNo int, line string) (core.Finding, bool) {
	match := workflowUsesPattern.FindStringSubmatch(line)
	if len(match) == 0 {
		return core.Finding{}, false
	}
	action := match[1]
	ref := ""
	if len(match) > 2 {
		ref = strings.TrimSpace(match[2])
	}
	if strings.HasPrefix(action, "./") || strings.HasPrefix(strings.ToLower(action), "docker://") || action == "" {
		return core.Finding{}, false
	}
	if ref != "" && !isMutableActionRef(ref) {
		return core.Finding{}, false
	}
	message := "external GitHub Action uses a mutable or missing ref"
	if ref != "" {
		message = fmt.Sprintf("external GitHub Action ref %q is mutable", ref)
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     "ci.mutable-deployment-reference",
		Level:      "fail",
		Path:       rel,
		Line:       lineNo,
		Column:     1,
		Message:    message,
		Confidence: core.ConfidenceHigh,
		Metadata: map[string]string{
			"reference_type": "github_action",
		},
	}), true
}

func latestImageFinding(env support.Context, rel string, lineNo int, line string) (core.Finding, bool) {
	if !latestImagePattern.MatchString(line) && !deployImageRunPattern.MatchString(line) {
		return core.Finding{}, false
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     "ci.mutable-deployment-reference",
		Level:      "fail",
		Path:       rel,
		Line:       lineNo,
		Column:     1,
		Message:    "container image reference uses the mutable latest tag",
		Confidence: core.ConfidenceHigh,
		Metadata: map[string]string{
			"reference_type": "container_image",
		},
	}), true
}

func isMutableActionRef(ref string) bool {
	return !immutableActionRefPattern.MatchString(strings.TrimSpace(ref))
}

type ciFile struct {
	rel  string
	data []byte
}

func workflowFiles(env support.Context, target core.TargetConfig) []ciFile {
	return collectCIFiles(env, target, func(rel string) bool {
		normalized := strings.ToLower(filepath.ToSlash(rel))
		return strings.HasPrefix(normalized, ".github/workflows/") &&
			(strings.HasSuffix(normalized, ".yml") || strings.HasSuffix(normalized, ".yaml"))
	})
}

func deliveryReferenceFiles(env support.Context, target core.TargetConfig) []ciFile {
	return collectCIFiles(env, target, func(rel string) bool {
		normalized := strings.ToLower(filepath.ToSlash(rel))
		base := strings.ToLower(filepath.Base(normalized))
		return strings.HasPrefix(normalized, ".github/workflows/") ||
			strings.HasPrefix(normalized, "deploy/") ||
			strings.HasPrefix(normalized, "deployment/") ||
			strings.HasPrefix(normalized, "k8s/") ||
			strings.HasPrefix(normalized, "kubernetes/") ||
			strings.Contains(normalized, "/deploy/") ||
			strings.Contains(normalized, "/deployment/") ||
			strings.HasPrefix(base, "dockerfile")
	})
}

func collectCIFiles(env support.Context, target core.TargetConfig, include func(string) bool) []ciFile {
	files := make([]ciFile, 0)
	if env.VisitTargetFiles != nil {
		env.VisitTargetFiles(target, include, func(rel string, data []byte) {
			files = append(files, ciFile{rel: filepath.ToSlash(rel), data: append([]byte(nil), data...)})
		})
		return files
	}
	return files
}

func normalizedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
