package codeguard_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// A configuration that says nothing about confidence must behave exactly as it
// does today: every finding is admitted and nothing is demoted.
func TestConfidencePolicyDefaultsToPermissive(t *testing.T) {
	var policy codeguard.ConfidencePolicyConfig
	if got := policy.Threshold("security"); got != codeguard.ConfidenceLow {
		t.Fatalf("zero-value threshold = %q, want %q", got, codeguard.ConfidenceLow)
	}

	cfg := codeguard.ExampleConfig()
	if got := cfg.Checks.MinConfidence.Threshold("security"); got != codeguard.ConfidenceLow {
		t.Fatalf("example config threshold = %q, want %q", got, codeguard.ConfidenceLow)
	}
	if cfg.Checks.ConfidenceDemotion {
		t.Fatal("confidence_demotion must default to off")
	}
}

func TestConfidencePolicyThresholdPrecedence(t *testing.T) {
	policy := codeguard.ConfidencePolicyConfig{
		Default:  codeguard.ConfidenceMedium,
		Sections: map[string]string{"security": codeguard.ConfidenceHigh},
	}
	tests := []struct {
		name    string
		section string
		want    string
	}{
		{name: "section override wins", section: "security", want: codeguard.ConfidenceHigh},
		{name: "unlisted section falls back to default", section: "quality", want: codeguard.ConfidenceMedium},
		{name: "empty section falls back to default", section: "", want: codeguard.ConfidenceMedium},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := policy.Threshold(tt.section); got != tt.want {
				t.Fatalf("Threshold(%q) = %q, want %q", tt.section, got, tt.want)
			}
		})
	}
}

// Levels and section keys are normalized the way the rest of the config is, so
// a stray capital or space is not a silent misconfiguration.
func TestConfidencePolicyThresholdNormalizesInput(t *testing.T) {
	policy := codeguard.ConfidencePolicyConfig{
		Default:  " HIGH ",
		Sections: map[string]string{" Security ": " medium "},
	}
	if got := policy.Threshold("security"); got != codeguard.ConfidenceMedium {
		t.Fatalf("Threshold(security) = %q, want %q", got, codeguard.ConfidenceMedium)
	}
	if got := policy.Threshold("quality"); got != codeguard.ConfidenceHigh {
		t.Fatalf("Threshold(quality) = %q, want %q", got, codeguard.ConfidenceHigh)
	}
}

func TestValidateConfidencePolicyRejectsUnknownValues(t *testing.T) {
	tests := []struct {
		name   string
		policy codeguard.ConfidencePolicyConfig
		want   string
	}{
		{
			name:   "unknown default level",
			policy: codeguard.ConfidencePolicyConfig{Default: "sideways"},
			want:   "checks.min_confidence.default",
		},
		{
			name:   "unknown section level",
			policy: codeguard.ConfidencePolicyConfig{Sections: map[string]string{"security": "certain"}},
			want:   "checks.min_confidence.sections",
		},
		{
			name:   "unknown section key",
			policy: codeguard.ConfidencePolicyConfig{Sections: map[string]string{"securty": codeguard.ConfidenceHigh}},
			want:   "unknown section",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfidencePolicy(tt.policy)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateConfidencePolicyAcceptsCompletePolicy(t *testing.T) {
	policy := codeguard.ConfidencePolicyConfig{
		Default: codeguard.ConfidenceMedium,
		Sections: map[string]string{
			"security":     codeguard.ConfidenceHigh,
			"quality":      codeguard.ConfidenceLow,
			"supply_chain": codeguard.ConfidenceMedium,
		},
	}
	if err := validateConfidencePolicy(policy); err != nil {
		t.Fatalf("validate complete policy: %v", err)
	}
}

// An empty level anywhere means "unspecified" and must stay valid, so a config
// can name a section without pinning it.
func TestValidateConfidencePolicyAcceptsEmptyLevels(t *testing.T) {
	policy := codeguard.ConfidencePolicyConfig{
		Sections: map[string]string{"security": ""},
	}
	if err := validateConfidencePolicy(policy); err != nil {
		t.Fatalf("validate empty levels: %v", err)
	}
}

func TestConfidencePolicyYAMLRoundTrip(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.MinConfidence = codeguard.ConfidencePolicyConfig{
		Default:  codeguard.ConfidenceMedium,
		Sections: map[string]string{"security": codeguard.ConfidenceHigh},
	}
	cfg.Checks.ConfidenceDemotion = true

	path := filepath.Join(t.TempDir(), "codeguard.yaml")
	if err := codeguard.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := codeguard.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := loaded.Checks.MinConfidence.Threshold("security"); got != codeguard.ConfidenceHigh {
		t.Fatalf("loaded security threshold = %q, want %q", got, codeguard.ConfidenceHigh)
	}
	if got := loaded.Checks.MinConfidence.Threshold("quality"); got != codeguard.ConfidenceMedium {
		t.Fatalf("loaded quality threshold = %q, want %q", got, codeguard.ConfidenceMedium)
	}
	if !loaded.Checks.ConfidenceDemotion {
		t.Fatal("loaded confidence_demotion = false, want true")
	}
}

func validateConfidencePolicy(policy codeguard.ConfidencePolicyConfig) error {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.MinConfidence = policy
	return codeguard.ValidateConfig(cfg)
}
