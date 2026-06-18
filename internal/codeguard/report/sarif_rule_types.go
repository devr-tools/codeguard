package report

type sarifRule struct {
	ID               string `json:"id"`
	ShortDescription struct {
		Text string `json:"text"`
	} `json:"shortDescription"`
	FullDescription struct {
		Text string `json:"text"`
	} `json:"fullDescription"`
	Help struct {
		Text string `json:"text,omitempty"`
	} `json:"help,omitempty"`
	Properties    *sarifRuleProperties `json:"properties,omitempty"`
	Relationships []sarifRelationship  `json:"relationships,omitempty"`
}

type sarifRuleProperties struct {
	Tags  []string `json:"tags,omitempty"`
	OWASP string   `json:"owasp,omitempty"`
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
