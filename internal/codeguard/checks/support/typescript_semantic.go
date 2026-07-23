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
	"sync"

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

var (
	typeScriptSemanticCacheMu sync.Mutex
	typeScriptSemanticCache   = make(map[string]TypeScriptSemanticResults)
	typeScriptSemanticFlights = make(map[string]*typeScriptSemanticFlight)
)

type typeScriptSemanticFlight struct {
	done    chan struct{}
	results TypeScriptSemanticResults
	err     error
}

func AnalyzeTypeScriptTarget(ctx context.Context, target core.TargetConfig, cfg core.Config) (TypeScriptSemanticResults, bool, error) {
	return analyzeTypeScriptTarget(ctx, target, cfg, nil)
}

// AnalyzeTypeScriptTargetForContext uses the runner's already-filtered corpus
// as the TypeScript program roots. Calling TypeScript's recursive discovery
// directly would bypass Codeguard's target exclusions.
func AnalyzeTypeScriptTargetForContext(ctx context.Context, env Context, target core.TargetConfig) (TypeScriptSemanticResults, bool, error) {
	return analyzeTypeScriptTarget(ctx, target, env.Config, TypeScriptTargetSourceFiles(env, target))
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

// TypeScriptTargetSourceFiles filters the shared corpus list for semantic
// analysis. A nil result retains the direct-call fallback for unit consumers
// that do not construct a runner Context.
func TypeScriptTargetSourceFiles(env Context, target core.TargetConfig) []string {
	if env.ListTargetFiles == nil {
		return nil
	}
	files, err := env.ListTargetFiles(target)
	if err != nil {
		return nil
	}
	sourceFiles := make([]string, 0, len(files))
	for _, file := range files {
		if IsTypeScriptLikeFile(file) {
			sourceFiles = append(sourceFiles, file)
		}
	}
	return sourceFiles
}

func typeScriptSemanticFlightFor(cacheKey string) (*typeScriptSemanticFlight, bool) {
	typeScriptSemanticCacheMu.Lock()
	defer typeScriptSemanticCacheMu.Unlock()
	if flight, ok := typeScriptSemanticFlights[cacheKey]; ok {
		return flight, false
	}
	flight := &typeScriptSemanticFlight{done: make(chan struct{})}
	typeScriptSemanticFlights[cacheKey] = flight
	return flight, true
}

func typeScriptSemanticFinishFlight(cacheKey string, flight *typeScriptSemanticFlight) {
	typeScriptSemanticCacheMu.Lock()
	delete(typeScriptSemanticFlights, cacheKey)
	close(flight.done)
	typeScriptSemanticCacheMu.Unlock()
}

func cachedTypeScriptSemanticResults(cacheKey string) (TypeScriptSemanticResults, bool) {
	typeScriptSemanticCacheMu.Lock()
	defer typeScriptSemanticCacheMu.Unlock()
	results, ok := typeScriptSemanticCache[cacheKey]
	return results, ok
}

func storeTypeScriptSemanticResults(cacheKey string, results TypeScriptSemanticResults) {
	typeScriptSemanticCacheMu.Lock()
	defer typeScriptSemanticCacheMu.Unlock()
	typeScriptSemanticCache[cacheKey] = results
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
