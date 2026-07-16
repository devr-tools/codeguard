package triage

import (
	"encoding/json"
	"net/http"
)

func decodeTextVerdicts[T any](resp *http.Response, decode func(*json.Decoder) (T, error), extract func(T) (string, error)) (map[string]providerVerdict, error) {
	return decodeJSONVerdicts(resp, func(decoder *json.Decoder) (string, error) {
		payload, err := decode(decoder)
		if err != nil {
			return "", err
		}
		return extract(payload)
	})
}
