package semantic

import (
	"context"
	"sync"
)

// The quality and performance sections issue the same combined semantic
// request (all lenses in one payload, demultiplexed by rule id) and sections
// run in parallel, so a cold cache would otherwise trigger two identical
// runtime invocations. runCommandShared collapses concurrent identical
// requests (by request hash) into a single command run; the on-disk verdict
// cache then serves every later scan.
var (
	inflightMu    sync.Mutex
	inflightCalls = map[string]*inflightCall{}

	// cacheMu serializes read-modify-write cycles on the on-disk verdict
	// cache so concurrent sections cannot drop each other's entries.
	cacheMu sync.Mutex
)

type inflightCall struct {
	done chan struct{}
	resp Response
	err  error
}

func runCommandShared(ctx context.Context, key string, command string, req Request) (Response, error) {
	if key == "" {
		return runCommand(ctx, command, req)
	}
	inflightMu.Lock()
	if call, ok := inflightCalls[key]; ok {
		inflightMu.Unlock()
		select {
		case <-call.done:
			return call.resp, call.err
		case <-ctx.Done():
			return Response{}, ctx.Err()
		}
	}
	call := &inflightCall{done: make(chan struct{})}
	inflightCalls[key] = call
	inflightMu.Unlock()

	call.resp, call.err = runCommand(ctx, command, req)
	close(call.done)

	inflightMu.Lock()
	delete(inflightCalls, key)
	inflightMu.Unlock()
	return call.resp, call.err
}

func cachedResponse(path string, key string) (Response, bool) {
	if key == "" {
		return Response{}, false
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache := loadVerdictCache(path)
	entry, ok := cache.entries[key]
	return entry.Response, ok
}

func storeCachedResponse(path string, key string, resp Response) {
	if key == "" {
		return
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache := loadVerdictCache(path)
	cache.entries[key] = cacheEntry{Response: resp}
	cache.dirty = true
	_ = cache.save()
}
