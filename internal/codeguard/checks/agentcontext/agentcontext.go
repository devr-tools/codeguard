// Package agentcontext implements the "context" check family: how legible a
// repository is to AI coding agents. It verifies that agent instruction docs
// exist and stay truthful, that the README's commands still work, and that
// the codebase fits agent-shaped navigation (context-budget file sizes,
// unambiguous basenames). Every scan also publishes a repo_legibility
// artifact scoring the target 0-100 with an explainable breakdown.
package agentcontext

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Run executes the agent-context family across all configured targets and finalizes
// the "context" section.
func Run(ctx context.Context, env support.Context) core.SectionResult {
	return support.RunTargetSection(ctx, env, "context", "Agent Context",
		func(_ context.Context, env support.Context, target core.TargetConfig) []core.Finding {
			return targetFindings(env, target)
		})
}

// targetFindings measures one target, emits the findings its toggles allow,
// and always publishes the repo_legibility artifact so the score is available
// even when individual rules are muted.
func targetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	rules := env.Config.Checks.ContextRules
	assessment, driftFound, readiness := assessTarget(env, target)
	findings := make([]core.Finding, 0)
	if ruleEnabled(rules.DetectMissingAgentDocs) && len(assessment.agentDocs) == 0 {
		findings = append(findings, missingAgentDocsFinding(env))
	}
	if ruleEnabled(rules.DetectAgentDocsDrift) {
		findings = append(findings, driftFound.agentDocs...)
	}
	if ruleEnabled(rules.DetectReadmeDrift) {
		findings = append(findings, driftFound.readme...)
	}
	if ruleEnabled(rules.DetectOversizedFiles) {
		findings = append(findings, oversizedFindings(env, assessment.inventory, assessment.maxFileLines)...)
	}
	if ruleEnabled(rules.DetectAmbiguousSymbols) {
		findings = append(findings, ambiguousBasenameFindings(env, assessment.inventory, ambiguousThreshold(rules))...)
	}
	if ruleEnabled(rules.DetectUndocumentedCommands) {
		findings = append(findings, readiness.undocumentedCommands...)
	}
	if ruleEnabled(rules.DetectOversizedAgentDocs) {
		findings = append(findings, readiness.oversizedAgentDocs...)
	}
	if ruleEnabled(rules.DetectDocLinkRot) {
		findings = append(findings, readiness.docLinkRot...)
	}
	if env.PutArtifact != nil {
		env.PutArtifact(legibilityArtifact(target, assessment))
	}
	return findings
}

// driftResults keeps the two drift rules' findings separate so toggles gate
// them independently while the artifact counts both.
type driftResults struct {
	agentDocs []core.Finding
	readme    []core.Finding
}

// readinessResults carries the AI-and-human-readiness rules' findings, each
// gated by its own toggle in targetFindings.
type readinessResults struct {
	undocumentedCommands []core.Finding
	oversizedAgentDocs   []core.Finding
	docLinkRot           []core.Finding
}

// assessTarget performs every measurement once: doc presence, drift and
// readiness resolution, and the source inventory walk shared by the size and
// basename rules and the legibility score.
func assessTarget(env support.Context, target core.TargetConfig) (targetAssessment, driftResults, readinessResults) {
	rules := env.Config.Checks.ContextRules
	resolver := newRepoResolver(target.Path)
	assessment := targetAssessment{
		agentDocs:     presentAgentDocs(target.Path),
		readmePresent: resolver.pathExists("README.md"),
		maxFileLines:  contextBudgetLines(rules),
	}
	drift := driftResults{
		agentDocs: agentDocsDriftFindings(env, resolver, assessment.agentDocs),
		readme:    readmeDriftFindings(env, resolver),
	}
	readiness := readinessResults{
		undocumentedCommands: undocumentedCommandFindings(env, resolver, assessment.agentDocs),
		oversizedAgentDocs:   oversizedAgentDocFindings(env, resolver.root, assessment.agentDocs, maxAgentDocLinesBudget(rules)),
		docLinkRot:           docLinkRotFindings(env, resolver, assessment.agentDocs),
	}
	assessment.driftReferences = len(drift.agentDocs) + len(drift.readme)
	assessment.inventory = collectSourceInventory(env, target, assessment.maxFileLines)
	assessment.ambiguousGroups = ambiguousBasenameGroups(assessment.inventory, ambiguousThreshold(rules))
	return assessment, drift, readiness
}

// ruleEnabled treats a nil toggle as enabled: the family's rules are opt-out.
func ruleEnabled(flag *bool) bool {
	return flag == nil || *flag
}

// contextBudgetLines resolves the configured context budget, falling back to
// the documented default for configs assembled without ApplyDefaults.
func contextBudgetLines(rules core.ContextRulesConfig) int {
	if rules.MaxFileLines > 0 {
		return rules.MaxFileLines
	}
	return 1500
}

// ambiguousThreshold resolves the configured basename threshold, falling back
// to the documented default for configs assembled without ApplyDefaults.
func ambiguousThreshold(rules core.ContextRulesConfig) int {
	if rules.AmbiguousSymbolThreshold > 1 {
		return rules.AmbiguousSymbolThreshold
	}
	return 4
}
