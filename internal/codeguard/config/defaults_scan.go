package config

import (
	"fmt"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// DefaultScanConfig returns the configuration a scan uses when the repository
// has no config file at all — the `codeguard scan -folder <path>` path, which
// is the first thing a new user runs.
//
// It is the conservative baseline (baseDefaultConfig) plus section activation
// and one target. It is deliberately NOT the starter config that `codeguard
// init` writes: doing that made a user with no config file inherit codeguard's
// own house style — its workflow filenames, its `go test ./...` content
// marker, its `tests/**` test layout and its cmd//pkg layering rules — all at
// fail level, on a repository codeguard had never seen.
//
// Section activation is expressed as checks.use_recommended_defaults rather
// than by assembling the section booleans here. That keeps exactly one
// definition of "the recommended set" (enableRecommendedChecks), keeps
// checks.disabled working, and means a future baseline change reaches this
// path automatically instead of needing a second edit.
//
// The target keeps path "." and declares no language. The runner's
// ApplyTargetPath retargets the single target onto the operator-supplied scan
// folder, and the per-language dispatch switches treat an empty language
// exactly like "go" (`case "", "go":`) — so behavior is unchanged while the
// config stops asserting a language, and an entrypoint, it cannot know.
func DefaultScanConfig(profile string) (core.Config, error) {
	normalized := normalizeProfile(profile)
	if normalized != "" {
		if _, ok := profileCatalog[normalized]; !ok {
			return core.Config{}, fmt.Errorf("unknown profile %q", profile)
		}
	}

	cfg := core.Config{
		Profile: normalized,
		Targets: []core.TargetConfig{{Name: "repository", Path: "."}},
		Checks:  core.CheckConfig{UseRecommendedDefaults: true},
	}
	ApplyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return core.Config{}, err
	}
	return cfg, nil
}
