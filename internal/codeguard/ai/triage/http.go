package triage

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/httpretry"
	"github.com/devr-tools/codeguard/internal/codeguard/ai/safehttp"
)

// triageViaHTTP runs the shared triage flow: build the candidate prompt,
// shape the provider-specific request body, send it, and decode verdicts.
func triageViaHTTP(
	ctx context.Context,
	candidates []candidate,
	requestBody func(prompt string) ([]byte, error),
	doRequest func(ctx context.Context, body []byte) (*http.Response, error),
	decode func(resp *http.Response) (map[string]providerVerdict, error),
) (map[string]providerVerdict, error) {
	prompt, err := buildPrompt(candidates)
	if err != nil {
		return nil, err
	}
	body, err := requestBody(prompt)
	if err != nil {
		return nil, err
	}
	resp, err := doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	return decode(resp)
}

// postTriageJSON POSTs a JSON triage request with retry, applying the
// provider-specific headers on top of the shared Content-Type.
func postTriageJSON(ctx context.Context, cfg runtimeConfig, url string, body []byte, headers map[string]string) (*http.Response, error) {
	httpClient := safehttp.Client(cfg.Timeout)
	resp, err := httpretry.Do(ctx, httpClient, httpretry.FromEnv(), func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	// Bound the response body so a malicious endpoint cannot exhaust memory.
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.LimitReader(resp.Body, safehttp.MaxResponseBytes), resp.Body}
	return resp, nil
}
