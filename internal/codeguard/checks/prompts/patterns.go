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
	}
	// mcpAutoInstallPattern is the line fallback for configs that cannot be
	// parsed (TOML, malformed JSON). The flag is matched after a quote,
	// bracket, comma or space: a word boundary never matches between `"`
	// and `-`, which is why the previous `\b-y\b` form could not fire on
	// "args": ["-y", ...].
	mcpAutoInstallPattern = regexp.MustCompile(`(?i)\b(?:npx|uvx|bunx)\b.{0,40}?(?:^|[\s"'\[,=])(?:-y|--yes)(?:$|[\s"',\]])`)
	mcpPinnedHint         = regexp.MustCompile(`@v?\d+\.\d+|==\s*\d`)
)

// mcpDirConfigExtensions are the config formats an MCP client reads its server
// definitions from; they are the files a directory named mcp/ governs.
var mcpDirConfigExtensions = map[string]struct{}{
	".json": {}, ".jsonc": {}, ".yaml": {}, ".yml": {}, ".toml": {},
}

func isGovernedPromptFile(env support.Context, rel string) bool {
	return env.IsPromptFile(rel) || isAgentConfigFile(rel) || isMCPConfigFile(rel)
}

func isMCPConfigFile(rel string) bool {
	rel = strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(rel)
	switch base {
	case ".mcp.json", "mcp.json", "mcp.yaml", "mcp.yml", "claude_desktop_config.json":
		return true
	case "settings.json":
		// Gemini CLI declares its MCP servers in .gemini/settings.json.
		if strings.HasSuffix("/"+rel, "/.gemini/settings.json") {
			return true
		}
	}
	ext := filepath.Ext(base)
	// A directory named mcp holds MCP server code as often as MCP config, so
	// it only governs config-format files: Go, TypeScript, or Python source
	// for an MCP server is code, and a string literal in it (a test fixture,
	// a transcript) is not a prompt asset.
	if strings.Contains("/"+rel, "/mcp/") {
		_, config := mcpDirConfigExtensions[ext]
		return config
	}
	if strings.Contains(base, "mcp") {
		switch ext {
		case ".json", ".yaml", ".yml":
			return true
		}
	}
	return strings.HasSuffix(rel, "/mcp.json") || strings.HasSuffix(rel, "/mcp.yaml") || strings.HasSuffix(rel, "/mcp.yml")
}

func basePromptFindings(env support.Context, file string, data []byte) []core.Finding {
	// Structured config files inject credentials through ${VAR} references by
	// design - it is the remediation agent-config-literal-secret recommends -
	// so secret interpolation is only a finding in prose prompt assets.
	checkInterpolation := *env.Config.Checks.PromptRules.ForbidSecretInterpolation && !isStructuredConfigFile(file, data)
	findings := make([]core.Finding, 0)
	for idx, line := range splitLines(data) {
		if checkInterpolation && secretInterpolationRegex.MatchString(line) {
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

// isStructuredConfigFile reports whether file is an agent settings file or a
// config that actually declares MCP servers. A YAML prompt that merely sits
// in an mcp/ directory is still prose and keeps the interpolation check.
func isStructuredConfigFile(file string, data []byte) bool {
	if isClaudeSettingsFile(file) {
		return true
	}
	if !isMCPConfigFile(file) {
		return false
	}
	root, ok := parseConfigDocument(file, data)
	return ok && len(collectMCPServers(root)) > 0
}

func agentConfigFindings(env support.Context, file string, data []byte) []core.Finding {
	switch {
	case isClaudeSettingsFile(file):
		return claudeSettingsFindings(env, file, data)
	case isAgentInstructionFile(file):
		findings := dangerousInstructionFindings(env, file, data)
		return append(findings, standingPermissionRegexFindings(env, file, data)...)
	default:
		return nil
	}
}

// mcpConfigFindings parses JSON, JSONC, and YAML MCP configs and evaluates
// each declared server. A file that cannot be parsed (TOML, or a malformed
// document) falls back to line heuristics rather than being skipped.
func mcpConfigFindings(env support.Context, file string, data []byte) []core.Finding {
	if !isMCPConfigFile(file) || isClaudeSettingsFile(file) {
		return nil
	}
	if root, ok := parseConfigDocument(file, data); ok {
		return mcpStructuredFindings(env, file, root)
	}
	return mcpRegexFindings(env, file, data)
}

func mcpRegexFindings(env support.Context, file string, data []byte) []core.Finding {
	content := string(data)
	findings := make([]core.Finding, 0)
	risky := make([]*regexp.Regexp, 0, len(standingPermissionPatterns)+len(mcpConfigRiskPatterns))
	risky = append(append(risky, standingPermissionPatterns...), mcpConfigRiskPatterns...)
	for _, pattern := range risky {
		if loc := pattern.FindStringIndex(content); loc != nil {
			findings = append(findings, settingsFinding(env, file, lineAtOffset(content, loc[0]), "prompts.mcp-config-risk", "fail", core.ConfidenceMedium,
				"MCP config allows risky tool execution or overly broad permissions"))
			break
		}
	}
	for idx, line := range splitLines(data) {
		if mcpAutoInstallPattern.MatchString(line) && !mcpPinnedHint.MatchString(line) {
			findings = append(findings, settingsFinding(env, file, idx+1, "prompts.mcp-unpinned-server", "warn", core.ConfidenceLow,
				"MCP server auto-installs a package (-y) with no pinned version visible on this line; pin an exact version"))
		}
	}
	return findings
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
