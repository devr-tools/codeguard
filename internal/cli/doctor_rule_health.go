package cli

import (
	"fmt"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// Rule-health thresholds: a rule is flagged when at least
// ruleHealthMinFindings findings were seen in the last scan and more than
// ruleHealthMaxSuppressionRatio of them were suppressed.
const (
	ruleHealthMinFindings         = 5
	ruleHealthMaxSuppressionRatio = 0.5
)

// ruleHealthDoctorChecks surfaces broken-rule signals without running a scan:
// waivers that match no catalog rule, waivers past their expiry, and — when a
// previous scan persisted rule stats — rules whose findings are mostly
// suppressed.
func ruleHealthDoctorChecks(cfg service.Config, today time.Time) []doctorCheck {
	checks := waiverDoctorChecks(cfg, today)
	return append(checks, ruleStatsDoctorChecks(cfg)...)
}

func waiverDoctorChecks(cfg service.Config, today time.Time) []doctorCheck {
	if len(cfg.Waivers) == 0 {
		return nil
	}
	catalog := catalogRuleIDs(cfg)
	checks := make([]doctorCheck, 0, len(cfg.Waivers))
	for _, waiver := range cfg.Waivers {
		if check, flagged := waiverHealthCheck(waiver, catalog, today); flagged {
			checks = append(checks, check)
		}
	}
	if len(checks) == 0 {
		return []doctorCheck{passDoctorCheck("waivers", fmt.Sprintf("all %d waiver(s) match catalog rules and are unexpired", len(cfg.Waivers)))}
	}
	return checks
}

func waiverHealthCheck(waiver service.WaiverConfig, catalog map[string]bool, today time.Time) (doctorCheck, bool) {
	if waiver.Rule != "*" && !catalog[waiver.Rule] {
		return warnDoctorCheck("waiver:"+waiver.Rule, "waiver matches no catalog rule and never suppresses anything; remove it or fix the rule id"), true
	}
	if waiverExpired(waiver.ExpiresOn, today) {
		return warnDoctorCheck("waiver:"+waiver.Rule, fmt.Sprintf("waiver expired on %s and no longer suppresses findings; remove it or extend expires_on", waiver.ExpiresOn)), true
	}
	return doctorCheck{}, false
}

// waiverExpired mirrors the runner's suppression expiry: a waiver stops
// matching once its expires_on date is strictly before today (date-only).
func waiverExpired(expiresOn string, today time.Time) bool {
	if expiresOn == "" {
		return false
	}
	parsed, err := time.Parse("2006-01-02", expiresOn)
	if err != nil {
		return false
	}
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, parsed.Location())
	return parsed.Before(day)
}

func catalogRuleIDs(cfg service.Config) map[string]bool {
	rules := service.RulesForConfig(cfg)
	ids := make(map[string]bool, len(rules))
	for _, rule := range rules {
		ids[rule.ID] = true
	}
	return ids
}

// ruleStatsDoctorChecks reads the rule stats persisted by the most recent scan
// (see runner/support rule-stats history) and flags rules that were mostly
// suppressed. When no scan has recorded stats yet it stays silent.
func ruleStatsDoctorChecks(cfg service.Config) []doctorCheck {
	history := service.LoadRuleStatsHistory(service.RuleStatsHistoryPath(cfg))
	if len(history) == 0 {
		return nil
	}
	latest := history[len(history)-1]
	checks := make([]doctorCheck, 0, len(latest.Rules))
	for _, entry := range latest.Rules {
		suppressed := entry.Suppressed()
		total := entry.Emitted + suppressed
		if total < ruleHealthMinFindings || entry.SuppressionRatio <= ruleHealthMaxSuppressionRatio {
			continue
		}
		checks = append(checks, warnDoctorCheck("rule-health:"+entry.RuleID,
			fmt.Sprintf("%d of %d findings suppressed in the last scan; consider tuning or disabling %s", suppressed, total, entry.RuleID)))
	}
	if len(checks) == 0 {
		return []doctorCheck{passDoctorCheck("rule-health", "no rule exceeded the suppression threshold in the last scan")}
	}
	return checks
}
