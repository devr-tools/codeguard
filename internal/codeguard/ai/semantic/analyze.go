package semantic

import (
	"context"
	"os"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const enableEnvKey = "CODEGUARD_SEMANTIC_CHECKS"

func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(enableEnvKey))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func Command() string {
	return strings.TrimSpace(os.Getenv(commandEnvKey))
}

func Analyze(ctx context.Context, opts Options) ([]core.Finding, error) {
	if !opts.Enabled || !commandConfigured(opts.Command) {
		return nil, nil
	}
	req, ok := buildRequest(opts)
	if !ok {
		return nil, nil
	}
	cache := loadVerdictCache(opts.CachePath)
	key := requestHash(req)
	if key != "" {
		if entry, ok := cache.entries[key]; ok {
			return findingsFromResponse(opts.NewFinding, entry.Response), nil
		}
	}
	resp, err := runCommand(ctx, opts.Command, req)
	if err != nil {
		return nil, err
	}
	if key != "" {
		cache.entries[key] = cacheEntry{Response: resp}
		cache.dirty = true
		_ = cache.save()
	}
	return findingsFromResponse(opts.NewFinding, resp), nil
}

type Options struct {
	Target     core.TargetConfig
	Language   string
	BaseRef    string
	DiffText   string
	CachePath  string
	Command    string
	Enabled    bool
	NewFinding func(ruleID string, level string, path string, line int, message string) core.Finding
}
