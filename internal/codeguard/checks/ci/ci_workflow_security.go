package ci

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"gopkg.in/yaml.v3"
)

const (
	ruleWorkflowScriptInjection  = "ci.workflow-script-injection"
	ruleWorkflowDangerousTrigger = "ci.workflow-dangerous-trigger"
	ruleWorkflowWriteAll         = "ci.workflow-write-all-permissions"
	ruleWorkflowMissingPerms     = "ci.workflow-missing-permissions"
	ruleWorkflowRemoteScriptPipe = "ci.workflow-remote-script-pipe"
	ruleWorkflowForkTrigger      = "ci.workflow-fork-trigger-secrets"

	defaultScopesNote = "GITHUB_TOKEN then gets the repository or organization default, which is read-write on every scope for repositories created before February 2023 or set to the permissive default"
)

var (
	remoteScriptPipePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:curl|wget)\b[^|;&\n]*\|\s*(?:sudo\s+(?:-\S+\s+)*)?(?:ba|z|k|da)?sh\b`),
		regexp.MustCompile(`(?i)\b(?:ba|z|k)?sh\s+(?:-\S+\s+)*<\(\s*(?:curl|wget)\b`),
		regexp.MustCompile(`(?i)\b(?:ba|z|k)?sh\s+-c\s+["']?\$\(\s*(?:curl|wget)\b`),
	}
	ghPRCheckoutPattern  = regexp.MustCompile(`(?i)\bgh\s+pr\s+checkout\b`)
	gitFetchRefPattern   = regexp.MustCompile(`(?i)\bgit\s+(?:checkout|fetch|switch|pull)\b`)
	pullRefLiteral       = regexp.MustCompile(`(?i)\brefs/pull/|\bpull/\$\{\{`)
	forkTriggerNames     = []string{"pull_request_target", "workflow_run"}
	reusableOnlyTriggers = map[string]bool{"workflow_call": true}
)

// workflowSecurityFindings applies the GitHub Actions attack-surface rules to
// workflow files and composite actions. The per-file results depend only on
// the file's own bytes, so they go through the cached per-file scan.
func workflowSecurityFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "ci-workflow-security", isWorkflowSecurityFile, func(rel string, data []byte) []core.Finding {
		doc := parseWorkflowDoc(rel, data)
		if doc == nil {
			return nil
		}
		return evaluateWorkflowDoc(env, doc)
	})
}

func evaluateWorkflowDoc(env support.Context, doc *workflowDoc) []core.Finding {
	findings := make([]core.Finding, 0) //nolint:prealloc // each rule contributes a variable number of findings
	findings = append(findings, scriptInjectionFindings(env, doc)...)
	findings = append(findings, dangerousTriggerFindings(env, doc)...)
	findings = append(findings, permissionFindings(env, doc)...)
	findings = append(findings, remoteScriptPipeFindings(env, doc)...)
	findings = append(findings, forkTriggerSecretFindings(env, doc)...)
	return findings
}

func newWorkflowFinding(env support.Context, doc *workflowDoc, ruleID, level string, line, column int, message string) core.Finding {
	kind := "workflow"
	if doc.composite {
		kind = "composite_action"
	}
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      level,
		Path:       doc.rel,
		Line:       line,
		Column:     column,
		Message:    message,
		Confidence: core.ConfidenceHigh,
		Metadata:   map[string]string{"file_kind": kind},
	})
}

// injectionSink is a scalar whose text GitHub evaluates as code once the
// ${{ }} expressions in it are substituted.
type injectionSink struct {
	key, value *yaml.Node
	label      string
}

func stepInjectionSinks(step workflowStep) []injectionSink {
	sinks := make([]injectionSink, 0, 2)
	if step.run != nil && step.run.Kind == yaml.ScalarNode {
		sinks = append(sinks, injectionSink{key: step.runKey, value: step.run, label: "run"})
	}
	if step.usesAction("actions/github-script") {
		if key, script := mappingEntry(step.with, "script"); script != nil && script.Kind == yaml.ScalarNode {
			sinks = append(sinks, injectionSink{key: key, value: script, label: "actions/github-script script"})
		}
	}
	return sinks
}

func scriptInjectionFindings(env support.Context, doc *workflowDoc) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, job := range doc.jobs {
		for _, step := range job.steps {
			for _, sink := range stepInjectionSinks(step) {
				findings = append(findings, sinkInjectionFindings(env, doc, sink)...)
			}
		}
	}
	return findings
}

func sinkInjectionFindings(env support.Context, doc *workflowDoc, sink injectionSink) []core.Finding {
	cursor := doc.cursor(doc.spanFor(sink.key, sink.value))
	findings := make([]core.Finding, 0)
	for _, expression := range workflowExpressions(sink.value.Value) {
		line, column := cursor.locate(expression.raw)
		context := untrustedContext(expression.body, doc.composite)
		if context == "" {
			continue
		}
		findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowScriptInjection, "fail", line, column,
			fmt.Sprintf("untrusted expression %s is interpolated directly into %s; pass it through env: and reference it as a quoted shell variable instead", context, sink.label)))
	}
	return findings
}

func (doc *workflowDoc) hasForkTrigger() (string, bool) {
	for _, name := range forkTriggerNames {
		if doc.triggers[name] {
			return name, true
		}
	}
	return "", false
}

func dangerousTriggerFindings(env support.Context, doc *workflowDoc) []core.Finding {
	trigger, ok := doc.hasForkTrigger()
	if !ok {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, job := range doc.jobs {
		for _, step := range job.steps {
			if line, column, ref, found := prCheckoutLocation(doc, step); found {
				findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowDangerousTrigger, "fail", line, column,
					fmt.Sprintf("%s workflow job %q checks out pull request code (%s) while running with the base repository's secrets and write token; check out the base ref, or move the build to an unprivileged pull_request workflow", trigger, job.id, ref)))
			}
		}
	}
	return findings
}

// prCheckoutLocation reports where a step fetches the pull request's own code:
// an actions/checkout ref, or a git/gh command in a run script.
func prCheckoutLocation(doc *workflowDoc, step workflowStep) (int, int, string, bool) {
	if step.usesAction("actions/checkout") {
		key, ref := mappingEntry(step.with, "ref")
		if ref != nil && ref.Kind == yaml.ScalarNode {
			if found := prRefIn(ref.Value); found != "" {
				line, column := doc.cursor(doc.spanFor(key, ref)).locate(strings.TrimSpace(ref.Value))
				return line, column, found, true
			}
		}
	}
	if step.run == nil || step.run.Kind != yaml.ScalarNode {
		return 0, 0, "", false
	}
	cursor := doc.cursor(doc.spanFor(step.runKey, step.run))
	for _, line := range strings.Split(step.run.Value, "\n") {
		found := ""
		switch {
		case ghPRCheckoutPattern.MatchString(line):
			found = "gh pr checkout"
		case gitFetchRefPattern.MatchString(line):
			found = prRefIn(line)
		}
		if found != "" {
			lineNo, column := cursor.locate(strings.TrimSpace(line))
			return lineNo, column, found, true
		}
	}
	return 0, 0, "", false
}

func prRefIn(text string) string {
	for _, expression := range workflowExpressions(text) {
		if ref := prCodeRef(expression.body); ref != "" {
			return ref
		}
	}
	if pullRefLiteral.MatchString(text) {
		return "refs/pull/*"
	}
	return ""
}

func permissionFindings(env support.Context, doc *workflowDoc) []core.Finding {
	if doc.composite {
		return nil
	}
	findings := make([]core.Finding, 0)
	_, topLevel := mappingEntry(doc.root, "permissions")
	if isWriteAll(topLevel) {
		findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowWriteAll, "fail", topLevel.Line, topLevel.Column,
			"workflow grants permissions: write-all, giving every job a token that can push code, edit releases and change repository settings; list only the scopes each job needs"))
	}
	missing := make([]string, 0)
	for _, job := range doc.jobs {
		if isWriteAll(job.permissions) {
			findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowWriteAll, "fail", job.permissions.Line, job.permissions.Column,
				fmt.Sprintf("job %q grants permissions: write-all; list only the scopes the job needs", job.id)))
		}
		if job.permissions == nil {
			missing = append(missing, job.id)
		}
	}
	if topLevel == nil && len(missing) > 0 && !onlyReusableTriggers(doc.triggers) {
		findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowMissingPerms, "warn", 1, 1,
			fmt.Sprintf("workflow has no top-level permissions block and job(s) %s set none; %s. Declare permissions: contents: read at the top and widen per job", quotedList(missing), defaultScopesNote)))
	}
	return findings
}

func isWriteAll(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode && strings.EqualFold(strings.TrimSpace(node.Value), "write-all")
}

// onlyReusableTriggers reports a workflow that only runs via workflow_call:
// its token is capped by the caller's permissions, so the caller is where a
// missing block matters.
func onlyReusableTriggers(triggers map[string]bool) bool {
	if len(triggers) == 0 {
		return false
	}
	for name := range triggers {
		if !reusableOnlyTriggers[name] {
			return false
		}
	}
	return true
}

func quotedList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}

func remoteScriptPipeFindings(env support.Context, doc *workflowDoc) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, job := range doc.jobs {
		for _, step := range job.steps {
			if step.run == nil || step.run.Kind != yaml.ScalarNode {
				continue
			}
			cursor := doc.cursor(doc.spanFor(step.runKey, step.run))
			for _, logical := range logicalShellLines(step.run.Value) {
				if !matchesRemoteScriptPipe(logical.text) {
					continue
				}
				line, column := cursor.locate(logical.first)
				findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowRemoteScriptPipe, "warn", line, column,
					"downloads a remote script and pipes it straight into a shell, so whoever controls that URL (or its CDN) runs code in CI; download to a file, verify a pinned checksum or signature, then execute it"))
			}
		}
	}
	return findings
}

func matchesRemoteScriptPipe(text string) bool {
	for _, pattern := range remoteScriptPipePatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

type logicalShellLine struct {
	text  string // the command with backslash continuations joined
	first string // the first physical line, trimmed, for locating it
}

func logicalShellLines(script string) []logicalShellLine {
	physical := strings.Split(script, "\n")
	out := make([]logicalShellLine, 0, len(physical))
	var builder strings.Builder
	first := ""
	for _, line := range physical {
		trimmed := strings.TrimSpace(line)
		if builder.Len() == 0 {
			first = trimmed
		}
		if continued, ok := strings.CutSuffix(trimmed, "\\"); ok {
			builder.WriteString(continued)
			builder.WriteString(" ")
			continue
		}
		builder.WriteString(trimmed)
		if text := builder.String(); strings.TrimSpace(text) != "" {
			out = append(out, logicalShellLine{text: text, first: first})
		}
		builder.Reset()
	}
	if builder.Len() > 0 {
		out = append(out, logicalShellLine{text: builder.String(), first: first})
	}
	return out
}

func forkTriggerSecretFindings(env support.Context, doc *workflowDoc) []core.Finding {
	if !doc.triggers["pull_request_target"] {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, job := range doc.jobs {
		names, line, column := jobSecretReferences(doc, job)
		if len(names) == 0 {
			continue
		}
		findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowForkTrigger, "warn", line, column,
			fmt.Sprintf("pull_request_target job %q exposes %s to a run any fork can trigger; keep secrets out of jobs that touch pull request content, or gate the job behind an environment with required reviewers", job.id, strings.Join(names, ", "))))
	}
	return findings
}

// jobSecretReferences collects the secrets a job reads (excluding the
// automatic GITHUB_TOKEN, which the permissions rules cover) and the location
// of the first reference.
func jobSecretReferences(doc *workflowDoc, job workflowJob) ([]string, int, int) {
	seen := map[string]bool{}
	firstLine, firstColumn := 0, 0
	record := func(name string, line, column int) {
		seen[name] = true
		if firstLine == 0 || line < firstLine {
			firstLine, firstColumn = line, column
		}
	}
	walkScalars(job.node, nil, func(key, value *yaml.Node) {
		if key != nil && key.Value == "secrets" && strings.EqualFold(strings.TrimSpace(value.Value), "inherit") {
			record("all secrets (secrets: inherit)", value.Line, value.Column)
			return
		}
		cursor := doc.cursor(doc.spanFor(key, value))
		for _, expression := range workflowExpressions(value.Value) {
			found := referencedSecrets(expression.body)
			if len(found) == 0 {
				continue
			}
			line, column := cursor.locate(expression.raw)
			for _, name := range found {
				record("secrets."+name, line, column)
			}
		}
	})
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, firstLine, firstColumn
}

// walkScalars visits every scalar value under node with the mapping key that
// introduced it (nil for sequence items).
func walkScalars(node, key *yaml.Node, visit func(key, value *yaml.Node)) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.ScalarNode:
		visit(key, node)
	case yaml.MappingNode:
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			walkScalars(node.Content[idx+1], node.Content[idx], visit)
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			walkScalars(item, nil, visit)
		}
	}
}
