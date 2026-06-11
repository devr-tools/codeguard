package support

import (
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

//go:embed typescript_semantic_runner.js
var typeScriptSemanticRunner string

var (
	typeScriptSemanticCacheMu sync.Mutex
	typeScriptSemanticCache   = make(map[string]TypeScriptSemanticResults)
)

func AnalyzeTypeScriptTarget(ctx context.Context, target core.TargetConfig, cfg core.Config) (TypeScriptSemanticResults, bool, error) {
	libPath := discoverTypeScriptLibPath(target.Path)
	if libPath == "" {
		return TypeScriptSemanticResults{}, false, nil
	}
	input := newTypeScriptSemanticInput(target, cfg, libPath)
	cacheKey, err := typeScriptSemanticCacheKey(input)
	if err != nil {
		return TypeScriptSemanticResults{}, false, err
	}
	if cached, ok := cachedTypeScriptSemanticResults(cacheKey); ok {
		return cached, true, nil
	}
	results, err := runTypeScriptSemanticRunner(ctx, input)
	if err != nil {
		return TypeScriptSemanticResults{}, true, err
	}
	storeTypeScriptSemanticResults(cacheKey, results)
	return results, true, nil
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
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:]), nil
}

func runTypeScriptSemanticRunner(ctx context.Context, input typeScriptSemanticInput) (TypeScriptSemanticResults, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return TypeScriptSemanticResults{}, err
	}
	cmd := exec.CommandContext(ctx, "node", "-e", typeScriptSemanticRunner)
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
