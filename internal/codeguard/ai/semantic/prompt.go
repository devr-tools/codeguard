package semantic

import "strings"

func buildPromptTemplate(checks []CheckSpec, frameworks []FrameworkRef) PromptTemplate {
	template := PromptTemplate{
		Overview: "Review only the changed behavior and nearby tests. Be conservative: emit a verdict only when the local diff, source snapshots, tests, and framework hints provide concrete evidence of a user-visible mismatch, silent contract drift, misleading error semantics, missing behavior coverage, or inadequate tests.",
		ResponseRequirements: []string{
			"Return JSON only with the shape {\"verdicts\":[...]} and use only the provided rule_ids.",
			"Do not speculate about code outside the provided diff, source snapshots, test snapshots, and framework instructions.",
			"If uncertain, omit the verdict rather than guessing.",
		},
		RuleInstructions:      make([]RulePromptTemplate, 0, len(checks)),
		FrameworkInstructions: frameworkPromptRefs(frameworks),
	}
	for _, check := range checks {
		template.RuleInstructions = append(template.RuleInstructions, rulePromptTemplate(check.RuleID, frameworks))
	}
	return template
}

func rulePromptTemplate(ruleID string, frameworks []FrameworkRef) RulePromptTemplate {
	switch ruleID {
	case "quality.ai.contract-drift":
		return RulePromptTemplate{
			RuleID:    ruleID,
			Focus:     "Find changed behavior that silently shifts the observable contract without a matching caller-facing rename, documentation update, error update, test update, or framework-consistent interface change.",
			Consider:  contractDriftConsiderations(frameworks),
			Avoid:     []string{"Do not flag internal refactors that preserve the observable behavior.", "Do not treat style or implementation detail changes as contract drift without a changed input/output or user-visible semantic effect."},
			Threshold: "Emit only when the drift is concrete from the local evidence and would plausibly surprise a caller, operator, or downstream component.",
		}
	case "quality.ai.semantic-test-adequacy":
		return RulePromptTemplate{
			RuleID:    ruleID,
			Focus:     "Find changed behavior where nearby tests appear too weak, too indirect, too happy-path, or too sparse to prove the new contract or failure mode.",
			Consider:  testAdequacyConsiderations(frameworks),
			Avoid:     []string{"Do not require exhaustive tests.", "Do not flag missing tests when the existing nearby tests clearly assert the changed contract or failure mode."},
			Threshold: "Emit only when the weakness is specific and tied to the changed behavior, not to general preferences about test style.",
		}
	case "quality.ai.semantic-doc-mismatch":
		return RulePromptTemplate{
			RuleID:    ruleID,
			Focus:     "Compare changed implementation behavior with nearby names, comments, and documentation text.",
			Threshold: "Emit only when the code and adjacent documentation materially disagree.",
		}
	case "quality.ai.semantic-error-message":
		return RulePromptTemplate{
			RuleID:    ruleID,
			Focus:     "Check whether changed error strings misstate the failing condition, requested operation, input, or recovery path.",
			Threshold: "Emit only when the error text would likely mislead an operator or caller.",
		}
	case "quality.ai.semantic-test-coverage":
		return RulePromptTemplate{
			RuleID:    ruleID,
			Focus:     "Check whether changed branches, outputs, or failure paths appear unexercised by nearby changed or local tests.",
			Threshold: "Emit only when the changed behavior has no clear nearby test exercise path.",
		}
	default:
		return RulePromptTemplate{RuleID: ruleID}
	}
}

func contractDriftConsiderations(frameworks []FrameworkRef) []string {
	items := []string{
		"Check whether changed inputs, outputs, status codes, errors, or side effects diverge from the surrounding contract signals.",
		"Use nearby tests and docs as evidence of the prior expected behavior when they are specific.",
	}
	for _, framework := range frameworks {
		if hasHint(framework, "component-props-contract") {
			items = append(items, "For component files, treat changed props shape, required props, or children expectations as contract changes when callers would need to adapt.")
		}
		if hasHint(framework, "route-props-contract") {
			items = append(items, "For Next.js route-segment components, check whether changed params or searchParams handling changes the expected route input contract.")
		}
		if hasHint(framework, "route-handler-contract") {
			items = append(items, "For route handlers, check whether changed request parsing, status codes, or response payloads shift the handler contract.")
		}
		if hasHint(framework, "middleware-order-sensitive") || hasHint(framework, "middleware-next-chain") {
			items = append(items, "For Express middleware, check whether changed next() flow, early returns, or middleware ordering alters which downstream handlers run or which request state they receive.")
		}
	}
	return uniqueNonEmptyStrings(items)
}

func testAdequacyConsiderations(frameworks []FrameworkRef) []string {
	items := []string{
		"Look for missing negative-path checks, boundary checks, concrete assertions, and assertions that match the changed output or failure mode.",
		"Prefer specific test gaps tied to the diff over general statements that more tests would be nice.",
	}
	for _, framework := range frameworks {
		if hasHint(framework, "component-props-contract") {
			items = append(items, "For React or Next.js components, check whether tests cover changed prop combinations, rendered states, and caller-visible UI behavior.")
		}
		if hasHint(framework, "stateful-component") {
			items = append(items, "For stateful components, check whether tests exercise the changed interaction or state transition rather than only snapshotting the initial render.")
		}
		if hasHint(framework, "route-props-contract") {
			items = append(items, "For Next.js route-segment components, check whether tests cover changed params or searchParams inputs, including missing or malformed values when relevant.")
		}
		if hasHint(framework, "route-handler-contract") {
			items = append(items, "For route handlers, check whether tests cover changed request shapes, status codes, and response bodies, especially failure or validation paths.")
		}
		if hasHint(framework, "middleware-order-sensitive") || hasHint(framework, "middleware-next-chain") {
			items = append(items, "For Express middleware, check whether tests prove next() chaining, early termination, res.locals or request mutation, and downstream behavior after the middleware runs.")
		}
	}
	return uniqueNonEmptyStrings(items)
}

func frameworkPromptRefs(frameworks []FrameworkRef) []FrameworkPromptRef {
	refs := make([]FrameworkPromptRef, 0, len(frameworks))
	for _, framework := range frameworks {
		refs = append(refs, FrameworkPromptRef{
			Name:   framework.Name,
			Path:   framework.Path,
			Hints:  framework.Hints,
			Advice: frameworkAdvice(framework),
		})
	}
	return refs
}

func frameworkAdvice(framework FrameworkRef) []string {
	advice := make([]string, 0, 4)
	if hasHint(framework, "component-props-contract") {
		advice = append(advice, "Use prop names, optionality, and destructuring patterns as evidence of the caller-facing component contract.")
	}
	if hasHint(framework, "route-props-contract") {
		advice = append(advice, "Treat params and searchParams as route inputs whose shape and defaults can form part of the observable contract.")
	}
	if hasHint(framework, "route-handler-contract") {
		advice = append(advice, "Treat request parsing, status codes, and response payloads as part of the API contract.")
	}
	if hasHint(framework, "middleware-order-sensitive") || hasHint(framework, "middleware-next-chain") {
		advice = append(advice, "Treat next() flow, early returns, and request or response mutation as part of middleware sequencing semantics.")
	}
	return uniqueNonEmptyStrings(advice)
}

func hasHint(framework FrameworkRef, want string) bool {
	for _, hint := range framework.Hints {
		if hint == want {
			return true
		}
	}
	return false
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
