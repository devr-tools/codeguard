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
	req, ok := buildRequest(opts) //nolint:contextcheck // git helpers use a contained timeout; deeper ctx threading is a tracked follow-up
	if !ok {
		return nil, nil
	}
	key := requestHash(req)
	if resp, ok := cachedResponse(opts.CachePath, key); ok {
		return findingsFromResponse(opts.NewFinding, opts.EmitRule, resp), nil
	}
	resp, err := runCommandShared(ctx, key, opts.Command, req)
	if err != nil {
		return nil, err
	}
	storeCachedResponse(opts.CachePath, key, resp)
	return findingsFromResponse(opts.NewFinding, opts.EmitRule, resp), nil
}

type Options struct {
	Target         core.TargetConfig
	Language       string
	BaseRef        string
	DiffText       string
	CachePath      string
	Command        string
	Enabled        bool
	CheckSelection CheckSelection
	// EmitRule filters which verdict rule ids this caller emits as findings
	// (nil emits every supported rule). The quality and performance sections
	// share one combined request/response and demultiplex it by rule id here.
	EmitRule   func(ruleID string) bool
	NewFinding func(ruleID string, level string, path string, line int, message string) core.Finding
}
