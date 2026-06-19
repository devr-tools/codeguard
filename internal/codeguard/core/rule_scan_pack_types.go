package core

type ScanMode string

const (
	ScanModeFull ScanMode = "full"
	ScanModeDiff ScanMode = "diff"
)

type ScanOptions struct {
	Mode      ScanMode
	BaseRef   string
	DiffText  string
	EnableAI  bool
	EnableFix bool
	// OnSectionComplete, when set, is invoked once per section as soon as that
	// section finishes, enabling callers (e.g. the MCP server) to stream
	// partial results. It is never serialized — json.Marshal errors on a
	// non-nil func field, so the json:"-" tag is required.
	OnSectionComplete func(SectionResult) `json:"-"`
}

type RulePackConfig struct {
	Name        string             `json:"name" yaml:"name"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
	Rules       []CustomRuleConfig `json:"rules" yaml:"rules"`
}

type CustomRuleConfig struct {
	ID              string   `json:"id" yaml:"id"`
	Section         string   `json:"section,omitempty" yaml:"section,omitempty"`
	Severity        string   `json:"severity,omitempty" yaml:"severity,omitempty"`
	Title           string   `json:"title" yaml:"title"`
	Description     string   `json:"description,omitempty" yaml:"description,omitempty"`
	Message         string   `json:"message" yaml:"message"`
	HowToFix        string   `json:"how_to_fix,omitempty" yaml:"how_to_fix,omitempty"`
	NaturalLanguage string   `json:"natural_language,omitempty" yaml:"natural_language,omitempty"`
	Paths           []string `json:"paths,omitempty" yaml:"paths,omitempty"`
	Exclude         []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	FileExtensions  []string `json:"file_extensions,omitempty" yaml:"file_extensions,omitempty"`
	PathRegex       string   `json:"path_regex,omitempty" yaml:"path_regex,omitempty"`
	ContentRegex    string   `json:"content_regex,omitempty" yaml:"content_regex,omitempty"`
	AIPrompt        string   `json:"ai_prompt,omitempty" yaml:"ai_prompt,omitempty"`
}

type PolicyProfile struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}
