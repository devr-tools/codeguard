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
}

type RulePackConfig struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Rules       []CustomRuleConfig `json:"rules"`
}

type CustomRuleConfig struct {
	ID              string   `json:"id"`
	Section         string   `json:"section,omitempty"`
	Severity        string   `json:"severity,omitempty"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Message         string   `json:"message"`
	HowToFix        string   `json:"how_to_fix,omitempty"`
	NaturalLanguage string   `json:"natural_language,omitempty"`
	Paths           []string `json:"paths,omitempty"`
	Exclude         []string `json:"exclude,omitempty"`
	FileExtensions  []string `json:"file_extensions,omitempty"`
	PathRegex       string   `json:"path_regex,omitempty"`
	ContentRegex    string   `json:"content_regex,omitempty"`
	AIPrompt        string   `json:"ai_prompt,omitempty"`
}

type PolicyProfile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
