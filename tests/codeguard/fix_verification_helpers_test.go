package codeguard_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

type stubFixGenerator struct {
	candidate codeguard.FixCandidate
	calls     int
}

func (s *stubFixGenerator) GenerateFix(_ context.Context, input codeguard.FixGenerateInput) (codeguard.FixCandidate, error) {
	if strings.TrimSpace(input.Analysis) == "" {
		return codeguard.FixCandidate{}, fmt.Errorf("analysis should be forwarded to the generator")
	}
	s.calls++
	return s.candidate, nil
}

func firstFinding(t *testing.T, cfg codeguard.Config) codeguard.Finding {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, section := range report.Sections {
		if len(section.Findings) > 0 {
			return section.Findings[0]
		}
	}
	t.Fatalf("expected at least one finding in %#v", report)
	return codeguard.Finding{}
}

func qualityOnlyConfig(dir string, name string) codeguard.Config {
	return qualityOnlyConfigForLanguage(dir, name, "go")
}

func qualityOnlyConfigForLanguage(dir string, name string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}
