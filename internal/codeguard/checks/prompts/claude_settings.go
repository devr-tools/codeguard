package prompts

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// codeExecutingCommands are programs whose argument is itself a program. A
// prefix rule such as Bash(python3:*) therefore allows arbitrary code, so it
// is as broad as Bash(*) even though it looks scoped.
var codeExecutingCommands = map[string]struct{}{
	"bash": {}, "sh": {}, "zsh": {}, "fish": {}, "dash": {}, "ksh": {},
	"eval": {}, "exec": {}, "env": {}, "xargs": {}, "sudo": {}, "su": {}, "doas": {},
	"python": {}, "python3": {}, "node": {}, "deno": {}, "bun": {}, "perl": {},
	"ruby": {}, "php": {}, "pwsh": {}, "powershell": {}, "osascript": {},
}

// claudeSettingsFindings governs a .claude/settings*.json file structurally.
// A file that does not parse falls back to the line heuristics the other
// agent configs use, so a malformed settings file degrades to the old
// coverage instead of being skipped or failing the section.
func claudeSettingsFindings(env support.Context, file string, data []byte) []core.Finding {
	root, ok := parseJSONC(data)
	if !ok || root.kind != nodeObject {
		return standingPermissionRegexFindings(env, file, data)
	}
	findings := make([]core.Finding, 0)
	permissions := root.field("permissions")
	if allow := permissions.field("allow"); allow != nil && allow.kind == nodeArray {
		for _, item := range allow.items {
			rule, isString := item.str()
			if !isString {
				continue
			}
			if reason, confidence, broad := broadPermissionRule(rule); broad {
				findings = append(findings, settingsFinding(env, file, item.line, "prompts.agent-standing-permissions", "fail", confidence,
					fmt.Sprintf("permissions.allow entry %q %s; scope it to specific commands such as Bash(go test:*)", rule, reason)))
			}
		}
	}
	if mode, isString := permissions.field("defaultMode").str(); isString && mode == "bypassPermissions" {
		findings = append(findings, settingsFinding(env, file, permissions.field("defaultMode").line, "prompts.agent-standing-permissions", "fail", core.ConfidenceHigh,
			"permissions.defaultMode is bypassPermissions, so every tool call runs without approval for anyone using this repository"))
	}
	if node := root.field("skipDangerousModePermissionPrompt"); node.isTrue() {
		findings = append(findings, settingsFinding(env, file, node.line, "prompts.agent-standing-permissions", "fail", core.ConfidenceHigh,
			"skipDangerousModePermissionPrompt suppresses the confirmation before entering bypass-permissions mode"))
	}
	if node := root.field("enableAllProjectMcpServers"); node.isTrue() {
		findings = append(findings, settingsFinding(env, file, node.line, "prompts.agent-standing-permissions", "warn", core.ConfidenceHigh,
			"enableAllProjectMcpServers auto-approves every MCP server any contributor adds to .mcp.json; list trusted servers in enabledMcpjsonServers instead"))
	}
	findings = append(findings, envSecretFindings(env, file, "settings", root.field("env"), "env")...)
	return findings
}

// broadPermissionRule classifies one permissions.allow entry. Claude Code
// rules are Tool or Tool(specifier); a bare Bash, a wildcard specifier, or a
// prefix that names a code-executing program grants an unrestricted shell.
func broadPermissionRule(rule string) (string, string, bool) {
	compact := strings.Join(strings.Fields(rule), "")
	if compact == "*" {
		return "allows every tool without approval", core.ConfidenceHigh, true
	}
	tool, specifier, hasSpecifier := splitPermissionRule(rule)
	if !strings.EqualFold(tool, "Bash") {
		return "", "", false
	}
	if !hasSpecifier {
		return "allows any shell command without approval", core.ConfidenceHigh, true
	}
	trimmed := strings.TrimSpace(specifier)
	switch strings.Join(strings.Fields(trimmed), "") {
	case "", "*", "**", ":*", "*:*":
		return "allows any shell command without approval", core.ConfidenceHigh, true
	}
	program := firstCommandWord(trimmed)
	if _, ok := codeExecutingCommands[strings.ToLower(program)]; ok && isPrefixWildcard(trimmed, program) {
		return fmt.Sprintf("lets the agent run arbitrary code through %s without approval", program), core.ConfidenceMedium, true
	}
	return "", "", false
}

func firstCommandWord(specifier string) string {
	if end := strings.IndexAny(specifier, " :*"); end >= 0 {
		return specifier[:end]
	}
	return specifier
}

func splitPermissionRule(rule string) (string, string, bool) {
	rule = strings.TrimSpace(rule)
	open := strings.IndexByte(rule, '(')
	if open < 0 || !strings.HasSuffix(rule, ")") {
		return rule, "", false
	}
	return strings.TrimSpace(rule[:open]), rule[open+1 : len(rule)-1], true
}

// isPrefixWildcard reports whether specifier is program followed only by a
// wildcard (Bash(sh:*), Bash(python3 *)), i.e. any arguments are allowed.
func isPrefixWildcard(specifier string, program string) bool {
	rest := strings.TrimSpace(strings.TrimPrefix(specifier, program))
	rest = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
	return rest == "*" || strings.HasPrefix(rest, "-c") || strings.HasPrefix(rest, "-e")
}

func settingsFinding(env support.Context, file string, line int, ruleID string, level string, confidence string, message string) core.Finding {
	if line < 1 {
		line = 1
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      level,
		Path:       file,
		Line:       line,
		Column:     1,
		Message:    message,
		Confidence: confidence,
	})
}

// envSecretFindings reports literal credentials in an env or headers map.
// owner names what the map belongs to for the message; field is the map's
// key in the config so the reader can find it.
func envSecretFindings(env support.Context, file string, owner string, values *configNode, field string) []core.Finding {
	if values == nil || values.kind != nodeObject {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, key := range values.keys {
		node := values.fields[key]
		value, isString := node.str()
		if !isString {
			continue
		}
		confidence, literal := literalSecretConfidence(key, value)
		if !literal {
			continue
		}
		findings = append(findings, settingsFinding(env, file, node.line, "prompts.agent-config-literal-secret", "fail", confidence,
			fmt.Sprintf("%s sets %s %q to a literal credential; reference an environment variable such as ${%s} and keep the secret out of version control",
				owner, field, key, envReferenceName(key))))
	}
	return findings
}

func envReferenceName(key string) string {
	name := strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(key))
	if name == "AUTHORIZATION" {
		return "API_TOKEN"
	}
	return name
}
