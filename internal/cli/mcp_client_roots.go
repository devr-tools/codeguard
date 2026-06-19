package cli

import "sync"

// rootsCache memoizes the client's roots per connection so a config-path check
// does not issue a fresh roots/list round trip every time. It is invalidated on
// notifications/roots/list_changed.
type rootsCache struct {
	mu     sync.Mutex
	loaded bool
	roots  []mcpRoot
}

func (c *rootsCache) invalidate() {
	c.mu.Lock()
	c.loaded = false
	c.roots = nil
	c.mu.Unlock()
}

// load returns the cached roots, fetching once via fetch on a miss. The lock is
// held across fetch so concurrent callers coalesce into a single round trip.
func (c *rootsCache) load(fetch func() ([]mcpRoot, error)) ([]mcpRoot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return c.roots, nil
	}
	roots, err := fetch()
	if err != nil {
		return nil, err
	}
	c.roots = roots
	c.loaded = true
	return roots, nil
}
