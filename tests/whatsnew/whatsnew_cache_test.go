package whatsnew_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/whatsnew"
)

func TestLatestVersionFetchesAndCaches(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	checker := &whatsnew.UpdateChecker{
		CacheDir: dir,
		TTL:      24 * time.Hour,
		Now:      fixedNow("2026-07-01T00:00:00Z"),
		Fetch: func(context.Context) (string, error) {
			calls++
			return "v0.7.0", nil
		},
	}
	got, ok := checker.LatestVersion(context.Background())
	if !ok || got != "0.7.0" {
		t.Fatalf("LatestVersion = %q,%v; want 0.7.0,true", got, ok)
	}
	checker2 := &whatsnew.UpdateChecker{
		CacheDir: dir,
		TTL:      24 * time.Hour,
		Now:      fixedNow("2026-07-01T01:00:00Z"),
		Fetch: func(context.Context) (string, error) {
			t.Fatal("fetch should not run when cache is fresh")
			return "", nil
		},
	}
	got, ok = checker2.LatestVersion(context.Background())
	if !ok || got != "0.7.0" {
		t.Fatalf("cached LatestVersion = %q,%v; want 0.7.0,true", got, ok)
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
	if _, err := os.Stat(filepath.Join(dir, "update-check.json")); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
}

func TestLatestVersionStaleTriggersRefetch(t *testing.T) {
	dir := t.TempDir()
	seed := &whatsnew.UpdateChecker{
		CacheDir: dir,
		TTL:      24 * time.Hour,
		Now:      fixedNow("2026-06-01T00:00:00Z"),
		Fetch:    func(context.Context) (string, error) { return "0.6.0", nil },
	}
	seed.LatestVersion(context.Background())

	refetched := false
	stale := &whatsnew.UpdateChecker{
		CacheDir: dir,
		TTL:      24 * time.Hour,
		Now:      fixedNow("2026-07-01T00:00:00Z"),
		Fetch: func(context.Context) (string, error) {
			refetched = true
			return "0.7.0", nil
		},
	}
	got, ok := stale.LatestVersion(context.Background())
	if !refetched {
		t.Fatal("expected refetch when cache is stale")
	}
	if !ok || got != "0.7.0" {
		t.Fatalf("stale refetch = %q,%v; want 0.7.0,true", got, ok)
	}
}

func TestLatestVersionFetchErrorFallsBackToStale(t *testing.T) {
	dir := t.TempDir()
	seed := &whatsnew.UpdateChecker{
		CacheDir: dir,
		TTL:      time.Hour,
		Now:      fixedNow("2026-06-01T00:00:00Z"),
		Fetch:    func(context.Context) (string, error) { return "0.6.0", nil },
	}
	seed.LatestVersion(context.Background())

	offline := &whatsnew.UpdateChecker{
		CacheDir: dir,
		TTL:      time.Hour,
		Now:      fixedNow("2026-07-01T00:00:00Z"),
		Fetch:    func(context.Context) (string, error) { return "", errors.New("offline") },
	}
	got, ok := offline.LatestVersion(context.Background())
	if !ok || got != "0.6.0" {
		t.Fatalf("expected stale fallback 0.6.0,true; got %q,%v", got, ok)
	}
}

func TestLatestVersionDisabled(t *testing.T) {
	checker := &whatsnew.UpdateChecker{
		Disabled: true,
		Fetch:    func(context.Context) (string, error) { t.Fatal("must not fetch when disabled"); return "", nil },
	}
	if _, ok := checker.LatestVersion(context.Background()); ok {
		t.Fatal("disabled checker must report ok=false")
	}
}

func TestNilCheckerIsSafe(t *testing.T) {
	var checker *whatsnew.UpdateChecker
	if _, ok := checker.LatestVersion(context.Background()); ok {
		t.Fatal("nil checker must report ok=false")
	}
}
