package report

type sarifResult struct {
	RuleID              string                 `json:"ruleId"`
	Level               string                 `json:"level"`
	Message             sarifMessage           `json:"message"`
	Locations           []sarifLocation        `json:"locations,omitempty"`
	CodeFlows           []sarifCodeFlow        `json:"codeFlows,omitempty"`
	PartialFingerprints map[string]string      `json:"partialFingerprints,omitempty"`
	Properties          *sarifResultProperties `json:"properties,omitempty"`
}

// sarifResultProperties is the SARIF result property bag. Confidence carries
// the finding's confidence ("high", "medium", "low") when the check set one.
type sarifResultProperties struct {
	Confidence string            `json:"confidence,omitempty"`
	Kind       string            `json:"kind,omitempty"`
	Evidence   []string          `json:"evidence,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation *sarifPhysicalLocation `json:"physicalLocation,omitempty"`
	Message          *sarifMessage          `json:"message,omitempty"`
}

// sarifPhysicalLocation leaves Region nil when no line is known: the schema
// requires a region to carry startLine (or an offset), so an empty region
// object is invalid.
type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
}

type sarifCodeFlow struct {
	Message     *sarifMessage     `json:"message,omitempty"`
	ThreadFlows []sarifThreadFlow `json:"threadFlows"`
}

type sarifThreadFlow struct {
	Locations []sarifThreadFlowLocation `json:"locations"`
}

type sarifThreadFlowLocation struct {
	Location sarifLocation `json:"location"`
	Kinds    []string      `json:"kinds,omitempty"`
}
