package core

const ReportArtifactKindWaiverAudit = "waiver_audit"

const (
	WaiverAuditStatusActive      = "active"
	WaiverAuditStatusUnused      = "unused"
	WaiverAuditStatusExpired     = "expired"
	WaiverAuditStatusUnknownRule = "unknown_rule"
)

const (
	WaiverAuditUpgradeStatusStaleAfterUpgrade = "stale_after_upgrade"
	WaiverAuditUpgradeStatusInconclusive      = "inconclusive"
)

// WaiverAuditArtifact reports whether each configured waiver matched findings
// during the scan. It is intended for upgrade cleanup workflows where a newer
// CodeGuard release may have fixed old false positives.
type WaiverAuditArtifact struct {
	Waivers []WaiverAuditEntry `json:"waivers"`
}

type WaiverAuditEntry struct {
	Index           int    `json:"index"`
	Rule            string `json:"rule"`
	Path            string `json:"path,omitempty"`
	Reason          string `json:"reason,omitempty"`
	ExpiresOn       string `json:"expires_on,omitempty"`
	Status          string `json:"status"`
	MatchedFindings int    `json:"matched_findings"`
	// MatchedFingerprints records stable finding fingerprints matched by this
	// waiver. Empty means the waiver matched no findings in this scan.
	MatchedFingerprints []string `json:"matched_fingerprints,omitempty"`
	PreviousVersion     string   `json:"previous_version,omitempty"`
	PreviousMatches     int      `json:"previous_matched_findings,omitempty"`
	UpgradeStatus       string   `json:"upgrade_status,omitempty"`
	UpgradeReason       string   `json:"upgrade_reason,omitempty"`
}

func NewWaiverAuditArtifact(entries []WaiverAuditEntry) Artifact {
	return Artifact{
		ID:   "waiver_audit",
		Kind: ReportArtifactKindWaiverAudit,
		WaiverAudit: &WaiverAuditArtifact{
			Waivers: entries,
		},
	}
}

type WaiverAuditHistoryEntry struct {
	Timestamp         string             `json:"timestamp"`
	CodeGuardVersion  string             `json:"codeguard_version"`
	ConfigFingerprint string             `json:"config_fingerprint"`
	ScanMode          string             `json:"scan_mode"`
	BaseRef           string             `json:"base_ref,omitempty"`
	TargetPath        string             `json:"target_path,omitempty"`
	Waivers           []WaiverAuditEntry `json:"waivers"`
}
