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
	ruleWorkflowOverprovisionedSecrets = "ci.workflow-overprovisioned-secrets"
	ruleWorkflowBotConditions          = "ci.workflow-bot-conditions"
)

var (
	expressionStringLiteral = regexp.MustCompile(`'(?:[^']|'')*'`)
	// secretsContextUse captures what follows a bare secrets reference: "."
	// for a named secret, "[" for a computed index (literal ['NAME'] access
	// is already rewritten to .name), "" for the whole context.
	secretsContextUse = regexp.MustCompile(`(?:^|[^a-z0-9_.-])secrets\b\s*(\.|\[|)`)

	// spoofableBotCheck matches github.actor (or triggering_actor) compared
	// equal to a bot login, in either operand order, or github.actor_id
	// compared to a well-known bot account id: dependabot (49699333),
	// renovate (29139614), github-actions (41898282).
	spoofableBotCheck    = regexp.MustCompile(`github\.(?:actor|triggering_actor)\s*==\s*'[^']*\[bot\]'|'[^']*\[bot\]'\s*==\s*github\.(?:actor|triggering_actor)\b|github\.(?:actor_id|triggering_actor_id)\s*==\s*'?(?:49699333|29139614|41898282)\b`)
	botConditionTriggers = []string{"pull_request_target", "workflow_run"}
)

// overprovisionedSecretFindings reports expressions that hand the runner the
// whole secrets context: toJSON(secrets) (or any other use of the bare
// context) fails, and a computed index such as secrets[matrix.name], which
// forces GitHub to send every secret because it cannot tell which one is
// read, warns.
func overprovisionedSecretFindings(env support.Context, doc *workflowDoc) []core.Finding {
	findings := make([]core.Finding, 0)
	walkScalars(doc.root, nil, func(key, value *yaml.Node) {
		if !strings.Contains(value.Value, "${{") {
			return
		}
		cursor := doc.cursor(doc.spanFor(key, value))
		for _, expression := range workflowExpressions(value.Value) {
			level, kind := secretsContextExposure(expression.body)
			if level == "" {
				continue
			}
			line, column := cursor.locate(expression.raw)
			findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowOverprovisionedSecrets, level, line, column,
				fmt.Sprintf("%s %s, so every repository and organization secret is sent to the runner and any step can read them; reference each secret the step needs by name (secrets.NAME)", strings.TrimSpace(expression.raw), kind)))
		}
	})
	return findings
}

func secretsContextExposure(body string) (string, string) {
	stripped := expressionStringLiteral.ReplaceAllString(body, "''")
	level, kind := "", ""
	for _, match := range secretsContextUse.FindAllStringSubmatch(stripped, -1) {
		switch match[1] {
		case "":
			return "fail", "reads the entire secrets context"
		case "[":
			level, kind = "warn", "indexes secrets with a computed key"
		}
	}
	return level, kind
}

// botConditionFindings reports zizmor's bot-conditions audit on privileged
// triggers: a job or step if: that trusts github.actor == '<name>[bot]'.
// github.actor is whoever caused the latest event, so an attacker can get a
// bot to act last on their pull request and pass the check. A condition that
// also checks the pull request author (github.event.pull_request.user.login)
// is not reported.
func botConditionFindings(env support.Context, doc *workflowDoc) []core.Finding {
	trigger := firstTrigger(doc, botConditionTriggers)
	if doc.composite || trigger == "" {
		return nil
	}
	findings := make([]core.Finding, 0)
	report := func(owner *yaml.Node, label string) {
		key, condition := mappingEntry(owner, "if")
		if condition == nil || condition.Kind != yaml.ScalarNode {
			return
		}
		body := normalizeExpressionBody(strings.NewReplacer("${{", "", "}}", "").Replace(condition.Value))
		match := spoofableBotCheck.FindString(body)
		if match == "" || strings.Contains(body, "github.event.pull_request.user.login") {
			return
		}
		findings = append(findings, newWorkflowFinding(env, doc, ruleWorkflowBotConditions, "warn", key.Line, key.Column,
			fmt.Sprintf("%s in this %s workflow trusts %s, but github.actor is whoever triggered the latest event, not the pull request's author, so an attacker can make the bot act last on their pull request and pass the check; compare github.event.pull_request.user.login instead", label, trigger, match)))
	}
	for _, job := range doc.jobs {
		report(job.node, fmt.Sprintf("job %q condition", job.id))
		for _, step := range job.steps {
			report(step.node, "step condition")
		}
	}
	return findings
}
