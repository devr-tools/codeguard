package core

type RuleExecutionModel string

const (
	RuleExecutionModelGoNative         RuleExecutionModel = "go-native"
	RuleExecutionModelLanguageAgnostic RuleExecutionModel = "language-agnostic"
	RuleExecutionModelCommandDriven    RuleExecutionModel = "command-driven"
)

type RuleLanguage string

const (
	RuleLanguageGo         RuleLanguage = "go"
	RuleLanguagePython     RuleLanguage = "python"
	RuleLanguageTypeScript RuleLanguage = "typescript"
	RuleLanguageJavaScript RuleLanguage = "javascript"
	RuleLanguageRust       RuleLanguage = "rust"
	RuleLanguageJava       RuleLanguage = "java"
	RuleLanguageCPP        RuleLanguage = "cpp"
	RuleLanguageCSharp     RuleLanguage = "csharp"
	RuleLanguageRuby       RuleLanguage = "ruby"
)

type RuleLanguageCoverageMode string

const (
	RuleLanguageCoverageFixed          RuleLanguageCoverageMode = "fixed"
	RuleLanguageCoverageRepositoryWide RuleLanguageCoverageMode = "repository-wide"
	RuleLanguageCoverageConfigurable   RuleLanguageCoverageMode = "configurable"
)

type RuleLanguageCoverage struct {
	Mode      RuleLanguageCoverageMode `json:"mode"`
	Languages []RuleLanguage           `json:"languages,omitempty"`
}

// FixTemplateKind classifies how much judgment applying a fix template needs.
type FixTemplateKind string

const (
	// FixTemplateKindDeterministic marks a mechanical fix an agent or codemod
	// can apply with near-zero judgment, such as removing a debugger
	// statement, pinning a version, or running a formatter.
	FixTemplateKindDeterministic FixTemplateKind = "deterministic"
	// FixTemplateKindGuided marks a fix that requires judgment; the template
	// shows the shape of the change rather than an exact rewrite.
	FixTemplateKindGuided FixTemplateKind = "guided"
)

// FixTemplate is a concrete, agent-actionable fix instruction: a short
// imperative summary plus a before/after snippet, classified by how
// mechanically it can be applied.
type FixTemplate struct {
	Kind FixTemplateKind `json:"kind,omitempty"`
	Text string          `json:"text,omitempty"`
}

// IsZero reports whether the template carries no content.
func (t FixTemplate) IsZero() bool {
	return t.Kind == "" && t.Text == ""
}

type RuleMetadata struct {
	ID               string               `json:"id"`
	Section          string               `json:"section"`
	DefaultLevel     string               `json:"default_level"`
	ExecutionModel   RuleExecutionModel   `json:"execution_model"`
	LanguageCoverage RuleLanguageCoverage `json:"language_coverage"`
	Title            string               `json:"title"`
	Description      string               `json:"description"`
	HowToFix         string               `json:"how_to_fix,omitempty"`
	FixTemplate      FixTemplate          `json:"fix_template,omitzero"`
	// OWASPCategory maps the rule to an OWASP Top 10 (2021) category. Empty when
	// the rule is not associated with a fixed category (e.g. command-driven
	// rules whose category depends on the external tool).
	OWASPCategory OWASPCategory `json:"owasp_category,omitempty"`
	// CWE lists the MITRE CWE weaknesses the rule detects, as "CWE-<n>"
	// identifiers. SARIF output turns them into external/cwe tags so GitHub
	// code scanning can filter and group codeguard alerts with other tools'.
	CWE []string `json:"cwe,omitempty"`
	// Confidence is the confidence the rule's findings typically carry
	// (high, medium, or low). Empty means unspecified: individual findings
	// set their own, and consumers treat an empty value as medium.
	Confidence string `json:"confidence,omitempty"`
	// Examples are short bad/good snippets. tests/rules verifies that each
	// bad snippet triggers the rule and each good snippet stays silent.
	Examples []RuleExample `json:"examples,omitempty"`
	// ConfigKeys are the dotted config paths (as accepted by `scan -set`)
	// that turn the rule on or off or tune what it reports.
	ConfigKeys []RuleConfigKey `json:"config_keys,omitempty"`
}
