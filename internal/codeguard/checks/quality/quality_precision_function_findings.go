package quality

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	namingBehaviorMismatchRuleID        = "naming.behavior-mismatch"
	namingBooleanNotPredicateRuleID     = "naming.boolean-not-predicate"
	namingDomainVocabularyDriftRuleID   = "naming.domain-vocabulary-drift"
	namingUnknownAbbreviationRuleID     = "naming.unknown-abbreviation"
	namingCardinalityMismatchRuleID     = "naming.cardinality-mismatch"
	namingImplementationLeakRuleID      = "naming.implementation-leak"
	namingMissingUnitRuleID             = "naming.missing-unit"
	namingRoleSuffixOveruseRuleID       = "naming.role-suffix-overuse"
	namingCrossLayerInconsistencyRuleID = "naming.cross-layer-inconsistency"

	functionHiddenMutationRuleID             = "function.hidden-mutation"
	functionInconsistentReturnContractRuleID = "function.inconsistent-return-contract"
	functionMultipleResponsibilitiesRuleID   = "function.multiple-responsibilities"
	functionOrchestrationDomainMixRuleID     = "function.orchestration-domain-mix"
	functionPartialResultRuleID              = "function.partial-result"
)

func additionalPrecisionFunctionFindings(env support.Context, file string, fn precisionFunction) []core.Finding {
	findings := make([]core.Finding, 0, 8)
	if behaviorMismatch(file, fn) {
		findings = append(findings, precisionWarnFinding(env, namingBehaviorMismatchRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s name conflicts with observed query/command behavior", fn.Name), core.ConfidenceMedium))
	}
	if evidence, ok := hiddenMutationEvidence(file, fn); ok {
		findings = append(findings, precisionWarnFindingWithMetadata(env, functionHiddenMutationRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s mutates state owned by %s through %s", fn.Name, evidence.Target, evidence.Detail),
			core.ConfidenceMedium, mutationEvidenceMetadata(evidence)))
	}
	if !isReactComponentOrHookBoundary(file, fn) && !isUIHelperOrMappingContext(file, fn) &&
		!isSeedOrScriptSourcePath(file) && !isScriptEntrypoint(file, fn.Name) &&
		!isSecurityOrConfigUtilityFunction(file, fn) && inconsistentReturnContract(fn) {
		findings = append(findings, precisionWarnFinding(env, functionInconsistentReturnContractRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s mixes empty and value return shapes; make the success/error contract explicit", fn.Name), core.ConfidenceMedium))
	}
	if !isUIHelperOrMappingContext(file, fn) && !isSeedOrScriptSourcePath(file) && partialResult(file, fn) {
		findings = append(findings, precisionWarnFinding(env, functionPartialResultRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s can return a value alongside an error without an explicit partial-result contract", fn.Name), core.ConfidenceMedium))
	}
	if count, labels := responsibilityCount(fn); !isSeedOrScriptSourcePath(file) && !isAdapterOrOrchestrationFunction(file, fn) && count >= responsibilityThreshold(file, fn) {
		findings = append(findings, precisionWarnFinding(env, functionMultipleResponsibilitiesRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s combines %d responsibilities (%s); split orchestration from focused work", fn.Name, count, strings.Join(labels, ", ")), core.ConfidenceMedium))
	}
	if orchestrationDomainMix(file, fn) {
		findings = append(findings, precisionWarnFinding(env, functionOrchestrationDomainMixRuleID, file, fn.StartLine,
			fmt.Sprintf("function %s mixes request/job orchestration with domain decisions", fn.Name), core.ConfidenceMedium))
	}
	findings = append(findings, precisionNamingFindings(env, file, fn)...)
	return findings
}
