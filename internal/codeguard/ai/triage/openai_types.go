package triage

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

type openAIVerdictPayload struct {
	Verdicts []struct {
		ContentHash string `json:"content_hash"`
		Decision    string `json:"decision"`
		Summary     string `json:"summary"`
	} `json:"verdicts"`
}
