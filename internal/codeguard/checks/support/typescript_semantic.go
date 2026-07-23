package support

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

//go:embed typescript_semantic_runner_core.js
var typeScriptSemanticRunnerCore string

//go:embed typescript_semantic_runner_security.js
var typeScriptSemanticRunnerSecurity string

//go:embed typescript_semantic_runner_taint.js
var typeScriptSemanticRunnerTaint string

//go:embed typescript_semantic_runner_bootstrap.js
var typeScriptSemanticRunnerBootstrap string

var typeScriptSemanticRunner = strings.Join([]string{
	typeScriptSemanticRunnerCore,
	typeScriptSemanticRunnerSecurity,
	typeScriptSemanticRunnerTaint,
	typeScriptSemanticRunnerBootstrap,
}, "\n")

func AnalyzeTypeScriptTarget(ctx context.Context, target core.TargetConfig, cfg core.Config) (TypeScriptSemanticResults, bool, error) {
	return analyzeTypeScriptTarget(ctx, target, cfg, nil)
}

func analyzeTypeScriptTarget(ctx context.Context, target core.TargetConfig, cfg core.Config, sourceFiles []string) (TypeScriptSemanticResults, bool, error) {
	libPath := discoverTypeScriptLibPath(target.Path)
	if libPath == "" {
		return TypeScriptSemanticResults{}, false, nil
	}
	input := newTypeScriptSemanticInput(target, cfg, libPath, sourceFiles)
	cacheKey, err := typeScriptSemanticCacheKey(input)
	if err != nil {
		return TypeScriptSemanticResults{}, false, err
	}
	if cached, ok := cachedTypeScriptSemanticResults(cacheKey); ok {
		return cached, true, nil
	}

	flight, leader := typeScriptSemanticFlightFor(cacheKey)
	if !leader {
		select {
		case <-flight.done:
			return flight.results, true, flight.err
		case <-ctx.Done():
			return TypeScriptSemanticResults{}, true, ctx.Err()
		}
	}

	flight.results, flight.err = runTypeScriptSemanticRunner(ctx, input)
	if flight.err == nil {
		storeTypeScriptSemanticResults(cacheKey, flight.results)
	}
	typeScriptSemanticFinishFlight(cacheKey, flight)
	return flight.results, true, flight.err
}

func typeScriptSemanticCacheKey(input typeScriptSemanticInput) (string, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func runTypeScriptSemanticRunner(ctx context.Context, input typeScriptSemanticInput) (TypeScriptSemanticResults, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return TypeScriptSemanticResults{}, err
	}
	cmd := exec.CommandContext(ctx, "node", "-e", typeScriptSemanticRunner) //nolint:gosec // fixed node binary running an embedded constant script; input passed via stdin
	cmd.Stdin = strings.NewReader(string(payload))
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return TypeScriptSemanticResults{}, errors.New(message)
	}
	var results TypeScriptSemanticResults
	if err := json.Unmarshal(output, &results); err != nil {
		return TypeScriptSemanticResults{}, err
	}
	return results, nil
}
