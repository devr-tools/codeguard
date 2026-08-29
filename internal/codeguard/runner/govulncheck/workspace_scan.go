package govulncheck

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

type ModuleStatus string

const (
	ModuleSucceeded ModuleStatus = "succeeded"
	ModuleFailed    ModuleStatus = "failed"
	ModuleTimedOut  ModuleStatus = "timed_out"
	ModuleSkipped   ModuleStatus = "skipped"
)

type Occurrence struct {
	Module    string
	Package   string
	CallStack []string
}

type Vulnerability struct {
	AdvisoryID  string
	Package     string
	CallStack   []string
	Occurrences []Occurrence
}

type ModuleResult struct {
	Module Module
	Status ModuleStatus
	Err    error
}

type ScanResult struct {
	Vulnerabilities []Vulnerability
	Modules         []ModuleResult
}

type ScanOptions struct {
	Concurrency   int
	ModuleTimeout time.Duration
	Execute       func(context.Context, Module) ([]Vulnerability, error)
}

func ScanWorkspace(ctx context.Context, workspace Workspace, opts ScanOptions) ScanResult {
	if opts.Concurrency < 1 {
		opts.Concurrency = 4
	}
	if opts.ModuleTimeout <= 0 {
		opts.ModuleTimeout = 2 * time.Minute
	}
	result := ScanResult{Modules: make([]ModuleResult, len(workspace.Modules))}
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	all := make([]Vulnerability, 0)
	for i, module := range workspace.Modules {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			moduleCtx, cancel := context.WithTimeout(ctx, opts.ModuleTimeout)
			defer cancel()
			vulns, err := opts.Execute(moduleCtx, module)
			status := ModuleSucceeded
			if err != nil {
				status = ModuleFailed
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(moduleCtx.Err(), context.DeadlineExceeded) {
					status = ModuleTimedOut
				}
			}
			mu.Lock()
			result.Modules[i] = ModuleResult{Module: module, Status: status, Err: err}
			for _, vulnerability := range vulns {
				vulnerability.Occurrences = append(vulnerability.Occurrences, Occurrence{Module: module.ModulePath, Package: vulnerability.Package, CallStack: append([]string(nil), vulnerability.CallStack...)})
				all = append(all, vulnerability)
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	result.Vulnerabilities = deduplicateVulnerabilities(all)
	return result
}

func deduplicateVulnerabilities(input []Vulnerability) []Vulnerability {
	byAdvisory := make(map[string]*Vulnerability)
	for _, vulnerability := range input {
		current := byAdvisory[vulnerability.AdvisoryID]
		if current == nil {
			vulnerabilityCopy := vulnerability
			vulnerabilityCopy.Occurrences = nil
			current = &vulnerabilityCopy
			byAdvisory[vulnerability.AdvisoryID] = current
		}
		current.Occurrences = append(current.Occurrences, vulnerability.Occurrences...)
	}
	keys := make([]string, 0, len(byAdvisory))
	for key := range byAdvisory {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Vulnerability, 0, len(keys))
	for _, key := range keys {
		out = append(out, *byAdvisory[key])
	}
	return out
}
