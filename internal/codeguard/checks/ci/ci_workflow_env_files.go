package ci

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"gopkg.in/yaml.v3"
)

const (
	ruleWorkflowEnvFileInjection   = "ci.workflow-env-file-injection"
	ruleWorkflowInsecureCommands   = "ci.workflow-insecure-commands"
	insecureCommandsVariable       = "ACTIONS_ALLOW_UNSECURE_COMMANDS"
	envFileInjectionConsequenceMsg = "a newline in the value lets the sender define any variable (BASH_ENV, LD_PRELOAD, NODE_OPTIONS) or put their own directory first on PATH for every later step"
)

var (
	// envFileWritePattern matches a redirect, tee or PowerShell write into
	// the runner's environment or path file.
	envFileWritePattern  = regexp.MustCompile(`(?i)(?:>>?|\btee\b(?:\s+-a)?|\badd-content\b[^|;]*?|\bout-file\b[^|;]*?)\s*["']?\$(?:\{|env:)?(GITHUB_ENV|GITHUB_PATH)\b`)
	shellVariablePattern = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)`)
	// shellAssignmentPattern matches a whole-line shell variable assignment.
	shellAssignmentPattern = regexp.MustCompile(`^(?:export\s+|local\s+|declare\s+(?:-\S+\s+)*)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

	// envFileTriggers run in the base repository's context (secrets, a
	// write token) on events whose payload an outsider writes.
	envFileTriggers = []string{"pull_request_target", "workflow_run", "issue_comment", "issues", "discussion", "discussion_comment"}
)

// envFileInjectionFindings reports zizmor's github-env audit: a run: script
// in a workflow with an outsider-triggered privileged event that writes an
// attacker-controlled value into $GITHUB_ENV or $GITHUB_PATH, either as a
// ${{ }} expression or through a shell variable whose env: value (or an
// earlier assignment in the same script) carries one. Composite actions are
// skipped: their inputs are already reported by ci.workflow-script-injection,
// and their caller's trigger is unknown.
func envFileInjectionFindings(env support.Context, doc *workflowDoc) []core.Finding {
	trigger := firstTrigger(doc, envFileTriggers)
	if doc.composite || trigger == "" {
		return nil
	}
	_, workflowEnv := mappingEntry(doc.root, "env")
	findings := make([]core.Finding, 0)
	for _, job := range doc.jobs {
		_, jobEnv := mappingEntry(job.node, "env")
		for _, step := range job.steps {
			if step.run == nil || step.run.Kind != yaml.ScalarNode {
				continue
			}
			_, stepEnv := mappingEntry(step.node, "env")
			tainted := taintedEnvVariables(workflowEnv, jobEnv, stepEnv)
			findings = append(findings, stepEnvFileFindings(env, doc, trigger, step, tainted)...)
		}
	}
	return findings
}

// stepEnvFileFindings walks one script in order, so a variable assigned from
// untrusted data (message=$(echo "$TITLE" | ...)) stays tainted for the
// lines after it and a reassignment from trusted data clears it.
func stepEnvFileFindings(env support.Context, doc *workflowDoc, trigger string, step workflowStep, tainted map[string]string) []core.Finding {
	cursor := doc.cursor(doc.spanFor(step.runKey, step.run))
	var findings []core.Finding
	for _, logical := range logicalShellLines(step.run.Value) {
		if match := shellAssignmentPattern.FindStringSubmatch(logical.text); match != nil {
			_, tainted[match[1]] = untrustedShellSource(match[2], tainted)
			continue
		}
		match := envFileWritePattern.FindStringSubmatch(logical.text)
		if match == nil {
			continue
		}
		source, _ := untrustedShellSource(logical.text, tainted)
		if source == "" {
			continue
		}
		line, column := cursor.locate(logical.first)
		findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowEnvFileInjection, "fail", line, column,
			fmt.Sprintf("%s workflow writes %s to $%s; %s. Validate the value against a strict pattern first, or pass it to later steps through $GITHUB_OUTPUT and env: instead", trigger, source, strings.ToUpper(match[1]), envFileInjectionConsequenceMsg)))
	}
	return findings
}

// untrustedShellSource describes the attacker-controlled value text
// interpolates and the context it comes from, or "", "" when every value it
// interpolates is trusted.
func untrustedShellSource(text string, tainted map[string]string) (string, string) {
	for _, expression := range workflowExpressions(text) {
		if context := untrustedContext(expression.body, false); context != "" {
			return "untrusted " + context, context
		}
	}
	for _, match := range shellVariablePattern.FindAllStringSubmatch(text, -1) {
		if context := tainted[match[1]]; context != "" {
			return fmt.Sprintf("$%s (set from %s)", match[1], context), context
		}
	}
	return "", ""
}

// taintedEnvVariables maps each env: variable in scope to the untrusted
// context its value reads ("" for a trusted value). Later scopes override
// earlier ones, as step env overrides job and workflow env in GitHub.
func taintedEnvVariables(scopes ...*yaml.Node) map[string]string {
	tainted := map[string]string{}
	for _, scope := range scopes {
		if scope == nil || scope.Kind != yaml.MappingNode {
			continue
		}
		for idx := 0; idx+1 < len(scope.Content); idx += 2 {
			tainted[scope.Content[idx].Value] = untrustedScalarContext(scope.Content[idx+1])
		}
	}
	return tainted
}

func untrustedScalarContext(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	for _, expression := range workflowExpressions(node.Value) {
		if context := untrustedContext(expression.body, false); context != "" {
			return context
		}
	}
	return ""
}

// insecureCommandsFindings reports ACTIONS_ALLOW_UNSECURE_COMMANDS enabled in
// any env: block (workflow, job or step), which re-enables the deprecated
// ::set-env and ::add-path log commands: any text a step prints, including
// attacker-controlled output, can then set variables or PATH.
func insecureCommandsFindings(env support.Context, doc *workflowDoc) []core.Finding {
	scopes := make([]*yaml.Node, 0)
	if !doc.composite {
		_, workflowEnv := mappingEntry(doc.root, "env")
		scopes = append(scopes, workflowEnv)
	}
	for _, job := range doc.jobs {
		_, jobEnv := mappingEntry(job.node, "env")
		scopes = append(scopes, jobEnv)
		for _, step := range job.steps {
			_, stepEnv := mappingEntry(step.node, "env")
			scopes = append(scopes, stepEnv)
		}
	}
	findings := make([]core.Finding, 0)
	for _, scope := range scopes {
		key, value := mappingEntry(scope, insecureCommandsVariable)
		if value == nil || value.Kind != yaml.ScalarNode {
			continue
		}
		level, ok := insecureCommandsLevel(value.Value)
		if !ok {
			continue
		}
		findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowInsecureCommands, level, key.Line, key.Column,
			insecureCommandsVariable+" re-enables the deprecated ::set-env and ::add-path workflow commands, so anything a step prints (including attacker-controlled text in logs) can set environment variables or PATH; remove it and write to $GITHUB_ENV / $GITHUB_PATH instead"))
	}
	return findings
}

// insecureCommandsLevel fails a literal opt-in and warns on an expression
// that may evaluate to one; an explicit false is fine.
func insecureCommandsLevel(value string) (string, bool) {
	text := strings.TrimSpace(value)
	switch {
	case strings.Contains(text, "${{"):
		return "warn", true
	case isTruthyLiteral(text):
		return "fail", true
	default:
		return "", false
	}
}
