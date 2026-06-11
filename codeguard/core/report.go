package core

import "time"

type Report struct {
	Name        string          `json:"name"`
	GeneratedAt time.Time       `json:"generated_at"`
	ScanMode    ScanMode        `json:"scan_mode,omitempty"`
	BaseRef     string          `json:"base_ref,omitempty"`
	Sections    []SectionResult `json:"sections"`
	Summary     Summary         `json:"summary"`
}

type Summary struct {
	PassedSections  int `json:"passed_sections"`
	WarnedSections  int `json:"warned_sections"`
	FailedSections  int `json:"failed_sections"`
	SkippedSections int `json:"skipped_sections"`
}

type SectionResult struct {
	Name     string    `json:"name"`
	Status   Status    `json:"status"`
	Note     string    `json:"note,omitempty"`
	Findings []Finding `json:"findings,omitempty"`
}

type Finding struct {
	Path     string   `json:"path,omitempty"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
}

type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)
