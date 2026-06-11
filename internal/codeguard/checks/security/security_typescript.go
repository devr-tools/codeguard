package security

import (
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	typeScriptExecPattern        = regexp.MustCompile(`\b(?:child_process\.)?(?:exec|execSync)\s*\(`)
	typeScriptSpawnShellPattern  = regexp.MustCompile(`\b(?:child_process\.)?(?:spawn|spawnSync)\s*\(`)
	typeScriptEvalPattern        = regexp.MustCompile(`\beval\s*\(|\bnew\s+Function\s*\(`)
	typeScriptInsecureTLSPattern = regexp.MustCompile(`\brejectUnauthorized\s*:\s*false\b`)
	typeScriptNodeTLSPattern     = regexp.MustCompile(`NODE_TLS_REJECT_UNAUTHORIZED\s*=\s*["']?0["']?`)
	typeScriptUnsafeHTMLPattern  = regexp.MustCompile(`(?:\.\s*(?:innerHTML|outerHTML)\s*=|\.\s*insertAdjacentHTML\s*\(|\bdocument\.(?:write|writeln)\s*\()`)
	typeScriptStringTimerPattern = regexp.MustCompile(`\b(?:setTimeout|setInterval)\s*\(`)
	typeScriptPostMessagePattern = regexp.MustCompile(`\b(?:window\s*\.\s*)?postMessage\s*\(`)
)

type typeScriptScanContext struct {
	env    support.Context
	file   string
	source string
	code   string
}

type typeScriptFindingSpec struct {
	pattern *regexp.Regexp
	ruleID  string
	level   string
	message string
}

func appendTypeScriptLineFindings(env support.Context, file string, lineNo int, line string) []core.Finding {
	switch {
	case typeScriptInsecureTLSPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: securityRuleID(file, "insecure-tls"), Level: "fail", Path: file, Line: lineNo, Column: 1, Message: support.ScriptLabelForPath(file) + " TLS verification is disabled"})}
	case typeScriptNodeTLSPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: securityRuleID(file, "insecure-tls"), Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "NODE_TLS_REJECT_UNAUTHORIZED disables TLS verification"})}
	default:
		return nil
	}
}

func typeScriptFindingsForFile(env support.Context, file string, source string) []core.Finding {
	ctx := typeScriptScanContext{
		env:    env,
		file:   file,
		source: source,
		code:   support.StripTypeScriptCommentsAndStrings(source),
	}
	findings := make([]core.Finding, 0, 8)
	findings = append(findings, regexTypeScriptSecurityFindings(ctx, typeScriptFindingSpec{pattern: typeScriptExecPattern, ruleID: securityRuleID(ctx.file, "shell-execution"), level: "warn", message: "shell execution primitive should be reviewed"})...)
	findings = append(findings, typeScriptSpawnFindings(ctx)...)
	findings = append(findings, regexTypeScriptSecurityFindings(ctx, typeScriptFindingSpec{pattern: typeScriptEvalPattern, ruleID: securityRuleID(ctx.file, "dynamic-code"), level: "warn", message: "dynamic code execution should be reviewed"})...)
	findings = append(findings, regexTypeScriptSecurityFindings(ctx, typeScriptFindingSpec{pattern: typeScriptUnsafeHTMLPattern, ruleID: securityRuleID(ctx.file, "unsafe-html-sink"), level: "warn", message: "unsafe HTML injection sink should be reviewed"})...)
	findings = append(findings, typeScriptStringTimerFindings(ctx)...)
	findings = append(findings, typeScriptPostMessageFindings(ctx)...)
	findings = append(findings, typeScriptAliasedShellFindings(ctx)...)
	findings = append(findings, typeScriptVMFindings(ctx)...)
	return findings
}

func regexTypeScriptSecurityFindings(ctx typeScriptScanContext, spec typeScriptFindingSpec) []core.Finding {
	matches := spec.pattern.FindAllStringIndex(ctx.code, -1)
	if len(matches) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0, len(matches))
	seenLines := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		line := support.LineNumberForOffset(ctx.source, match[0])
		if _, exists := seenLines[line]; exists {
			continue
		}
		seenLines[line] = struct{}{}
		findings = append(findings, ctx.env.NewFinding(support.FindingInput{
			RuleID:  spec.ruleID,
			Level:   spec.level,
			Path:    ctx.file,
			Line:    line,
			Column:  1,
			Message: spec.message,
		}))
	}
	return findings
}

func typeScriptAliasedShellFindings(ctx typeScriptScanContext) []core.Finding {
	findings := make([]core.Finding, 0)
	execAliases := collectTypeScriptNamedModuleBindings(ctx.source, "child_process", []string{"exec", "execSync"})
	spawnAliases := collectTypeScriptNamedModuleBindings(ctx.source, "child_process", []string{"spawn", "spawnSync"})
	childProcessNamespaces := collectTypeScriptNamespaceBindings(ctx.source, "child_process")
	ruleID := securityRuleID(ctx.file, "shell-execution")
	message := support.ScriptLabelForPath(ctx.file) + " shell execution primitive should be reviewed"

	for alias := range execAliases {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(alias) + `\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, typeScriptFindingSpec{pattern: pattern, ruleID: ruleID, level: "warn", message: "shell execution primitive should be reviewed"})...)
	}
	for alias := range spawnAliases {
		for _, line := range typeScriptCallLinesWithShellOption(ctx, alias, false) {
			findings = append(findings, newTypeScriptSecurityFinding(ctx, ruleID, line, message))
		}
	}
	for namespace := range childProcessNamespaces {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(namespace) + `\s*\.\s*(?:exec|execSync)\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, typeScriptFindingSpec{pattern: pattern, ruleID: ruleID, level: "warn", message: "shell execution primitive should be reviewed"})...)
		for _, line := range typeScriptCallLinesWithShellOption(ctx, namespace, true) {
			findings = append(findings, newTypeScriptSecurityFinding(ctx, ruleID, line, message))
		}
	}
	return dedupeTypeScriptFindings(findings)
}

func typeScriptVMFindings(ctx typeScriptScanContext) []core.Finding {
	findings := make([]core.Finding, 0)
	vmMethods := []string{"runInContext", "runInNewContext", "runInThisContext", "compileFunction"}
	directAliases := collectTypeScriptNamedModuleBindings(ctx.source, "vm", append([]string{"Script"}, vmMethods...))
	vmNamespaces := collectTypeScriptNamespaceBindings(ctx.source, "vm")

	for alias, original := range directAliases {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(alias) + `\s*\(`)
		if original == "Script" {
			pattern = regexp.MustCompile(`\bnew\s+` + regexp.QuoteMeta(alias) + `\s*\(`)
		}
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, typeScriptFindingSpec{pattern: pattern, ruleID: securityRuleID(ctx.file, "vm-dynamic-code"), level: "warn", message: "Node vm dynamic code execution should be reviewed"})...)
	}
	for namespace := range vmNamespaces {
		methodPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(namespace) + `\s*\.\s*(?:runInContext|runInNewContext|runInThisContext|compileFunction)\s*\(`)
		scriptPattern := regexp.MustCompile(`\bnew\s+` + regexp.QuoteMeta(namespace) + `\s*\.\s*Script\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, typeScriptFindingSpec{pattern: methodPattern, ruleID: securityRuleID(ctx.file, "vm-dynamic-code"), level: "warn", message: "Node vm dynamic code execution should be reviewed"})...)
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, typeScriptFindingSpec{pattern: scriptPattern, ruleID: securityRuleID(ctx.file, "vm-dynamic-code"), level: "warn", message: "Node vm dynamic code execution should be reviewed"})...)
	}
	return dedupeTypeScriptFindings(findings)
}

func typeScriptSpawnFindings(ctx typeScriptScanContext) []core.Finding {
	lines := typeScriptCallLinesWithShellOption(ctx, "child_process", true)
	findings := make([]core.Finding, 0, len(lines))
	ruleID := securityRuleID(ctx.file, "shell-execution")
	message := support.ScriptLabelForPath(ctx.file) + " shell execution primitive should be reviewed"
	for _, line := range lines {
		findings = append(findings, newTypeScriptSecurityFinding(ctx, ruleID, line, message))
	}
	return findings
}

func typeScriptStringTimerFindings(ctx typeScriptScanContext) []core.Finding {
	ruleID := securityRuleID(ctx.file, "string-timer-code")
	findings := make([]core.Finding, 0)
	for _, call := range support.FindScriptCalls(ctx.source, ctx.code, typeScriptStringTimerPattern) {
		if len(call.Args) == 0 || !support.HasStringLiteralValue(call.Args[0]) {
			continue
		}
		findings = append(findings, newTypeScriptSecurityFinding(ctx, ruleID, call.Line, support.ScriptLabelForPath(ctx.file)+" string-based timer execution should be reviewed"))
	}
	return dedupeTypeScriptFindings(findings)
}

func typeScriptPostMessageFindings(ctx typeScriptScanContext) []core.Finding {
	ruleID := securityRuleID(ctx.file, "postmessage-wildcard")
	findings := make([]core.Finding, 0)
	for _, call := range support.FindScriptCalls(ctx.source, ctx.code, typeScriptPostMessagePattern) {
		if len(call.Args) < 2 || !support.HasStringLiteralValue(call.Args[1], "*") {
			continue
		}
		findings = append(findings, newTypeScriptSecurityFinding(ctx, ruleID, call.Line, support.ScriptLabelForPath(ctx.file)+" postMessage wildcard origin should be reviewed"))
	}
	return dedupeTypeScriptFindings(findings)
}
