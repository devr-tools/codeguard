package support

// Per-file findings cache section ids.
//
// The per-file findings cache keys an entry on (cache section id, target path,
// file path). Two passes that pass the same id and visit the same file replay
// each other's cached findings in place of their own, so every per-file scan
// pass needs its own id. A shared id has caused this twice, both times for
// "design".
//
// Every id a pass passes to ScanTargetFiles, ScanTargetFilesWithDependencies,
// ScanTargetFilesWithDiagnostics, ScanTargetFilesSequential, the Scan*Files
// language helpers, or TypeScriptTargetScan.SectionID must be one of these
// constants, and each constant must be used by exactly one pass. The test
// TestCacheSectionIDsAreUniquePerPass (tests/support) enforces both rules.
//
// The text before the first '-' picks the config family whose settings
// invalidate the entry (see sectionConfigFamily in runner/support), so a new
// id must keep its section's prefix: "quality-…", "design-…", and so on.
// Prefixes outside the known families ("reliability", "data", "observability",
// "custom") fall back to the all-checks fingerprint.
const (
	CacheSectionQualityGo                       = "quality-go"
	CacheSectionQualityPython                   = "quality-python"
	CacheSectionQualityRust                     = "quality-rust"
	CacheSectionQualityCPP                      = "quality-cpp"
	CacheSectionQualityJava                     = "quality-java"
	CacheSectionQualityCSharp                   = "quality-csharp"
	CacheSectionQualityRuby                     = "quality-ruby"
	CacheSectionQualityTypeScript               = "quality-typescript"
	CacheSectionQualityTypeScriptFileLength     = "quality-typescript-file-length"
	CacheSectionQualityTypeScriptAI             = "quality-typescript-ai"
	CacheSectionQualityTypeScriptLocalPrecision = "quality-typescript-local-precision"
	CacheSectionQualityEnvironmentBranching     = "quality-environment-branching"
	CacheSectionObservability                   = "observability"

	CacheSectionPerformanceGo     = "performance-go"
	CacheSectionPerformancePython = "performance-python"
	CacheSectionPerformanceRust   = "performance-rust"
	CacheSectionPerformanceCPP    = "performance-cpp"

	CacheSectionReliabilityGo     = "reliability-go"
	CacheSectionReliabilityPython = "reliability-python"
	CacheSectionReliabilityCPP    = "reliability-cpp"

	CacheSectionDataGo     = "data-go"
	CacheSectionDataPython = "data-python"
	CacheSectionDataCPP    = "data-cpp"

	CacheSectionDesignGoStructural         = "design-go-structural"
	CacheSectionDesignPythonStructural     = "design-python-structural"
	CacheSectionDesignRustStructural       = "design-rust-structural"
	CacheSectionDesignCPPStructural        = "design-cpp-structural"
	CacheSectionDesignTypeScriptStructural = "design-typescript-structural"
	CacheSectionDesignLocalAbstraction     = "design-local-abstraction"

	CacheSectionSecuritySecrets    = "security-secrets"
	CacheSectionSecurityA09        = "security-a09"
	CacheSectionSecurityLanguage   = "security-language"
	CacheSectionSecurityTypeScript = "security-typescript"

	CacheSectionCIWorkflowSecurity = "ci-workflow-security"
	CacheSectionCITestQuality      = "ci-test-quality"
	CacheSectionCITestFileLocation = "ci-test-file-location"

	CacheSectionContractsMigrations = "contracts-migrations"
	CacheSectionPrompts             = "prompts"
	CacheSectionCustom              = "custom"
)
