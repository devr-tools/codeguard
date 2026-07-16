package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// performanceAIFixTemplates covers the LLM-assisted performance lens.
var performanceAIFixTemplates = map[string]core.FixTemplate{
	"performance.ai.semantic-perf":    {Kind: guided, Text: "Compute the expensive result once and reuse it: cache or memoize repeated calls, hoist invariant work out of the loop or request path, batch per-item lookups, or pick a data structure that matches the realistic input sizes.\n\nBefore:\nfor _, order := range orders {\n\trate, _ := fetchExchangeRate(order.Currency) // same currencies re-fetched per order\n\ttotal += order.Amount * rate\n}\n\nAfter:\nrates := map[string]float64{}\nfor _, order := range orders {\n\trate, ok := rates[order.Currency]\n\tif !ok {\n\t\trate, _ = fetchExchangeRate(order.Currency)\n\t\trates[order.Currency] = rate\n\t}\n\ttotal += order.Amount * rate\n}"},
	"performance.ai.semantic-runtime": {Kind: guided, Text: "Point the semantic review provider at an installed command that returns valid JSON, or disable semantic review (or the performance section) explicitly.\n\nBefore:\n{\n  \"checks\": { \"performance\": true },\n  \"ai\": {\n    \"semantic\": { \"enabled\": true },\n    \"provider\": { \"type\": \"command\", \"command\": \"missing-reviewer\" }\n  }\n}\n\nAfter:\n{\n  \"checks\": { \"performance\": true },\n  \"ai\": {\n    \"semantic\": { \"enabled\": true },\n    \"provider\": { \"type\": \"command\", \"command\": \"/usr/local/bin/semantic-reviewer\" }\n  }\n}\n// run the command by hand and fix any crash or malformed JSON it reports"},
}
