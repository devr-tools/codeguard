package whatsnew

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/safehttp"
)

const (
	// defaultRepo is the GitHub owner/repo whose releases are checked.
	defaultRepo = "devr-tools/codeguard"
	// defaultCacheTTL bounds how often the upstream release API is queried.
	defaultCacheTTL = 24 * time.Hour
	// fetchTimeout bounds a single upstream release lookup so the banner never
	// blocks the menu for long.
	fetchTimeout = 1500 * time.Millisecond
	// cacheFileName is the on-disk cache under the user cache directory.
	cacheFileName = "update-check.json"
	// maxReleaseBytes caps how much of the release API response is read.
	maxReleaseBytes = 1 << 20 // 1 MiB
	// disableEnv opts out of the upstream update check entirely.
	disableEnv = "CODEGUARD_NO_UPDATE_CHECK"
)

// FetchFunc returns the latest available version tag from upstream.
type FetchFunc func(ctx context.Context) (string, error)

// UpdateChecker resolves the latest available codeguard version, caching the
// result on disk and degrading gracefully when offline or opted out.
type UpdateChecker struct {
	// CacheDir is the directory holding the cache file. When empty, the check
	// runs without persistence.
	CacheDir string
	// TTL is how long a cached result is considered fresh.
	TTL time.Duration
	// Now returns the current time (injectable for tests).
	Now func() time.Time
	// Disabled short-circuits the check, returning no version.
	Disabled bool
	// Fetch performs the upstream lookup when the cache is stale/missing.
	Fetch FetchFunc
}

// DefaultChecker builds the production update checker: disabled under CI or the
// opt-out env var, cached under the user cache directory, and backed by an
// SSRF-guarded GitHub releases lookup.
func DefaultChecker() *UpdateChecker {
	c := &UpdateChecker{
		TTL:      defaultCacheTTL,
		Now:      time.Now,
		Disabled: updateCheckDisabled(),
		Fetch:    NewGitHubFetcher(safehttp.Client(fetchTimeout), "https://api.github.com", defaultRepo),
	}
	if dir, err := os.UserCacheDir(); err == nil {
		c.CacheDir = filepath.Join(dir, "codeguard")
	}
	return c
}

// LatestVersion returns the newest available version and whether one is known.
// It prefers a fresh cache entry, falls back to an upstream fetch, and returns
// a stale cached value if the fetch fails. It never returns an error: any
// failure simply yields ok=false so the banner degrades gracefully.
func (c *UpdateChecker) LatestVersion(ctx context.Context) (string, bool) {
	if c == nil || c.Disabled {
		return "", false
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}

	cached, haveCache := c.readCache()
	if haveCache && now().Sub(cached.CheckedAt) < ttl {
		return cached.LatestVersion, cached.LatestVersion != ""
	}

	if c.Fetch == nil {
		if haveCache {
			return cached.LatestVersion, cached.LatestVersion != ""
		}
		return "", false
	}

	version, err := c.Fetch(ctx)
	if err != nil {
		// Fetch failed (offline, rate-limited): fall back to any stale value.
		if haveCache {
			return cached.LatestVersion, cached.LatestVersion != ""
		}
		return "", false
	}
	version = NormalizeVersion(version)
	c.writeCache(cacheEntry{CheckedAt: now(), LatestVersion: version})
	return version, version != ""
}

// NewGitHubFetcher returns a FetchFunc that reads the latest release tag from
// the GitHub releases API. baseURL is the API root (e.g. https://api.github.com)
// and repo is "owner/name". It is exported so callers and tests can supply a
// custom client or endpoint.
func NewGitHubFetcher(client *http.Client, baseURL, repo string) FetchFunc {
	return func(ctx context.Context) (string, error) {
		endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(baseURL, "/"), repo)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil) //nolint:gosec // endpoint host is codeguard's own constant GitHub API root.
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "codeguard-update-check")

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("github releases: unexpected status %d", resp.StatusCode)
		}

		var payload struct {
			TagName string `json:"tag_name"`
		}
		body := io.LimitReader(resp.Body, maxReleaseBytes)
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return "", err
		}
		tag := strings.TrimSpace(payload.TagName)
		if tag == "" {
			return "", fmt.Errorf("github releases: empty tag_name")
		}
		return tag, nil
	}
}
