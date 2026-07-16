package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// performanceRegressionFixTemplates covers the diff-only performance
// regression rules. Kept separate from performanceFixTemplates so parallel
// additions to the performance family do not conflict.
var performanceRegressionFixTemplates = map[string]core.FixTemplate{
	"performance.complexity-regression": {Kind: guided, Text: "Restructure the added nesting so it does not multiply the iteration space, or confirm the inner collection is small and bounded.\n\nBefore:\nfunc UpdateAll(users []User, orders []Order) {\n\tfor _, user := range users {\n\t\tfor _, order := range orders { // new inner loop: O(users x orders)\n\t\t\tif order.UserID == user.ID {\n\t\t\t\tapply(user, order)\n\t\t\t}\n\t\t}\n\t}\n}\n\nAfter:\nfunc UpdateAll(users []User, orders []Order) {\n\tbyUser := make(map[string][]Order, len(orders))\n\tfor _, order := range orders {\n\t\tbyUser[order.UserID] = append(byUser[order.UserID], order)\n\t}\n\tfor _, user := range users {\n\t\tfor _, order := range byUser[user.ID] { // bounded per-user slice\n\t\t\tapply(user, order)\n\t\t}\n\t}\n}"},
}
