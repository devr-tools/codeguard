package security

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	secretPattern                = regexp.MustCompile(`(?i)(secret|token|api[_-]?key|password)\s*[:=]\s*["'][^"']{8,}["']`)
	privateKeyPattern            = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	typeScriptExecPattern        = regexp.MustCompile(`\b(?:child_process\.)?(?:exec|execSync)\s*\(`)
	typeScriptSpawnShellPattern  = regexp.MustCompile(`\b(?:child_process\.)?(?:spawn|spawnSync)\s*\([^)]*shell\s*:\s*true`)
	typeScriptEvalPattern        = regexp.MustCompile(`\beval\s*\(|\bnew\s+Function\s*\(`)
	typeScriptInsecureTLSPattern = regexp.MustCompile(`\brejectUnauthorized\s*:\s*false\b`)
	typeScriptNodeTLSPattern     = regexp.MustCompile(`NODE_TLS_REJECT_UNAUTHORIZED\s*=\s*["']?0["']?`)
	typeScriptUnsafeHTMLPattern  = regexp.MustCompile(`(?:\.\s*(?:innerHTML|outerHTML)\s*=|\.\s*insertAdjacentHTML\s*\(|\bdocument\.(?:write|writeln)\s*\()`)
	tsNamedImportPattern         = regexp.MustCompile(`(?m)^\s*import\s*{\s*([^}]+)\s*}\s*from\s*["'](?:node:)?%s["']`)
	tsNamespaceImportPattern     = regexp.MustCompile(`(?m)^\s*import\s+\*\s+as\s+([A-Za-z_$][\w$]*)\s*from\s*["'](?:node:)?%s["']`)
	tsDefaultImportPattern       = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z_$][\w$]*)\s*from\s*["'](?:node:)?%s["']`)
	tsNamedRequirePattern        = regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+{\s*([^}]+)\s*}\s*=\s*require\(\s*["'](?:node:)?%s["']\s*\)`)
	tsNamespaceRequirePattern    = regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*require\(\s*["'](?:node:)?%s["']\s*\)`)
	pythonShellPattern           = regexp.MustCompile(`\bsubprocess\.(?:run|Popen|call|check_call|check_output|getoutput|getstatusoutput)\s*\(.*shell\s*=\s*True`)
	pythonSystemPattern          = regexp.MustCompile(`\bos\.system\s*\(`)
	pythonEvalPattern            = regexp.MustCompile(`\b(?:eval|exec)\s*\(`)
	pythonInsecureTLSPattern     = regexp.MustCompile(`\bverify\s*=\s*False\b|\bssl\._create_unverified_context\s*\(`)
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		findings = append(findings, env.ScanTargetFiles(target, "security", func(string) bool { return true }, func(file string, data []byte) []core.Finding {
			return findingsForFile(env, file, data)
		})...)
		findings = append(findings, commandFindings(ctx, env, target)...)

		if isGoTarget(target) {
			mode := strings.ToLower(strings.TrimSpace(env.Config.Checks.SecurityRules.GovulncheckMode))
			switch mode {
			case "", "off":
			case "auto", "required":
				govulnFindings, err := env.RunGovulncheck(ctx, target.Path, env.Config.Checks.SecurityRules.GovulncheckCommand)
				if err != nil {
					level := "warn"
					if mode == "required" {
						level = "fail"
					}
					findings = append(findings, env.NewFinding(support.FindingInput{
						RuleID:  "security.govulncheck",
						Level:   level,
						Message: err.Error(),
					}))
				}
				findings = append(findings, govulnFindings...)
			default:
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID:  "security.govulncheck",
					Level:   "fail",
					Message: "govulncheck_mode must be off, auto, or required",
				}))
			}
		}
	}
	return env.FinalizeSection("security", "Security", findings)
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	checks := env.Config.Checks.SecurityRules.LanguageCommands[normalizedLanguage(target.Language)]
	findings := make([]core.Finding, 0, len(checks))
	for _, check := range checks {
		output, err := env.RunCommandCheck(ctx, target.Path, check)
		if err == nil {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "security.command-check",
			Level:   "fail",
			Message: commandFailureMessage(target, check, output, err),
		}))
	}
	return findings
}

func commandFailureMessage(target core.TargetConfig, check core.CommandCheckConfig, output string, err error) string {
	message := fmt.Sprintf("target %q security command %q failed", target.Name, check.Name)
	output = trimmedOutput(output)
	if output != "" {
		message += ": " + output
	} else if err != nil {
		message += ": " + err.Error()
	}
	return message
}

func isGoTarget(target core.TargetConfig) bool {
	language := normalizedLanguage(target.Language)
	return language == "" || language == "go"
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func trimmedOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	output = strings.Join(strings.Fields(output), " ")
	if len(output) > 240 {
		return output[:237] + "..."
	}
	return output
}

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	isTypeScript := isTypeScriptFile(file)
	for idx, line := range lines {
		lineNo := idx + 1
		switch {
		case secretPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.hardcoded-secret", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "possible hardcoded secret detected"}))
		case privateKeyPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.private-key", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "private key material detected"}))
		case strings.Contains(line, "InsecureSkipVerify: true"):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "InsecureSkipVerify is enabled"}))
		case strings.Contains(line, "exec.Command(") || strings.Contains(line, "os/exec"):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "shell execution primitive should be reviewed"}))
		}

		switch {
		case isTypeScript && typeScriptInsecureTLSPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.typescript.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "TypeScript TLS verification is disabled"}))
		case isTypeScript && typeScriptNodeTLSPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.typescript.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "NODE_TLS_REJECT_UNAUTHORIZED disables TLS verification"}))
		case isPythonFile(file) && pythonInsecureTLSPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.python.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Python TLS verification is disabled"}))
		case isPythonFile(file) && (pythonShellPattern.MatchString(line) || pythonSystemPattern.MatchString(line)):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.python.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "Python shell execution primitive should be reviewed"}))
		case isPythonFile(file) && pythonEvalPattern.MatchString(line):
			findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.python.dynamic-code", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "dynamic code execution should be reviewed"}))
		}
	}
	if isTypeScript {
		findings = append(findings, typeScriptFindingsForFile(env, file, strings.ReplaceAll(string(data), "\r\n", "\n"))...)
	}
	return findings
}

func typeScriptFindingsForFile(env support.Context, file string, source string) []core.Finding {
	code := stripTypeScriptCommentsAndStrings(source)
	findings := make([]core.Finding, 0, 8)
	findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, typeScriptExecPattern, "security.typescript.shell-execution", "warn", "TypeScript shell execution primitive should be reviewed")...)
	findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, typeScriptSpawnShellPattern, "security.typescript.shell-execution", "warn", "TypeScript shell execution primitive should be reviewed")...)
	findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, typeScriptEvalPattern, "security.typescript.dynamic-code", "warn", "dynamic code execution should be reviewed")...)
	findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, typeScriptUnsafeHTMLPattern, "security.typescript.unsafe-html-sink", "warn", "unsafe HTML injection sink should be reviewed")...)
	findings = append(findings, typeScriptAliasedShellFindings(env, file, source, code)...)
	findings = append(findings, typeScriptVMFindings(env, file, source, code)...)
	return findings
}

func regexTypeScriptSecurityFindings(env support.Context, file string, source string, code string, pattern *regexp.Regexp, ruleID string, level string, message string) []core.Finding {
	matches := pattern.FindAllStringIndex(code, -1)
	if len(matches) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0, len(matches))
	seenLines := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		line := lineNumberForOffset(source, match[0])
		if _, exists := seenLines[line]; exists {
			continue
		}
		seenLines[line] = struct{}{}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  ruleID,
			Level:   level,
			Path:    file,
			Line:    line,
			Column:  1,
			Message: message,
		}))
	}
	return findings
}

func typeScriptAliasedShellFindings(env support.Context, file string, source string, code string) []core.Finding {
	findings := make([]core.Finding, 0)
	execAliases := collectTypeScriptNamedModuleBindings(source, "child_process", []string{"exec", "execSync"})
	spawnAliases := collectTypeScriptNamedModuleBindings(source, "child_process", []string{"spawn", "spawnSync"})
	childProcessNamespaces := collectTypeScriptNamespaceBindings(source, "child_process")

	for alias := range execAliases {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(alias) + `\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, pattern, "security.typescript.shell-execution", "warn", "TypeScript shell execution primitive should be reviewed")...)
	}

	for alias := range spawnAliases {
		for _, line := range typeScriptCallLinesWithShellOption(code, alias, false) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "security.typescript.shell-execution",
				Level:   "warn",
				Path:    file,
				Line:    line,
				Column:  1,
				Message: "TypeScript shell execution primitive should be reviewed",
			}))
		}
	}

	for namespace := range childProcessNamespaces {
		execPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(namespace) + `\s*\.\s*(?:exec|execSync)\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, execPattern, "security.typescript.shell-execution", "warn", "TypeScript shell execution primitive should be reviewed")...)
		for _, line := range typeScriptCallLinesWithShellOption(code, namespace, true) {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "security.typescript.shell-execution",
				Level:   "warn",
				Path:    file,
				Line:    line,
				Column:  1,
				Message: "TypeScript shell execution primitive should be reviewed",
			}))
		}
	}
	return dedupeTypeScriptFindings(findings)
}

func typeScriptVMFindings(env support.Context, file string, source string, code string) []core.Finding {
	findings := make([]core.Finding, 0)
	vmMethods := []string{"runInContext", "runInNewContext", "runInThisContext", "compileFunction"}
	directAliases := collectTypeScriptNamedModuleBindings(source, "vm", append([]string{"Script"}, vmMethods...))
	vmNamespaces := collectTypeScriptNamespaceBindings(source, "vm")

	for alias, original := range directAliases {
		var pattern *regexp.Regexp
		if original == "Script" {
			pattern = regexp.MustCompile(`\bnew\s+` + regexp.QuoteMeta(alias) + `\s*\(`)
		} else {
			pattern = regexp.MustCompile(`\b` + regexp.QuoteMeta(alias) + `\s*\(`)
		}
		findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, pattern, "security.typescript.vm-dynamic-code", "warn", "Node vm dynamic code execution should be reviewed")...)
	}

	for namespace := range vmNamespaces {
		methodPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(namespace) + `\s*\.\s*(?:runInContext|runInNewContext|runInThisContext|compileFunction)\s*\(`)
		scriptPattern := regexp.MustCompile(`\bnew\s+` + regexp.QuoteMeta(namespace) + `\s*\.\s*Script\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, methodPattern, "security.typescript.vm-dynamic-code", "warn", "Node vm dynamic code execution should be reviewed")...)
		findings = append(findings, regexTypeScriptSecurityFindings(env, file, source, code, scriptPattern, "security.typescript.vm-dynamic-code", "warn", "Node vm dynamic code execution should be reviewed")...)
	}

	return dedupeTypeScriptFindings(findings)
}

func collectTypeScriptNamedModuleBindings(source string, module string, allowed []string) map[string]string {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = struct{}{}
	}
	aliases := make(map[string]string)
	for _, spec := range collectTypeScriptBindingSpecs(source, module, tsNamedImportPattern, tsNamedRequirePattern) {
		original, alias := parseTypeScriptBindingSpec(spec)
		if _, ok := allowedSet[original]; !ok {
			continue
		}
		aliases[alias] = original
	}
	return aliases
}

func collectTypeScriptNamespaceBindings(source string, module string) map[string]struct{} {
	namespaces := make(map[string]struct{})
	for _, pattern := range []*regexp.Regexp{tsNamespaceImportPattern, tsDefaultImportPattern, tsNamespaceRequirePattern} {
		re := regexp.MustCompile(strings.ReplaceAll(pattern.String(), "%s", regexp.QuoteMeta(module)))
		matches := re.FindAllStringSubmatch(source, -1)
		for _, match := range matches {
			if len(match) > 1 {
				namespaces[match[1]] = struct{}{}
			}
		}
	}
	return namespaces
}

func collectTypeScriptBindingSpecs(source string, module string, patterns ...*regexp.Regexp) []string {
	specs := make([]string, 0)
	for _, pattern := range patterns {
		re := regexp.MustCompile(strings.ReplaceAll(pattern.String(), "%s", regexp.QuoteMeta(module)))
		matches := re.FindAllStringSubmatch(source, -1)
		for _, match := range matches {
			if len(match) > 1 {
				specs = append(specs, splitTypeScriptBindingSpecs(match[1])...)
			}
		}
	}
	return specs
}

func splitTypeScriptBindingSpecs(source string) []string {
	parts := strings.Split(source, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func parseTypeScriptBindingSpec(spec string) (string, string) {
	if before, after, ok := strings.Cut(spec, " as "); ok {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	if before, after, ok := strings.Cut(spec, ":"); ok {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	spec = strings.TrimSpace(spec)
	return spec, spec
}

func typeScriptCallLinesWithShellOption(code string, alias string, namespaced bool) []int {
	lines := make([]int, 0)
	seen := make(map[int]struct{})
	patternText := `\b` + regexp.QuoteMeta(alias)
	if namespaced {
		patternText += `\s*\.\s*(?:spawn|spawnSync)\s*\(`
	} else {
		patternText += `\s*\(`
	}
	pattern := regexp.MustCompile(patternText)
	for _, match := range pattern.FindAllStringIndex(code, -1) {
		if !hasShellTrueNearOffset(code, match[0]) {
			continue
		}
		line := lineNumberForOffset(code, match[0])
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func hasShellTrueNearOffset(code string, offset int) bool {
	limit := offset + 240
	if limit > len(code) {
		limit = len(code)
	}
	return strings.Contains(code[offset:limit], "shell") && regexp.MustCompile(`shell\s*:\s*true`).MatchString(code[offset:limit])
}

func dedupeTypeScriptFindings(findings []core.Finding) []core.Finding {
	if len(findings) <= 1 {
		return findings
	}
	seen := make(map[string]struct{}, len(findings))
	deduped := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		key := finding.RuleID + "|" + finding.Path + "|" + fmt.Sprintf("%d", finding.Line)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, finding)
	}
	return deduped
}

func isTypeScriptFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func isPythonFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".py")
}

func stripTypeScriptCommentsAndStrings(source string) string {
	out := []byte(source)
	state := "code"
	for idx := 0; idx < len(out); idx++ {
		switch state {
		case "code":
			if idx+1 < len(out) && out[idx] == '/' && out[idx+1] == '/' {
				out[idx], out[idx+1] = ' ', ' '
				state = "line-comment"
				idx++
				continue
			}
			if idx+1 < len(out) && out[idx] == '/' && out[idx+1] == '*' {
				out[idx], out[idx+1] = ' ', ' '
				state = "block-comment"
				idx++
				continue
			}
			switch out[idx] {
			case '\'', '"':
				quote := out[idx]
				out[idx] = ' '
				state = string(quote)
			case '`':
				out[idx] = ' '
				state = "template"
			}
		case "line-comment":
			if out[idx] == '\n' {
				state = "code"
				continue
			}
			out[idx] = ' '
		case "block-comment":
			if idx+1 < len(out) && out[idx] == '*' && out[idx+1] == '/' {
				out[idx], out[idx+1] = ' ', ' '
				state = "code"
				idx++
				continue
			}
			if out[idx] != '\n' {
				out[idx] = ' '
			}
		case "'":
			if out[idx] == '\\' && idx+1 < len(out) {
				out[idx], out[idx+1] = ' ', ' '
				idx++
				continue
			}
			if out[idx] == '\n' {
				state = "code"
				continue
			}
			if out[idx] == '\'' {
				state = "code"
			}
			out[idx] = ' '
		case `"`:
			if out[idx] == '\\' && idx+1 < len(out) {
				out[idx], out[idx+1] = ' ', ' '
				idx++
				continue
			}
			if out[idx] == '\n' {
				state = "code"
				continue
			}
			if out[idx] == '"' {
				state = "code"
			}
			out[idx] = ' '
		case "template":
			if out[idx] == '\\' && idx+1 < len(out) {
				if out[idx] != '\n' {
					out[idx] = ' '
				}
				if out[idx+1] != '\n' {
					out[idx+1] = ' '
				}
				idx++
				continue
			}
			if out[idx] == '`' {
				out[idx] = ' '
				state = "code"
				continue
			}
			if out[idx] != '\n' {
				out[idx] = ' '
			}
		}
	}
	return string(out)
}

func lineNumberForOffset(source string, offset int) int {
	if offset <= 0 {
		return 1
	}
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(source[:offset], "\n")
}
