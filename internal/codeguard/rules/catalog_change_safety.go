package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var changeSafetyCatalog = map[string]core.RuleMetadata{
	"naming.generic-identifier":             localQualityRule("naming.generic-identifier", "warn", "Generic identifier", "Warns when a function, parameter, or local variable uses placeholder names such as foo, tmp, thing, or obj instead of domain vocabulary.", "Rename the identifier to describe the role it plays in the surrounding behavior."),
	"function.excessive-parameters":         localQualityRule("function.excessive-parameters", "warn", "Excessive parameters", "Warns when a function exceeds the configured parameter threshold and should likely group related inputs or split responsibilities.", "Group cohesive inputs into a named object or split the function along separate responsibilities."),
	"function.mixed-abstraction-level":      localQualityRule("function.mixed-abstraction-level", "warn", "Mixed abstraction level", "Warns when one function combines orchestration-level calls with low-level infrastructure operations such as SQL, HTTP, filesystem, or environment access.", "Move low-level infrastructure details behind a helper or boundary so the function operates at one clear level of abstraction."),
	"function.command-query-mix":            localQualityRule("function.command-query-mix", "warn", "Command/query mix", "Warns when a function returns a value while also invoking mutating side-effect operations.", "Separate state-changing commands from value-returning queries, or make the side effect explicit in the function name and tests."),
	"error.logged-and-ignored":              localQualityRule("error.logged-and-ignored", "warn", "Logged and ignored error", "Warns when an error is logged and then ignored, converted to a success value, or allowed to continue without propagation.", "Return, wrap, or otherwise handle the error instead of only logging it."),
	"error.context-lost":                    localQualityRule("error.context-lost", "warn", "Error context lost", "Warns when an error is rethrown or returned bare from a lower-level call without contextual wrapping.", "Wrap the error with operation-specific context while preserving the original error for callers."),
	"defensive.unchecked-type-assertion":    localQualityRule("defensive.unchecked-type-assertion", "warn", "Unchecked type assertion", "Warns when a type assertion or cast bypasses runtime validation or omits the safe comma-ok form.", "Use a checked assertion, runtime validation, or type narrowing before consuming the value."),
	"defensive.unsafe-numeric-conversion":   localQualityRule("defensive.unsafe-numeric-conversion", "warn", "Unsafe numeric conversion", "Warns when a narrowing or sign-changing numeric conversion can truncate, wrap, or lose precision.", "Validate bounds before converting, or keep values in a type wide enough for the source range."),
	"maintainability.public-surface-growth": maintainabilityDeltaRule("maintainability.public-surface-growth", "warn", "Public surface growth", "Warns in diff scans when a changed file exports more public symbols than it did at the base ref.", "Keep newly exported symbols intentional, documented, and covered by tests; avoid widening API surface for internal-only behavior."),
	"maintainability.dependency-growth":     maintainabilityDeltaRule("maintainability.dependency-growth", "warn", "Dependency growth", "Warns in diff scans when a changed file imports or includes more direct dependencies than it did at the base ref.", "Remove unnecessary imports/includes or hide optional integrations behind a narrow boundary."),
	"maintainability.high-churn-hotspot":    maintainabilityHistoryRule("maintainability.high-churn-hotspot", "warn", "High-churn hotspot", "Warns when a changed file combines repeated churn with current complexity hints, making safe review and future changes harder.", "Reduce local complexity, split the change if possible, and add focused regression coverage around the behavior being touched."),
	"maintainability.repeat-defect-area":    maintainabilityHistoryRule("maintainability.repeat-defect-area", "warn", "Repeat defect area", "Warns when a changed file has multiple recent fix, regression, incident, or defect-linked commits in git history.", "Add regression tests for the failure modes that have changed here before and keep the patch narrow."),
	"maintainability.unstable-interface":    maintainabilityHistoryRule("maintainability.unstable-interface", "warn", "Unstable interface", "Warns when a changed public-surface file has repeated churn or defect history, suggesting compatibility risk.", "Keep interface changes explicit, document caller impact, and preserve backwards compatibility or add migration tests."),
	"maintainability.change-amplification":  maintainabilityHistoryRule("maintainability.change-amplification", "warn", "Change amplification", "Warns when a changed file historically fans out into many co-changed partners.", "Identify the coupled responsibilities and consider extracting a narrower boundary or updating the usual partner files intentionally."),
	"maintainability.hotspot":               maintainabilityHistoryRule("maintainability.hotspot", "warn", "Maintainability hotspot", "Warns when a changed file has high recent churn, defect history, or both.", "Treat the file as risky legacy surface: keep changes small, add characterization tests, and note the hotspot evidence for reviewers."),
	"smell.shotgun-surgery-history":         smellHistoryRule("smell.shotgun-surgery-history", "warn", "Shotgun surgery history", "Warns when a changed file repeatedly co-changes with several partners, indicating one concept may be spread across files.", "Consider consolidating the scattered responsibility or make the related partner updates explicit in this PR."),
	"smell.divergent-change-history":        smellHistoryRule("smell.divergent-change-history", "warn", "Divergent change history", "Warns when a changed file has recent commit subjects spanning several concern families.", "Split unrelated responsibilities out of the file or isolate the concern being changed behind a clearer boundary."),

	"testing.behavior-change-without-test":  testabilityRule("testing.behavior-change-without-test", "fail", "Behavior change without test", "Fails when production behavior changes without nearby test evidence in the same diff.", "Add or update tests that exercise the changed behavior, including observable success and failure outcomes."),
	"testing.failure-path-missing":          testabilityRule("testing.failure-path-missing", "warn", "Failure path missing", "Warns when high-risk branches add error, retry, fallback, authorization, or external dependency paths without failure-path tests.", "Add tests that force the failure path and assert the returned error, fallback behavior, or partial-failure result."),
	"testing.hardwired-dependency":          testabilityRule("testing.hardwired-dependency", "warn", "Hardwired dependency", "Warns when business logic constructs clocks, random sources, network clients, filesystem access, or infrastructure dependencies directly.", "Inject the dependency or route it through a narrow interface so tests can provide deterministic fakes."),
	"testing.nondeterministic-domain-logic": testabilityRule("testing.nondeterministic-domain-logic", "warn", "Nondeterministic domain logic", "Warns when domain code reads time, randomness, filesystem, network, or environment state directly.", "Move nondeterministic access to the boundary and pass explicit values or interfaces into domain logic."),
	"testing.legacy-hotspot-uncovered":      testabilityRule("testing.legacy-hotspot-uncovered", "warn", "Legacy hotspot without characterization coverage", "Warns when a high-churn or complex legacy hotspot is touched without characterization or regression-test evidence.", "Add characterization tests around the current behavior before changing the hotspot."),

	"change.mixed-concerns":              changeRepoRule("change.mixed-concerns", "warn", "Mixed concerns", "Warns when a PR combines unrelated subsystems, architectural layers, or rule families in one review unit.", "Split unrelated concerns into smaller PRs or document why they must ship atomically."),
	"change.oversized-diff":              changeRepoRule("change.oversized-diff", "warn", "Oversized diff", "Warns when the changed-file, changed-directory, changed-line, public-interface, or test-ratio budget makes the PR hard to review safely.", "Reduce scope, split mechanical movement from behavior changes, or add focused review notes and tests for the highest-risk paths."),
	"change.mixed-refactor-and-behavior": changeRepoRule("change.mixed-refactor-and-behavior", "warn", "Mixed refactor and behavior change", "Warns when a diff combines moves, renames, or extraction with observable behavior changes.", "Separate behavior-preserving refactors from behavior changes, or provide explicit verification evidence."),
	"change.too-many-concerns":           changeRepoRule("change.too-many-concerns", "warn", "Too many concerns", "Warns when change concentration evidence shows too many unrelated concepts being modified at once.", "Split the PR by concern, public contract, architectural layer, or rollout unit."),
	"change.unnecessary-surface-area":    changeRepoRule("change.unnecessary-surface-area", "warn", "Unnecessary surface area", "Warns when a narrow change touches more files, directories, or public interfaces than the behavior requires.", "Trim incidental edits and keep public API changes limited to what the feature requires."),
	"change.one-use-abstraction":         testabilityRule("change.one-use-abstraction", "warn", "One-use abstraction", "Warns when a new abstraction is introduced but has only one consumer or delegates without simplifying the caller.", "Inline the abstraction until a second concrete use appears, or make the boundary carry meaningful policy."),
	"change.duplicate-helper":            testabilityRule("change.duplicate-helper", "warn", "Duplicate helper", "Warns when a change introduces helper logic that overlaps existing project vocabulary or utilities.", "Reuse or extend the existing helper, keeping one source of truth for the shared behavior."),
	"change.cleanup-regression":          testabilityRule("change.cleanup-regression", "warn", "Cleanup regression", "Warns when a cleanup-labeled change increases complexity, duplication, public surface, or dependency count.", "Keep cleanup PRs behavior-preserving and ensure maintainability metrics move in the intended direction."),
	"change.complexity-increased":        testabilityRule("change.complexity-increased", "warn", "Complexity increased", "Warns when touched functions, files, or hotspots become materially more complex in the diff.", "Extract decisions, reduce nesting, or add tests explaining why the added complexity is necessary."),
	"change.move-without-verification":   changeRepoRule("change.move-without-verification", "warn", "Move without verification", "Warns when files or symbols move without test, build, or behavior-preservation evidence.", "Preserve tests through the move and run the narrow verification target that exercises the moved behavior."),

	"refactor.behavior-change-detected":             refactorRule("refactor.behavior-change-detected", "fail", "Behavior change detected in refactor", "Fails when a refactor-labeled diff changes return paths, side effects, authorization checks, writes, emitted events, or external calls.", "Move behavior changes into a separate PR or update the PR label and add tests for the new behavior."),
	"refactor.public-contract-changed":              refactorRule("refactor.public-contract-changed", "fail", "Public contract changed in refactor", "Fails when exported signatures, API schemas, events, or persistence contracts change in a refactor-only PR.", "Keep public contracts stable during the refactor or make the contract change explicit with compatibility tests."),
	"refactor.test-coverage-reduced":                refactorRule("refactor.test-coverage-reduced", "warn", "Test coverage reduced by refactor", "Warns when a refactor removes or weakens tests over the moved or reshaped behavior.", "Move tests with the code and preserve characterization coverage before deleting old tests."),
	"refactor.error-path-changed":                   refactorRule("refactor.error-path-changed", "fail", "Error path changed in refactor", "Fails when a refactor changes wrapping, returned errors, ignored errors, panic behavior, or partial-failure handling.", "Preserve existing error contracts or split the error-behavior change into a tested follow-up PR."),
	"refactor.side-effect-order-changed":            refactorRule("refactor.side-effect-order-changed", "fail", "Side-effect order changed in refactor", "Fails when database writes, event publishing, network calls, cleanup, or authorization side effects are reordered.", "Keep side-effect ordering stable or add explicit tests and rollout notes for the new order."),
	"refactor.visibility-expanded":                  refactorRule("refactor.visibility-expanded", "warn", "Visibility expanded in refactor", "Warns when private symbols become public or cross-package visible during a refactor.", "Keep visibility as narrow as possible, or document the new consumer that requires the broader API."),
	"refactor.dependency-direction-worsened":        refactorRule("refactor.dependency-direction-worsened", "warn", "Dependency direction worsened in refactor", "Warns when refactoring introduces an inward dependency on infrastructure, UI, persistence, or framework code.", "Invert the dependency or introduce a stable interface owned by the inner layer."),
	"refactor.duplicate-implementation-left-behind": refactorRule("refactor.duplicate-implementation-left-behind", "warn", "Duplicate implementation left behind", "Warns when an extraction or move leaves the previous implementation active in another path.", "Delete or delegate the old implementation after proving callers use the new path."),
	"refactor.dead-path-left-behind":                refactorRule("refactor.dead-path-left-behind", "warn", "Dead path left behind", "Warns when refactoring leaves obsolete branches, feature flags, wrappers, or compatibility paths without consumers.", "Remove the dead path or add a removal plan with ownership and an expiry trigger."),
}

func testabilityRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	return core.RuleMetadata{
		ID:             id,
		Section:        "Change Safety / Testability",
		DefaultLevel:   level,
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
			core.RuleLanguagePython,
			core.RuleLanguageCPP,
		),
		Title:       title,
		Description: description,
		HowToFix:    howToFix,
	}
}

func localQualityRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	return core.RuleMetadata{
		ID:             id,
		Section:        "Code Quality / Local Precision",
		DefaultLevel:   level,
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
			core.RuleLanguagePython,
			core.RuleLanguageCPP,
		),
		Title:       title,
		Description: description,
		HowToFix:    howToFix,
	}
}

func maintainabilityDeltaRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	meta := localQualityRule(id, level, title, description, howToFix)
	meta.Section = "Maintainability Delta"
	return meta
}

func maintainabilityHistoryRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	meta := localQualityRule(id, level, title, description, howToFix)
	meta.Section = "Maintainability History"
	return meta
}

func smellHistoryRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	meta := localQualityRule(id, level, title, description, howToFix)
	meta.Section = "Code Smells / History"
	return meta
}

func refactorRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	meta := testabilityRule(id, level, title, description, howToFix)
	meta.Section = "Change Safety / Refactors"
	return meta
}

func changeRepoRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	return core.RuleMetadata{
		ID:               id,
		Section:          "Change Safety",
		DefaultLevel:     level,
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            title,
		Description:      description,
		HowToFix:         howToFix,
	}
}
