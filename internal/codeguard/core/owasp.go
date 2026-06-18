package core

import "sort"

// OWASPCategory identifies an OWASP Top 10 (2021) risk category. Values use the
// canonical "Axx:2021-Name" form so they are stable to display and to match on.
type OWASPCategory string

const (
	OWASPA01BrokenAccessControl      OWASPCategory = "A01:2021-Broken Access Control"
	OWASPA02CryptographicFailures    OWASPCategory = "A02:2021-Cryptographic Failures"
	OWASPA03Injection                OWASPCategory = "A03:2021-Injection"
	OWASPA04InsecureDesign           OWASPCategory = "A04:2021-Insecure Design"
	OWASPA05SecurityMisconfiguration OWASPCategory = "A05:2021-Security Misconfiguration"
	OWASPA06VulnerableComponents     OWASPCategory = "A06:2021-Vulnerable and Outdated Components"
	OWASPA07AuthFailures             OWASPCategory = "A07:2021-Identification and Authentication Failures"
	OWASPA08IntegrityFailures        OWASPCategory = "A08:2021-Software and Data Integrity Failures"
	OWASPA09LoggingFailures          OWASPCategory = "A09:2021-Security Logging and Monitoring Failures"
	OWASPA10SSRF                     OWASPCategory = "A10:2021-Server-Side Request Forgery (SSRF)"
)

// OWASPTop10 lists every OWASP Top 10 (2021) category in canonical order. Used
// for coverage reporting so categories without a matching rule are visible.
var OWASPTop10 = []OWASPCategory{
	OWASPA01BrokenAccessControl,
	OWASPA02CryptographicFailures,
	OWASPA03Injection,
	OWASPA04InsecureDesign,
	OWASPA05SecurityMisconfiguration,
	OWASPA06VulnerableComponents,
	OWASPA07AuthFailures,
	OWASPA08IntegrityFailures,
	OWASPA09LoggingFailures,
	OWASPA10SSRF,
}

// Code returns the short identifier of the category (e.g. "A03:2021"), or the
// empty string when the category is unset/malformed.
func (c OWASPCategory) Code() string {
	s := string(c)
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			return s[:i]
		}
	}
	return s
}

// Name returns the human-readable category name without the code prefix
// (e.g. "Injection" for "A03:2021-Injection").
func (c OWASPCategory) Name() string {
	s := string(c)
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			return s[i+1:]
		}
	}
	return s
}

// OWASPCoverageEntry records which rules cover a single OWASP Top 10 category.
type OWASPCoverageEntry struct {
	Category OWASPCategory `json:"category"`
	Code     string        `json:"code"`
	Covered  bool          `json:"covered"`
	RuleIDs  []string      `json:"rule_ids"`
}

// OWASPCoverageForRules computes, for every OWASP Top 10 (2021) category, the
// set of rules that map to it. Categories with no matching rule are returned
// with Covered=false so coverage gaps are explicit.
func OWASPCoverageForRules(rules []RuleMetadata) []OWASPCoverageEntry {
	byCategory := make(map[OWASPCategory][]string)
	for _, rule := range rules {
		if rule.OWASPCategory == "" {
			continue
		}
		byCategory[rule.OWASPCategory] = append(byCategory[rule.OWASPCategory], rule.ID)
	}
	entries := make([]OWASPCoverageEntry, 0, len(OWASPTop10))
	for _, category := range OWASPTop10 {
		ids := byCategory[category]
		sort.Strings(ids)
		entries = append(entries, OWASPCoverageEntry{
			Category: category,
			Code:     category.Code(),
			Covered:  len(ids) > 0,
			RuleIDs:  ids,
		})
	}
	return entries
}
