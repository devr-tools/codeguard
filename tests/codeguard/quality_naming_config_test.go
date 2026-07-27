package codeguard_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestExampleConfigIncludesQualityNamingDefaults(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	naming := cfg.Checks.QualityRules.Naming
	if naming.RoleSuffixWarnThreshold != 4 {
		t.Fatalf("role_suffix_warn_threshold = %d, want 4", naming.RoleSuffixWarnThreshold)
	}
	for _, want := range []string{"api", "id", "json", "sql", "url"} {
		if !containsString(naming.AllowedAbbreviations, want) {
			t.Fatalf("allowed_abbreviations missing %q: %#v", want, naming.AllowedAbbreviations)
		}
	}
}

func TestValidateQualityNamingConfig(t *testing.T) {
	tests := []struct {
		name   string
		naming codeguard.QualityNamingConfig
		want   string
	}{
		{
			name:   "negative role suffix threshold",
			naming: codeguard.QualityNamingConfig{RoleSuffixWarnThreshold: -1},
			want:   "role_suffix_warn_threshold",
		},
		{
			name: "blank glossary concept",
			naming: codeguard.QualityNamingConfig{Glossary: map[string]codeguard.QualityNamingGlossaryEntry{
				" ": {Avoid: []string{"venue"}},
			}},
			want: "blank concept",
		},
		{
			name: "blank glossary avoid entry",
			naming: codeguard.QualityNamingConfig{Glossary: map[string]codeguard.QualityNamingGlossaryEntry{
				"restaurant": {Avoid: []string{" "}},
			}},
			want: "avoid[0]",
		},
		{
			name:   "blank allowed abbreviation",
			naming: codeguard.QualityNamingConfig{AllowedAbbreviations: []string{"id", " "}},
			want:   "allowed_abbreviations[1]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := codeguard.ExampleConfig()
			cfg.Checks.QualityRules.Naming = tt.naming
			err := codeguard.ValidateConfig(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateConfig error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateQualityNamingConfigAcceptsGlossary(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.QualityRules.Naming = codeguard.QualityNamingConfig{
		Glossary: map[string]codeguard.QualityNamingGlossaryEntry{
			"restaurant": {Avoid: []string{"venue", "merchant"}},
		},
		AllowedAbbreviations:    []string{"id", "url"},
		RoleSuffixWarnThreshold: 4,
	}
	if err := codeguard.ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
