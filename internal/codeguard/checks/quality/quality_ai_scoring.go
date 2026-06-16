package quality

var aiSlopRuleWeights = map[string]int{
	"quality.ai.swallowed-error":        4,
	"quality.ai.narrative-comment":      1,
	"quality.ai.hallucinated-import":    5,
	"quality.ai.dead-code":              3,
	"quality.ai.over-mocked-test":       3,
	"quality.ai.local-idiom-drift":      2,
	"quality.ai.error-style-drift":      2,
	"quality.ai.naming-drift":           1,
	"quality.ai.provenance-policy":      2,
	"quality.ai.semantic-doc-mismatch":  3,
	"quality.ai.semantic-error-message": 4,
	"quality.ai.semantic-test-coverage": 4,
}

func scoreFindings(findings []string) int {
	total := 0
	for _, ruleID := range findings {
		total += aiSlopRuleWeights[ruleID]
	}
	return minInt(total*10, 100)
}
