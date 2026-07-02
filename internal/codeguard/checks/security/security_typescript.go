package security

import (
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	typeScriptExecPattern        = regexp.MustCompile(`\b(?:child_process\.)?(?:exec|execSync)\s*\(`)
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
	findings = append(findings, regexTypeScriptSecurityFindings(ctx, support.ScriptRegexSpec{Pattern: typeScriptExecPattern, RuleID: securityRuleID(ctx.file, "shell-execution"), Level: "warn", Message: "shell execution primitive should be reviewed"})...)
	findings = append(findings, typeScriptSpawnFindings(ctx)...)
	findings = append(findings, regexTypeScriptSecurityFindings(ctx, support.ScriptRegexSpec{Pattern: typeScriptEvalPattern, RuleID: securityRuleID(ctx.file, "dynamic-code"), Level: "warn", Message: "dynamic code execution should be reviewed"})...)
	findings = append(findings, typeScriptUnsafeHTMLSinkFindings(ctx, support.ScriptSyntaxTree(env, file, source))...)
	findings = append(findings, typeScriptStringTimerFindings(ctx)...)
	findings = append(findings, typeScriptPostMessageFindings(ctx)...)
	findings = append(findings, typeScriptAliasedShellFindings(ctx)...)
	findings = append(findings, typeScriptVMFindings(ctx)...)
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
		pattern := compileDynamicPattern(`\b` + regexp.QuoteMeta(alias) + `\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, support.ScriptRegexSpec{Pattern: pattern, RuleID: ruleID, Level: "warn", Message: "shell execution primitive should be reviewed"})...)
	}
	for alias := range spawnAliases {
		for _, line := range typeScriptCallLinesWithShellOption(ctx, alias, false) {
			findings = append(findings, newTypeScriptSecurityFinding(ctx, ruleID, line, message))
		}
	}
	for namespace := range childProcessNamespaces {
		pattern := compileDynamicPattern(`\b` + regexp.QuoteMeta(namespace) + `\s*\.\s*(?:exec|execSync)\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, support.ScriptRegexSpec{Pattern: pattern, RuleID: ruleID, Level: "warn", Message: "shell execution primitive should be reviewed"})...)
		for _, line := range typeScriptCallLinesWithShellOption(ctx, namespace, true) {
			findings = append(findings, newTypeScriptSecurityFinding(ctx, ruleID, line, message))
		}
	}
	return dedupeTypeScriptFindings(findings)
}

func typeScriptVMFindings(ctx typeScriptScanContext) []core.Finding {
	findings := make([]core.Finding, 0) //nolint:prealloc // count not known up front; each alias appends a variable number
	vmMethods := []string{"runInContext", "runInNewContext", "runInThisContext", "compileFunction"}
	directAliases := collectTypeScriptNamedModuleBindings(ctx.source, "vm", append([]string{"Script"}, vmMethods...))
	vmNamespaces := collectTypeScriptNamespaceBindings(ctx.source, "vm")

	for alias, original := range directAliases {
		pattern := compileDynamicPattern(`\b` + regexp.QuoteMeta(alias) + `\s*\(`)
		if original == "Script" {
			pattern = compileDynamicPattern(`\bnew\s+` + regexp.QuoteMeta(alias) + `\s*\(`)
		}
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, support.ScriptRegexSpec{Pattern: pattern, RuleID: securityRuleID(ctx.file, "vm-dynamic-code"), Level: "warn", Message: "Node vm dynamic code execution should be reviewed"})...)
	}
	for namespace := range vmNamespaces {
		methodPattern := compileDynamicPattern(`\b` + regexp.QuoteMeta(namespace) + `\s*\.\s*(?:runInContext|runInNewContext|runInThisContext|compileFunction)\s*\(`)
		scriptPattern := compileDynamicPattern(`\bnew\s+` + regexp.QuoteMeta(namespace) + `\s*\.\s*Script\s*\(`)
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, support.ScriptRegexSpec{Pattern: methodPattern, RuleID: securityRuleID(ctx.file, "vm-dynamic-code"), Level: "warn", Message: "Node vm dynamic code execution should be reviewed"})...)
		findings = append(findings, regexTypeScriptSecurityFindings(ctx, support.ScriptRegexSpec{Pattern: scriptPattern, RuleID: securityRuleID(ctx.file, "vm-dynamic-code"), Level: "warn", Message: "Node vm dynamic code execution should be reviewed"})...)
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

func regexTypeScriptSecurityFindings(ctx typeScriptScanContext, spec support.ScriptRegexSpec) []core.Finding {
	return support.ScriptRegexFindings(ctx.env, ctx.file, support.ScriptScanContext{Source: ctx.source, Code: ctx.code}, spec)
}
