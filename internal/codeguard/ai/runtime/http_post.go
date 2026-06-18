package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/httpretry"
	"github.com/devr-tools/codeguard/internal/codeguard/ai/safehttp"
)

// postProviderJSON marshals body, POSTs it to url with the provider-specific
// headers, and returns the raw response payload after status validation.
func postProviderJSON(ctx context.Context, providerName string, url string, headers map[string]string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := httpretry.Do(ctx, providerHTTPClient(), httpretry.FromEnv(), func() (*http.Request, error) {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			httpReq.Header.Set(key, value)
		}
		return httpReq, nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(io.LimitReader(resp.Body, safehttp.MaxResponseBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai provider %s returned %s: %s", providerName, resp.Status, strings.TrimSpace(string(respData)))
	}
	return respData, nil
}
