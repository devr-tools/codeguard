package core

type QualityRulesConfig struct {
	MaxFileLines            int                             `json:"max_file_lines" yaml:"max_file_lines"`
	MaxFunctionLines        int                             `json:"max_function_lines" yaml:"max_function_lines"`
	MaxParameters           int                             `json:"max_parameters" yaml:"max_parameters"`
	MaxCyclomaticComplexity int                             `json:"max_cyclomatic_complexity" yaml:"max_cyclomatic_complexity"`
	CloneTokenThreshold     int                             `json:"clone_token_threshold,omitempty" yaml:"clone_token_threshold,omitempty"`
	LanguageCommands        map[string][]CommandCheckConfig `json:"language_commands,omitempty" yaml:"language_commands,omitempty"`
	AIProvenance            AIProvenanceConfig              `json:"ai_provenance,omitempty" yaml:"ai_provenance,omitempty"`
	AIChangeRisk            AIChangeRiskConfig              `json:"ai_change_risk,omitempty" yaml:"ai_change_risk,omitempty"`
	AIChecks                AIChecksConfig                  `json:"ai_checks,omitempty" yaml:"ai_checks,omitempty"`
	CoverageDelta           CoverageDeltaConfig             `json:"coverage_delta,omitempty" yaml:"coverage_delta,omitempty"`
}

// PerformanceRulesConfig tunes the performance section (checks.performance).
// The detect_* toggles moved here from quality_rules when the performance
// rules were promoted out of the quality section; nil toggles default to
// enabled except detect_prealloc_in_loop.
type PerformanceRulesConfig struct {
	// DetectNPlusOneQuery gates query/fetch-in-loop detection across languages.
	DetectNPlusOneQuery *bool `json:"detect_n_plus_one_query,omitempty" yaml:"detect_n_plus_one_query,omitempty"`
	// DetectAllocInLoop gates allocation-heavy loop detection: Go string growth
	// and fmt.Sprintf accumulation, plus string concatenation in Python and
	// TypeScript/JavaScript loops.
	DetectAllocInLoop *bool `json:"detect_alloc_in_loop,omitempty" yaml:"detect_alloc_in_loop,omitempty"`
	// DetectPreallocInLoop gates the append-without-preallocation branch of
	// performance.go.alloc-in-loop. Defaults to false: preallocating is a
	// micro-optimization, and idiomatic accumulation loops legitimately skip it.
	DetectPreallocInLoop   *bool `json:"detect_prealloc_in_loop,omitempty" yaml:"detect_prealloc_in_loop,omitempty"`
	DetectSyncIOInHandlers *bool `json:"detect_sync_io_in_handlers,omitempty" yaml:"detect_sync_io_in_handlers,omitempty"`
	// DetectUnboundedConcurrency gates goroutines-in-loop (Go), promise
	// creation in loops (TS/JS), and asyncio task creation in loops (Python).
	DetectUnboundedConcurrency *bool `json:"detect_unbounded_concurrency,omitempty" yaml:"detect_unbounded_concurrency,omitempty"`
	// DetectRegexCompileInLoop flags regex compilation inside loop bodies
	// (regexp.Compile/MustCompile, re.compile, new RegExp).
	DetectRegexCompileInLoop *bool `json:"detect_regex_compile_in_loop,omitempty" yaml:"detect_regex_compile_in_loop,omitempty"`
	// DetectDeferInLoop flags Go defer statements inside loop bodies, where
	// they accumulate until function exit.
	DetectDeferInLoop *bool `json:"detect_defer_in_loop,omitempty" yaml:"detect_defer_in_loop,omitempty"`
	// DetectSleepInLoop flags time.Sleep inside Go loop bodies, which usually
	// marks a poll that wants a ticker, channel, or backoff helper.
	DetectSleepInLoop *bool `json:"detect_sleep_in_loop,omitempty" yaml:"detect_sleep_in_loop,omitempty"`
	// DetectAwaitInLoop flags await inside TS/JS loop bodies, which serializes
	// work that could run concurrently via Promise.all.
	DetectAwaitInLoop *bool `json:"detect_await_in_loop,omitempty" yaml:"detect_await_in_loop,omitempty"`
	// DetectTimerLeaks flags timer/listener leaks: time.After in Go loops,
	// setInterval without clearInterval and addEventListener in TS/JS loops.
	DetectTimerLeaks *bool `json:"detect_timer_leaks,omitempty" yaml:"detect_timer_leaks,omitempty"`
	// DetectUnboundedReads flags whole-input reads without a size bound:
	// io.ReadAll in Go handlers/loops, .read()/.readlines() in Python loops.
	DetectUnboundedReads *bool `json:"detect_unbounded_reads,omitempty" yaml:"detect_unbounded_reads,omitempty"`
	// DetectComplexityRegression gates the diff-only loop-nesting regression
	// check: it compares each changed function's maximum loop-nesting depth
	// against the diff base ref and warns on increases. Full scans are
	// unaffected (the rule only activates in diff mode).
	DetectComplexityRegression *bool `json:"detect_complexity_regression,omitempty" yaml:"detect_complexity_regression,omitempty"`
	// DetectFrameworkPatterns gates the framework-aware rules: Django relation
	// access and ORM point queries in Python loops (Django/SQLAlchemy),
	// expensive per-render work in React components, and CPU-heavy synchronous
	// calls in Express middleware. Each rule additionally requires file-level
	// framework evidence (imports or obvious idioms), so non-framework code
	// never matches.
	DetectFrameworkPatterns *bool `json:"detect_framework_patterns,omitempty" yaml:"detect_framework_patterns,omitempty"`
	// Budgets lists measured size gates over build artifacts (see
	// PerformanceBudgetConfig); findings report as performance.budget.
	Budgets []PerformanceBudgetConfig `json:"budgets,omitempty" yaml:"budgets,omitempty"`
	// Benchmarks configures the opt-in benchmark-regression gate (see
	// PerformanceBenchmarksConfig); findings report as
	// performance.benchmark-regression.
	Benchmarks PerformanceBenchmarksConfig `json:"benchmarks,omitempty" yaml:"benchmarks,omitempty"`
	// ScoreHistory gates persistence of the performance_score trend next to
	// the scan cache (nil = enabled, mirroring ai_checks.slop_history).
	ScoreHistory *bool `json:"score_history,omitempty" yaml:"score_history,omitempty"`
	// ScoreHistoryLimit caps retained performance_score history entries per
	// target (0 = default limit).
	ScoreHistoryLimit int `json:"score_history_limit,omitempty" yaml:"score_history_limit,omitempty"`
}

// AIChecksConfig toggles individual AI-quality heuristics. A nil pointer
// leaves the check enabled, matching the rest of the rule pack defaults.
type AIChecksConfig struct {
	HallucinatedImport *bool `json:"hallucinated_import,omitempty" yaml:"hallucinated_import,omitempty"`
	DeadCode           *bool `json:"dead_code,omitempty" yaml:"dead_code,omitempty"`
	ErrorStyleDrift    *bool `json:"error_style_drift,omitempty" yaml:"error_style_drift,omitempty"`
	NamingDrift        *bool `json:"naming_drift,omitempty" yaml:"naming_drift,omitempty"`
	SlopHistory        *bool `json:"slop_history,omitempty" yaml:"slop_history,omitempty"`
	SlopHistoryLimit   int   `json:"slop_history_limit,omitempty" yaml:"slop_history_limit,omitempty"`
}

type AIProvenanceConfig struct {
	Enabled                *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	EnvVars                []string `json:"env_vars,omitempty" yaml:"env_vars,omitempty"`
	CommitTrailers         []string `json:"commit_trailers,omitempty" yaml:"commit_trailers,omitempty"`
	SlopScoreWarnThreshold int      `json:"slop_score_warn_threshold,omitempty" yaml:"slop_score_warn_threshold,omitempty"`
	SlopScoreFailThreshold int      `json:"slop_score_fail_threshold,omitempty" yaml:"slop_score_fail_threshold,omitempty"`
}

type AIChangeRiskConfig struct {
	Enabled       *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	WarnThreshold int   `json:"warn_threshold,omitempty" yaml:"warn_threshold,omitempty"`
	FailThreshold int   `json:"fail_threshold,omitempty" yaml:"fail_threshold,omitempty"`
}

type DesignRulesConfig struct {
	RequireCmdThroughInternalCLI *bool                           `json:"require_cmd_through_internal_cli,omitempty" yaml:"require_cmd_through_internal_cli,omitempty"`
	ForbidInternalImportCmd      *bool                           `json:"forbid_internal_import_cmd,omitempty" yaml:"forbid_internal_import_cmd,omitempty"`
	ForbidServiceImportInternal  *bool                           `json:"forbid_service_import_internal,omitempty" yaml:"forbid_service_import_internal,omitempty"`
	ForbidServiceImportCmd       *bool                           `json:"forbid_service_import_cmd,omitempty" yaml:"forbid_service_import_cmd,omitempty"`
	MaxDeclsPerFile              int                             `json:"max_decls_per_file" yaml:"max_decls_per_file"`
	MaxMethodsPerType            int                             `json:"max_methods_per_type" yaml:"max_methods_per_type"`
	MaxInterfaceMethods          int                             `json:"max_interface_methods" yaml:"max_interface_methods"`
	DetectImportCycles           *bool                           `json:"detect_import_cycles,omitempty" yaml:"detect_import_cycles,omitempty"`
	DetectGodModules             *bool                           `json:"detect_god_modules,omitempty" yaml:"detect_god_modules,omitempty"`
	GodModuleThreshold           int                             `json:"god_module_threshold" yaml:"god_module_threshold"`
	DetectHighImpactChanges      *bool                           `json:"detect_high_impact_changes,omitempty" yaml:"detect_high_impact_changes,omitempty"`
	HighImpactChangeThreshold    int                             `json:"high_impact_change_threshold" yaml:"high_impact_change_threshold"`
	ForbiddenPackageNames        []string                        `json:"forbidden_package_names,omitempty" yaml:"forbidden_package_names,omitempty"`
	LanguageCommands             map[string][]CommandCheckConfig `json:"language_commands,omitempty" yaml:"language_commands,omitempty"`
	LanguageDiffCommands         map[string][]CommandCheckConfig `json:"language_diff_commands,omitempty" yaml:"language_diff_commands,omitempty"`
}

type PromptRulesConfig struct {
	FileExtensions            []string `json:"file_extensions,omitempty" yaml:"file_extensions,omitempty"`
	PathContains              []string `json:"path_contains,omitempty" yaml:"path_contains,omitempty"`
	ForbidSecretInterpolation *bool    `json:"forbid_secret_interpolation,omitempty" yaml:"forbid_secret_interpolation,omitempty"`
	ForbidUnsafeInstructions  *bool    `json:"forbid_unsafe_instructions,omitempty" yaml:"forbid_unsafe_instructions,omitempty"`
}

type CIRulesConfig struct {
	RequireWorkflowDir      *bool                  `json:"require_workflow_dir,omitempty" yaml:"require_workflow_dir,omitempty"`
	RequiredWorkflowFiles   []string               `json:"required_workflow_files,omitempty" yaml:"required_workflow_files,omitempty"`
	WorkflowContentRules    []WorkflowRuleConfig   `json:"workflow_content_rules,omitempty" yaml:"workflow_content_rules,omitempty"`
	RequiredReleaseFiles    []string               `json:"required_release_files,omitempty" yaml:"required_release_files,omitempty"`
	RequiredAutomationPaths []string               `json:"required_automation_paths,omitempty" yaml:"required_automation_paths,omitempty"`
	AllowedTestPaths        []string               `json:"allowed_test_paths,omitempty" yaml:"allowed_test_paths,omitempty"`
	TestQuality             TestQualityRulesConfig `json:"test_quality,omitempty" yaml:"test_quality,omitempty"`
}

type WorkflowRuleConfig struct {
	Path             string   `json:"path" yaml:"path"`
	RequiredContains []string `json:"required_contains,omitempty" yaml:"required_contains,omitempty"`
}

type SecurityRulesConfig struct {
	GovulncheckMode         string                          `json:"govulncheck_mode,omitempty" yaml:"govulncheck_mode,omitempty"`
	GovulncheckCommand      string                          `json:"govulncheck_command,omitempty" yaml:"govulncheck_command,omitempty"`
	TaintGo                 *bool                           `json:"taint_go,omitempty" yaml:"taint_go,omitempty"`
	TaintPython             *bool                           `json:"taint_python,omitempty" yaml:"taint_python,omitempty"`
	TypeScriptTaintMaxDepth int                             `json:"typescript_taint_max_depth,omitempty" yaml:"typescript_taint_max_depth,omitempty"`
	LanguageCommands        map[string][]CommandCheckConfig `json:"language_commands,omitempty" yaml:"language_commands,omitempty"`
	Secrets                 *SecretsRulesConfig             `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	// DemoteFixtureFindings downgrades hardcoded-secret, hardcoded-credential,
	// and high-entropy-string findings located in test/fixture paths (testdata/,
	// fixtures/, __fixtures__/, *_test.go, *.test.ts, *_test.py, *.spec.ts):
	// fail becomes warn, confidence drops to low, and the message notes the
	// demotion. Fixture credentials are still reported — never silenced — but no
	// longer fail the scan. Defaults to true when unset.
	DemoteFixtureFindings *bool `json:"demote_fixture_findings,omitempty" yaml:"demote_fixture_findings,omitempty"`
}

type SupplyChainRulesConfig struct {
	RequireLockfile     *bool                         `json:"require_lockfile,omitempty" yaml:"require_lockfile,omitempty"`
	DetectLockfileDrift *bool                         `json:"detect_lockfile_drift,omitempty" yaml:"detect_lockfile_drift,omitempty"`
	DetectUnpinned      *bool                         `json:"detect_unpinned,omitempty" yaml:"detect_unpinned,omitempty"`
	AllowedLicenses     []string                      `json:"allowed_licenses,omitempty" yaml:"allowed_licenses,omitempty"`
	DeniedLicenses      []string                      `json:"denied_licenses,omitempty" yaml:"denied_licenses,omitempty"`
	LicenseCommands     map[string]CommandCheckConfig `json:"license_commands,omitempty" yaml:"license_commands,omitempty"`
}

// ContextRulesConfig tunes the agent-context legibility family. Nil toggles
// default to enabled, matching the rest of the rule pack defaults.
type ContextRulesConfig struct {
	DetectMissingAgentDocs *bool `json:"detect_missing_agent_docs,omitempty" yaml:"detect_missing_agent_docs,omitempty"`
	DetectAgentDocsDrift   *bool `json:"detect_agent_docs_drift,omitempty" yaml:"detect_agent_docs_drift,omitempty"`
	DetectReadmeDrift      *bool `json:"detect_readme_drift,omitempty" yaml:"detect_readme_drift,omitempty"`
	DetectOversizedFiles   *bool `json:"detect_oversized_files,omitempty" yaml:"detect_oversized_files,omitempty"`
	DetectAmbiguousSymbols *bool `json:"detect_ambiguous_symbols,omitempty" yaml:"detect_ambiguous_symbols,omitempty"`
	// MaxFileLines is the agent context budget for a single source file.
	// Distinct from quality_rules.max_file_lines: this threshold is about how
	// much of an agent's context window one unit of work consumes, so its
	// default (1500) is intentionally looser than the maintainability limit.
	MaxFileLines int `json:"max_file_lines,omitempty" yaml:"max_file_lines,omitempty"`
	// AmbiguousSymbolThreshold is the number of source files sharing one
	// basename at which the basename is reported as ambiguous (default 4).
	AmbiguousSymbolThreshold int `json:"ambiguous_symbol_threshold,omitempty" yaml:"ambiguous_symbol_threshold,omitempty"`
}

type CommandCheckConfig struct {
	Name    string   `json:"name" yaml:"name"`
	Command string   `json:"command" yaml:"command"`
	Args    []string `json:"args,omitempty" yaml:"args,omitempty"`
}
