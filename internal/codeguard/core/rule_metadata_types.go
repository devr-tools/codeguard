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

type RuleMetadata struct {
	ID               string               `json:"id"`
	Section          string               `json:"section"`
	DefaultLevel     string               `json:"default_level"`
	ExecutionModel   RuleExecutionModel   `json:"execution_model"`
	LanguageCoverage RuleLanguageCoverage `json:"language_coverage"`
	Title            string               `json:"title"`
	Description      string               `json:"description"`
	HowToFix         string               `json:"how_to_fix,omitempty"`
	FixTemplate      string               `json:"fix_template,omitempty"`
}
