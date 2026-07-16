package whatsnew_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devr-tools/codeguard/internal/whatsnew"
)

func TestGitHubFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/devr-tools/codeguard/releases/latest" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.9.9","name":"ignored"}`))
	}))
	defer srv.Close()

	fetch := whatsnew.NewGitHubFetcher(srv.Client(), srv.URL, "devr-tools/codeguard")
	tag, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if tag != "v0.9.9" {
		t.Fatalf("tag = %q, want v0.9.9", tag)
	}
}

func TestGitHubFetcherNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	fetch := whatsnew.NewGitHubFetcher(srv.Client(), srv.URL, "devr-tools/codeguard")
	if _, err := fetch(context.Background()); err == nil {
		t.Fatal("expected error on non-200 response")
	}
}
