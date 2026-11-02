package triage

import (
	"context"
	"sync"
	"sync/atomic"
)

// TestProvider is a deterministic triage provider for the external test
// packages under tests/. It returns the same verdict for every candidate and
// counts how many provider requests it served. It cannot be selected through
// configuration or environment variables; a test installs it with
// UseTestProviderForTest.
type TestProvider struct {
	// Decision is the verdict for every candidate ("keep" when empty).
	Decision string
	// Summary is the verdict summary for every candidate.
	Summary string
	calls   atomic.Int64
}

// Calls returns how many provider requests the provider has served.
func (p *TestProvider) Calls() int64 {
	return p.calls.Load()
}

// testProviderAdapter serves a TestProvider through the provider interface.
type testProviderAdapter struct {
	p *TestProvider
}

func (adapter testProviderAdapter) Triage(_ context.Context, candidates []candidate) (map[string]providerVerdict, error) {
	p := adapter.p
	p.calls.Add(1)
	decision := p.Decision
	if decision == "" {
		decision = "keep"
	}
	verdicts := make(map[string]providerVerdict, len(candidates))
	for _, item := range candidates {
		verdicts[item.hash] = providerVerdict{Decision: decision, Summary: p.Summary}
	}
	return verdicts, nil
}

var (
	testProviderMu   sync.RWMutex
	testProviderName string
	testProvider     *TestProvider
)

// UseTestProviderForTest makes triage accept name as a provider (selected as
// usual through CODEGUARD_AI_TRIAGE_PROVIDER or ai.provider.type, with a
// model set) and serve it with p, until the returned restore function runs.
// Only tests call it.
func UseTestProviderForTest(name string, p *TestProvider) (restore func()) {
	testProviderMu.Lock()
	testProviderName, testProvider = name, p
	testProviderMu.Unlock()
	return func() {
		testProviderMu.Lock()
		testProviderName, testProvider = "", nil
		testProviderMu.Unlock()
	}
}

// injectedTestProvider returns the provider a test installed under name, or
// nil.
func injectedTestProvider(name string) provider {
	testProviderMu.RLock()
	defer testProviderMu.RUnlock()
	if testProvider == nil || name == "" || name != testProviderName {
		return nil
	}
	return testProviderAdapter{p: testProvider}
}
