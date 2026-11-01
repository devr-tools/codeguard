package quality

// precisionExemption reports whether a precision rule should stay silent for a
// function because of where it lives or what its name declares it to be.
// Rules list their exemptions as tables so each rule's suppression policy
// reads as data instead of a long chain of negated conditions.
type precisionExemption func(file string, fn precisionFunction) bool

// exemptPath adapts a path-only predicate to a precisionExemption.
func exemptPath(predicate func(file string) bool) precisionExemption {
	return func(file string, _ precisionFunction) bool { return predicate(file) }
}

// exemptName adapts a function-name predicate to a precisionExemption.
func exemptName(predicate func(name string) bool) precisionExemption {
	return func(_ string, fn precisionFunction) bool { return predicate(fn.Name) }
}

// exemptFileAndName adapts a (file, function name) predicate to a
// precisionExemption.
func exemptFileAndName(predicate func(file string, name string) bool) precisionExemption {
	return func(file string, fn precisionFunction) bool { return predicate(file, fn.Name) }
}

// anyPrecisionExemption evaluates exemptions in order and stops at the first
// one that applies.
func anyPrecisionExemption(file string, fn precisionFunction, exemptions []precisionExemption) bool {
	for _, exempt := range exemptions {
		if exempt(file, fn) {
			return true
		}
	}
	return false
}

// uiOrScriptIdentifierExemptions silence generic parameter and local
// identifier names in UI mapping helpers and seed/script sources.
var uiOrScriptIdentifierExemptions = []precisionExemption{
	isUIHelperOrMappingContext,
	exemptPath(isSeedOrScriptSourcePath),
}

// booleanArgumentExemptions cover functions where a boolean flag parameter is
// conventional: setters/options, React boundaries, UI helpers, and scripts.
var booleanArgumentExemptions = []precisionExemption{
	exemptName(isAllowedBooleanArgumentFunction),
	isReactComponentOrHookBoundary,
	isUIHelperOrMappingContext,
	exemptPath(isSeedOrScriptSourcePath),
}

// mixedAbstractionExemptions cover functions whose job is to bridge
// orchestration and infrastructure.
var mixedAbstractionExemptions = []precisionExemption{
	exemptPath(isPostgresRepositoryPath),
	isAdapterOrOrchestrationFunction,
	isFrameworkOrchestrationBoundary,
	exemptFileAndName(isScriptEntrypoint),
	exemptPath(isSeedOrScriptSourcePath),
	isSecurityOrConfigUtilityFunction,
	exemptName(isDomainSideEffectBoundaryName),
	exemptName(isFactoryHelperName),
	exemptName(isPureComputationHelperName),
	isReactComponentOrHookBoundary,
	isUIHelperOrMappingContext,
}

// primitiveObsessionExemptions cover UI, script, frontend-library, boundary,
// and validation helpers where raw primitives are the expected currency.
var primitiveObsessionExemptions = []precisionExemption{
	isUIHelperOrMappingContext,
	exemptPath(isSeedOrScriptSourcePath),
	exemptPath(isFrontendLibraryPath),
	exemptName(isDomainSideEffectBoundaryName),
	exemptName(isValidationOrExtractionHelperName),
}

// sideEffectBoundaryExemptions cover functions that are expected to perform
// side effects, so a query-like name does not make them misleading.
var sideEffectBoundaryExemptions = []precisionExemption{
	isFrameworkOrchestrationBoundary,
	isReactComponentOrNamedHookBoundary,
	isUIHelperOrMappingContext,
	exemptPath(isSeedOrScriptSourcePath),
	isAdapterOrOrchestrationFunction,
	isSecurityOrConfigUtilityFunction,
	exemptName(explicitMutationName),
	exemptFileAndName(isUICommandHelperName),
	exemptName(isDomainSideEffectBoundaryName),
}

// commandQueryExemptions extend the side-effect boundaries with script
// entrypoints and Postgres repositories, which legitimately return results
// from commands.
var commandQueryExemptions = append(append([]precisionExemption{}, sideEffectBoundaryExemptions...),
	exemptFileAndName(isScriptEntrypoint),
	exemptPath(isPostgresRepositoryPath),
)
