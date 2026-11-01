package report

type sarifRule struct {
	ID                   string                   `json:"id"`
	ShortDescription     sarifTextBlock           `json:"shortDescription"`
	FullDescription      sarifTextBlock           `json:"fullDescription"`
	Help                 *sarifMultiformatMessage `json:"help,omitempty"`
	HelpURI              string                   `json:"helpUri,omitempty"`
	DefaultConfiguration *sarifRuleConfiguration  `json:"defaultConfiguration,omitempty"`
	Properties           *sarifRuleProperties     `json:"properties,omitempty"`
	Relationships        []sarifRelationship      `json:"relationships,omitempty"`
}

// sarifMultiformatMessage is SARIF's multiformatMessageString: GitHub renders
// markdown in the rule help panel and falls back to text elsewhere.
type sarifMultiformatMessage struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown,omitempty"`
}

type sarifRuleConfiguration struct {
	Level string `json:"level"`
}

// sarifRuleProperties carries the property-bag keys GitHub code scanning
// reads: tags (filtering, the "security" classification and CWE grouping),
// precision (alert ordering), and security-severity (the critical/high/
// medium/low label, read only when the "security" tag is present).
type sarifRuleProperties struct {
	Tags             []string `json:"tags,omitempty"`
	Precision        string   `json:"precision,omitempty"`
	SecuritySeverity string   `json:"security-severity,omitempty"`
	OWASP            string   `json:"owasp,omitempty"`
	CWE              []string `json:"cwe,omitempty"`
}

type sarifRelationship struct {
	Target sarifReportingDescriptorReference `json:"target"`
	Kinds  []string                          `json:"kinds"`
}

type sarifReportingDescriptorReference struct {
	ID            string             `json:"id"`
	ToolComponent sarifToolComponent `json:"toolComponent"`
}

type sarifToolComponent struct {
	Name string `json:"name"`
	GUID string `json:"guid,omitempty"`
}

const (
	owaspTaxonomyName = "OWASP Top 10"
	owaspTaxonomyGUID = "f1b2c3d4-0a21-4e10-9b00-000000002021"
)
