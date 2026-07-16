package security

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// Match is a single secret/credential hit on a line. It is the unit shared by
// the in-tree finding pass and the git-history scan.
type Match struct {
	RuleID     string
	Level      string
	Message    string
	Line       int
	Column     int
	Confidence string
}

// Scanner holds the per-scan compiled allowlist, custom patterns, and entropy
// settings. Build it once with BuildScanner and reuse it across files/lines.
type Scanner struct {
	enabled        bool
	allowPaths     []string
	allowRes       []*regexp.Regexp
	customPatterns []compiledCustomPattern
	entropy        entropySettings
}

type compiledCustomPattern struct {
	id    string
	re    *regexp.Regexp
	level string
	msg   string
}

func (s Scanner) Enabled() bool { return s.enabled }

func BuildScanner(cfg *core.SecretsRulesConfig) (Scanner, []string) {
	scanner := Scanner{enabled: true}
	if cfg == nil {
		return scanner, nil
	}
	var issues []string
	if cfg.Enabled != nil {
		scanner.enabled = *cfg.Enabled
	}
	scanner.allowPaths = append([]string(nil), cfg.AllowPaths...)
	for idx, pattern := range cfg.AllowPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			issues = append(issues, fmt.Sprintf("secrets allow_patterns[%d] skipped: %v", idx, err))
			continue
		}
		scanner.allowRes = append(scanner.allowRes, re)
	}
	for _, custom := range cfg.CustomPatterns {
		if strings.TrimSpace(custom.ID) == "" {
			issues = append(issues, "secrets custom_patterns entry with an empty id skipped")
			continue
		}
		re, err := regexp.Compile(custom.Regex)
		if err != nil {
			issues = append(issues, fmt.Sprintf("secrets custom_patterns[%q] skipped: %v", custom.ID, err))
			continue
		}
		level := normalizeSecretLevel(custom.Level, "fail")
		message := strings.TrimSpace(custom.Message)
		if message == "" {
			message = "possible hardcoded credential detected (" + custom.ID + ")"
		}
		scanner.customPatterns = append(scanner.customPatterns, compiledCustomPattern{id: custom.ID, re: re, level: level, msg: message})
	}
	scanner.entropy = buildEntropySettings(cfg.Entropy)
	return scanner, issues
}

func (s Scanner) SkipPath(file string) bool {
	for _, pattern := range s.allowPaths {
		if runnersupport.MatchPattern(pattern, file) {
			return true
		}
	}
	return false
}

func (s Scanner) lineAllowed(line string) bool {
	for _, re := range s.allowRes {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func (s Scanner) matchCustom(line string) *Match {
	for _, pattern := range s.customPatterns {
		if match := pattern.re.FindStringSubmatch(line); match != nil {
			return &Match{RuleID: pattern.id, Level: pattern.level, Message: pattern.msg + ": " + maskSecret(credentialMatchValue(match))}
		}
	}
	return nil
}
