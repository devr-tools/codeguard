package prompts

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	secretInterpolationRegex = regexp.MustCompile(`(\$\{[A-Z0-9_]+\}|{{\s*[^}]*secret[^}]*}})`)
	unsafePromptPatterns     = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore previous instructions`),
		regexp.MustCompile(`(?i)reveal the system prompt`),
		regexp.MustCompile(`(?i)disregard all prior instructions`),
		regexp.MustCompile(`(?i)(print|dump|reveal|exfiltrate|return)\s+(all\s+)?(env|environment variables|secrets|tokens)`),
	}
	agentDangerousInstructionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bnever ask for approval\b`),
		regexp.MustCompile(`(?i)\b(skip|disable|bypass|ignore|override)\b.{0,40}\b(approval|sandbox|policy|guardrail|safety|permission)\b`),
		regexp.MustCompile(`(?i)\balways comply\b.{0,40}\bwithout\b.{0,20}\bapproval\b`),
	}
	standingPermissionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)"(?:allow|allowed_tools|tools|permissions|tool_permissions|trusted_tools)"\s*:\s*\[[^\]]*"\*"`),
		regexp.MustCompile(`(?is)(?:^|\n)\s*(?:allow|allowed_tools|tools|permissions|tool_permissions)\s*:\s*\[[^\]]*['"]\*['"]`),
		regexp.MustCompile(`(?is)\b(?:allow|allowed_tools|tools|permissions|tool_permissions)\b\s*:\s*(?:\r?\n\s*)+-\s*['"]\*['"]`),
		regexp.MustCompile(`(?i)(allowAllTools|autoApprove|skipApproval|skip_approval|bypassApprovals?)\s*["']?\s*:\s*true`),
		regexp.MustCompile(`(?i)\b(always allow|full access to all tools|passwordless sudo|unrestricted shell)\b`),
	}
	mcpConfigRiskPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)"command"\s*:\s*"(?:bash|sh|zsh)".{0,200}"-(?:c|lc)"`),
		regexp.MustCompile(`(?is)"(?:command|args)"\s*:\s*(?:".*curl[^"\n]*\|[^"\n]*(?:sh|bash).*"|\[[^\]]*curl[^\]]*\|[^\]]*(?:sh|bash)[^\]]*\])`),
		regexp.MustCompile(`(?i)\b(npx|uvx)\b.{0,30}\b-y\b`),
	}
)

func isGovernedPromptFile(env support.Context, rel string) bool {
	return env.IsPromptFile(rel) || isAgentConfigFile(rel) || isMCPConfigFile(rel)
}

func isAgentConfigFile(rel string) bool {
	base := strings.ToLower(filepath.Base(filepath.ToSlash(rel)))
	switch base {
	case "agents.md", "claude.md", ".cursorrules":
		return true
	default:
		return false
	}
}

func isMCPConfigFile(rel string) bool {
	rel = strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(rel)
	switch base {
	case ".mcp.json", "mcp.json", "mcp.yaml", "mcp.yml", "claude_desktop_config.json":
		return true
	}
	if strings.Contains(rel, "/mcp/") {
		return true
	}
	if strings.Contains(base, "mcp") {
		switch filepath.Ext(base) {
		case ".json", ".yaml", ".yml":
			return true
		}
	}
	return strings.HasSuffix(rel, "/mcp.json") || strings.HasSuffix(rel, "/mcp.yaml") || strings.HasSuffix(rel, "/mcp.yml")
}

func basePromptFindings(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for idx, line := range splitLines(data) {
		if *env.Config.Checks.PromptRules.ForbidSecretInterpolation && secretInterpolationRegex.MatchString(line) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "prompts.secret-interpolation",
				Level:   "fail",
				Path:    file,
				Line:    idx + 1,
				Column:  1,
				Message: "prompt contains secret interpolation pattern",
			}))
		}
		if !*env.Config.Checks.PromptRules.ForbidUnsafeInstructions {
			continue
		}
		if matchesAny(line, unsafePromptPatterns) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "prompts.unsafe-instructions",
				Level:   "warn",
				Path:    file,
				Line:    idx + 1,
				Column:  1,
				Message: "prompt contains unsafe instruction pattern",
			}))
		}
	}
	return findings
}

func agentConfigFindings(env support.Context, file string, data []byte) []core.Finding {
	if !isAgentConfigFile(file) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for idx, line := range splitLines(data) {
		if matchesAny(line, agentDangerousInstructionPatterns) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "prompts.agent-dangerous-instructions",
				Level:   "fail",
				Path:    file,
				Line:    idx + 1,
				Column:  1,
				Message: "agent config contains dangerous instruction or approval bypass pattern",
			}))
		}
	}
	content := string(data)
	if matchesAny(content, standingPermissionPatterns) {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "prompts.agent-standing-permissions",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: "agent config grants standing wildcard permissions or unrestricted tool access",
		}))
	}
	return findings
}

func mcpConfigFindings(env support.Context, file string, data []byte) []core.Finding {
	if !isMCPConfigFile(file) {
		return nil
	}
	content := string(data)
	if !matchesAny(content, standingPermissionPatterns) && !matchesAny(content, mcpConfigRiskPatterns) {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "prompts.mcp-config-risk",
		Level:   "fail",
		Path:    file,
		Line:    1,
		Column:  1,
		Message: "MCP config allows risky tool execution or overly broad permissions",
	})}
}

func matchesAny(value string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func splitLines(data []byte) []string {
	return strings.Split(string(data), "\n")
}
