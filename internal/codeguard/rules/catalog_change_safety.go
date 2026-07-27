package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var changeSafetyCatalog = map[string]core.RuleMetadata{
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
