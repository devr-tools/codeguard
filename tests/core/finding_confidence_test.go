package core_test

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestConfidenceRankOrdersLevels(t *testing.T) {
	if core.ConfidenceRank(core.ConfidenceHigh) <= core.ConfidenceRank(core.ConfidenceMedium) {
		t.Fatal("high must rank above medium")
	}
	if core.ConfidenceRank(core.ConfidenceMedium) <= core.ConfidenceRank(core.ConfidenceLow) {
		t.Fatal("medium must rank above low")
	}
}

// Empty and unrecognized confidence are documented as medium, so ranking must
// place them exactly where an explicit medium sits.
func TestConfidenceRankTreatsUnspecifiedAsMedium(t *testing.T) {
	medium := core.ConfidenceRank(core.ConfidenceMedium)
	for _, value := range []string{"", "   ", "unknown", "CERTAIN"} {
		if got := core.ConfidenceRank(value); got != medium {
			t.Fatalf("ConfidenceRank(%q) = %d, want medium rank %d", value, got, medium)
		}
	}
}

func TestConfidenceRankNormalizesCasingAndSpace(t *testing.T) {
	if core.ConfidenceRank(" HIGH ") != core.ConfidenceRank(core.ConfidenceHigh) {
		t.Fatal("ConfidenceRank must normalize casing and surrounding space")
	}
}

func TestMeetsConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence string
		threshold  string
		want       bool
	}{
		{name: "low threshold admits low", confidence: core.ConfidenceLow, threshold: core.ConfidenceLow, want: true},
		{name: "low threshold admits unspecified", confidence: "", threshold: core.ConfidenceLow, want: true},
		{name: "medium threshold rejects low", confidence: core.ConfidenceLow, threshold: core.ConfidenceMedium, want: false},
		{name: "medium threshold admits unspecified", confidence: "", threshold: core.ConfidenceMedium, want: true},
		{name: "medium threshold admits medium", confidence: core.ConfidenceMedium, threshold: core.ConfidenceMedium, want: true},
		{name: "medium threshold admits high", confidence: core.ConfidenceHigh, threshold: core.ConfidenceMedium, want: true},
		{name: "high threshold rejects medium", confidence: core.ConfidenceMedium, threshold: core.ConfidenceHigh, want: false},
		{name: "high threshold rejects unspecified", confidence: "", threshold: core.ConfidenceHigh, want: false},
		{name: "high threshold admits high", confidence: core.ConfidenceHigh, threshold: core.ConfidenceHigh, want: true},
		{name: "empty threshold admits everything", confidence: core.ConfidenceLow, threshold: "", want: true},
		{name: "unknown threshold admits everything", confidence: core.ConfidenceLow, threshold: "sideways", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := core.MeetsConfidence(tt.confidence, tt.threshold); got != tt.want {
				t.Fatalf("MeetsConfidence(%q, %q) = %v, want %v", tt.confidence, tt.threshold, got, tt.want)
			}
		})
	}
}
