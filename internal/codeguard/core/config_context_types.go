package core

// ContextRulesConfig tunes the agent-context legibility family. Nil toggles
// default to enabled, matching the rest of the rule pack defaults.
type ContextRulesConfig struct {
	DetectMissingAgentDocs *bool `json:"detect_missing_agent_docs,omitempty" yaml:"detect_missing_agent_docs,omitempty"`
	DetectAgentDocsDrift   *bool `json:"detect_agent_docs_drift,omitempty" yaml:"detect_agent_docs_drift,omitempty"`
	DetectReadmeDrift      *bool `json:"detect_readme_drift,omitempty" yaml:"detect_readme_drift,omitempty"`
	DetectOversizedFiles   *bool `json:"detect_oversized_files,omitempty" yaml:"detect_oversized_files,omitempty"`
	DetectAmbiguousSymbols *bool `json:"detect_ambiguous_symbols,omitempty" yaml:"detect_ambiguous_symbols,omitempty"`
	// DetectUndocumentedCommands warns when a high-signal Makefile target or
	// package.json script is not mentioned by any agent instruction file or the
	// root README. Silent when the repo has no agent docs at all.
	DetectUndocumentedCommands *bool `json:"detect_undocumented_commands,omitempty" yaml:"detect_undocumented_commands,omitempty"`
	// DetectOversizedAgentDocs warns when an agent instruction file exceeds
	// MaxAgentDocLines, consuming the context budget it exists to save.
	DetectOversizedAgentDocs *bool `json:"detect_oversized_agent_docs,omitempty" yaml:"detect_oversized_agent_docs,omitempty"`
	// DetectDocLinkRot warns when a markdown link in an agent doc or the root
	// README points at a repository path that does not exist.
	DetectDocLinkRot *bool `json:"detect_doc_link_rot,omitempty" yaml:"detect_doc_link_rot,omitempty"`
	// MaxFileLines is the agent context budget for a single source file.
	// Distinct from quality_rules.max_file_lines: this threshold is about how
	// much of an agent's context window one unit of work consumes, so its
	// default (1500) is intentionally looser than the maintainability limit.
	MaxFileLines int `json:"max_file_lines,omitempty" yaml:"max_file_lines,omitempty"`
	// AmbiguousSymbolThreshold is the number of source files sharing one
	// basename at which the basename is reported as ambiguous (default 4).
	AmbiguousSymbolThreshold int `json:"ambiguous_symbol_threshold,omitempty" yaml:"ambiguous_symbol_threshold,omitempty"`
	// MaxAgentDocLines is the line budget for a single agent instruction file
	// (default 600); larger docs crowd out the working context they document.
	MaxAgentDocLines int `json:"max_agent_doc_lines,omitempty" yaml:"max_agent_doc_lines,omitempty"`
	// AmbiguousSymbolIgnore lists source-file basenames that never count as
	// ambiguous: conventional names imposed by a language or framework
	// (index.ts, __init__.py, mod.rs, ...) are expected to repeat. When set it
	// REPLACES the built-in default list entirely (set it to [] to disable
	// ignoring); when omitted the documented default set applies. Ignored
	// basenames are excluded from both context.ambiguous-symbol findings and
	// the repo_legibility navigability component.
	AmbiguousSymbolIgnore []string `json:"ambiguous_symbol_ignore,omitempty" yaml:"ambiguous_symbol_ignore,omitempty"`
	// LegibilityWarnThreshold and LegibilityFailThreshold gate the
	// repo_legibility score (0-100, higher is better). Unlike the slop-score
	// thresholds — where a HIGH score is bad and the finding fires when the
	// score rises to the threshold — legibility is good-high, so the
	// context.legibility-threshold finding fires when the computed score falls
	// BELOW a threshold. 0 disables a threshold; when both are set the fail
	// threshold must be less than or equal to the warn threshold.
	LegibilityWarnThreshold int `json:"legibility_warn_threshold,omitempty" yaml:"legibility_warn_threshold,omitempty"`
	LegibilityFailThreshold int `json:"legibility_fail_threshold,omitempty" yaml:"legibility_fail_threshold,omitempty"`
	// LegibilityHistory gates persistence of the repo_legibility score trend
	// next to the scan cache (nil = enabled, mirroring
	// performance_rules.score_history and ai_checks.slop_history).
	LegibilityHistory *bool `json:"legibility_history,omitempty" yaml:"legibility_history,omitempty"`
	// LegibilityHistoryLimit caps retained repo_legibility history entries per
	// target (0 = default limit of 100).
	LegibilityHistoryLimit int `json:"legibility_history_limit,omitempty" yaml:"legibility_history_limit,omitempty"`
}

type CommandCheckConfig struct {
	Name    string   `json:"name" yaml:"name"`
	Command string   `json:"command" yaml:"command"`
	Args    []string `json:"args,omitempty" yaml:"args,omitempty"`
}
