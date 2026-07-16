package triage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func decodeJSONVerdicts(resp *http.Response, decode func(*json.Decoder) (string, error)) (map[string]providerVerdict, error) {
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai triage provider returned %s", resp.Status)
	}

	text, err := decode(json.NewDecoder(resp.Body))
	if err != nil {
		return nil, err
	}
	return parseVerdictText(text)
}

var errNoChoices = providerDecodeError("ai triage provider returned no choices")
var errNoContentBlocks = providerDecodeError("ai triage provider returned no content blocks")

type providerDecodeError string

func (err providerDecodeError) Error() string {
	return string(err)
}

func defaultBaseURL(baseURL string, fallback string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return fallback
	}
	return baseURL
}
