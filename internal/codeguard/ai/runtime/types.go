package runtime

import "context"

type Provider interface {
	Name() string
	Evaluate(ctx context.Context, req Request) (Response, error)
}

type Request struct {
	Kind      string `json:"kind"`
	System    string `json:"system,omitempty"`
	Prompt    string `json:"prompt"`
	InputJSON string `json:"input_json,omitempty"`
}

type Response struct {
	Raw string `json:"raw"`
}

type CachedVerdict struct {
	Kind        string `json:"kind"`
	ContentHash string `json:"content_hash"`
	Raw         string `json:"raw"`
}
