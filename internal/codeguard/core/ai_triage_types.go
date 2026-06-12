package core

type AITriageCacheVerdict struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Decision string `json:"decision,omitempty"`
	Summary  string `json:"summary,omitempty"`
}
